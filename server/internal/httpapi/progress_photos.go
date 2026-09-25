package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"fitty/server/internal/storage"
	"github.com/jackc/pgx/v5"
)

type ProgressPhoto struct {
	Attachment
	Date string `json:"date"`
	View string `json:"view"`
}

func validProgressView(view string) bool {
	return view == "front" || view == "side" || view == "back" || view == "other"
}

// The cursor includes the immutable UUID so photos on the same date paginate.
func progressCursor(cursor string) (date, id string, valid bool) {
	date, id, found := strings.Cut(cursor, ":")
	return date, strings.ToLower(id), found && validDate(date) && uuidPattern.MatchString(id)
}

func (a *App) progressPhotos(w http.ResponseWriter, r *http.Request, user string) {
	view, before := r.URL.Query().Get("view"), r.URL.Query().Get("before")
	var beforeDate, beforeID *string
	if view != "" && !validProgressView(view) {
		fail(w, 400, "Bitte eine gültige Perspektive wählen.")
		return
	}
	if before != "" {
		date, id, valid := progressCursor(before)
		if !valid {
			fail(w, 400, "Ungültiger Fotoverlauf. Bitte neu laden.")
			return
		}
		beforeDate, beforeID = &date, &id
	}
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `select id::text,taken_on::text,view,mime_type,byte_size,width,height
 from fitty.progress_photos where user_id=$1 and state='ready' and deleted_at is null
 and ($2='' or view=$2) and ($3::date is null or (taken_on,id)<($3::date,$4::uuid))
 order by taken_on desc,id desc limit 25`, user, view, beforeDate, beforeID)
	if err != nil {
		serverError(w, err)
		return
	}
	photos := []ProgressPhoto{}
	for rows.Next() {
		var photo ProgressPhoto
		if err = rows.Scan(&photo.ID, &photo.Date, &photo.View, &photo.MIMEType, &photo.ByteSize, &photo.Width, &photo.Height); err != nil {
			break
		}
		photos = append(photos, photo)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	var pending int
	if err == nil {
		err = tx.QueryRow(r.Context(), `select count(*) from fitty.progress_photos where user_id=$1 and deleted_at is not null`, user).Scan(&pending)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	var next *string
	if len(photos) > 24 {
		photos = photos[:24]
		cursor := photos[23].Date + ":" + photos[23].ID
		next = &cursor
	}
	writeJSON(w, 200, map[string]any{"photos": photos, "next_before": next, "photos_enabled": a.Storage != nil, "pending_deletions": pending})
}

func (a *App) uploadProgressPhoto(w http.ResponseWriter, r *http.Request, user string) {
	if a.Storage == nil {
		fail(w, 503, "Fotos sind auf dem Server noch nicht eingerichtet.")
		return
	}
	id, date, view := strings.ToLower(r.PathValue("id")), r.URL.Query().Get("date"), r.URL.Query().Get("view")
	if !uuidPattern.MatchString(id) || !validDate(date) || !validProgressView(view) {
		fail(w, 400, "Bitte ein gültiges Aufnahmedatum und eine Perspektive angeben.")
		return
	}
	var zone string
	err := a.DB.QueryRow(r.Context(), `select coalesce((select time_zone from fitty.profiles where user_id=$1),'Europe/Berlin')`, user).Scan(&zone)
	today, dateErr := targetsToday(zone, time.Now())
	if err != nil || dateErr != nil {
		if err == nil {
			err = dateErr
		}
		serverError(w, err)
		return
	}
	if date > today {
		// A timezone change must not strand an upload whose reservation was
		// already valid when it started. New future-dated photos stay invalid.
		var existing bool
		err = a.DB.QueryRow(r.Context(), `select exists(select 1 from fitty.progress_photos where id=$1 and user_id=$2 and taken_on=$3::date)`, id, user, date).Scan(&existing)
		if err != nil {
			serverError(w, err)
			return
		}
		if !existing {
			fail(w, 400, "Das Aufnahmedatum darf nicht in der Zukunft liegen.")
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 413, "Das Bild ist zu groß. Bitte höchstens 8 MiB hochladen.")
		return
	}
	data, width, height, err := normalizeImage(raw, r.Header.Get("Content-Type"))
	if err != nil {
		fail(w, 400, "Bitte ein lesbares JPEG-, PNG- oder WebP-Bild mit höchstens 12 Megapixeln und 4.096 Pixeln je Seite verwenden.")
		return
	}
	digest := sha256.Sum256(data)
	hash, path := hex.EncodeToString(digest[:]), user+"/progress/"+id+".jpg"
	photo := ProgressPhoto{Attachment: Attachment{ID: id, MIMEType: "image/jpeg", ByteSize: len(data), Width: width, Height: height}, Date: date, View: view}
	retired, err := a.reserveProgressPhoto(r.Context(), user, photo, path, hash)
	if err != nil {
		serverError(w, err)
		return
	}
	if retired {
		fail(w, 409, "Dieses Fortschrittsfoto wurde entfernt. Bitte ein neues Foto auswählen.")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var storedHash, storedDate, storedView, state string
	err = tx.QueryRow(r.Context(), `select sha256,taken_on::text,view,state from fitty.progress_photos
 where id=$1 and user_id=$2 and deleted_at is null for update`, id, user).Scan(&storedHash, &storedDate, &storedView, &state)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (storedHash != hash || storedDate != date || storedView != view)) {
		fail(w, 409, "Diese Foto-ID gehört bereits zu anderen Angaben oder einem entfernten Foto. Bitte den Verlauf neu laden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if state != "ready" {
		if a.Storage.Put(r.Context(), path, data) != nil {
			fail(w, 503, "Das Fortschrittsfoto konnte nicht gespeichert werden. Bitte erneut versuchen.")
			return
		}
		_, err = tx.Exec(r.Context(), `update fitty.progress_photos set state='ready' where id=$1 and user_id=$2`, id, user)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"photo": photo})
}

func (a *App) reserveProgressPhoto(ctx context.Context, user string, photo ProgressPhoto, path, hash string) (bool, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// Serialize reservation and retirement even before a metadata row exists.
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, photo.ID); err != nil {
		return false, err
	}
	var retired bool
	err = tx.QueryRow(ctx, `select exists(select 1 from fitty.progress_photo_tombstones where user_id=$1 and id=$2)`, user, photo.ID).Scan(&retired)
	if err != nil || retired {
		return retired, err
	}
	_, err = tx.Exec(ctx, `insert into fitty.progress_photos(id,user_id,taken_on,view,object_path,sha256,byte_size,width,height)
 values($1,$2,$3::date,$4,$5,$6,$7,$8,$9) on conflict(id) do nothing`, photo.ID, user, photo.Date, photo.View, path, hash, photo.ByteSize, photo.Width, photo.Height)
	if err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

func (a *App) progressPhotoURL(w http.ResponseWriter, r *http.Request, user string) {
	if a.Storage == nil {
		fail(w, 503, "Fotos sind auf dem Server noch nicht eingerichtet.")
		return
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		fail(w, 404, "Fortschrittsfoto nicht gefunden.")
		return
	}
	var path string
	err := a.DB.QueryRow(r.Context(), `select object_path from fitty.progress_photos where id=$1 and user_id=$2 and state='ready' and deleted_at is null`, id, user).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Fortschrittsfoto nicht gefunden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	signed, err := a.Storage.Sign(r.Context(), path)
	if err != nil {
		fail(w, 503, "Das Foto ist gerade nicht erreichbar.")
		return
	}
	writeJSON(w, 200, map[string]any{"path": signed, "expires_in": 600})
}
