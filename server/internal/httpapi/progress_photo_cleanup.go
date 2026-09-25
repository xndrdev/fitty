package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) removeProgressPhoto(w http.ResponseWriter, r *http.Request, user string) {
	id := strings.ToLower(r.PathValue("id"))
	if !uuidPattern.MatchString(id) {
		fail(w, 404, "Fortschrittsfoto nicht gefunden.")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `select pg_advisory_xact_lock(hashtextextended($1,0))`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	var retired bool
	err = tx.QueryRow(r.Context(), `select exists(select 1 from fitty.progress_photo_tombstones where user_id=$1 and id=$2)`, user, id).Scan(&retired)
	if err != nil {
		serverError(w, err)
		return
	}
	if !retired {
		tag, updateErr := tx.Exec(r.Context(), `update fitty.progress_photos set deleted_at=coalesce(deleted_at,now()) where id=$1 and user_id=$2`, id, user)
		if updateErr != nil {
			serverError(w, updateErr)
			return
		}
		if tag.RowsAffected() == 0 {
			fail(w, 404, "Fortschrittsfoto nicht gefunden.")
			return
		}
		_, err = tx.Exec(r.Context(), `insert into fitty.progress_photo_tombstones(user_id,id) values($1,$2) on conflict do nothing`, user, id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "removed"})
}

func (a *App) cleanupProgressPhotos(ctx context.Context, includeOrphans bool) error {
	for range 50 {
		if err := a.cleanupProgressPhoto(ctx, includeOrphans); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) cleanupProgressPhoto(ctx context.Context, includeOrphans bool) error {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, path string
	var user *string
	err = tx.QueryRow(ctx, `select id::text from fitty.progress_photos
 where deleted_at is not null or ($1 and (user_id is null or (state='pending' and created_at<now()-interval '24 hours')))
 order by created_at limit 1`, includeOrphans).Scan(&id)
	if err != nil {
		return err
	}
	// Use the same lock order as reservation/deletion: identity before row.
	// Otherwise a waiting first insert could recreate an expired reservation.
	var locked bool
	err = tx.QueryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1,0))`, id).Scan(&locked)
	if err != nil {
		return err
	}
	if !locked {
		return pgx.ErrNoRows
	}
	err = tx.QueryRow(ctx, `select user_id::text,object_path from fitty.progress_photos where id=$1
 and (deleted_at is not null or ($2 and (user_id is null or (state='pending' and created_at<now()-interval '24 hours'))))
 for update skip locked`, id, includeOrphans).Scan(&user, &path)
	if err != nil {
		return err
	}
	if err = a.Storage.Delete(ctx, path); err != nil {
		return err
	}
	if user != nil {
		_, err = tx.Exec(ctx, `insert into fitty.progress_photo_tombstones(user_id,id) values($1,$2) on conflict do nothing`, *user, id)
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `delete from fitty.progress_photos where id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
