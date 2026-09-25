package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type progressPage struct {
	Photos  []ProgressPhoto `json:"photos"`
	Next    *string         `json:"next_before"`
	Enabled bool            `json:"photos_enabled"`
	Pending int             `json:"pending_deletions"`
}

func progressImage() []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, 32, 48))
	canvas.Set(1, 1, color.White)
	var data bytes.Buffer
	_ = png.Encode(&data, canvas)
	return data.Bytes()
}

func (h *trackingHarness) progressUpload(user, id, date, view string, data []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("PUT", "/v1/progress-photos/"+id+"?date="+url.QueryEscape(date)+"&view="+url.QueryEscape(view), bytes.NewReader(data))
	r.Header.Set("Authorization", "Bearer "+user)
	r.Header.Set("Content-Type", "image/png")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	return w
}

func (h *trackingHarness) addProgressPhoto(user, id, date, view string, want int) ProgressPhoto {
	h.t.Helper()
	w := h.progressUpload(user, id, date, view, progressImage())
	if w.Code != want {
		h.t.Fatalf("progress upload got %d want %d: %s", w.Code, want, w.Body.String())
	}
	var value struct {
		Photo ProgressPhoto `json:"photo"`
	}
	if want == 200 && json.Unmarshal(w.Body.Bytes(), &value) != nil {
		h.t.Fatal("invalid photo response")
	}
	return value.Photo
}

func (h *trackingHarness) progressPage(user, query string) progressPage {
	h.t.Helper()
	var page progressPage
	h.request(user, "GET", "/v1/progress-photos"+query, nil, 200, &page)
	return page
}

func (h *trackingHarness) progressStore() *photoStore {
	s := &photoStore{files: map[string][]byte{}}
	h.app.Storage = s
	h.t.Cleanup(func() {
		_, err := h.app.DB.Exec(context.Background(), `delete from fitty.progress_photos where user_id in ($1::uuid,$2::uuid)`, h.a, h.b)
		if err != nil {
			h.t.Error("could not remove temporary progress metadata")
		}
	})
	return s
}

func testProgressPhotos(h *trackingHarness) {
	s := h.progressStore()
	const date = "2026-08-17"
	id := trackingUUID(h.t)
	path := "/v1/progress-photos/" + id
	h.request("invalid", "GET", "/v1/progress-photos", nil, 401, nil)
	initial := h.progressPage(h.a, "")
	if len(initial.Photos) != 0 || initial.Next != nil || !initial.Enabled {
		h.t.Fatal("unexpected empty list")
	}
	for _, q := range []string{"?view=unknown", "?before=2026-08-17", "?before=2026-02-30:" + id} {
		h.request(h.a, "GET", "/v1/progress-photos"+q, nil, 400, nil)
	}
	h.addProgressPhoto(h.a, "invalid", date, "front", 400)
	h.addProgressPhoto(h.a, id, "2026-02-30", "front", 400)
	h.addProgressPhoto(h.a, id, "9999-12-31", "front", 400)
	h.addProgressPhoto(h.a, id, date, "unknown", 400)
	if w := h.progressUpload(h.a, id, date, "front", []byte("not an image")); w.Code != 400 {
		h.t.Fatal("accepted invalid image")
	}
	s.unavailable = true
	h.addProgressPhoto(h.a, id, date, "front", 503)
	if len(h.progressPage(h.a, "").Photos) != 0 {
		h.t.Fatal("pending photo visible")
	}
	s.unavailable = false
	first := h.addProgressPhoto(h.a, id, date, "front", 200)
	retry := h.addProgressPhoto(h.a, strings.ToUpper(id), date, "front", 200)
	if first != retry || s.puts != 1 || first.Width != 32 || first.Height != 48 || first.MIMEType != "image/jpeg" {
		h.t.Fatal("retry changed photo")
	}
	if s.files[h.a+"/progress/"+id+".jpg"] == nil {
		h.t.Fatal("photo not in separate object namespace")
	}
	h.addProgressPhoto(h.a, id, "2026-08-18", "front", 409)
	h.addProgressPhoto(h.a, id, date, "side", 409)
	h.addProgressPhoto(h.b, id, date, "front", 409)
	different := image.NewRGBA(image.Rect(0, 0, 32, 48))
	different.Set(1, 1, color.Black)
	var changed bytes.Buffer
	_ = png.Encode(&changed, different)
	if h.progressUpload(h.a, id, date, "front", changed.Bytes()).Code != 409 {
		h.t.Fatal("changed photo accepted for saved identity")
	}
	h.request(h.b, "GET", path+"/url", nil, 404, nil)
	h.request(h.b, "DELETE", path, nil, 404, nil)
	h.request(h.a, "GET", path+"/url", nil, 200, nil)
	h.request(h.a, "GET", "/v1/attachments/"+id+"/url", nil, 404, nil)
	if len(h.progressPage(h.b, "").Photos) != 0 {
		h.t.Fatal("other user's progress leaked")
	}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", map[string]any{"client_id": trackingUUID(h.t), "content": "", "attachment_ids": []string{id}}, 409, nil)
	message := h.send(h.a, date, "Bitte fasse meinen Tag zusammen.")
	job := h.claim(message)
	input, err := h.app.analysisInput(h.ctx, job)
	if err != nil || len(input.Images) != 0 {
		h.t.Fatalf("progress photo reached AI context: %v", err)
	}
	h.app.processAnalysis(h.ctx, job)
	h.request(h.a, "DELETE", "/v1/days/"+date, deletionRequest(h, date), 200, nil)
	if len(h.progressPage(h.a, "").Photos) != 1 {
		h.t.Fatal("day deletion removed progress photo")
	}
	s.unavailable = true
	h.request(h.a, "DELETE", path, nil, 200, nil)
	deleted := h.progressPage(h.a, "")
	if len(deleted.Photos) != 0 || deleted.Pending != 1 {
		h.t.Fatal("deleted photo must disappear immediately")
	}
	h.request(h.a, "GET", path+"/url", nil, 404, nil)
	if err := h.app.cleanupProgressPhotos(h.ctx, false); err == nil {
		h.t.Fatal("storage failure did not preserve cleanup")
	}
	h.addProgressPhoto(h.a, id, date, "front", 409)
	s.unavailable = false
	if err := h.app.cleanupProgressPhotos(h.ctx, false); err != nil {
		h.t.Fatal(err)
	}
	if len(s.files) != 0 || h.progressPage(h.a, "").Pending != 0 {
		h.t.Fatal("private object not removed")
	}
	h.request(h.a, "DELETE", path, nil, 200, nil)
	h.addProgressPhoto(h.a, id, date, "front", 409)
	h.request(h.a, "DELETE", "/v1/progress-photos/"+trackingUUID(h.t), nil, 404, nil)
	// A saved/reserved request can be confirmed after moving the profile date
	// back across the date line; a brand-new future upload is still refused.
	var profile Profile
	h.request(h.a, "GET", "/v1/profile", nil, 200, &profile)
	original := profile
	profile.TimeZone = "Pacific/Kiritimati"
	h.request(h.a, "PUT", "/v1/profile", profile, 200, nil)
	eastToday, err := targetsToday(profile.TimeZone, time.Now())
	if err != nil {
		h.t.Fatal(err)
	}
	dateLineID := trackingUUID(h.t)
	h.addProgressPhoto(h.a, dateLineID, eastToday, "front", 200)
	profile.TimeZone = "Etc/GMT+12"
	h.request(h.a, "PUT", "/v1/profile", profile, 200, nil)
	h.addProgressPhoto(h.a, dateLineID, eastToday, "front", 200)
	h.addProgressPhoto(h.a, trackingUUID(h.t), eastToday, "front", 400)
	h.request(h.a, "PUT", "/v1/profile", original, 200, nil)
}

func testProgressPagination(h *trackingHarness) {
	s := h.progressStore()
	ids := map[string]bool{}
	for i := range 27 {
		view := "front"
		if i%2 == 0 {
			view = "side"
		}
		id := trackingUUID(h.t)
		ids[id] = true
		h.addProgressPhoto(h.a, id, "2026-08-18", view, 200)
	}
	first := h.progressPage(h.a, "")
	if len(first.Photos) != 24 || first.Next == nil {
		h.t.Fatal("first page bound")
	}
	second := h.progressPage(h.a, "?before="+url.QueryEscape(*first.Next))
	if len(second.Photos) != 3 || second.Next != nil {
		h.t.Fatal("second page bound")
	}
	for _, photo := range append(first.Photos, second.Photos...) {
		if !ids[photo.ID] {
			h.t.Fatal("duplicate or unrelated photo in pagination")
		}
		delete(ids, photo.ID)
	}
	if len(ids) != 0 {
		h.t.Fatal("missing same-day photos")
	}
	if len(h.progressPage(h.a, "?view=side").Photos) != 14 {
		h.t.Fatal("perspective filter mismatch")
	}
	// Pending uploads age out; ready photos are retained regardless of age.
	expired := trackingUUID(h.t)
	s.unavailable = true
	h.addProgressPhoto(h.a, expired, "2026-08-18", "other", 503)
	s.unavailable = false
	_, err := h.app.DB.Exec(h.ctx, `update fitty.progress_photos set created_at=now()-interval '25 hours' where user_id=$1`, h.a)
	if err != nil {
		h.t.Fatal(err)
	}
	if err = h.app.cleanupProgressPhotos(h.ctx, true); err != nil {
		h.t.Fatal(err)
	}
	if len(h.progressPage(h.a, "").Photos) != 24 || len(s.files) != 27 {
		h.t.Fatal("cleanup removed ready photos")
	}
	h.addProgressPhoto(h.a, expired, "2026-08-18", "other", 409)
	// Account removal leaves a cleanup record instead of an orphaned image.
	id := first.Photos[0].ID
	_, err = h.app.DB.Exec(h.ctx, `update fitty.progress_photos set user_id=null where id=$1`, id)
	if err != nil {
		h.t.Fatal(err)
	}
	if err = h.app.cleanupProgressPhotos(h.ctx, true); err != nil {
		h.t.Fatal(err)
	}
	if len(s.files) != 26 {
		h.t.Fatal("account orphan not removed")
	}
	h.app.Storage = nil
	if h.progressPage(h.a, "").Enabled {
		h.t.Fatal("unconfigured storage reported enabled")
	}
	h.addProgressPhoto(h.a, trackingUUID(h.t), "2026-08-18", "front", 503)
}

type blockingProgressStore struct {
	*photoStore
	started chan struct{}
	finish  chan struct{}
}

func (s *blockingProgressStore) Delete(ctx context.Context, path string) error {
	close(s.started)
	select {
	case <-s.finish:
		return s.photoStore.Delete(ctx, path)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func testProgressConcurrency(h *trackingHarness) {
	s := h.progressStore()
	id := trackingUUID(h.t)
	data := progressImage()
	var wait sync.WaitGroup
	responses := make(chan int, 2)
	for range 2 {
		wait.Go(func() { responses <- h.progressUpload(h.a, id, "2026-08-19", "front", data).Code })
	}
	wait.Wait()
	close(responses)
	for code := range responses {
		if code != 200 {
			h.t.Fatalf("parallel identical upload: %d", code)
		}
	}
	if s.puts != 1 || len(h.progressPage(h.a, "").Photos) != 1 {
		h.t.Fatal("parallel upload duplicated writes")
	}
	// Hold cleanup inside Storage while a stale upload tries to reserve the ID.
	_, err := h.app.DB.Exec(h.ctx, `update fitty.progress_photos set state='pending',created_at=now()-interval '25 hours' where id=$1`, id)
	if err != nil {
		h.t.Fatal(err)
	}
	blocking := &blockingProgressStore{photoStore: s, started: make(chan struct{}), finish: make(chan struct{})}
	h.app.Storage = blocking
	cleanup := make(chan error, 1)
	go func() { cleanup <- h.app.cleanupProgressPhoto(h.ctx, true) }()
	select {
	case <-blocking.started:
	case <-time.After(5 * time.Second):
		h.t.Fatal("cleanup did not start")
	}
	upload := make(chan int, 1)
	go func() { upload <- h.progressUpload(h.a, id, "2026-08-19", "front", data).Code }()
	close(blocking.finish)
	if err = <-cleanup; err != nil {
		h.t.Fatal(err)
	}
	if code := <-upload; code != 409 {
		h.t.Fatalf("late upload restored expired photo: %d", code)
	}
	if len(h.progressPage(h.a, "").Photos) != 0 || len(s.files) != 0 {
		h.t.Fatal("cleanup race restored a photo")
	}
	if err = h.app.cleanupProgressPhoto(h.ctx, true); !errors.Is(err, pgx.ErrNoRows) {
		h.t.Fatalf("unexpected leftover: %v", err)
	}
}
