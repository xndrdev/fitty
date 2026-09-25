package intelligence

import "math"

// DailyTargets are explicit user settings, never inferred consumption or a
// computed recommendation. A nil value leaves that metric without a target.
type DailyTargets struct {
	Calories *float64 `json:"calories"`
	ProteinG *float64 `json:"protein_g"`
	CarbsG   *float64 `json:"carbs_g"`
	FatG     *float64 `json:"fat_g"`
}

func ValidDailyTargets(targets DailyTargets) bool {
	for index, value := range []*float64{targets.Calories, targets.ProteinG, targets.CarbsG, targets.FatG} {
		maximum := 2000.0
		if index == 0 {
			maximum = 20000
		}
		if value != nil && (!validNumber(value, maximum) || *value < 0.01 || math.Abs(*value*100-math.Round(*value*100)) > 0.000001) {
			return false
		}
	}
	return true
}
