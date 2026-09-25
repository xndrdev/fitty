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
	"reflect"

	"fitty/server/internal/intelligence"
)

type photoStore struct {
	files       map[string][]byte
	unavailable bool
	puts        int
}

func (s *photoStore) Put(_ context.Context, p string, data []byte) error {
	if s.unavailable {
		return errors.New("offline")
	}
	s.puts++
	s.files[p] = append([]byte{}, data...)
	return nil
}
func (s *photoStore) Get(_ context.Context, p string) ([]byte, error) {
	if s.unavailable || s.files[p] == nil {
		return nil, errors.New("offline")
	}
	return s.files[p], nil
}
func (s *photoStore) Delete(_ context.Context, p string) error {
	if s.unavailable {
		return errors.New("offline")
	}
	delete(s.files, p)
	return nil
}
func (s *photoStore) Sign(_ context.Context, p string) (string, error) {
	return "/storage/v1/object/sign/chat-attachments/" + p + "?token=test", nil
}

func testPhotoIntegration(h *trackingHarness) {
	s := &photoStore{files: map[string][]byte{}}
	h.app.Storage = s
	h.t.Cleanup(func() {
		_, err := h.app.DB.Exec(context.Background(), `delete from fitty.attachments where user_id in ($1::uuid,$2::uuid)`, h.a, h.b)
		if err != nil {
			h.t.Error("Could not clean test attachment metadata")
		}
	})
	date := "2026-08-20"
	canvas := image.NewRGBA(image.Rect(0, 0, 32, 24))
	canvas.Set(1, 1, color.White)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		h.t.Fatal(err)
	}
	data := buffer.Bytes()
	upload := func(user, day, id string, want int) Attachment {
		h.t.Helper()
		r := httptest.NewRequest("PUT", "/v1/days/"+day+"/attachments/"+id, bytes.NewReader(data))
		r.Header.Set("Content-Type", "image/png")
		r.Header.Set("Authorization", "Bearer "+user)
		w := httptest.NewRecorder()
		h.router.ServeHTTP(w, r)
		if w.Code != want {
			h.t.Fatalf("upload: status %d want %d: %s", w.Code, want, w.Body.String())
		}
		var result struct {
			Attachment Attachment `json:"attachment"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result.Attachment
	}
	id, second := trackingUUID(h.t), trackingUUID(h.t)
	s.unavailable = true
	upload(h.a, date, id, 503)
	var state string
	if err := h.app.DB.QueryRow(h.ctx, `select state from fitty.attachments where id=$1`, id).Scan(&state); err != nil || state != "pending" {
		h.t.Fatal("Interrupted upload must leave a durable pending record")
	}
	s.unavailable = false
	first := upload(h.a, date, id, 200)
	retry := upload(h.a, date, id, 200)
	if first != retry || s.puts != 1 || first.MIMEType != "image/jpeg" {
		h.t.Fatal("Upload retry changed the stored image")
	}
	upload(h.b, date, id, 409)
	upload(h.a, "2026-08-21", id, 409)
	h.request(h.b, "GET", "/v1/attachments/"+id+"/url", nil, 404, nil)
	h.request(h.a, "GET", "/v1/attachments/"+id+"/url", nil, 200, nil)
	upload(h.a, date, second, 200)
	clientID := trackingUUID(h.t)
	body := map[string]any{"client_id": clientID, "content": "", "attachment_ids": []string{id, second}}
	h.request(h.b, "POST", "/v1/days/"+date+"/messages", body, 409, nil)
	h.request(h.a, "POST", "/v1/days/2026-08-21/messages", body, 409, nil)
	var saved struct {
		Message Message `json:"message"`
	}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", body, 201, &saved)
	h.jobs = append(h.jobs, saved.Message.ID)
	if len(saved.Message.Attachments) != 2 || saved.Message.Attachments[0].ID != id {
		h.t.Fatal("Photo-only message lost its attachment order")
	}
	var repeated struct {
		Message Message `json:"message"`
	}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", body, 200, &repeated)
	if !reflect.DeepEqual(saved, repeated) {
		h.t.Fatal("Message retry changed the photo message")
	}
	body["attachment_ids"] = []string{second, id}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", body, 409, nil)
	body["client_id"] = trackingUUID(h.t)
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", body, 409, nil)
	h.request(h.a, "DELETE", "/v1/attachments/"+id, nil, 409, nil)
	var page struct {
		Messages []Message `json:"messages"`
	}
	h.request(h.a, "GET", "/v1/days/"+date+"/messages", nil, 200, &page)
	if len(page.Messages) != 1 || len(page.Messages[0].Attachments) != 2 {
		h.t.Fatal("Persisted photo message did not load")
	}
	job := h.claim(saved.Message)
	input, err := h.app.analysisInput(h.ctx, job)
	if err != nil || len(input.Images) != 2 || input.Images[0].ID != id || input.Images[0].MessageID != saved.Message.ID || !bytes.Equal(input.Images[0].Data, s.files[h.a+"/"+id+".jpg"]) {
		h.t.Fatalf("Current photo context unavailable: %v", err)
	}
	clarification := intelligence.Result{Reply: "Hast du das gegessen?", Intent: "clarification", Actions: []intelligence.Action{}}
	h.returnResult(clarification)
	h.app.processAnalysis(h.ctx, job)
	h.status(saved.Message, "completed", 1)
	follow := h.send(h.a, date, "Ja, das habe ich gegessen.")
	followJob := h.claim(follow)
	followInput, err := h.app.analysisInput(h.ctx, followJob)
	if err != nil || len(followInput.Images) != 2 || followInput.Images[0].MessageID != saved.Message.ID {
		h.t.Fatalf("Follow-up lost historical photos: %v", err)
	}
	food := trackingFood(420)
	h.returnResult(intelligence.Result{Reply: "Geschätzt 420 kcal erfasst.", Intent: "record", Actions: []intelligence.Action{{Operation: "create", Entry: &food, Evidence: follow.Content}}})
	h.app.processAnalysis(h.ctx, followJob)
	h.status(follow, "completed", 1)
	h.counts(follow, 1, 1, 1)
	// A failed image fetch must preserve the user's message without any booking.
	s.unavailable = true
	next := h.send(h.a, date, "Bitte prüfe das Foto noch einmal.")
	h.app.processAnalysis(h.ctx, h.claim(next))
	h.status(next, "failed", 1)
	h.counts(next, 0, 0, 0)
	s.unavailable = false
	unused := trackingUUID(h.t)
	upload(h.a, date, unused, 200)
	h.request(h.a, "DELETE", "/v1/attachments/"+unused, nil, 200, nil)
	h.request(h.a, "DELETE", "/v1/attachments/"+unused, nil, 200, nil)
	if s.files[h.a+"/"+unused+".jpg"] != nil {
		h.t.Fatal("Discarded photo remained in storage")
	}
	// Only test-owned records are made stale. Do not run global cleanup if any
	// other user's abandoned uploads would also be eligible.
	var foreign int
	err = h.app.DB.QueryRow(h.ctx, `select count(*) from fitty.attachments where user_id not in ($1::uuid,$2::uuid) or user_id is null`, h.a, h.b).Scan(&foreign)
	if err != nil {
		h.t.Fatal(err)
	}
	if foreign == 0 {
		stale := trackingUUID(h.t)
		upload(h.a, date, stale, 200)
		_, err = h.app.DB.Exec(h.ctx, `update fitty.attachments set created_at=now()-interval '25 hours' where id=$1 and user_id=$2`, stale, h.a)
		if err != nil {
			h.t.Fatal(err)
		}
		s.unavailable = true
		if h.app.cleanupAttachments(h.ctx) == nil {
			h.t.Fatal("Unavailable cleanup should retain its durable job")
		}
		s.unavailable = false
		if err = h.app.cleanupAttachments(h.ctx); err != nil {
			h.t.Fatal(err)
		}
		if s.files[h.a+"/"+stale+".jpg"] != nil {
			h.t.Fatal("Abandoned photo was not removed")
		}
		if s.files[h.a+"/"+id+".jpg"] == nil {
			h.t.Fatal("Cleanup removed an attached photo")
		}
	}
}
