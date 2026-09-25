package intelligence

import (
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Validate checks the complete result before a caller may persist any action.
// It checks data and references; the meaning of natural language remains a model
// decision and is not established by a quoted evidence fragment alone.
func Validate(result Result, input Input) error {
	if !textWithin(result.Reply, 1, 8000) || result.Actions == nil || len(result.Actions) > 20 {
		return ErrInvalidResult
	}
	switch result.Intent {
	case "record", "correction", "mixed":
	case "advice", "clarification":
		if len(result.Actions) != 0 {
			return ErrInvalidResult
		}
	default:
		return ErrInvalidResult
	}

	existing := make(map[string]Entry, len(input.Entries))
	for _, entry := range input.Entries {
		existing[entry.ID] = entry
	}
	changed := make(map[string]bool)
	for _, action := range result.Actions {
		if !validEvidence(action.Evidence, input) {
			return ErrInvalidResult
		}
		switch action.Operation {
		case "create":
			if action.EntryID != nil || action.Entry == nil {
				return ErrInvalidResult
			}
		case "update", "delete":
			if action.EntryID == nil || changed[*action.EntryID] {
				return ErrInvalidResult
			}
			entry, ok := existing[*action.EntryID]
			if !ok || *action.EntryID == "" {
				return ErrInvalidResult
			}
			changed[*action.EntryID] = true
			if action.Operation == "delete" {
				if action.Entry != nil {
					return ErrInvalidResult
				}
			} else if action.Entry == nil || action.Entry.Kind != entry.Kind {
				return ErrInvalidResult
			}
		default:
			return ErrInvalidResult
		}
		if action.Entry != nil && !ValidValues(*action.Entry) {
			return ErrInvalidResult
		}
	}
	return nil
}

func validEvidence(evidence string, input Input) bool {
	if !textWithin(evidence, 1, 1000) {
		return false
	}
	if strings.HasPrefix(evidence, "image:") || strings.HasPrefix(evidence, "Bild:") {
		for _, image := range input.Images {
			if image.ID != "" && image.MessageID == input.Message.ID &&
				(evidence == "image:"+image.ID || evidence == "Bild:"+image.ID) {
				return true
			}
		}
		return false
	}
	return strings.Contains(input.Message.Content, evidence)
}

// ValidValues checks the shared domain limits for AI and direct user changes.
func ValidValues(value Values) bool {
	if !textWithin(value.Label, 1, 160) || !textWithin(value.Amount, 0, 160) || !textWithin(value.Notes, 0, 1000) {
		return false
	}
	if value.Source != "estimate" && value.Source != "user" && value.Source != "device" {
		return false
	}
	if !validNumber(value.Calories, 20000) || !validNumber(value.ProteinG, 2000) ||
		!validNumber(value.CarbsG, 2000) || !validNumber(value.FatG, 2000) ||
		!validNumber(value.DurationMinutes, 1440) || !validNumber(value.DistanceKM, 500) {
		return false
	}
	switch value.Kind {
	case "food":
		return value.Calories != nil && value.ProteinG != nil && value.CarbsG != nil && value.FatG != nil &&
			value.DurationMinutes == nil && value.DistanceKM == nil
	case "activity":
		return value.ProteinG == nil && value.CarbsG == nil && value.FatG == nil &&
			(value.DurationMinutes != nil || value.DistanceKM != nil)
	default:
		return false
	}
}

func validNumber(value *float64, maximum float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0 && *value <= maximum)
}

func textWithin(value string, minimum, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum &&
		utf8.RuneCountInString(strings.TrimSpace(value)) >= minimum
}

func validInput(input Input) bool {
	date, err := time.Parse(time.DateOnly, input.Date)
	if err != nil || date.Format(time.DateOnly) != input.Date || !ValidDailyTargets(input.DailyTargets) || input.Message.Role != "user" ||
		!validImages(input) {
		return false
	}
	minimum := 1
	if len(input.Images) > 0 && input.Images[0].MessageID == input.Message.ID {
		minimum = 0
	}
	if !textWithin(input.Message.Content, minimum, 8000) {
		return false
	}
	// The application supplies bounded daily context. Refuse ambiguous references.
	ids := make(map[string]bool, len(input.Entries))
	for _, entry := range input.Entries {
		if entry.ID == "" || ids[entry.ID] || !ValidValues(entry.Values) {
			return false
		}
		ids[entry.ID] = true
	}
	for _, message := range input.History {
		// Earlier image-only user messages remain in history even when their
		// image bytes are no longer included in this bounded request.
		minimum := 1
		if message.Role == "user" {
			minimum = 0
		}
		if (message.Role != "user" && message.Role != "assistant") || !textWithin(message.Content, minimum, 8000) {
			return false
		}
	}
	return true
}

func validImages(input Input) bool {
	if len(input.Images) > maxImages {
		return false
	}
	ids := make(map[string]bool, len(input.Images))
	for _, image := range input.Images {
		if !textWithin(image.ID, 1, 128) || !textWithin(image.MessageID, 1, 128) || ids[image.ID] ||
			image.MessageID != input.Images[0].MessageID || len(image.Data) == 0 || len(image.Data) > maxImageBytes {
			return false
		}
		ids[image.ID] = true
		if image.MIMEType != "image/jpeg" && image.MIMEType != "image/png" && image.MIMEType != "image/webp" {
			return false
		}
		if http.DetectContentType(image.Data) != image.MIMEType {
			return false
		}
		if image.MessageID == input.Message.ID {
			continue
		}
		found := false
		for _, message := range input.History {
			if message.ID == image.MessageID && message.Role == "user" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
