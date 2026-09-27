package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
)

const maxFoodFavorites = 200

type foodFavorite struct {
	ID    string              `json:"id"`
	Entry intelligence.Values `json:"entry"`
}

type favoriteChange struct {
	EntryVersion int64 `json:"entry_version"`
}

func favoriteEntryID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0 && strconv.FormatInt(id, 10) == raw
}

func scanFavorite(row interface{ Scan(...any) error }) (foodFavorite, error) {
	var favorite foodFavorite
	var raw []byte
	if err := row.Scan(&favorite.ID, &raw); err != nil {
		return favorite, err
	}
	if err := json.Unmarshal(raw, &favorite.Entry); err != nil {
		return favorite, err
	}
	if favorite.Entry.Kind != "food" || !intelligence.ValidValues(favorite.Entry) {
		return favorite, errors.New("invalid stored food favorite")
	}
	return favorite, nil
}

func (a *App) getFavorites(w http.ResponseWriter, r *http.Request, user string) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	rows, err := a.DB.Query(ctx, `select entry_id::text,entry from fitty.food_favorites where user_id=$1 order by created_at desc,entry_id desc limit $2`, user, maxFoodFavorites)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	favorites := []foodFavorite{}
	for rows.Next() {
		favorite, err := scanFavorite(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		favorites = append(favorites, favorite)
	}
	if err = rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"favorites": favorites})
}

func (a *App) putFavorite(w http.ResponseWriter, r *http.Request, user string) {
	id, valid := favoriteEntryID(r.PathValue("id"))
	if !valid {
		fail(w, 400, "Ungültiger Eintrag.")
		return
	}
	var input favoriteChange
	if !decode(w, r, &input) {
		return
	}
	if input.EntryVersion < 1 {
		fail(w, 400, "Bitte eine gültige Version des Eintrags senden.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Serialize additions per account so simultaneous first-time saves and
	// competing additions at the limit cannot produce duplicates or exceed it.
	_, err = tx.Exec(ctx, `insert into fitty.profiles(user_id) values($1) on conflict(user_id) do nothing`, user)
	if err == nil {
		_, err = tx.Exec(ctx, `select user_id from fitty.profiles where user_id=$1 for update`, user)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	favorite, err := scanFavorite(tx.QueryRow(ctx, `select entry_id::text,entry from fitty.food_favorites where user_id=$1 and entry_id=$2`, user, id))
	if err == nil {
		// A retry keeps the original snapshot, even after its source is changed
		// or deleted. Only removing and saving again selects newer values.
		writeJSON(w, 200, map[string]any{"favorite": favorite})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	entry, err := scanEntry(tx.QueryRow(ctx, `select `+entryColumns+` from fitty.tracking_entries where id=$1 and user_id=$2 and deleted_at is null for share`, id, user))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Dieser Eintrag ist nicht mehr vorhanden. Bitte die Tagesübersicht neu laden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if entry.Kind != "food" || !intelligence.ValidValues(entry.Values) {
		fail(w, 400, "Nur Gerichte können als Favorit gespeichert werden.")
		return
	}
	if entry.Version != input.EntryVersion {
		fail(w, 409, "Dieser Eintrag wurde inzwischen geändert. Bitte die aktuellen Werte neu laden und erneut als Favorit speichern.")
		return
	}
	var count int
	err = tx.QueryRow(ctx, `select count(*) from fitty.food_favorites where user_id=$1`, user).Scan(&count)
	if err != nil {
		serverError(w, err)
		return
	}
	if count >= maxFoodFavorites {
		fail(w, 409, "Du kannst bis zu 200 Gerichte als Favoriten speichern. Bitte zuerst einen Favoriten entfernen.")
		return
	}
	raw, err := json.Marshal(entry.Values)
	if err == nil {
		_, err = tx.Exec(ctx, `insert into fitty.food_favorites(user_id,entry_id,entry) values($1,$2,$3::jsonb)`, user, id, raw)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"favorite": foodFavorite{ID: entry.ID, Entry: entry.Values}})
}

func (a *App) deleteFavorite(w http.ResponseWriter, r *http.Request, user string) {
	id, valid := favoriteEntryID(r.PathValue("id"))
	if !valid {
		fail(w, 400, "Ungültiger Favorit.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if _, err := a.DB.Exec(ctx, `delete from fitty.food_favorites where user_id=$1 and entry_id=$2`, user, id); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}
