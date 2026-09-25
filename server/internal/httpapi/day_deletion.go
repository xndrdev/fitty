package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type DaySnapshot struct {
	ID           string `json:"id"`
	Version      int64  `json:"version"`
	MessageCount int    `json:"message_count"`
	EntryCount   int    `json:"entry_count"`
	PhotoCount   int    `json:"photo_count"`
}

type dayDeletionState struct {
	Day                   *DaySnapshot `json:"day"`
	PendingPhotoDeletions int          `json:"pending_photo_deletions"`
}

type dayDeletionRequest struct {
	RequestID string `json:"request_id"`
	DayID     string `json:"day_id"`
	Version   int64  `json:"version"`
}

func loadDayDeletionState(ctx context.Context, db rowQueryer, user, date string) (dayDeletionState, error) {
	var state dayDeletionState
	var day DaySnapshot
	err := db.QueryRow(ctx, `select d.id::text,d.version,
 (select count(*) from fitty.messages where user_id=$1 and day_id=d.id),
 (select count(*) from fitty.tracking_entries where user_id=$1 and day_id=d.id and deleted_at is null),
 (select count(*) from fitty.attachments where user_id=$1 and local_date=$2::date and delete_requested_at is null)
 from fitty.days d where d.user_id=$1 and d.local_date=$2::date`, user, date).Scan(&day.ID, &day.Version, &day.MessageCount, &day.EntryCount, &day.PhotoCount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return state, err
	}
	if err == nil {
		state.Day = &day
	}
	err = db.QueryRow(ctx, `select count(*) from fitty.attachments where user_id=$1 and local_date=$2::date and delete_requested_at is not null`, user, date).Scan(&state.PendingPhotoDeletions)
	return state, err
}

func (a *App) dayDeletionPreview(w http.ResponseWriter, r *http.Request, user string) {
	date := r.PathValue("date")
	if !validDate(date) {
		fail(w, 400, "Ungültiges Datum.")
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	state, err := loadDayDeletionState(r.Context(), tx, user, date)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func (a *App) deleteDay(w http.ResponseWriter, r *http.Request, user string) {
	date := r.PathValue("date")
	if !validDate(date) {
		fail(w, 400, "Ungültiges Datum.")
		return
	}
	var input dayDeletionRequest
	if !decode(w, r, &input) {
		return
	}
	dayID, err := strconv.ParseInt(input.DayID, 10, 64)
	if err != nil || dayID < 1 || input.Version < 1 || !uuidPattern.MatchString(input.RequestID) {
		fail(w, 400, "Bitte die aktuelle Löschvorschau erneut öffnen.")
		return
	}
	input.RequestID = strings.ToLower(input.RequestID)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Same order as direct edits/claims and attachment reservations. Analysis
	// commits lock the day before the job so cascading deletes cannot deadlock.
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, analysisQueueLock); err != nil {
		serverError(w, err)
		return
	}
	var same bool
	err = tx.QueryRow(ctx, `select day_id=$3 and local_date=$4::date and expected_version=$5 from fitty.day_deletions where user_id=$1 and request_id=$2`, user, input.RequestID, dayID, date, input.Version).Scan(&same)
	if err == nil && !same {
		fail(w, 409, "Diese Löschanfrage gehört zu einem anderen Stand. Bitte die Vorschau neu laden.")
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var currentID, version int64
		err = tx.QueryRow(ctx, `select id,version from fitty.days where user_id=$1 and local_date=$2::date for update`, user, date).Scan(&currentID, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Dieser Tageschat wurde bereits gelöscht.")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		if currentID != dayID || version != input.Version {
			fail(w, 409, "Dieser Tageschat wurde inzwischen geändert. Bitte die aktuelle Vorschau prüfen und erneut bestätigen.")
			return
		}
		if err = deleteDayData(ctx, tx, user, date, dayID, input); err != nil {
			serverError(w, err)
			return
		}
	}
	var pending int
	err = tx.QueryRow(ctx, `select count(*) from fitty.attachments where user_id=$1 and deletion_request_id=$2 and delete_requested_at is not null`, user, input.RequestID).Scan(&pending)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "deleted", "pending_photo_deletions": pending})
}

func deleteDayData(ctx context.Context, tx pgx.Tx, user, date string, day int64, input dayDeletionRequest) error {
	_, err := tx.Exec(ctx, `insert into fitty.day_deletions(user_id,request_id,day_id,local_date,expected_version) values($1,$2,$3,$4::date,$5)`, user, input.RequestID, day, date, input.Version)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into fitty.deleted_identifiers(user_id,kind,id) select user_id,'message',client_id from fitty.messages where user_id=$1 and day_id=$2 on conflict do nothing`, user, day)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `with marked as (
 update fitty.attachments set delete_requested_at=now(),deletion_request_id=$3
 where user_id=$1 and local_date=$2::date and delete_requested_at is null returning user_id,id
) insert into fitty.deleted_identifiers(user_id,kind,id) select user_id,'attachment',id from marked on conflict do nothing`, user, date, input.RequestID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `delete from fitty.days where id=$1 and user_id=$2`, day, user)
	return err
}
