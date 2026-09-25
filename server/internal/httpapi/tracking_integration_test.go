package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type trackingProvider func(context.Context, intelligence.Input) (intelligence.Result, error)

func (provider trackingProvider) Analyze(ctx context.Context, input intelligence.Input) (intelligence.Result, error) {
	return provider(ctx, input)
}

type trackingHarness struct {
	t      *testing.T
	ctx    context.Context
	app    *App
	router http.Handler
	a, b   string
	jobs   []string
}

type trackingSummary struct {
	Entries      []intelligence.Entry `json:"entries"`
	Totals       Totals               `json:"totals"`
	AIEnabled    bool                 `json:"ai_enabled"`
	PendingCount int                  `json:"pending_count"`
}

func TestTrackingIntegration(t *testing.T) {
	database, userA, userB := os.Getenv("FITTY_TEST_DATABASE_URL"), os.Getenv("FITTY_TEST_USER_A"), os.Getenv("FITTY_TEST_USER_B")
	if database == "" || userA == "" || userB == "" {
		t.Skip("Run scripts/test-tracking.mjs against local Supabase to supply temporary users and the test database.")
	}
	endpoint, err := url.Parse(database)
	if err != nil || (endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "localhost") || endpoint.Port() != "54322" ||
		endpoint.User.Username() != "fitty_api" || !uuidPattern.MatchString(userA) || !uuidPattern.MatchString(userB) || userA == userB {
		t.Fatal("Tracking integration requires local fitty_api credentials and two distinct temporary users")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := pgxpool.New(ctx, database)
	if err != nil {
		t.Fatal("Could not initialize local test database connection")
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatal("Could not connect to local test database")
	}
	// claimAnalysis coordinates the entire queue. Never process unrelated local
	// jobs while exercising it, or run this alongside an active production worker.
	var unrelated int
	err = db.QueryRow(ctx, `select count(*) from fitty.analysis_jobs where user_id not in ($1::uuid,$2::uuid) and status in ('queued','processing')`, userA, userB).Scan(&unrelated)
	if err != nil {
		t.Fatal("Tracking schema is unavailable; apply local migrations first")
	}
	if unrelated != 0 {
		t.Fatal("Unrelated local analyses are pending; leave their queue untouched and finish them before running integration tests")
	}
	for _, test := range []struct {
		name string
		run  func(*trackingHarness)
	}{
		{"record_correction_advice_activity_delete_and_ownership", testTrackingLifecycle},
		{"invalid_result_and_transaction_rollback", testTrackingRollback},
		{"order_failure_retry_limit_and_skip", testTrackingOrdering},
		{"stale_lease_recovery_and_exhaustion", testTrackingLeases},
		{"disabled_job_explicit_retry", testTrackingDisabled},
		{"photos_ownership_retry_and_worker_context", testPhotoIntegration},
		{"direct_edits_deletion_versions_and_audit", testDirectEntries},
		{"direct_edit_retries_and_concurrent_writers", testDirectEntryConcurrency},
		{"whole_day_deletion_versions_cascades_and_late_retries", testDayDeletion},
		{"whole_day_parallel_deletion_retries", testDayDeletionConcurrency},
		{"whole_day_durable_photo_cleanup_and_upload_retries", testDayDeletionPhotos},
		{"daily_targets_history_ownership_context_and_retries", testTargets},
		{"daily_target_concurrent_writes", testTargetConcurrency},
		{"progress_photos_private_lifecycle_and_chat_separation", testProgressPhotos},
		{"progress_photos_pagination_filters_and_cleanup", testProgressPagination},
		{"progress_photos_parallel_upload_and_cleanup", testProgressConcurrency},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := &trackingHarness{t: t, ctx: ctx, a: userA, b: userB}
			h.app = &App{DB: db, Model: "offline-test-provider"}
			h.returnResult(intelligence.Result{Reply: "Testantwort.", Intent: "advice", Actions: []intelligence.Action{}})
			h.app.Auth = &Auth{URL: "https://auth.example.test", Key: "test-public-key", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				user := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				status, body := 401, `{}`
				if user == userA || user == userB {
					status, body = 200, `{"id":"`+user+`"}`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			h.router = New(nil, h.app)
			t.Cleanup(func() {
				// Only this subtest's unfinished jobs are stopped. The wrapper removes
				// the temporary accounts and their cascaded data after Go exits.
				for _, id := range h.jobs {
					if _, err := db.Exec(context.Background(), `update fitty.analysis_jobs set status='skipped',lease=null where message_id=$1::bigint and user_id in ($2::uuid,$3::uuid) and status in ('queued','processing','failed')`, id, userA, userB); err != nil {
						t.Error("Could not finalize temporary test job")
					}
				}
			})
			test.run(h)
		})
	}
}

func (h *trackingHarness) request(user, method, path string, body any, status int, value any) {
	h.t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, strings.NewReader(string(data)))
	request.Header.Set("Authorization", "Bearer "+user)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	if recorder.Code != status {
		h.t.Fatalf("%s %s: status %d, want %d; response %s", method, path, recorder.Code, status, recorder.Body.String())
	}
	if value != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), value); err != nil {
			h.t.Fatal(err)
		}
	}
}

func trackingUUID(t *testing.T) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	value[6], value[8] = value[6]&0x0f|0x40, value[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}

func (h *trackingHarness) send(user, date, text string) Message {
	h.t.Helper()
	var response struct {
		Message Message `json:"message"`
	}
	h.request(user, "POST", "/v1/days/"+date+"/messages", map[string]any{"client_id": trackingUUID(h.t), "content": text}, 201, &response)
	h.jobs = append(h.jobs, response.Message.ID)
	return response.Message
}

func (h *trackingHarness) summary(user, date string) trackingSummary {
	h.t.Helper()
	var result trackingSummary
	h.request(user, "GET", "/v1/days/"+date+"/summary", nil, 200, &result)
	return result
}

func (h *trackingHarness) claim(message Message) analysisJob {
	h.t.Helper()
	job, err := h.app.claimAnalysis(h.ctx)
	if err != nil {
		h.t.Fatalf("could not claim expected job: %v", err)
	}
	if strconv.FormatInt(job.MessageID, 10) != message.ID || (job.UserID != h.a && job.UserID != h.b) {
		h.t.Fatal("claimed a different job; integration tests require an isolated local queue")
	}
	return job
}

func (h *trackingHarness) noClaim() {
	h.t.Helper()
	if _, err := h.app.claimAnalysis(h.ctx); !errors.Is(err, pgx.ErrNoRows) {
		h.t.Fatalf("expected no eligible job, got %v", err)
	}
}

func (h *trackingHarness) returnResult(result intelligence.Result) {
	h.app.AI = trackingProvider(func(context.Context, intelligence.Input) (intelligence.Result, error) { return result, nil })
}

func (h *trackingHarness) process(message Message, result intelligence.Result) analysisJob {
	h.t.Helper()
	h.returnResult(result)
	job := h.claim(message)
	h.app.processAnalysis(h.ctx, job)
	h.status(message, "completed", 1)
	return job
}

func (h *trackingHarness) status(message Message, want string, attempts int) {
	h.t.Helper()
	var state string
	var actual int
	if err := h.app.DB.QueryRow(h.ctx, `select status,attempts from fitty.analysis_jobs where message_id=$1::bigint`, message.ID).Scan(&state, &actual); err != nil {
		h.t.Fatal(err)
	}
	if state != want || actual != attempts {
		h.t.Fatalf("job state %s/%d, want %s/%d", state, actual, want, attempts)
	}
}

func (h *trackingHarness) counts(message Message, entries, changes, replies int) {
	h.t.Helper()
	var gotEntries, gotChanges, gotReplies int
	err := h.app.DB.QueryRow(h.ctx, `select
 (select count(*) from fitty.tracking_entries where source_message_id=$1::bigint),
 (select count(*) from fitty.entry_changes where message_id=$1::bigint),
 (select count(*) from fitty.messages where reply_to=$1::bigint)`, message.ID).Scan(&gotEntries, &gotChanges, &gotReplies)
	if err != nil {
		h.t.Fatal(err)
	}
	if gotEntries != entries || gotChanges != changes || gotReplies != replies {
		h.t.Fatalf("entry/change/reply counts %d/%d/%d, want %d/%d/%d", gotEntries, gotChanges, gotReplies, entries, changes, replies)
	}
}

func trackingPtr[T any](value T) *T { return &value }

func trackingFood(calories float64) intelligence.Values {
	return intelligence.Values{Kind: "food", Label: "Testmahlzeit", Amount: "1 Portion", Calories: &calories,
		ProteinG: trackingPtr(30.0), CarbsG: trackingPtr(60.0), FatG: trackingPtr(15.0), Source: "estimate", Notes: "Testschätzung."}
}

func trackingResult(message Message, operation string, id *string, values *intelligence.Values) intelligence.Result {
	intent := "record"
	if operation != "create" {
		intent = "correction"
	}
	return intelligence.Result{Reply: "Testbuchung verarbeitet.", Intent: intent,
		Actions: []intelligence.Action{{Operation: operation, EntryID: id, Entry: values, Evidence: message.Content}}}
}

func trackingAdvice() intelligence.Result {
	return intelligence.Result{Reply: "Eine ausgewogene Mahlzeit mit Gemüse und einer Proteinquelle passt gut.", Intent: "advice", Actions: []intelligence.Action{}}
}

func testTrackingLifecycle(h *trackingHarness) {
	const date = "2030-01-01"
	profile := Profile{DisplayName: "Tracking-Test", TimeZone: "Europe/Berlin", Goals: "Regelmäßig essen", Preferences: "Vegetarisch"}
	h.request(h.a, "PUT", "/v1/profile", profile, 200, nil)
	meal := h.send(h.a, date, "Ich habe eine Portion der Testmahlzeit gegessen.")
	if meal.AnalysisStatus != "queued" {
		h.t.Fatal("new message must be queued")
	}
	if summary := h.summary(h.a, date); summary.PendingCount != 1 || len(summary.Entries) != 0 || !summary.AIEnabled {
		h.t.Fatal("pending summary must not anticipate model values")
	}
	job := h.claim(meal)
	input, err := h.app.analysisInput(h.ctx, job)
	if err != nil || input.Date != date || input.Profile.Preferences != profile.Preferences || len(input.History) != 0 || len(input.Entries) != 0 {
		h.t.Fatalf("unexpected initial model context: %v", err)
	}
	mealResult := trackingResult(meal, "create", nil, trackingPtr(trackingFood(500)))
	h.returnResult(mealResult)
	h.app.processAnalysis(h.ctx, job)
	h.status(meal, "completed", 1)
	h.counts(meal, 1, 1, 1)
	summary := h.summary(h.a, date)
	want := Totals{Calories: 500, ProteinG: 30, CarbsG: 60, FatG: 15, EstimatedEntries: 1}
	if summary.Totals != want || len(summary.Entries) != 1 || summary.PendingCount != 0 {
		h.t.Fatalf("wrong food summary: %#v", summary)
	}
	entryID := summary.Entries[0].ID
	var repeated struct {
		Message Message `json:"message"`
	}
	h.request(h.a, "POST", "/v1/days/"+date+"/messages", map[string]any{"client_id": meal.ClientID, "content": meal.Content}, 200, &repeated)
	if repeated.Message.ID != meal.ID || repeated.Message.AnalysisStatus != "completed" {
		h.t.Fatal("message resend changed job identity or state")
	}
	if err := h.app.commitAnalysis(h.ctx, job, input, mealResult); err == nil {
		h.t.Fatal("completed job accepted a repeated commit")
	}
	h.app.processAnalysis(h.ctx, job)
	h.counts(meal, 1, 1, 1)
	h.status(meal, "completed", 1)

	other := h.send(h.b, date, "Ich habe meine eigene Testmahlzeit gegessen.")
	h.process(other, trackingResult(other, "create", nil, trackingPtr(trackingFood(250))))
	if own, foreign := h.summary(h.a, date), h.summary(h.b, date); own.Totals.Calories != 500 || foreign.Totals.Calories != 250 || foreign.Entries[0].ID == entryID {
		h.t.Fatal("same calendar date mixed tracking data across users")
	}
	h.request(h.b, "POST", "/v1/messages/"+meal.ID+"/analysis/retry", nil, 409, nil)
	correction := h.send(h.a, date, "Korrektur: Die Testmahlzeit hatte 650 kcal, 40 g Protein, 80 g Kohlenhydrate und 20 g Fett.")
	corrected := trackingFood(650)
	corrected.ProteinG, corrected.CarbsG, corrected.FatG, corrected.Source = trackingPtr(40.0), trackingPtr(80.0), trackingPtr(20.0), "user"
	h.process(correction, trackingResult(correction, "update", &entryID, &corrected))
	summary = h.summary(h.a, date)
	if len(summary.Entries) != 1 || summary.Entries[0].ID != entryID || summary.Totals != (Totals{Calories: 650, ProteinG: 40, CarbsG: 80, FatG: 20}) {
		h.t.Fatalf("correction did not replace existing values: %#v", summary)
	}
	h.counts(correction, 0, 1, 1)
	advice := h.send(h.a, date, "Ich gehe später ins Restaurant. Was wäre eine gute Wahl?")
	adviceJob := h.claim(advice)
	adviceInput, err := h.app.analysisInput(h.ctx, adviceJob)
	if err != nil || len(adviceInput.Entries) != 1 || *adviceInput.Entries[0].Calories != 650 || len(adviceInput.History) != 4 {
		h.t.Fatalf("advice must see corrected data and preceding conversation: %v", err)
	}
	if adviceInput.DailyTotals["food_calories"] != 650 || adviceInput.DailyTotals["protein_g"] != 40 || adviceInput.DailyTotals["activity_calories"] != 0 {
		h.t.Fatal("model context must include the authoritative corrected totals")
	}
	var previousID int64
	for _, message := range adviceInput.History {
		id, parseErr := strconv.ParseInt(message.ID, 10, 64)
		if parseErr != nil || id <= previousID {
			h.t.Fatal("model conversation must remain in numeric message order")
		}
		previousID = id
	}
	h.returnResult(trackingAdvice())
	h.app.processAnalysis(h.ctx, adviceJob)
	h.counts(advice, 0, 0, 1)
	if after := h.summary(h.a, date); !reflect.DeepEqual(after, summary) {
		h.t.Fatal("restaurant advice changed daily tracking")
	}
	walk := h.send(h.a, date, "Ich bin 45 Minuten und 3,2 km auf dem Walking Pad gegangen, Kalorien unbekannt.")
	walkValues := intelligence.Values{Kind: "activity", Label: "Walking Pad", Amount: "45 Minuten", DurationMinutes: trackingPtr(45.0), DistanceKM: trackingPtr(3.2), Source: "user"}
	h.process(walk, trackingResult(walk, "create", nil, &walkValues))
	summary = h.summary(h.a, date)
	if summary.Entries[1].Calories != nil || summary.Totals.UnknownActivityCalories != 1 || summary.Totals.ActivityCalories != 0 || summary.Totals.Calories != 650 || summary.Totals.DurationMinutes != 45 || summary.Totals.DistanceKM != 3.2 {
		h.t.Fatalf("unknown activity calories must stay null and separate: %#v", summary)
	}
	activity := h.send(h.a, date, "20 Minuten Bewegung abgeschlossen, das Gerät meldet 120 kcal.")
	activityValues := intelligence.Values{Kind: "activity", Label: "Bewegung", Calories: trackingPtr(120.0), DurationMinutes: trackingPtr(20.0), Source: "device"}
	h.process(activity, trackingResult(activity, "create", nil, &activityValues))
	summary = h.summary(h.a, date)
	if summary.Totals.Calories != 650 || summary.Totals.ActivityCalories != 120 || summary.Totals.DurationMinutes != 65 || summary.Totals.UnknownActivityCalories != 1 {
		h.t.Fatalf("food and activity totals must remain separate: %#v", summary.Totals)
	}
	deletion := h.send(h.a, date, "Bitte entferne die Testmahlzeit, sie gehörte nicht zu diesem Tag.")
	h.process(deletion, trackingResult(deletion, "delete", &entryID, nil))
	summary = h.summary(h.a, date)
	if len(summary.Entries) != 2 || summary.Totals.Calories != 0 || summary.Totals.ProteinG != 0 || summary.Totals.ActivityCalories != 120 {
		h.t.Fatal("deleted food is still counted")
	}
	var deleted, before, after bool
	err = h.app.DB.QueryRow(h.ctx, `select e.deleted_at is not null,c.before_value->>'calories'='650',c.after_value is null
 from fitty.tracking_entries e join fitty.entry_changes c on c.entry_id=e.id where e.id=$1::bigint and c.message_id=$2::bigint`, entryID, deletion.ID).Scan(&deleted, &before, &after)
	if err != nil || !deleted || !before || !after {
		h.t.Fatalf("soft deletion must retain previous values in audit: %v", err)
	}
	var updateBefore, updateAfter float64
	err = h.app.DB.QueryRow(h.ctx, `select (before_value->>'calories')::float8,(after_value->>'calories')::float8 from fitty.entry_changes where message_id=$1::bigint`, correction.ID).Scan(&updateBefore, &updateAfter)
	if err != nil || updateBefore != 500 || updateAfter != 650 {
		h.t.Fatalf("correction audit missing before/after values: %v", err)
	}
	h.request(h.a, "GET", "/v1/days/2030-02-30/summary", nil, 400, nil)
	if empty := h.summary(h.a, "2030-01-02"); len(empty.Entries) != 0 || empty.Totals != (Totals{}) {
		h.t.Fatal("an empty day must have empty tracking")
	}
}

func testTrackingRollback(h *trackingHarness) {
	const date = "2030-02-01"
	foreign := h.send(h.b, date, "Meine eigene Testmahlzeit ist gegessen.")
	h.process(foreign, trackingResult(foreign, "create", nil, trackingPtr(trackingFood(200))))
	foreignID := h.summary(h.b, date).Entries[0].ID
	for index, name := range []string{"negative", "foreign_entry", "invalid_evidence"} {
		message := h.send(h.a, fmt.Sprintf("2030-02-%02d", index+2), "Ich habe eine Testmahlzeit gegessen und korrigiere einen Eintrag.")
		result := trackingResult(message, "create", nil, trackingPtr(trackingFood(300)))
		invalid := result.Actions[0]
		switch name {
		case "negative":
			invalid.Entry = trackingPtr(trackingFood(-5))
		case "foreign_entry":
			invalid.Operation, invalid.EntryID = "update", &foreignID
		case "invalid_evidence":
			invalid.Evidence = "Dieser Text steht nicht in der Nachricht."
		}
		result.Actions = append(result.Actions, invalid)
		h.returnResult(result)
		job := h.claim(message)
		h.app.processAnalysis(h.ctx, job)
		h.status(message, "failed", 1)
		h.counts(message, 0, 0, 0)
		var code string
		if err := h.app.DB.QueryRow(h.ctx, `select error_code from fitty.analysis_jobs where message_id=$1::bigint`, message.ID).Scan(&code); err != nil || code != "invalid_result" {
			h.t.Fatalf("invalid result failure must be persisted: %s, %v", code, err)
		}
	}
	if summary := h.summary(h.b, date); len(summary.Entries) != 1 || summary.Totals.Calories != 200 {
		h.t.Fatal("foreign tracking changed after invalid correction")
	}
	// Simulate an audit write conflict after the entry INSERT to verify real
	// transaction rollback, not only validation before the first SQL mutation.
	message := h.send(h.a, "2030-02-10", "Ich habe die Testmahlzeit gegessen.")
	job := h.claim(message)
	_, err := h.app.DB.Exec(h.ctx, `insert into fitty.entry_changes(message_id,action_index,entry_id,operation,evidence) values($1::bigint,0,$2::bigint,'create','synthetic audit conflict')`, message.ID, foreignID)
	if err != nil {
		h.t.Fatal(err)
	}
	h.returnResult(trackingResult(message, "create", nil, trackingPtr(trackingFood(400))))
	h.app.processAnalysis(h.ctx, job)
	h.status(message, "failed", 1)
	h.counts(message, 0, 1, 0)
	if summary := h.summary(h.a, "2030-02-10"); len(summary.Entries) != 0 || summary.Totals.Calories != 0 {
		h.t.Fatal("entry insertion escaped a failed audit transaction")
	}
}

func testTrackingOrdering(h *trackingHarness) {
	const date = "2030-03-01"
	first := h.send(h.a, date, "Erste Nachricht zur Beratung.")
	second := h.send(h.a, date, "Zweite Nachricht zur Beratung.")
	independent := h.send(h.b, date, "Unabhängige Beratung für den anderen Nutzer.")
	firstJob := h.claim(first)
	independentJob := h.claim(independent)
	h.noClaim()
	h.returnResult(trackingAdvice())
	h.app.processAnalysis(h.ctx, independentJob)
	h.app.AI = trackingProvider(func(context.Context, intelligence.Input) (intelligence.Result, error) {
		return intelligence.Result{}, intelligence.ErrUnavailable
	})
	h.app.processAnalysis(h.ctx, firstJob)
	h.status(first, "failed", 1)
	h.noClaim()
	h.request(h.b, "POST", "/v1/messages/"+first.ID+"/analysis/retry", nil, 409, nil)
	for attempt := 2; attempt <= 3; attempt++ {
		h.request(h.a, "POST", "/v1/messages/"+first.ID+"/analysis/retry", nil, 200, nil)
		job := h.claim(first)
		h.app.processAnalysis(h.ctx, job)
		h.status(first, "failed", attempt)
		h.noClaim()
	}
	h.request(h.a, "POST", "/v1/messages/"+first.ID+"/analysis/retry", nil, 409, nil)
	h.request(h.b, "POST", "/v1/messages/"+first.ID+"/analysis/skip", nil, 409, nil)
	h.status(first, "failed", 3)
	h.request(h.a, "POST", "/v1/messages/"+first.ID+"/analysis/skip", nil, 200, nil)
	h.status(first, "skipped", 3)
	h.process(second, trackingAdvice())
	h.counts(first, 0, 0, 0)
	h.counts(second, 0, 0, 1)
	if summary := h.summary(h.a, date); summary.PendingCount != 0 || len(summary.Entries) != 0 {
		h.t.Fatal("skip did not release the next daily message")
	}
}

func testTrackingLeases(h *trackingHarness) {
	message := h.send(h.a, "2030-04-01", "Ich habe eine Testmahlzeit gegessen.")
	staleJob := h.claim(message)
	input, err := h.app.analysisInput(h.ctx, staleJob)
	if err != nil {
		h.t.Fatal(err)
	}
	newLease := trackingUUID(h.t)
	if _, err := h.app.DB.Exec(h.ctx, `update fitty.analysis_jobs set lease=$2::uuid where message_id=$1::bigint`, message.ID, newLease); err != nil {
		h.t.Fatal(err)
	}
	result := trackingResult(message, "create", nil, trackingPtr(trackingFood(300)))
	if err := h.app.commitAnalysis(h.ctx, staleJob, input, result); err == nil {
		h.t.Fatal("stale lease was permitted to commit")
	}
	h.returnResult(result)
	h.app.processAnalysis(h.ctx, staleJob)
	h.status(message, "processing", 1)
	h.counts(message, 0, 0, 0)
	var lease string
	if err := h.app.DB.QueryRow(h.ctx, `select lease::text from fitty.analysis_jobs where message_id=$1::bigint`, message.ID).Scan(&lease); err != nil || lease != newLease {
		h.t.Fatal("stale attempt overwrote the replacement lease")
	}
	if _, err := h.app.DB.Exec(h.ctx, `update fitty.analysis_jobs set started_at=now()-interval '3 minutes' where message_id=$1::bigint`, message.ID); err != nil {
		h.t.Fatal(err)
	}
	recovered := h.claim(message)
	if recovered.Lease == staleJob.Lease || recovered.Lease == newLease {
		h.t.Fatal("recovered attempt reused an expired lease")
	}
	h.app.processAnalysis(h.ctx, recovered)
	h.status(message, "completed", 2)
	h.counts(message, 1, 1, 1)

	exhausted := h.send(h.a, "2030-04-02", "Diese Testberatung wurde unterbrochen.")
	_ = h.claim(exhausted)
	if _, err := h.app.DB.Exec(h.ctx, `update fitty.analysis_jobs set attempts=3,started_at=now()-interval '3 minutes' where message_id=$1::bigint`, exhausted.ID); err != nil {
		h.t.Fatal(err)
	}
	h.noClaim()
	h.status(exhausted, "failed", 3)
	if summary := h.summary(h.a, "2030-04-02"); summary.PendingCount != 0 {
		h.t.Fatal("exhausted interrupted job must no longer stay pending")
	}
	h.request(h.a, "POST", "/v1/messages/"+exhausted.ID+"/analysis/retry", nil, 409, nil)
}

func testTrackingDisabled(h *trackingHarness) {
	h.app.AI = nil
	message := h.send(h.a, "2030-05-01", "Später auswertbare Testberatung.")
	if message.AnalysisStatus != "disabled" || h.summary(h.a, "2030-05-01").AIEnabled {
		h.t.Fatal("missing provider must retain saved message with disabled analysis")
	}
	h.request(h.a, "POST", "/v1/messages/"+message.ID+"/analysis/retry", nil, 503, nil)
	h.returnResult(trackingAdvice())
	h.request(h.a, "POST", "/v1/messages/"+message.ID+"/analysis/retry", nil, 200, nil)
	h.process(message, trackingAdvice())
	h.counts(message, 0, 0, 1)
	var saved struct {
		Message Message `json:"message"`
	}
	h.request(h.a, "POST", "/v1/days/2030-05-02/messages", map[string]any{"client_id": trackingUUID(h.t), "content": "Nur speichern.", "analyze": false}, 201, &saved)
	h.jobs = append(h.jobs, saved.Message.ID)
	if saved.Message.AnalysisStatus != "disabled" {
		h.t.Fatal("explicit analyze:false must not enqueue a model call")
	}
	h.noClaim()
}
