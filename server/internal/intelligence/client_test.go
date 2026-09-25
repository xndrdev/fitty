package intelligence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func fakeClient(t *testing.T, status int, body string, inspect func(*http.Request)) *Client {
	t.Helper()
	client := NewClient("test-key", "gpt-5-mini")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if inspect != nil {
			inspect(request)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	return client
}

func envelope(t *testing.T, result string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"status": "completed", "error": nil,
		"output": []any{
			map[string]any{"type": "reasoning", "summary": []any{}},
			map[string]any{
				"type": "message", "status": "completed", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": result, "annotations": []any{}}},
			},
		},
		"usage": map[string]int{"total_tokens": 123},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func encoded(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testInput() Input {
	return Input{
		Date: "2026-09-16", Profile: Profile{DisplayName: "Test", TimeZone: "Europe/Berlin", Preferences: "vegetarisch"},
		Message: Message{ID: "message-2", Role: "user", Content: "Ich habe einen Apfel gegessen."},
		History: []Message{{ID: "message-1", Role: "user", Content: "Zum Frühstück gab es Porridge."}},
		Entries: []Entry{{ID: "entry-1", Values: food()}},
	}
}

func TestAnalyzeRequestAndValidatedResult(t *testing.T) {
	input := testInput()
	calories, protein := 2100.25, 130.5
	input.DailyTargets = DailyTargets{Calories: &calories, ProteinG: &protein}
	want := recorded()
	client := fakeClient(t, http.StatusOK, envelope(t, encoded(t, want)), func(request *http.Request) {
		if request.URL.String() != defaultEndpoint || request.Method != http.MethodPost {
			t.Errorf("unexpected endpoint or method: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" || request.Header.Get("Content-Type") != "application/json" {
			t.Error("missing authentication or JSON header")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "gpt-5-mini" || body["store"] != false || body["max_output_tokens"] != float64(8192) {
			t.Error("model, retention or output budget configuration incorrect")
		}
		if body["reasoning"].(map[string]any)["effort"] != "low" {
			t.Error("unexpected reasoning configuration")
		}
		for _, name := range []string{"tools", "previous_response_id", "conversation", "metadata"} {
			if _, exists := body[name]; exists {
				t.Errorf("unexpected request property %s", name)
			}
		}
		messages := body["input"].([]any)
		if len(messages) != 1 || messages[0].(map[string]any)["role"] != "user" {
			t.Fatal("daily context must be one data message")
		}
		var supplied Input
		content := messages[0].(map[string]any)["content"].([]any)
		if len(content) != 1 || content[0].(map[string]any)["type"] != "input_text" {
			t.Fatal("text-only context must contain one input_text part")
		}
		if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &supplied); err != nil || !reflect.DeepEqual(supplied, input) {
			t.Error("current message, history, profile or tracked entries were not preserved")
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if format["type"] != "json_schema" || format["strict"] != true {
			t.Error("strict structured output is required")
		}
		assertStrictSchema(t, format["schema"].(map[string]any))
		prompt := body["instructions"].(string)
		for _, rule := range []string{"ausschließlich die aktuelle message", "abgeschlossene Bewegung", "Restaurantfragen", "passenden Tag auszuwählen", "keine persönlichen Kalorienziele", "ohne Diagnosen", "wörtlicher"} {
			if !strings.Contains(prompt, rule) {
				t.Errorf("missing instruction: %s", rule)
			}
		}
	})
	got, err := client.Analyze(context.Background(), input)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, %v; want %#v", got, err, want)
	}
}

func assertStrictSchema(t *testing.T, schema map[string]any) {
	t.Helper()
	if schema["type"] == "object" {
		properties := schema["properties"].(map[string]any)
		required := schema["required"].([]any)
		if schema["additionalProperties"] != false || len(required) != len(properties) {
			t.Fatal("every object must disallow unknown fields and require all properties")
		}
		for _, name := range required {
			if _, exists := properties[name.(string)]; !exists {
				t.Fatal("required property missing from schema")
			}
		}
		for _, property := range properties {
			assertStrictSchema(t, property.(map[string]any))
		}
	}
	if items, exists := schema["items"]; exists {
		assertStrictSchema(t, items.(map[string]any))
	}
	if options, exists := schema["anyOf"]; exists {
		for _, option := range options.([]any) {
			assertStrictSchema(t, option.(map[string]any))
		}
	}
}

func TestAnalyzeRejectsProviderFailuresWithoutReturningActions(t *testing.T) {
	valid := envelope(t, encoded(t, recorded()))
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"HTTP failure", 429, `{"error":{"message":"private-provider-details"}}`, ErrUnavailable},
		{"HTML response", 200, "<html>private-provider-details</html>", ErrInvalidResult},
		{"oversize response", 200, strings.Repeat(" ", maxResponseBytes+1), ErrInvalidResult},
		{"truncated response", 200, valid[:len(valid)-10], ErrInvalidResult},
		{"trailing response", 200, valid + `{}`, ErrInvalidResult},
		{"incomplete", 200, strings.Replace(valid, `"status":"completed"`, `"status":"incomplete"`, -1), ErrIncomplete},
		{"response missing status", 200, `{"output":[]}`, ErrIncomplete},
		{"completed with error", 200, strings.Replace(valid, `"error":null`, `"error":{"message":"private-provider-details"}`, 1), ErrUnavailable},
		{"refusal", 200, `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"refusal","refusal":"private-provider-details"}]}]}`, ErrRefused},
		{"no message", 200, `{"status":"completed","output":[{"type":"reasoning"}]}`, ErrInvalidResult},
		{"unexpected tool", 200, `{"status":"completed","output":[{"type":"function_call"}]}`, ErrInvalidResult},
		{"wrong role", 200, strings.Replace(valid, `"role":"assistant"`, `"role":"user"`, 1), ErrInvalidResult},
		{"message incomplete", 200, strings.Replace(valid, `"role":"assistant","status":"completed"`, `"role":"assistant","status":"incomplete"`, 1), ErrInvalidResult},
		{"malformed result", 200, envelope(t, `{"reply":"private-provider-details"}`), ErrInvalidResult},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := fakeClient(t, test.status, test.body, nil).Analyze(context.Background(), testInput())
			if !errors.Is(err, test.want) || !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("got %#v, %v; want no result and %v", got, err, test.want)
			}
			if strings.Contains(err.Error(), "private-provider-details") {
				t.Error("provider data leaked through error")
			}
		})
	}
}

func TestAnalyzeAdviceKeepsPlannedMealUntracked(t *testing.T) {
	input := testInput()
	input.Message.Content = "Ich gehe heute Abend ins Restaurant. Worauf soll ich achten?"
	want := Result{Reply: "Achte auf eine sättigende Mahlzeit mit Gemüse und einer Proteinquelle.", Intent: "advice", Actions: []Action{}}
	got, err := fakeClient(t, 200, envelope(t, encoded(t, want)), nil).Analyze(context.Background(), input)
	if err != nil || len(got.Actions) != 0 || got.Intent != "advice" {
		t.Fatalf("planned meal was not handled as advice: %#v, %v", got, err)
	}
	want.Actions = []Action{{Operation: "create", Entry: ptr(food()), Evidence: "heute Abend ins Restaurant"}}
	if _, err := fakeClient(t, 200, envelope(t, encoded(t, want)), nil).Analyze(context.Background(), input); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("advice with a booking must be rejected: %v", err)
	}
}

func TestAnalyzeRejectsInvalidInputBeforeRequest(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Input)
	}{
		{"wrong date", func(input *Input) { input.Date = "2026-02-30" }},
		{"wrong role", func(input *Input) { input.Message.Role = "assistant" }},
		{"empty message", func(input *Input) { input.Message.Content = " " }},
		{"long message", func(input *Input) { input.Message.Content = strings.Repeat("a", 8001) }},
		{"duplicate entry IDs", func(input *Input) { input.Entries = append(input.Entries, input.Entries[0]) }},
		{"invalid history role", func(input *Input) { input.History[0].Role = "system" }},
		{"oversize context", func(input *Input) { input.Profile.Goals = strings.Repeat("a", maxInputBytes) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := testInput()
			test.change(&input)
			client := fakeClient(t, 200, "", func(_ *http.Request) { t.Error("must not call provider") })
			if _, err := client.Analyze(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestAnalyzeConfigurationAndTransportErrors(t *testing.T) {
	client := NewClient("", "gpt-5-mini")
	if _, err := client.Analyze(context.Background(), testInput()); !errors.Is(err, ErrConfiguration) {
		t.Fatalf("missing key: %v", err)
	}
	client = NewClient("test-key", "")
	if _, err := client.Analyze(context.Background(), testInput()); !errors.Is(err, ErrConfiguration) {
		t.Fatalf("missing model: %v", err)
	}
	client = NewClient("test-key", "gpt-5-mini")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("private transport detail")
	})}
	if _, err := client.Analyze(context.Background(), testInput()); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "private") {
		t.Fatalf("transport error must be sanitized: %v", err)
	}
}

func TestAnalyzeDoesNotFollowRedirects(t *testing.T) {
	client := NewClient("test-key", "gpt-5-mini")
	calls := 0
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {"https://redirect.example.test/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	if _, err := client.Analyze(context.Background(), testInput()); !errors.Is(err, ErrUnavailable) || calls != 1 {
		t.Fatalf("redirect followed or unexpected error: calls=%d err=%v", calls, err)
	}
}
