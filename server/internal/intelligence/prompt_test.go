package intelligence

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestActionSchemaRequiresCompleteValuesForWrites(t *testing.T) {
	// Check the actual serialized provider contract, where each operation must
	// constrain its own ID and values rather than independently allowing null.
	var schema map[string]any
	if err := json.Unmarshal([]byte(encoded(t, resultSchema())), &schema); err != nil {
		t.Fatal(err)
	}
	if schema["type"] != "object" || schema["anyOf"] != nil {
		t.Fatal("the Responses structured-output root must remain an object")
	}
	assertStrictSchema(t, schema)
	items := schema["properties"].(map[string]any)["actions"].(map[string]any)["items"].(map[string]any)
	variants, ok := items["anyOf"].([]any)
	if !ok || len(variants) != 3 {
		t.Fatal("actions need separate create, update and delete contracts")
	}
	seen := make(map[string]bool)
	for _, variant := range variants {
		properties := variant.(map[string]any)["properties"].(map[string]any)
		operations := properties["operation"].(map[string]any)["enum"].([]any)
		if len(operations) != 1 {
			t.Fatal("each action variant must describe exactly one operation")
		}
		operation := operations[0].(string)
		if seen[operation] {
			t.Fatalf("duplicate operation %q", operation)
		}
		seen[operation] = true
		entryID := properties["entry_id"].(map[string]any)
		entry := properties["entry"].(map[string]any)
		switch operation {
		case "create":
			if !reflect.DeepEqual(entryID, map[string]any{"type": "null"}) {
				t.Fatal("create must forbid an existing entry ID")
			}
		case "update", "delete":
			if entryID["type"] != "string" || entryID["minLength"] != float64(1) {
				t.Fatal("update and delete must require a non-empty entry ID")
			}
		default:
			t.Fatalf("unexpected operation %q", operation)
		}
		if operation == "delete" {
			if !reflect.DeepEqual(entry, map[string]any{"type": "null"}) {
				t.Fatal("delete must not carry replacement values")
			}
			continue
		}
		if entry["type"] != "object" || entry["anyOf"] != nil || len(entry["required"].([]any)) != len(valueFields) {
			t.Fatal("create and update must require the complete values object, never null")
		}
		for _, name := range valueFields {
			if _, exists := entry["properties"].(map[string]any)[name]; !exists {
				t.Fatalf("write schema is missing entry field %s", name)
			}
		}
	}
}

func TestAnalyzeRejectsPhotoBookingsWithoutValues(t *testing.T) {
	input := testInput()
	input.Images = []Image{testImage()}
	// Regression for a real response: the model emitted one empty create per
	// text/image evidence. Parsing alone must never turn these into bookings.
	result := Result{
		Reply: "Deine Mahlzeit ist erfasst.", Intent: "record",
		Actions: []Action{
			{Operation: "create", Evidence: "einen Apfel gegessen"},
			{Operation: "create", Evidence: "Bild:photo-1"},
		},
	}
	client := fakeClient(t, 200, envelope(t, encoded(t, result)), nil)
	got, err := client.Analyze(context.Background(), input)
	if !errors.Is(err, ErrInvalidResult) || !reflect.DeepEqual(got, Result{}) {
		t.Fatalf("invalid photo bookings were accepted: %#v, %v", got, err)
	}

	result = recorded()
	result.Actions[0].Evidence = "image:photo-1"
	client = fakeClient(t, 200, envelope(t, encoded(t, result)), nil)
	got, err = client.Analyze(context.Background(), input)
	if err != nil || !reflect.DeepEqual(got, result) {
		t.Fatalf("one complete photo booking should remain valid: %#v, %v", got, err)
	}
}
