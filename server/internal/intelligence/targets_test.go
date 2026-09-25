package intelligence

import (
	"math"
	"testing"
)

func TestDailyTargetsValidation(t *testing.T) {
	if !ValidDailyTargets(DailyTargets{}) {
		t.Fatal("unset targets must be valid")
	}
	for _, test := range []struct {
		value float64
		valid bool
	}{
		{0, false}, {-1, false}, {0.0000000001, false}, {0.01, true}, {123.45, true}, {12.345, false}, {2000, true}, {2000.01, false}, {math.NaN(), false}, {math.Inf(1), false},
	} {
		if got := ValidDailyTargets(DailyTargets{ProteinG: &test.value}); got != test.valid {
			t.Errorf("protein target %v: %v", test.value, got)
		}
	}
	for _, value := range []float64{0.01, 2000.01, 20000} {
		if !ValidDailyTargets(DailyTargets{Calories: &value}) {
			t.Errorf("valid calories rejected: %v", value)
		}
	}
	value := 20000.01
	if ValidDailyTargets(DailyTargets{Calories: &value}) {
		t.Fatal("calorie bound not applied")
	}
}
