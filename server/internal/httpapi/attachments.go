package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fitty/server/internal/intelligence"
	"fitty/server/internal/storage"
	"github.com/jackc/pgx/v5"
	_ "golang.org/x/image/webp"
)

type Attachment struct {
	ID       string `json:"id"`
	MIMEType string `json:"mime_type"`
	ByteSize int    `json:"byte_size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Decode and re-encode bounded raster images to discard EXIF/location metadata
// and any non-image payload. Native orientation is applied by the client first.
func normalizeImage(data []byte, declared string) ([]byte, int, int, error) {
	mime := http.DetectContentType(data)
	if len(data) == 0 || len(data) > storage.MaxBytes || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") || declared != mime {
		return nil, 0, 0, errors.New("unsupported image")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > 12000000 {
		return nil, 0, 0, errors.New("invalid image dimensions")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, errors.New("invalid image")
	}
	canvas := image.NewRGBA(decoded.Bounds())
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), decoded, decoded.Bounds().Min, draw.Over)
	var out bytes.Buffer
	if jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 90}) != nil || out.Len() > storage.MaxBytes {
		return nil, 0, 0, errors.New("image too large")
	}
	return out.Bytes(), config.Width, config.Height, nil
}

func (a *App) uploadAttachment(w http.ResponseWriter, r *http.Request, user string) {
	if a.Storage == nil {
		fail(w, 503, "Fotos sind auf dem Server noch nicht eingerichtet.")
		return
	}
	id, date := strings.ToLower(r.PathValue("id")), r.PathValue("date")
	if !uuidPattern.MatchString(id) || !validDate(date) {
		fail(w, 400, "Ungültige Bild-ID oder ungültiger Tag.")
		return
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
	hash := hex.EncodeToString(digest[:])
	path := user + "/" + id + ".jpg"
	// Reserve durably before contacting Storage. An interrupted upload can be
	// retried with the same ID or removed by the cleanup worker.
	retired, err := a.reserveAttachment(r.Context(), id, user, date, path, hash, len(data), width, height)
	if err != nil {
		serverError(w, err)
		return
	}
	if retired {
		fail(w, 409, "Dieses Bild gehörte zu einem gelöschten Tageschat. Bitte ein neues Bild auswählen.")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var storedHash, state string
	err = tx.QueryRow(r.Context(), `select sha256,state from fitty.attachments where id=$1 and user_id=$2 and local_date=$3::date and delete_requested_at is null for update`, id, user, date).Scan(&storedHash, &state)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && storedHash != hash) {
		fail(w, 409, "Diese Bild-ID gehört bereits zu einem anderen Bild oder Tag.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if state != "ready" {
		if a.Storage.Put(r.Context(), path, data) != nil {
			fail(w, 503, "Das Bild konnte nicht gespeichert werden. Bitte erneut senden.")
			return
		}
		_, err = tx.Exec(r.Context(), `update fitty.attachments set state='ready' where id=$1 and user_id=$2`, id, user)
		if err != nil {
			serverError(w, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"attachment": Attachment{ID: id, MIMEType: "image/jpeg", ByteSize: len(data), Width: width, Height: height}})
}

// Keep the reservation durable while serializing new IDs with day deletion.
// Never hold the shared advisory lock during Storage network calls.
func (a *App) reserveAttachment(ctx context.Context, id, user, date, path, hash string, size, width, height int) (bool, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, analysisQueueLock); err != nil {
		return false, err
	}
	var retired bool
	err = tx.QueryRow(ctx, `select exists(select 1 from fitty.deleted_identifiers where user_id=$1 and kind='attachment' and id=$2)`, user, id).Scan(&retired)
	if err != nil || retired {
		return retired, err
	}
	tag, err := tx.Exec(ctx, `insert into fitty.attachments(id,user_id,local_date,object_path,sha256,byte_size,width,height)
 values($1,$2,$3::date,$4,$5,$6,$7,$8) on conflict(id) do nothing`, id, user, date, path, hash, size, width, height)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		_, err = tx.Exec(ctx, `update fitty.days set version=version+1 where user_id=$1 and local_date=$2::date`, user, date)
		if err != nil {
			return false, err
		}
	}
	return false, tx.Commit(ctx)
}

func (a *App) attachmentURL(w http.ResponseWriter, r *http.Request, user string) {
	if a.Storage == nil {
		fail(w, 503, "Fotos sind auf dem Server noch nicht eingerichtet.")
		return
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		fail(w, 404, "Bild nicht gefunden.")
		return
	}
	var path string
	err := a.DB.QueryRow(r.Context(), `select object_path from fitty.attachments where id=$1 and user_id=$2 and state='ready' and delete_requested_at is null`, id, user).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Bild nicht gefunden.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	signed, err := a.Storage.Sign(r.Context(), path)
	if err != nil {
		fail(w, 503, "Das Bild ist gerade nicht erreichbar.")
		return
	}
	writeJSON(w, 200, map[string]any{"path": signed, "expires_in": 600})
}

func (a *App) removeAttachment(w http.ResponseWriter, r *http.Request, user string) {
	if a.Storage == nil {
		fail(w, 503, "Fotos sind auf dem Server noch nicht eingerichtet.")
		return
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		fail(w, 404, "Bild nicht gefunden.")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var path string
	var attached bool
	err = tx.QueryRow(r.Context(), `select object_path,message_id is not null from fitty.attachments where id=$1 and user_id=$2 for update`, id, user).Scan(&path, &attached)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, map[string]string{"status": "removed"})
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if attached {
		fail(w, 409, "Das Bild gehört bereits zu einer gespeicherten Nachricht.")
		return
	}
	if a.Storage.Delete(r.Context(), path) != nil {
		fail(w, 503, "Das Bild konnte nicht entfernt werden. Bitte erneut versuchen.")
		return
	}
	if _, err = tx.Exec(r.Context(), `delete from fitty.attachments where id=$1 and user_id=$2`, id, user); err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "removed"})
}

func loadMessageAttachments(ctx context.Context, db queryer, user string, messages []Message) error {
	ids := make([]int64, len(messages))
	positions := make(map[string]int, len(messages))
	for i := range messages {
		ids[i], _ = strconv.ParseInt(messages[i].ID, 10, 64)
		positions[messages[i].ID] = i
		messages[i].Attachments = []Attachment{}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := db.Query(ctx, `select message_id::text,id::text,mime_type,byte_size,width,height from fitty.attachments
 where user_id=$1 and message_id=any($2::bigint[]) and state='ready' and delete_requested_at is null order by message_id,position`, user, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var messageID string
		var item Attachment
		if err = rows.Scan(&messageID, &item.ID, &item.MIMEType, &item.ByteSize, &item.Width, &item.Height); err != nil {
			return err
		}
		i := positions[messageID]
		messages[i].Attachments = append(messages[i].Attachments, item)
	}
	return rows.Err()
}

func (a *App) analysisImages(ctx context.Context, job analysisJob, input *intelligence.Input) error {
	// Only the current batch, or the latest image batch in the already bounded
	// history for a text follow-up. No photos from another day enter the context.
	ids := []int64{job.MessageID}
	for _, m := range input.History {
		if m.Role == "user" {
			id, _ := strconv.ParseInt(m.ID, 10, 64)
			ids = append(ids, id)
		}
	}
	rows, err := a.DB.Query(ctx, `select id::text,message_id::text,mime_type,object_path from fitty.attachments
 where user_id=$1 and message_id=(select max(message_id) from fitty.attachments where user_id=$1 and message_id=any($2::bigint[]) and state='ready' and delete_requested_at is null)
 and state='ready' and delete_requested_at is null order by position limit 4`, job.UserID, ids)
	if err != nil {
		return err
	}
	type file struct {
		image intelligence.Image
		path  string
	}
	files := []file{}
	for rows.Next() {
		var f file
		if err = rows.Scan(&f.image.ID, &f.image.MessageID, &f.image.MIMEType, &f.path); err != nil {
			rows.Close()
			return err
		}
		files = append(files, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(files) > 0 && a.Storage == nil {
		return intelligence.ErrConfiguration
	}
	for _, f := range files {
		f.image.Data, err = a.Storage.Get(ctx, f.path)
		if err != nil {
			return intelligence.ErrUnavailable
		}
		input.Images = append(input.Images, f.image)
	}
	return nil
}

func (a *App) RunAttachmentCleanup(ctx context.Context) {
	if a.Storage == nil {
		return
	}
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	nextOrphanScan := time.Time{}
	for {
		includeOrphans := !time.Now().Before(nextOrphanScan)
		if includeOrphans {
			nextOrphanScan = time.Now().Add(time.Hour)
		}
		if err := a.cleanupAttachmentBatch(ctx, includeOrphans); err != nil && ctx.Err() == nil {
			slog.Warn("attachment cleanup postponed")
		}
		if err := a.cleanupProgressPhotos(ctx, includeOrphans); err != nil && ctx.Err() == nil {
			slog.Warn("progress photo cleanup postponed")
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
func (a *App) cleanupAttachments(ctx context.Context) error {
	return a.cleanupAttachmentBatch(ctx, true)
}
func (a *App) cleanupAttachmentBatch(ctx context.Context, includeOrphans bool) error {
	for range 50 {
		if err := a.cleanupAttachmentItem(ctx, includeOrphans); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}
func (a *App) cleanupAttachment(ctx context.Context) error {
	return a.cleanupAttachmentItem(ctx, true)
}
func (a *App) cleanupAttachmentItem(ctx context.Context, includeOrphans bool) error {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, path string
	err = tx.QueryRow(ctx, `select id::text,object_path from fitty.attachments where delete_requested_at is not null or ($1 and (user_id is null or (message_id is null and created_at<now()-interval '24 hours')))
 order by delete_requested_at nulls last,created_at limit 1 for update skip locked`, includeOrphans).Scan(&id, &path)
	if err != nil {
		return err
	}
	if err = a.Storage.Delete(ctx, path); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `delete from fitty.attachments where id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
