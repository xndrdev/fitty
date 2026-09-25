package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"time"

	"fitty/server/internal/intelligence"
)

func (h *trackingHarness) targets(user string) targetSettings {
	h.t.Helper()
	var settings targetSettings
	h.request(user, "GET", "/v1/targets", nil, 200, &settings)
	return settings
}

func targetsRequest(h *trackingHarness, settings targetSettings, values intelligence.DailyTargets) targetChange {
	h.t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		h.t.Fatal(err)
	}
	return targetChange{RequestID: trackingUUID(h.t), Version: settings.Version, EffectiveFrom: settings.Today, Targets: raw}
}

func testTargets(h *trackingHarness) {
	initial := h.targets(h.a)
	if initial.Version != 0 || initial.EffectiveFrom != nil || initial.Targets != (intelligence.DailyTargets{}) {
		h.t.Fatal("new account must have no implicit targets")
	}
	h.request("invalid", "GET", "/v1/targets", nil, 401, nil)
	profile := Profile{DisplayName: "Zieltest", TimeZone: "Europe/Berlin", Goals: "Ausgewogen essen", Preferences: "vegetarisch"}
	h.request(h.a, "PUT", "/v1/profile", profile, 200, nil)
	initial = h.targets(h.a)
	date, _ := time.Parse(time.DateOnly, initial.Today)
	yesterday, tomorrow := date.AddDate(0, 0, -1).Format(time.DateOnly), date.AddDate(0, 0, 1).Format(time.DateOnly)
	if _, err := h.app.DB.Exec(h.ctx, `insert into fitty.daily_targets(user_id,effective_from,calories,protein_g) values($1,$2::date,1800,100)`, h.a, yesterday); err != nil {
		h.t.Fatal(err)
	}
	values := intelligence.DailyTargets{Calories: trackingPtr(2200.25), ProteinG: trackingPtr(125.5), FatG: trackingPtr(75.0)}
	change := targetsRequest(h, initial, values)
	h.request("invalid", "PUT", "/v1/targets", change, 401, nil)
	for _, bad := range []intelligence.DailyTargets{{Calories: trackingPtr(0.0)}, {Calories: trackingPtr(-1.0)}, {Calories: trackingPtr(0.0000000001)}, {Calories: trackingPtr(20000.01)}, {ProteinG: trackingPtr(2000.01)}, {FatG: trackingPtr(10.111)}} {
		h.request(h.a, "PUT", "/v1/targets", targetsRequest(h, initial, bad), 400, nil)
	}
	staleDate := change
	staleDate.EffectiveFrom = yesterday
	h.request(h.a, "PUT", "/v1/targets", staleDate, 409, nil)
	staleDate.EffectiveFrom = tomorrow
	h.request(h.a, "PUT", "/v1/targets", staleDate, 409, nil)
	h.request(h.a, "PUT", "/v1/targets", change, 200, nil)
	h.request(h.a, "PUT", "/v1/targets", change, 200, nil)
	current := h.targets(h.a)
	if current.Version != 1 || current.EffectiveFrom == nil || *current.EffectiveFrom != initial.Today || !reflect.DeepEqual(current.Targets, values) {
		h.t.Fatalf("unexpected saved targets: %+v", current)
	}
	if h.targets(h.b).Targets != (intelligence.DailyTargets{}) {
		h.t.Fatal("targets leaked to another account")
	}
	for _, test := range []struct {
		date     string
		calories float64
	}{{yesterday, 1800}, {initial.Today, 2200.25}, {tomorrow, 2200.25}} {
		var summary struct {
			Targets       intelligence.DailyTargets `json:"targets"`
			EffectiveFrom *string                   `json:"targets_effective_from"`
		}
		h.request(h.a, "GET", "/v1/days/"+test.date+"/summary", nil, 200, &summary)
		if summary.Targets.Calories == nil || *summary.Targets.Calories != test.calories {
			h.t.Errorf("wrong target for %s", test.date)
		}
	}
	conflict := change
	conflict.RequestID = trackingUUID(h.t)
	h.request(h.a, "PUT", "/v1/targets", conflict, 409, nil)
	conflict = targetsRequest(h, current, intelligence.DailyTargets{})
	conflict.RequestID = change.RequestID
	h.request(h.a, "PUT", "/v1/targets", conflict, 409, nil)
	// Legacy profile writes remain independent and cannot erase numeric targets.
	profile.DisplayName = "Neuer Name"
	h.request(h.a, "PUT", "/v1/profile", profile, 200, nil)
	if !reflect.DeepEqual(h.targets(h.a).Targets, values) {
		h.t.Fatal("profile update erased targets")
	}
	meal := h.send(h.a, initial.Today, "Eine Mahlzeit gegessen.")
	job := h.claim(meal)
	input, err := h.app.analysisInput(h.ctx, job)
	if err != nil || !reflect.DeepEqual(input.DailyTargets, values) {
		h.t.Fatalf("worker lost today's targets: %v", err)
	}
	h.returnResult(trackingResult(meal, "create", nil, trackingPtr(trackingFood(500))))
	h.app.processAnalysis(h.ctx, job)
	h.status(meal, "completed", 1)
	oldMessage := h.send(h.a, yesterday, "Frage zum früheren Tag.")
	oldJob := h.claim(oldMessage)
	oldInput, err := h.app.analysisInput(h.ctx, oldJob)
	if err != nil || oldInput.DailyTargets.Calories == nil || *oldInput.DailyTargets.Calories != 1800 {
		h.t.Fatal("historical worker received current targets")
	}
	h.returnResult(trackingAdvice())
	h.app.processAnalysis(h.ctx, oldJob)
	clear := targetsRequest(h, current, intelligence.DailyTargets{})
	h.request(h.a, "PUT", "/v1/targets", clear, 200, nil)
	h.request(h.a, "PUT", "/v1/targets", change, 200, nil) // An old successful retry never restores old goals.
	if h.targets(h.a).Version != 2 || h.targets(h.a).Targets != (intelligence.DailyTargets{}) {
		h.t.Fatal("clear or old retry changed target version/content")
	}
	old, err := loadTargetsForDate(h.ctx, h.app.DB, h.a, yesterday)
	if err != nil || old.Targets.Calories == nil || *old.Targets.Calories != 1800 {
		h.t.Fatal("clear changed historical target")
	}
	if h.summary(h.a, initial.Today).Totals.Calories != 500 {
		h.t.Fatal("targets changed actual tracking totals")
	}
	// Simulate a previously confirmed request from yesterday. Its retry must
	// succeed after midnight without applying those old targets again.
	oldReceipt := targetsRequest(h, current, values)
	oldReceipt.RequestID = trackingUUID(h.t)
	oldReceipt.EffectiveFrom = yesterday
	_, err = h.app.DB.Exec(h.ctx, `insert into fitty.daily_target_changes(user_id,request_id,expected_version,effective_from,targets) values($1,$2,$3,$4::date,$5::jsonb)`, h.a, oldReceipt.RequestID, oldReceipt.Version, yesterday, []byte(oldReceipt.Targets))
	if err != nil {
		h.t.Fatal(err)
	}
	h.request(h.a, "PUT", "/v1/targets", oldReceipt, 200, nil)
	if h.targets(h.a).Version != 2 {
		h.t.Fatal("confirmed retry after midnight mutated targets")
	}
	h.request(h.a, "DELETE", "/v1/days/"+initial.Today, deletionRequest(h, initial.Today), 200, nil)
	if h.targets(h.a).Version != 2 {
		h.t.Fatal("day deletion removed profile targets")
	}
	old, err = loadTargetsForDate(h.ctx, h.app.DB, h.a, yesterday)
	if err != nil || old.Targets.Calories == nil {
		h.t.Fatal("day deletion erased target history")
	}
}

func testTargetConcurrency(h *trackingHarness) {
	current := h.targets(h.a)
	requests := []targetChange{targetsRequest(h, current, intelligence.DailyTargets{Calories: trackingPtr(2000.0)}), targetsRequest(h, current, intelligence.DailyTargets{Calories: trackingPtr(2100.0)})}
	run := func(changes []targetChange) []int {
		var wait sync.WaitGroup
		results := make([]int, len(changes))
		for index, change := range changes {
			wait.Go(func() {
				raw, _ := json.Marshal(change)
				r := httptest.NewRequest("PUT", "/v1/targets", strings.NewReader(string(raw)))
				r.Header.Set("Authorization", "Bearer "+h.a)
				w := httptest.NewRecorder()
				h.router.ServeHTTP(w, r)
				results[index] = w.Code
			})
		}
		wait.Wait()
		return results
	}
	statuses := run(requests)
	if !((statuses[0] == 200 && statuses[1] == 409) || (statuses[0] == 409 && statuses[1] == 200)) {
		h.t.Fatalf("concurrent target writes: %v", statuses)
	}
	if h.targets(h.a).Version != current.Version+1 {
		h.t.Fatal("competing targets both applied")
	}
	retry := targetsRequest(h, h.targets(h.a), intelligence.DailyTargets{ProteinG: trackingPtr(150.0)})
	statuses = run([]targetChange{retry, retry})
	if statuses[0] != 200 || statuses[1] != 200 || h.targets(h.a).Version != current.Version+2 {
		h.t.Fatalf("parallel identical retry: %v", statuses)
	}
}
