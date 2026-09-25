package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"fitty/server/internal/intelligence"
)

func testDirectEntries(h *trackingHarness) {
	const date = "2031-01-01"
	meal := h.send(h.a, date, "Eine Testmahlzeit gegessen.")
	h.process(meal, trackingResult(meal, "create", nil, trackingPtr(trackingFood(500))))
	entry := h.summary(h.a, date).Entries[0]
	if entry.Version != 1 {
		h.t.Fatal("new entry version must be 1")
	}
	path := "/v1/days/" + date + "/entries/" + entry.ID
	values := entry.Values
	values.Calories, values.Amount = trackingPtr(500.25), "2 Portionen"
	input := entryChange{RequestID: trackingUUID(h.t), Version: entry.Version, Entry: &values}
	h.request(h.b, "PUT", path, input, 404, nil)
	h.request(h.a, "PUT", "/v1/days/2031-01-02/entries/"+entry.ID, input, 404, nil)
	h.request("invalid-user", "PUT", path, input, 401, nil)
	invalid := input
	invalid.Version = 0
	h.request(h.a, "PUT", path, invalid, 400, nil)
	invalid = input
	invalid.Entry = trackingPtr(intelligence.Values{Kind: "activity", Label: "Walking Pad", DurationMinutes: trackingPtr(10.0), Source: "user"})
	h.request(h.a, "PUT", path, invalid, 400, nil)

	h.request(h.a, "PUT", path, input, 200, nil)
	h.request(h.a, "PUT", path, input, 200, nil)
	updated := h.summary(h.a, date)
	if updated.Totals.Calories != 500.25 || updated.Entries[0].Version != 2 || updated.Totals.EstimatedEntries != 1 {
		h.t.Fatal("direct change must persist exact total, version and estimation source")
	}
	h.counts(meal, 1, 1, 1) // No fabricated chat message or AI audit for a direct edit.
	var count int
	var before, after float64
	var manual bool
	err := h.app.DB.QueryRow(h.ctx, `select count(*),min((before_value->>'calories')::float8),min((after_value->>'calories')::float8) from fitty.manual_entry_changes where user_id=$1 and entry_id=$2::bigint`, h.a, entry.ID).Scan(&count, &before, &after)
	if err != nil || count != 1 || before != 500 || after != 500.25 {
		h.t.Fatalf("unexpected direct audit: %v", err)
	}
	err = h.app.DB.QueryRow(h.ctx, `select updated_by_message_id is null from fitty.tracking_entries where id=$1::bigint`, entry.ID).Scan(&manual)
	if err != nil || !manual {
		h.t.Fatal("direct edit must not be attributed to an old chat message")
	}
	stale := input
	stale.RequestID = trackingUUID(h.t)
	h.request(h.a, "PUT", path, stale, 409, nil)
	collisionValues := values
	collisionValues.Calories = trackingPtr(600.0)
	collision := input
	collision.Entry = &collisionValues
	h.request(h.a, "PUT", path, collision, 409, nil)
	h.request(h.a, "DELETE", path, entryChange{RequestID: input.RequestID, Version: 1}, 409, nil)

	pending := h.send(h.a, date, "Korrigiere auf 700 kcal.")
	stale.Version = 2
	h.request(h.a, "PUT", path, stale, 409, nil) // Queued analysis blocks manual edits.
	h.request(h.a, "PUT", path, input, 200, nil) // Already committed retry remains safe.
	job := h.claim(pending)
	h.request(h.a, "DELETE", path, entryChange{RequestID: trackingUUID(h.t), Version: 2}, 409, nil)
	h.returnResult(trackingResult(pending, "update", &entry.ID, trackingPtr(trackingFood(700))))
	h.app.processAnalysis(h.ctx, job)
	h.status(pending, "completed", 1)
	updated = h.summary(h.a, date)
	if updated.Entries[0].Version != 3 || updated.Totals.Calories != 700 {
		h.t.Fatal("AI correction must advance version")
	}
	h.request(h.a, "PUT", path, stale, 409, nil)
	deletion := entryChange{RequestID: trackingUUID(h.t), Version: 3}
	h.request(h.a, "DELETE", path, deletion, 200, nil)
	h.request(h.a, "DELETE", path, deletion, 200, nil)
	h.request(h.a, "PUT", path, input, 200, nil) // Old successful retry must never resurrect it.
	updated = h.summary(h.a, date)
	if len(updated.Entries) != 0 || updated.Totals.Calories != 0 {
		h.t.Fatal("deleted entry still counted")
	}
	h.counts(meal, 1, 1, 1)
	var deleted bool
	var version int
	err = h.app.DB.QueryRow(h.ctx, `select deleted_at is not null,version from fitty.tracking_entries where id=$1::bigint`, entry.ID).Scan(&deleted, &version)
	if err != nil || !deleted || version != 4 {
		h.t.Fatal("delete retry changed the tombstone")
	}
	err = h.app.DB.QueryRow(h.ctx, `select count(*) from fitty.manual_entry_changes where user_id=$1 and entry_id=$2::bigint`, h.a, entry.ID).Scan(&count)
	if err != nil || count != 2 {
		h.t.Fatal("repeated requests duplicated manual audit")
	}
	deletion.RequestID = trackingUUID(h.t)
	h.request(h.a, "DELETE", path, deletion, 404, nil)

	activityMessage := h.send(h.a, date, "Walking Pad 45 Minuten.")
	activity := intelligence.Values{Kind: "activity", Label: "Walking Pad", DurationMinutes: trackingPtr(45.0), Source: "user"}
	h.process(activityMessage, trackingResult(activityMessage, "create", nil, &activity))
	activityEntry := h.summary(h.a, date).Entries[0]
	activity.Calories, activity.DurationMinutes = trackingPtr(0.0), trackingPtr(0.0)
	activityPath := "/v1/days/" + date + "/entries/" + activityEntry.ID
	h.request(h.a, "PUT", activityPath, entryChange{RequestID: trackingUUID(h.t), Version: 1, Entry: &activity}, 200, nil)
	if summary := h.summary(h.a, date); summary.Totals.UnknownActivityCalories != 0 || summary.Entries[0].Calories == nil || summary.Entries[0].DurationMinutes == nil {
		h.t.Fatal("known zero was treated as unknown")
	}
	activity.Calories = nil
	h.request(h.a, "PUT", activityPath, entryChange{RequestID: trackingUUID(h.t), Version: 2, Entry: &activity}, 200, nil)
	if h.summary(h.a, date).Totals.UnknownActivityCalories != 1 {
		h.t.Fatal("unknown activity calories were converted to zero")
	}
}

func testDirectEntryConcurrency(h *trackingHarness) {
	const date = "2031-02-01"
	meal := h.send(h.a, date, "Testmahlzeit für parallele Korrektur.")
	h.process(meal, trackingResult(meal, "create", nil, trackingPtr(trackingFood(500))))
	entry := h.summary(h.a, date).Entries[0]
	path := "/v1/days/" + date + "/entries/" + entry.ID
	call := func(input entryChange) int {
		body, _ := json.Marshal(input)
		r := httptest.NewRequest(http.MethodPut, path, strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+h.a)
		w := httptest.NewRecorder()
		h.router.ServeHTTP(w, r)
		return w.Code
	}
	values := trackingFood(600)
	input := entryChange{RequestID: trackingUUID(h.t), Version: 1, Entry: &values}
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for range 4 {
		wg.Go(func() { codes <- call(input) })
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			h.t.Fatalf("parallel identical retry: %d", code)
		}
	}
	if got := h.summary(h.a, date).Entries[0]; got.Version != 2 || *got.Calories != 600 {
		h.t.Fatal("parallel retries duplicated change")
	}
	codes = make(chan int, 2)
	for _, calories := range []float64{700, 800} {
		value := trackingFood(calories)
		request := entryChange{RequestID: trackingUUID(h.t), Version: 2, Entry: &value}
		wg.Go(func() { codes <- call(request) })
	}
	wg.Wait()
	close(codes)
	statuses := map[int]int{}
	for code := range codes {
		statuses[code]++
	}
	if statuses[200] != 1 || statuses[409] != 1 || h.summary(h.a, date).Entries[0].Version != 3 {
		h.t.Fatal("concurrent editors must have one winner and one conflict")
	}

	// Hold the exact lock used by the worker. A manual mutation must wait for it
	// rather than checking a queue snapshot from before a new claim.
	tx, err := h.app.DB.Begin(h.ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	defer tx.Rollback(h.ctx)
	if _, err = tx.Exec(h.ctx, `select pg_advisory_xact_lock($1)`, analysisQueueLock); err != nil {
		h.t.Fatal(err)
	}
	result := make(chan int, 1)
	input.RequestID, input.Version = trackingUUID(h.t), 3
	go func() { result <- call(input) }()
	select {
	case <-result:
		h.t.Fatal("manual mutation bypassed the claim lock")
	case <-time.After(30 * time.Millisecond):
	}
	if err = tx.Commit(h.ctx); err != nil {
		h.t.Fatal(err)
	}
	if code := <-result; code != 200 {
		h.t.Fatalf("blocked edit failed: %d", code)
	}
}
