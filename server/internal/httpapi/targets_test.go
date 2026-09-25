package httpapi

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTargetInput(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"calories":2000}`, `{"calories":"2000","protein_g":null,"carbs_g":null,"fat_g":null}`, `{"calories":2000,"protein_g":null,"carbs_g":null,"extra":null}`, `{"calories":0,"protein_g":null,"carbs_g":null,"fat_g":null}`} {
		if _, valid := parseTargets(json.RawMessage(raw)); valid {
			t.Errorf("accepted partial/invalid target body: %s", raw)
		}
	}
	if _, valid := parseTargets(json.RawMessage(`{"calories":2000.25,"protein_g":125.5,"carbs_g":null,"fat_g":null}`)); !valid {
		t.Fatal("valid independent targets rejected")
	}
}

func TestTargetDateInProfileZone(t *testing.T) {
	now := time.Date(2026, 9, 17, 23, 30, 0, 0, time.UTC)
	for _, test := range []struct{ zone, date string }{{"Europe/Berlin", "2026-09-18"}, {"America/New_York", "2026-09-17"}, {"Pacific/Kiritimati", "2026-09-18"}} {
		got, err := targetsToday(test.zone, now)
		if err != nil || got != test.date {
			t.Errorf("%s: %s, %v", test.zone, got, err)
		}
	}
	if _, err := targetsToday("invalid/zone", now); err == nil {
		t.Fatal("invalid zone accepted")
	}
}
