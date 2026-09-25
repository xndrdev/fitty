package httpapi

import (
	"math"
	"strings"
	"testing"

	"fitty/server/internal/intelligence"
)

func TestDirectEntryValidation(t *testing.T) {
	for _, test := range []struct {
		name  string
		edit  func(*intelligence.Values)
		valid bool
	}{
		{"decimal food", func(e *intelligence.Values) { e.Calories = trackingPtr(123.45) }, true},
		{"zero is known", func(e *intelligence.Values) { e.Calories = trackingPtr(0.0) }, true},
		{"missing food energy", func(e *intelligence.Values) { e.Calories = nil }, false},
		{"negative", func(e *intelligence.Values) { e.ProteinG = trackingPtr(-0.01) }, false},
		{"precision", func(e *intelligence.Values) { e.Calories = trackingPtr(123.456) }, false},
		{"nonfinite", func(e *intelligence.Values) { e.Calories = trackingPtr(math.Inf(1)) }, false},
		{"empty label", func(e *intelligence.Values) { e.Label = "  " }, false},
		{"NUL", func(e *intelligence.Values) { e.Notes = "abc\x00def" }, false},
		{"unicode limit", func(e *intelligence.Values) { e.Label = strings.Repeat("ä", 160) }, true},
		{"unicode over limit", func(e *intelligence.Values) { e.Label = strings.Repeat("ä", 161) }, false},
		{"domain maximum", func(e *intelligence.Values) { e.Calories = trackingPtr(20000.01) }, false},
		{"unknown activity calories", func(e *intelligence.Values) {
			*e = intelligence.Values{Kind: "activity", Label: "Walking Pad", DurationMinutes: trackingPtr(0.0), Source: "user"}
		}, true},
		{"activity without measurements", func(e *intelligence.Values) {
			*e = intelligence.Values{Kind: "activity", Label: "Walking Pad", Source: "user"}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := trackingFood(500)
			test.edit(&e)
			if got := validDirectEntry(&e); got != test.valid {
				t.Fatalf("valid = %t, want %t", got, test.valid)
			}
		})
	}
}
