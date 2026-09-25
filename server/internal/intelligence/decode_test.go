package intelligence

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeRequiresEveryFieldAtEveryLevel(t *testing.T) {
	for _, level := range []string{"result", "action", "entry"} {
		fields := resultFields
		if level == "action" {
			fields = actionFields
		} else if level == "entry" {
			fields = valueFields
		}
		for _, field := range fields {
			t.Run(level+"/"+field, func(t *testing.T) {
				var root map[string]any
				if err := json.Unmarshal([]byte(encoded(t, recorded())), &root); err != nil {
					t.Fatal(err)
				}
				object := root
				if level != "result" {
					object = root["actions"].([]any)[0].(map[string]any)
				}
				if level == "entry" {
					object = object["entry"].(map[string]any)
				}
				delete(object, field)
				if _, err := decodeResult([]byte(encoded(t, root))); !errors.Is(err, ErrInvalidResult) {
					t.Fatalf("missing field was accepted: %v", err)
				}
			})
		}
	}
}

func TestDecodeRejectsAmbiguousOrMalformedJSON(t *testing.T) {
	valid := encoded(t, recorded())
	tests := map[string]string{
		"trailing JSON":            valid + `{}`,
		"trailing text":            valid + `more`,
		"truncated JSON":           valid[:len(valid)-2],
		"duplicate root field":     strings.Replace(valid, `"intent":"record"`, `"intent":"advice","intent":"record"`, 1),
		"duplicate nested field":   strings.Replace(valid, `"kind":"food"`, `"kind":"activity","kind":"food"`, 1),
		"unknown root field":       strings.Replace(valid, `"intent":"record"`, `"extra":false,"intent":"record"`, 1),
		"unknown nested field":     strings.Replace(valid, `"kind":"food"`, `"extra":false,"kind":"food"`, 1),
		"wrong casing":             strings.Replace(valid, `"intent"`, `"Intent"`, 1),
		"null actions":             `{"reply":"Text","intent":"advice","actions":null}`,
		"null action":              `{"reply":"Text","intent":"record","actions":[null]}`,
		"null nonnullable reply":   `{"reply":null,"intent":"advice","actions":[]}`,
		"null nonnullable string":  strings.Replace(valid, `"notes":"Geschätzt für etwa 180 g."`, `"notes":null`, 1),
		"incorrect numeric type":   strings.Replace(valid, `"calories":95`, `"calories":"95"`, 1),
		"overflow numeric value":   strings.Replace(valid, `"calories":95`, `"calories":1e999`, 1),
		"wrong entry id type":      strings.Replace(valid, `"entry_id":null`, `"entry_id":12`, 1),
		"wrong entry type":         strings.Replace(valid, `"entry":{`, `"entry":[{`, 1),
		"unsupported code fencing": "```json\n" + valid + "\n```",
		"excessive JSON nesting":   strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if result, err := decodeResult([]byte(data)); !errors.Is(err, ErrInvalidResult) || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("ambiguous response accepted: %#v, %v", result, err)
			}
		})
	}
}

func TestDecodePreservesExplicitNullAndEmptyValues(t *testing.T) {
	want := Result{Reply: "Welchen Eintrag meinst du?", Intent: "clarification", Actions: []Action{}}
	got, err := decodeResult([]byte(encoded(t, want)))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("empty action array changed: %#v, %v", got, err)
	}
	want = Result{Reply: "Eintrag entfernt.", Intent: "correction", Actions: []Action{{Operation: "delete", EntryID: ptr("entry-1"), Entry: nil, Evidence: "Apfel"}}}
	got, err = decodeResult([]byte(encoded(t, want)))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit null entry changed: %#v, %v", got, err)
	}
}
