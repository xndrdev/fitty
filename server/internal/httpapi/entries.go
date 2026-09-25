package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
)

type entryChange struct {
	RequestID string               `json:"request_id"`
	Version   int64                `json:"version"`
	Entry     *intelligence.Values `json:"entry"`
}

func validDirectEntry(e *intelligence.Values) bool {
	if e == nil {
		return false
	}
	e.Label, e.Amount, e.Notes = strings.TrimSpace(e.Label), strings.TrimSpace(e.Amount), strings.TrimSpace(e.Notes)
	if !intelligence.ValidValues(*e) || strings.ContainsRune(e.Label+e.Amount+e.Notes, 0) {
		return false
	}
	for _, n := range []*float64{e.Calories, e.ProteinG, e.CarbsG, e.FatG, e.DurationMinutes, e.DistanceKM} {
		if n != nil && math.Abs(*n*100-math.Round(*n*100)) > 1e-7 {
			return false
		}
	}
	*e = roundEntry(*e)
	return true
}

func (a *App) changeEntry(w http.ResponseWriter, r *http.Request, user string) {
	date := r.PathValue("date")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if !validDate(date) || err != nil || id < 1 {
		fail(w, 400, "Ungültiger Tag oder Eintrag.")
		return
	}
	var input entryChange
	if !decode(w, r, &input) {
		return
	}
	operation, status := "update", "updated"
	if r.Method == http.MethodDelete {
		operation, status = "delete", "deleted"
	}
	if !uuidPattern.MatchString(input.RequestID) || input.Version < 1 ||
		(operation == "update" && !validDirectEntry(input.Entry)) || (operation == "delete" && input.Entry != nil) {
		fail(w, 400, "Bitte gültige Angaben mit höchstens zwei Nachkommastellen und eine gültige Änderung senden.")
		return
	}
	input.RequestID = strings.ToLower(input.RequestID)
	var after []byte
	if input.Entry != nil {
		after, _ = json.Marshal(input.Entry)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Coordinate with worker claims, then with new messages and other direct
	// edits. A retry may already be committed even if another job is now queued.
	_, err = tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, analysisQueueLock)
	if err != nil {
		serverError(w, err)
		return
	}
	var day int64
	err = tx.QueryRow(ctx, `select id from fitty.days where user_id=$1 and local_date=$2::date for update`, user, date).Scan(&day)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Dieser Eintrag ist nicht mehr vorhanden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	var same bool
	err = tx.QueryRow(ctx, `select day_id=$3 and entry_id=$4 and operation=$5 and expected_version=$6 and after_value is not distinct from $7::jsonb
 from fitty.manual_entry_changes where user_id=$1 and request_id=$2`, user, input.RequestID, day, id, operation, input.Version, after).Scan(&same)
	if err == nil {
		if !same {
			fail(w, 409, "Diese Änderung wurde bereits mit einem anderen Inhalt übermittelt. Bitte den aktuellen Eintrag neu laden.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": status})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	var active bool
	err = tx.QueryRow(ctx, `select exists(select 1 from fitty.analysis_jobs where user_id=$1 and day_id=$2 and status in ('queued','processing'))`, user, day).Scan(&active)
	if err != nil {
		serverError(w, err)
		return
	}
	if active {
		fail(w, 409, "Fitty wertet diesen Tag noch aus. Bitte kurz warten und den aktuellen Eintrag neu laden.")
		return
	}
	previous, err := scanEntry(tx.QueryRow(ctx, `select `+entryColumns+` from fitty.tracking_entries where id=$1 and user_id=$2 and day_id=$3 and deleted_at is null for update`, id, user, day))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Dieser Eintrag ist nicht mehr vorhanden. Bitte die Tagesübersicht neu laden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if input.Version != previous.Version {
		fail(w, 409, "Dieser Eintrag wurde inzwischen geändert. Bitte die aktuellen Werte neu laden und deine Änderung prüfen.")
		return
	}
	if input.Entry != nil && input.Entry.Kind != previous.Kind {
		fail(w, 400, "Essen und Bewegung können nicht ineinander umgewandelt werden.")
		return
	}
	if err = applyDirectEntry(ctx, tx, user, day, id, input); err != nil {
		serverError(w, err)
		return
	}
	before, _ := json.Marshal(previous.Values)
	_, err = tx.Exec(ctx, `insert into fitty.manual_entry_changes(user_id,request_id,day_id,entry_id,expected_version,operation,before_value,after_value)
 values($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb)`, user, input.RequestID, day, id, input.Version, operation, before, after)
	if err == nil {
		_, err = tx.Exec(ctx, `update fitty.days set updated_at=now(),version=version+1 where id=$1 and user_id=$2`, day, user)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}

func applyDirectEntry(ctx context.Context, tx pgx.Tx, user string, day, id int64, input entryChange) error {
	if input.Entry == nil {
		_, err := tx.Exec(ctx, `update fitty.tracking_entries set deleted_at=now(),updated_at=now(),updated_by_message_id=null,version=version+1
 where id=$1 and user_id=$2 and day_id=$3 and deleted_at is null`, id, user, day)
		return err
	}
	e := input.Entry
	_, err := tx.Exec(ctx, `update fitty.tracking_entries set label=$4,amount=$5,calories=$6,protein_g=$7,carbs_g=$8,fat_g=$9,
 duration_minutes=$10,distance_km=$11,source=$12,notes=$13,updated_at=now(),updated_by_message_id=null,version=version+1
 where id=$1 and user_id=$2 and day_id=$3 and deleted_at is null`, id, user, day,
		e.Label, e.Amount, e.Calories, e.ProteinG, e.CarbsG, e.FatG, e.DurationMinutes, e.DistanceKM, e.Source, e.Notes)
	return err
}
