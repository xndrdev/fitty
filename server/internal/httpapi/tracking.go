package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
)

type Totals struct {
	Calories                float64 `json:"calories"`
	ProteinG                float64 `json:"protein_g"`
	CarbsG                  float64 `json:"carbs_g"`
	FatG                    float64 `json:"fat_g"`
	ActivityCalories        float64 `json:"activity_calories"`
	DurationMinutes         float64 `json:"duration_minutes"`
	DistanceKM              float64 `json:"distance_km"`
	EstimatedEntries        int     `json:"estimated_entries"`
	UnknownActivityCalories int     `json:"unknown_activity_calories"`
}

type AnalysisState struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	Attempts  int    `json:"attempts"`
}

const entryColumns = `id::text,version,kind,label,amount,calories::float8,protein_g::float8,carbs_g::float8,fat_g::float8,duration_minutes::float8,distance_km::float8,source,notes`

func scanEntry(row interface{ Scan(...any) error }) (intelligence.Entry, error) {
	var e intelligence.Entry
	err := row.Scan(&e.ID, &e.Version, &e.Kind, &e.Label, &e.Amount, &e.Calories, &e.ProteinG, &e.CarbsG, &e.FatG, &e.DurationMinutes, &e.DistanceKM, &e.Source, &e.Notes)
	return e, err
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadEntries(ctx context.Context, db queryer, user string, day int64) ([]intelligence.Entry, error) {
	rows, err := db.Query(ctx, `select `+entryColumns+` from fitty.tracking_entries where user_id=$1 and day_id=$2 and deleted_at is null order by fitty.tracking_entries.id limit 201`, user, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []intelligence.Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if len(entries) > 200 {
		return nil, errors.New("daily entry limit reached")
	}
	return entries, rows.Err()
}

func (a *App) summary(w http.ResponseWriter, r *http.Request, user string) {
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
	if err != nil {
		serverError(w, err)
		return
	}
	var day int64
	if state.Day != nil {
		day, _ = strconv.ParseInt(state.Day.ID, 10, 64)
	}
	entries, err := loadEntries(r.Context(), tx, user, day)
	if err != nil {
		serverError(w, err)
		return
	}
	totals, err := loadTotals(r.Context(), tx, user, day)
	if err != nil {
		serverError(w, err)
		return
	}
	targets, err := loadTargetsForDate(r.Context(), tx, user, date)
	if err != nil {
		serverError(w, err)
		return
	}
	var pending int
	err = tx.QueryRow(r.Context(), `select count(*) from fitty.analysis_jobs where user_id=$1 and day_id=$2 and status in ('queued','processing')`, user, day).Scan(&pending)
	if err != nil {
		serverError(w, err)
		return
	}
	states := []AnalysisState{}
	rows, err := tx.Query(r.Context(), `select message_id::text,status,error_code,attempts from fitty.analysis_jobs where user_id=$1 and day_id=$2 order by fitty.analysis_jobs.message_id desc limit 1000`, user, day)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var state AnalysisState
		if err = rows.Scan(&state.MessageID, &state.Status, &state.Error, &state.Attempts); err != nil {
			rows.Close()
			serverError(w, err)
			return
		}
		states = append(states, state)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		serverError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"entries": entries, "totals": totals, "targets": targets.Targets, "targets_effective_from": targets.EffectiveFrom, "ai_enabled": a.AI != nil, "photos_enabled": a.Storage != nil, "pending_count": pending, "analysis": states, "day": state.Day, "pending_photo_deletions": state.PendingPhotoDeletions})
}

func (a *App) changeAnalysis(w http.ResponseWriter, r *http.Request, user string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		fail(w, 400, "Ungültige Nachricht.")
		return
	}
	retry := r.PathValue("action") == "retry"
	if !retry && r.PathValue("action") != "skip" {
		fail(w, 404, "Nicht gefunden.")
		return
	}
	if retry && a.AI == nil {
		fail(w, 503, "Die KI-Anbindung ist auf dem Server noch nicht eingerichtet.")
		return
	}
	if retry {
		_, err = a.DB.Exec(r.Context(), `insert into fitty.analysis_jobs(message_id,day_id,user_id,status)
		 select id,day_id,user_id,'disabled' from fitty.messages where id=$1 and user_id=$2 and role='user' on conflict(message_id) do nothing`, id, user)
		if err != nil {
			serverError(w, err)
			return
		}
	}
	status := "skipped"
	if retry {
		status = "queued"
	}
	result, err := a.DB.Exec(r.Context(), `update fitty.analysis_jobs set status=$3,error_code='',lease=null
 where message_id=$1 and user_id=$2 and status in ('failed','disabled') and (not $4::boolean or attempts<3)`, id, user, status, retry)
	if err != nil {
		serverError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 409, "Die Nachricht wird bereits verarbeitet, ist abgeschlossen oder hat das Wiederholungslimit erreicht.")
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}

type rowQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadTotals(ctx context.Context, db rowQueryer, user string, day int64) (Totals, error) {
	var totals Totals
	err := db.QueryRow(ctx, `select
 coalesce(sum(calories) filter(where kind='food'),0)::float8,
 coalesce(sum(protein_g),0)::float8,coalesce(sum(carbs_g),0)::float8,coalesce(sum(fat_g),0)::float8,
 coalesce(sum(calories) filter(where kind='activity'),0)::float8,
 coalesce(sum(duration_minutes),0)::float8,coalesce(sum(distance_km),0)::float8,
 count(*) filter(where source='estimate'),count(*) filter(where kind='activity' and calories is null)
 from fitty.tracking_entries where user_id=$1 and day_id=$2 and deleted_at is null`, user, day).Scan(&totals.Calories, &totals.ProteinG, &totals.CarbsG, &totals.FatG, &totals.ActivityCalories, &totals.DurationMinutes, &totals.DistanceKM, &totals.EstimatedEntries, &totals.UnknownActivityCalories)
	return totals, err
}
