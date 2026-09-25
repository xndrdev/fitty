package intelligence

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func ptr[T any](value T) *T { return &value }

func food() Values {
	return Values{Kind: "food", Label: "Apfel", Amount: "1 mittelgroßer Apfel", Calories: ptr(95.0), ProteinG: ptr(0.5), CarbsG: ptr(25.0), FatG: ptr(0.3), Source: "estimate", Notes: "Geschätzt für etwa 180 g."}
}

func recorded() Result {
	return Result{Reply: "Der Apfel ist mit geschätzt 95 kcal erfasst.", Intent: "record", Actions: []Action{{Operation: "create", Entry: ptr(food()), Evidence: "einen Apfel gegessen"}}}
}

func TestValidateAcceptedResults(t *testing.T) {
	tests := []struct {
		name   string
		result Result
	}{
		{"food estimate", recorded()},
		{"food exact", Result{Reply: "Erfasst.", Intent: "record", Actions: []Action{{Operation: "create", Entry: ptr(Values{Kind: "food", Label: "Wasser", Amount: "250 ml", Calories: ptr(0.0), ProteinG: ptr(0.0), CarbsG: ptr(0.0), FatG: ptr(0.0), Source: "user"}), Evidence: "Apfel"}}}},
		{"activity duration", Result{Reply: "Erfasst.", Intent: "mixed", Actions: []Action{{Operation: "create", Entry: ptr(Values{Kind: "activity", Label: "Walking Pad", DurationMinutes: ptr(45.0), Source: "user"}), Evidence: "Apfel"}}}},
		{"activity distance", Result{Reply: "Erfasst.", Intent: "record", Actions: []Action{{Operation: "create", Entry: ptr(Values{Kind: "activity", Label: "Spaziergang", DistanceKM: ptr(4.5), Calories: ptr(200.0), Source: "device"}), Evidence: "Apfel"}}}},
		{"existing food correction", Result{Reply: "Korrigiert.", Intent: "correction", Actions: []Action{{Operation: "update", EntryID: ptr("entry-1"), Entry: ptr(food()), Evidence: "Apfel"}}}},
		{"existing food deletion", Result{Reply: "Entfernt.", Intent: "correction", Actions: []Action{{Operation: "delete", EntryID: ptr("entry-1"), Evidence: "Apfel"}}}},
		{"advice", Result{Reply: "Zum Essen passt Gemüse.", Intent: "advice", Actions: []Action{}}},
		{"clarification", Result{Reply: "Wie groß war die Portion?", Intent: "clarification", Actions: []Action{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.result, testInput()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateRejectsInvalidActions(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Result)
	}{
		{"empty reply", func(result *Result) { result.Reply = " " }},
		{"long reply", func(result *Result) { result.Reply = strings.Repeat("ä", 8001) }},
		{"invalid utf8", func(result *Result) { result.Reply = string([]byte{0xff}) }},
		{"unknown intent", func(result *Result) { result.Intent = "new_intent" }},
		{"advice action", func(result *Result) { result.Intent = "advice" }},
		{"clarification action", func(result *Result) { result.Intent = "clarification" }},
		{"null actions", func(result *Result) { result.Actions = nil }},
		{"too many actions", func(result *Result) { result.Actions = make([]Action, 21) }},
		{"unknown operation", func(result *Result) { result.Actions[0].Operation = "upsert" }},
		{"create with ID", func(result *Result) { result.Actions[0].EntryID = ptr("entry-1") }},
		{"create without values", func(result *Result) { result.Actions[0].Entry = nil }},
		{"update without ID", func(result *Result) { result.Actions[0].Operation = "update" }},
		{"unknown update ID", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID = "update", ptr("not-this-day")
		}},
		{"unknown delete ID", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID, result.Actions[0].Entry = "delete", ptr("not-this-day"), nil
		}},
		{"delete includes values", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID = "delete", ptr("entry-1")
		}},
		{"update without values", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID, result.Actions[0].Entry = "update", ptr("entry-1"), nil
		}},
		{"changing kind", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID = "update", ptr("entry-1")
			result.Actions[0].Entry = &Values{Kind: "activity", Label: "Gehen", DurationMinutes: ptr(30.0), Source: "user"}
		}},
		{"same ID twice", func(result *Result) {
			result.Actions[0].Operation, result.Actions[0].EntryID = "update", ptr("entry-1")
			result.Actions = append(result.Actions, Action{Operation: "delete", EntryID: ptr("entry-1"), Evidence: "Apfel"})
		}},
		{"missing evidence", func(result *Result) { result.Actions[0].Evidence = " " }},
		{"evidence from history", func(result *Result) { result.Actions[0].Evidence = "Zum Frühstück gab es Porridge." }},
		{"paraphrased evidence", func(result *Result) { result.Actions[0].Evidence = "Apfel war gegessen" }},
		{"unknown kind", func(result *Result) { result.Actions[0].Entry.Kind = "sleep" }},
		{"unknown source", func(result *Result) { result.Actions[0].Entry.Source = "database" }},
		{"empty label", func(result *Result) { result.Actions[0].Entry.Label = " " }},
		{"long label", func(result *Result) { result.Actions[0].Entry.Label = strings.Repeat("ä", 161) }},
		{"long amount", func(result *Result) { result.Actions[0].Entry.Amount = strings.Repeat("a", 161) }},
		{"long notes", func(result *Result) { result.Actions[0].Entry.Notes = strings.Repeat("a", 1001) }},
		{"food unknown calories", func(result *Result) { result.Actions[0].Entry.Calories = nil }},
		{"food unknown protein", func(result *Result) { result.Actions[0].Entry.ProteinG = nil }},
		{"food unknown carbs", func(result *Result) { result.Actions[0].Entry.CarbsG = nil }},
		{"food unknown fat", func(result *Result) { result.Actions[0].Entry.FatG = nil }},
		{"food with duration", func(result *Result) { result.Actions[0].Entry.DurationMinutes = ptr(1.0) }},
		{"food with distance", func(result *Result) { result.Actions[0].Entry.DistanceKM = ptr(1.0) }},
		{"activity without measurement", func(result *Result) {
			result.Actions[0].Entry = &Values{Kind: "activity", Label: "Gehen", Source: "user"}
		}},
		{"activity with food macros", func(result *Result) {
			result.Actions[0].Entry = &Values{Kind: "activity", Label: "Gehen", DurationMinutes: ptr(30.0), ProteinG: ptr(0.0), Source: "user"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := recorded()
			test.change(&result)
			if err := Validate(result, testInput()); !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestNumericLimits(t *testing.T) {
	tests := []struct {
		name    string
		maximum float64
		assign  func(*Values, *float64)
	}{
		{"calories", 20000, func(value *Values, number *float64) { value.Calories = number }},
		{"protein", 2000, func(value *Values, number *float64) { value.ProteinG = number }},
		{"carbs", 2000, func(value *Values, number *float64) { value.CarbsG = number }},
		{"fat", 2000, func(value *Values, number *float64) { value.FatG = number }},
		{"duration", 1440, func(value *Values, number *float64) {
			*value = Values{Kind: "activity", Label: "Gehen", Source: "user", DurationMinutes: number}
		}},
		{"distance", 500, func(value *Values, number *float64) {
			*value = Values{Kind: "activity", Label: "Gehen", Source: "user", DistanceKM: number}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, number := range []float64{0, test.maximum} {
				result := recorded()
				test.assign(result.Actions[0].Entry, &number)
				if err := Validate(result, testInput()); err != nil {
					t.Errorf("boundary %g rejected: %v", number, err)
				}
			}
			for _, number := range []float64{-0.1, test.maximum + 0.1, math.NaN(), math.Inf(1), math.Inf(-1)} {
				result := recorded()
				test.assign(result.Actions[0].Entry, &number)
				if err := Validate(result, testInput()); !errors.Is(err, ErrInvalidResult) {
					t.Errorf("invalid number %g accepted", number)
				}
			}
		})
	}
}
