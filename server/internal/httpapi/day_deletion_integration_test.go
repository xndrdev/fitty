package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
)

func (h *trackingHarness) deletion(user, date string) dayDeletionState {
	h.t.Helper()
	var state dayDeletionState
	h.request(user, "GET", "/v1/days/"+date+"/deletion", nil, 200, &state)
	return state
}

func deletionRequest(h *trackingHarness, date string) dayDeletionRequest {
	h.t.Helper()
	day := h.deletion(h.a, date).Day
	if day == nil {
		h.t.Fatal("missing deletion preview")
	}
	return dayDeletionRequest{RequestID: trackingUUID(h.t), DayID: day.ID, Version: day.Version}
}

func testDayDeletion(h *trackingHarness) {
	const date = "2032-01-01"
	path := "/v1/days/" + date
	h.request(h.a, "GET", "/v1/days/invalid/deletion", nil, 400, nil)
	if h.deletion(h.a, date).Day != nil {
		h.t.Fatal("empty date has a persisted day")
	}
	meal := h.send(h.a, date, "Eine Testmahlzeit gegessen.")
	beforeAI := deletionRequest(h, date)
	h.process(meal, trackingResult(meal, "create", nil, trackingPtr(trackingFood(500))))
	h.request(h.a, "DELETE", path, beforeAI, 409, nil)
	entry := h.summary(h.a, date).Entries[0]
	beforeEdit := deletionRequest(h, date)
	h.request(h.a, "PUT", path+"/entries/"+entry.ID, entryChange{RequestID: trackingUUID(h.t), Version: entry.Version, Entry: &entry.Values}, 200, nil)
	h.request(h.a, "DELETE", path, beforeEdit, 409, nil)
	beforeMessage := deletionRequest(h, date)
	processing := h.send(h.a, date, "Bitte den Tag zusammenfassen.")
	job := h.claim(processing)
	input, err := h.app.analysisInput(h.ctx, job)
	if err != nil {
		h.t.Fatal(err)
	}
	queued := h.send(h.a, date, "Noch eine Rückfrage.")
	h.request(h.a, "DELETE", path, beforeMessage, 409, nil)
	other := h.send(h.b, date, "Anderer Nutzer.")
	otherDay := h.send(h.a, "2032-01-02", "Anderer Tag.")
	preview := h.deletion(h.a, date)
	if preview.Day.MessageCount != 4 || preview.Day.EntryCount != 1 || preview.Day.PhotoCount != 0 {
		h.t.Fatalf("unexpected preview: %+v", preview)
	}
	deletion := deletionRequest(h, date)
	h.request("invalid", "DELETE", path, deletion, 401, nil)
	h.request(h.b, "DELETE", path, deletion, 409, nil)
	h.request(h.a, "DELETE", "/v1/days/2032-12-31", deletion, 404, nil)
	invalid := deletion
	invalid.Version = 0
	h.request(h.a, "DELETE", path, invalid, 400, nil)
	invalid = deletion
	invalid.DayID = "0"
	h.request(h.a, "DELETE", path, invalid, 400, nil)
	invalid = deletion
	invalid.RequestID = "invalid"
	h.request(h.a, "DELETE", path, invalid, 400, nil)
	h.request(h.a, "DELETE", path, deletion, 200, nil)
	h.request(h.a, "DELETE", path, deletion, 200, nil)
	h.counts(meal, 0, 0, 0)
	for _, table := range []string{"days", "messages", "analysis_jobs", "tracking_entries", "manual_entry_changes"} {
		column := "day_id"
		if table == "days" {
			column = "id"
		}
		var n int
		err := h.app.DB.QueryRow(h.ctx, `select count(*) from fitty.`+table+` where user_id=$1 and `+column+`=$2::bigint`, h.a, deletion.DayID).Scan(&n)
		if err != nil || n != 0 {
			h.t.Fatalf("deleted day retained %s: %d, %v", table, n, err)
		}
	}
	if err := h.app.commitAnalysis(h.ctx, job, input, trackingAdvice()); !errors.Is(err, pgx.ErrNoRows) {
		h.t.Fatalf("late AI commit must be discarded: %v", err)
	}
	h.request(h.a, "POST", "/v1/messages/"+queued.ID+"/analysis/retry", nil, 409, nil)
	var page struct {
		DayID    *string   `json:"day_id"`
		Messages []Message `json:"messages"`
	}
	h.request(h.a, "GET", path+"/messages", nil, 200, &page)
	if page.DayID != nil || len(page.Messages) != 0 || h.deletion(h.a, date).Day != nil || h.summary(h.a, date).Totals.Calories != 0 {
		h.t.Fatal("deleted day remains visible")
	}
	h.request(h.a, "POST", path+"/messages", map[string]any{"client_id": meal.ClientID, "content": meal.Content}, 409, nil)
	if h.deletion(h.a, date).Day != nil {
		h.t.Fatal("late message retry created a ghost day")
	}
	newMessage := h.send(h.a, date, "Ein neuer Tageschat.")
	newDay := h.deletion(h.a, date).Day.ID
	if newDay == deletion.DayID {
		h.t.Fatal("recreated date reused day ID")
	}
	h.request(h.a, "DELETE", path, deletion, 200, nil)
	invalid = deletion
	invalid.RequestID = trackingUUID(h.t)
	h.request(h.a, "DELETE", path, invalid, 409, nil)
	invalid = deletion
	invalid.Version++
	h.request(h.a, "DELETE", path, invalid, 409, nil)
	h.request(h.a, "GET", path+"/messages", nil, 200, &page)
	if page.DayID == nil || *page.DayID != newDay || len(page.Messages) != 1 || page.Messages[0].ID != newMessage.ID {
		h.t.Fatal("old deletion retry removed a new chat")
	}
	h.status(other, "queued", 0)
	h.status(otherDay, "queued", 0)
}

func testDayDeletionConcurrency(h *trackingHarness) {
	const date = "2032-02-01"
	h.send(h.a, date, "Parallele Löschbestätigung.")
	input := deletionRequest(h, date)
	body, _ := json.Marshal(input)
	var wait sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		wait.Go(func() {
			r := httptest.NewRequest("DELETE", "/v1/days/"+date, strings.NewReader(string(body)))
			r.Header.Set("Authorization", "Bearer "+h.a)
			w := httptest.NewRecorder()
			h.router.ServeHTTP(w, r)
			results <- w.Code
		})
	}
	wait.Wait()
	close(results)
	for status := range results {
		if status != 200 {
			h.t.Fatalf("parallel retry returned %d", status)
		}
	}
	var n int
	err := h.app.DB.QueryRow(h.ctx, `select count(*) from fitty.day_deletions where user_id=$1 and request_id=$2`, h.a, input.RequestID).Scan(&n)
	if err != nil || n != 1 {
		h.t.Fatal("parallel retry duplicated receipt")
	}
}

func testDayDeletionPhotos(h *trackingHarness) {
	const date = "2032-03-01"
	s := &photoStore{files: map[string][]byte{}}
	h.app.Storage = s
	h.t.Cleanup(func() {
		_, err := h.app.DB.Exec(context.Background(), `delete from fitty.attachments where user_id in ($1::uuid,$2::uuid)`, h.a, h.b)
		if err != nil {
			h.t.Error(err)
		}
	})
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		h.t.Fatal(err)
	}
	upload := func(user, day, id string, want int) {
		h.t.Helper()
		r := httptest.NewRequest("PUT", "/v1/days/"+day+"/attachments/"+id, bytes.NewReader(raw.Bytes()))
		r.Header.Set("Authorization", "Bearer "+user)
		r.Header.Set("Content-Type", "image/png")
		w := httptest.NewRecorder()
		h.router.ServeHTTP(w, r)
		if w.Code != want {
			h.t.Fatalf("upload status %d want %d: %s", w.Code, want, w.Body.String())
		}
	}
	bound, draft, interrupted := trackingUUID(h.t), trackingUUID(h.t), trackingUUID(h.t)
	upload(h.a, date, bound, 200)
	var saved struct {
		Message Message `json:"message"`
	}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", map[string]any{"client_id": trackingUUID(h.t), "content": "Foto.", "attachment_ids": []string{bound}, "analyze": false}, 201, &saved)
	stale := deletionRequest(h, date)
	upload(h.a, date, draft, 200)
	h.request(h.a, "DELETE", "/v1/days/"+date, stale, 409, nil)
	s.unavailable = true
	upload(h.a, date, interrupted, 503)
	s.unavailable = false
	foreign, otherDay := trackingUUID(h.t), trackingUUID(h.t)
	upload(h.b, date, foreign, 200)
	upload(h.a, "2032-03-02", otherDay, 200)
	preview := h.deletion(h.a, date)
	if preview.Day.PhotoCount != 3 {
		h.t.Fatal("preview must include bound, draft and interrupted uploads")
	}
	input := deletionRequest(h, date)
	s.unavailable = true
	var response struct {
		Pending int `json:"pending_photo_deletions"`
	}
	h.request(h.a, "DELETE", "/v1/days/"+date, input, 200, &response)
	if response.Pending != 3 || h.deletion(h.a, date).PendingPhotoDeletions != 3 {
		h.t.Fatal("durable photo cleanup not recorded")
	}
	h.request(h.a, "GET", "/v1/attachments/"+bound+"/url", nil, 404, nil)
	upload(h.a, date, bound, 409)
	upload(h.a, date, interrupted, 409)
	// Never let a test worker consume another account's deletion requests.
	var foreignPending int
	err := h.app.DB.QueryRow(h.ctx, `select count(*) from fitty.attachments where delete_requested_at is not null and (user_id is null or user_id not in ($1::uuid,$2::uuid))`, h.a, h.b).Scan(&foreignPending)
	if err != nil || foreignPending != 0 {
		h.t.Fatal("photo cleanup test needs an isolated deletion queue")
	}
	if err := h.app.cleanupAttachmentBatch(h.ctx, false); err == nil {
		h.t.Fatal("Storage outage should keep the cleanup request")
	}
	if h.deletion(h.a, date).PendingPhotoDeletions != 3 {
		h.t.Fatal("failed cleanup discarded metadata")
	}
	s.unavailable = false
	fresh := trackingUUID(h.t)
	upload(h.a, date, fresh, 200)
	// A new app instance proves cleanup state survives a worker restart.
	restarted := &App{DB: h.app.DB, Storage: s}
	if err := restarted.cleanupAttachmentBatch(h.ctx, false); err != nil {
		h.t.Fatal(err)
	}
	h.request(h.a, "DELETE", "/v1/days/"+date, input, 200, &response)
	if response.Pending != 0 || h.deletion(h.a, date).PendingPhotoDeletions != 0 {
		h.t.Fatal("cleanup still pending")
	}
	if len(s.files) != 3 || s.files[h.a+"/"+fresh+".jpg"] == nil || s.files[h.b+"/"+foreign+".jpg"] == nil || s.files[h.a+"/"+otherDay+".jpg"] == nil {
		h.t.Fatal("cleanup removed an unrelated/new photo or retained deleted files")
	}
	upload(h.a, date, bound, 409)
	upload(h.a, date, draft, 409)
	upload(h.a, date, interrupted, 409)
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", map[string]any{"client_id": trackingUUID(h.t), "content": "Altes Foto.", "attachment_ids": []string{bound}}, 409, nil)
	h.request(h.a, "GET", "/v1/attachments/"+fresh+"/url", nil, 200, nil)
}
