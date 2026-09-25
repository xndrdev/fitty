package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
)

type targetSnapshot struct {
	EffectiveFrom *string                   `json:"effective_from"`
	Targets       intelligence.DailyTargets `json:"targets"`
}

type targetSettings struct {
	Version int64  `json:"version"`
	Today   string `json:"today"`
	targetSnapshot
}

type targetChange struct {
	RequestID     string          `json:"request_id"`
	Version       int64           `json:"version"`
	EffectiveFrom string          `json:"effective_from"`
	Targets       json.RawMessage `json:"targets"`
}

func parseTargets(raw json.RawMessage) (intelligence.DailyTargets, bool) {
	var fields map[string]json.RawMessage
	var targets intelligence.DailyTargets
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 4 {
		return targets, false
	}
	for _, name := range []string{"calories", "protein_g", "carbs_g", "fat_g"} {
		if _, ok := fields[name]; !ok {
			return targets, false
		}
	}
	if json.Unmarshal(raw, &targets) != nil || !intelligence.ValidDailyTargets(targets) {
		return targets, false
	}
	return targets, true
}

func targetsToday(zone string, now time.Time) (string, error) {
	location, err := time.LoadLocation(zone)
	if err != nil {
		return "", err
	}
	return now.In(location).Format(time.DateOnly), nil
}

func loadTargetsForDate(ctx context.Context, db rowQueryer, user, date string) (targetSnapshot, error) {
	var snapshot targetSnapshot
	err := db.QueryRow(ctx, `select effective_from::text,calories::float8,protein_g::float8,carbs_g::float8,fat_g::float8
 from fitty.daily_targets where user_id=$1 and effective_from<=$2::date order by effective_from desc limit 1`, user, date).Scan(&snapshot.EffectiveFrom, &snapshot.Targets.Calories, &snapshot.Targets.ProteinG, &snapshot.Targets.CarbsG, &snapshot.Targets.FatG)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return snapshot, err
}

func loadTargetSettings(ctx context.Context, db rowQueryer, user string, now time.Time) (targetSettings, error) {
	var settings targetSettings
	zone := "Europe/Berlin"
	err := db.QueryRow(ctx, `select targets_version,time_zone from fitty.profiles where user_id=$1`, user).Scan(&settings.Version, &zone)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return settings, err
	}
	settings.Today, err = targetsToday(zone, now)
	if err == nil {
		settings.targetSnapshot, err = loadTargetsForDate(ctx, db, user, settings.Today)
	}
	return settings, err
}

func (a *App) getTargets(w http.ResponseWriter, r *http.Request, user string) {
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	settings, err := loadTargetSettings(r.Context(), tx, user, time.Now())
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, settings)
}

func (a *App) putTargets(w http.ResponseWriter, r *http.Request, user string) {
	var input targetChange
	if !decode(w, r, &input) {
		return
	}
	targets, valid := parseTargets(input.Targets)
	if !valid || input.Version < 0 || !uuidPattern.MatchString(input.RequestID) || !validDate(input.EffectiveFrom) {
		fail(w, 400, "Bitte positive Ziele mit höchstens zwei Nachkommastellen eingeben oder die Felder leer lassen. Höchstens 20.000 kcal und 2.000 g je Makronährstoff.")
		return
	}
	input.RequestID = strings.ToLower(input.RequestID)
	encoded, _ := json.Marshal(targets)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// The profile is also the per-user serialization point for first-time
	// settings, timezone changes and duplicate target requests.
	_, err = tx.Exec(ctx, `insert into fitty.profiles(user_id) values($1) on conflict(user_id) do nothing`, user)
	if err != nil {
		serverError(w, err)
		return
	}
	var version int64
	err = tx.QueryRow(ctx, `select targets_version from fitty.profiles where user_id=$1 for update`, user).Scan(&version)
	if err != nil {
		serverError(w, err)
		return
	}
	var same bool
	err = tx.QueryRow(ctx, `select expected_version=$3 and effective_from=$4::date and targets=$5::jsonb from fitty.daily_target_changes where user_id=$1 and request_id=$2`, user, input.RequestID, input.Version, input.EffectiveFrom, encoded).Scan(&same)
	if err == nil && !same {
		fail(w, 409, "Diese Anfrage gehört bereits zu anderen Zielen. Bitte den aktuellen Stand laden.")
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	settings, loadErr := loadTargetSettings(ctx, tx, user, time.Now())
	if loadErr != nil {
		serverError(w, loadErr)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if version != input.Version || input.EffectiveFrom != settings.Today {
			fail(w, 409, "Deine Ziele oder der heutige Tag haben sich inzwischen geändert. Bitte den aktuellen Stand laden und die Angaben erneut prüfen.")
			return
		}
		if err = saveTargets(ctx, tx, user, input, targets, encoded); err != nil {
			serverError(w, err)
			return
		}
		settings.Version++
		settings.targetSnapshot = targetSnapshot{EffectiveFrom: &input.EffectiveFrom, Targets: targets}
	}
	if err = tx.Commit(ctx); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, settings)
}

func saveTargets(ctx context.Context, tx pgx.Tx, user string, input targetChange, targets intelligence.DailyTargets, encoded []byte) error {
	_, err := tx.Exec(ctx, `insert into fitty.daily_targets(user_id,effective_from,calories,protein_g,carbs_g,fat_g) values($1,$2::date,$3,$4,$5,$6)
 on conflict(user_id,effective_from) do update set calories=excluded.calories,protein_g=excluded.protein_g,carbs_g=excluded.carbs_g,fat_g=excluded.fat_g`, user, input.EffectiveFrom, targets.Calories, targets.ProteinG, targets.CarbsG, targets.FatG)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into fitty.daily_target_changes(user_id,request_id,expected_version,effective_from,targets) values($1,$2,$3,$4::date,$5::jsonb)`, user, input.RequestID, input.Version, input.EffectiveFrom, encoded)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update fitty.profiles set targets_version=targets_version+1,updated_at=now() where user_id=$1`, user)
	return err
}
