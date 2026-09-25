package intelligence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func testImage() Image {
	// A complete one-pixel PNG keeps the request tests independent of files.
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aLfoAAAAASUVORK5CYII=")
	return Image{ID: "photo-1", MessageID: "message-2", MIMEType: "image/png", Data: data}
}

func TestAnalyzeSendsImagesAsSeparateContentParts(t *testing.T) {
	for _, historical := range []bool{false, true} {
		t.Run(map[bool]string{false: "current photos", true: "historical follow-up"}[historical], func(t *testing.T) {
			input := testInput()
			input.Images = []Image{testImage(), testImage()}
			input.Images[1].ID = "photo-2"
			if historical {
				input.Images[0].MessageID = input.History[0].ID
				input.Images[1].MessageID = input.History[0].ID
			} else {
				input.Message.Content = ""
			}
			want := Result{Reply: "Wie viel hast du davon gegessen?", Intent: "clarification", Actions: []Action{}}
			client := fakeClient(t, 200, envelope(t, encoded(t, want)), func(request *http.Request) {
				var body struct {
					Store bool `json:"store"`
					Input []struct {
						Role    string              `json:"role"`
						Content []map[string]string `json:"content"`
					} `json:"input"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Store || len(body.Input) != 1 || body.Input[0].Role != "user" {
					t.Fatal("image context must remain one user message without storage")
				}
				parts := body.Input[0].Content
				if len(parts) != 5 || parts[0]["type"] != "input_text" {
					t.Fatal("expected metadata followed by a label and content for each image")
				}
				var supplied Input
				if err := json.Unmarshal([]byte(parts[0]["text"]), &supplied); err != nil {
					t.Fatal(err)
				}
				if len(supplied.Images) != len(input.Images) {
					t.Fatal("image metadata missing from context")
				}
				for index, photo := range input.Images {
					metadata := supplied.Images[index]
					if metadata.ID != photo.ID || metadata.MessageID != photo.MessageID || metadata.MIMEType != photo.MIMEType || metadata.Data != nil {
						t.Error("metadata must retain references without image bytes")
					}
					label, picture := parts[1+index*2], parts[2+index*2]
					var labeled struct {
						Image
						EvidenceRef *string `json:"evidence_ref"`
					}
					if label["type"] != "input_text" || !strings.HasPrefix(label["text"], "Bildmetadaten: ") ||
						json.Unmarshal([]byte(strings.TrimPrefix(label["text"], "Bildmetadaten: ")), &labeled) != nil || !reflect.DeepEqual(labeled.Image, metadata) {
						t.Error("each image must be labeled with its upload and message ID")
					}
					if historical {
						if labeled.EvidenceRef != nil {
							t.Error("historical images must not provide a direct booking reference")
						}
					} else if labeled.EvidenceRef == nil || *labeled.EvidenceRef != "image:"+photo.ID {
						t.Error("current image labels must provide the exact canonical reference to copy")
					}
					wantURL := "data:" + photo.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(photo.Data)
					if picture["type"] != "input_image" || picture["detail"] != "high" || picture["image_url"] != wantURL || len(picture) != 3 {
						t.Error("image bytes must use a high-detail Responses input_image data URL")
					}
				}
			})
			got, err := client.Analyze(context.Background(), input)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, want)
			}
		})
	}
}

func TestImageBytesDoNotConsumeTextContextBudget(t *testing.T) {
	input := testInput()
	input.Images = []Image{testImage()}
	input.Images[0].Data = append(input.Images[0].Data, make([]byte, maxInputBytes)...)
	got, err := fakeClient(t, 200, envelope(t, encoded(t, recorded())), nil).Analyze(context.Background(), input)
	if err != nil || len(got.Actions) != 1 {
		t.Fatalf("image bytes must have a separate size budget: %v", err)
	}
}

func TestAnalyzeKeepsImageEvidenceOutOfDisplayedReply(t *testing.T) {
	input := testInput()
	input.Images = []Image{testImage()}
	result := recorded()
	result.Reply = "Erfasst anhand image:photo-1 (Bild:photo-1)."
	result.Actions[0].Evidence = "image:photo-1"
	got, err := fakeClient(t, 200, envelope(t, encoded(t, result)), nil).Analyze(context.Background(), input)
	if err != nil || got.Reply != "Erfasst anhand Foto 1 (Foto 1)." || got.Actions[0].Evidence != result.Actions[0].Evidence {
		t.Fatalf("displayed image labels must retain the original structured evidence: %#v, %v", got, err)
	}
}

func TestAnalyzeRejectsInvalidImagesBeforeRequest(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Input)
	}{
		{"empty upload ID", func(input *Input) { input.Images[0].ID = "" }},
		{"long upload ID", func(input *Input) { input.Images[0].ID = strings.Repeat("a", 129) }},
		{"empty message ID", func(input *Input) { input.Images[0].MessageID = "" }},
		{"foreign message ID", func(input *Input) { input.Images[0].MessageID = "outside-this-chat" }},
		{"assistant origin", func(input *Input) {
			input.Images[0].MessageID = input.History[0].ID
			input.History[0].Role = "assistant"
		}},
		{"duplicate images", func(input *Input) { input.Images = append(input.Images, input.Images[0]) }},
		{"mixed origins", func(input *Input) {
			other := testImage()
			other.ID, other.MessageID = "photo-2", input.History[0].ID
			input.Images = append(input.Images, other)
		}},
		{"too many images", func(input *Input) { input.Images = make([]Image, maxImages+1) }},
		{"empty image data", func(input *Input) { input.Images[0].Data = nil }},
		{"oversize image", func(input *Input) {
			input.Images[0].Data = append(input.Images[0].Data, make([]byte, maxImageBytes)...)
		}},
		{"unsupported MIME type", func(input *Input) { input.Images[0].MIMEType = "image/svg+xml" }},
		{"MIME mismatch", func(input *Input) { input.Images[0].MIMEType = "image/jpeg" }},
		{"invalid image bytes", func(input *Input) { input.Images[0].Data = []byte("not an image") }},
		{"empty follow-up with historical image", func(input *Input) {
			input.Message.Content = ""
			input.Images[0].MessageID = input.History[0].ID
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := testInput()
			input.Images = []Image{testImage()}
			test.change(&input)
			client := fakeClient(t, 200, "", func(_ *http.Request) { t.Error("must not call provider") })
			if _, err := client.Analyze(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestValidImageInputBoundariesAndHistory(t *testing.T) {
	input := testInput()
	input.Message.Content = ""
	input.History[0].Content = "" // An earlier image-only message, omitted now.
	input.Images = []Image{testImage(), testImage(), testImage(), testImage()}
	for index := range input.Images {
		input.Images[index].ID = string(rune('a' + index))
	}
	input.Images[0].Data = append(input.Images[0].Data, make([]byte, maxImageBytes-len(input.Images[0].Data))...)
	if !validInput(input) {
		t.Fatal("four images of the current message and 8 MiB per image must be supported")
	}
	input.History[0].Role = "assistant"
	if validInput(input) {
		t.Fatal("an assistant reply must still contain text")
	}

	for mime, header := range map[string][]byte{
		"image/jpeg": {0xff, 0xd8, 0xff, 0xe0},
		"image/webp": []byte("RIFF\x00\x00\x00\x00WEBPVP8 "),
	} {
		input := testInput()
		input.Images = []Image{testImage()}
		input.Images[0].MIMEType, input.Images[0].Data = mime, header
		if !validInput(input) {
			t.Errorf("supported image type %s rejected", mime)
		}
	}
}

func TestImageEvidenceOnlyReferencesCurrentUploads(t *testing.T) {
	tests := []struct {
		name       string
		historical bool
		text       string
		evidence   string
		valid      bool
	}{
		{"current photo", false, "", "Bild:photo-1", true},
		{"current text", false, "Ich habe alles gegessen.", "alles gegessen", true},
		{"unknown photo", false, "", "Bild:unknown", false},
		{"quoted unknown token", false, "Bild:unknown", "Bild:unknown", false},
		{"partial photo ID", false, "", "Bild:photo", false},
		{"suffixed photo ID", false, "", "Bild:photo-1, Portion", false},
		{"historical photo", true, "Davon 150 g gegessen.", "Bild:photo-1", false},
		{"quoted historical token", true, "Bild:photo-1", "Bild:photo-1", false},
		{"follow-up evidence", true, "Davon 150 g gegessen.", "150 g gegessen", true},
	}
	for _, prefix := range []string{"image:", "Bild:"} {
		for _, test := range tests {
			t.Run(prefix+test.name, func(t *testing.T) {
				input := testInput()
				input.Message.Content = strings.ReplaceAll(test.text, "Bild:", prefix)
				input.Images = []Image{testImage()}
				if test.historical {
					input.Images[0].MessageID = input.History[0].ID
				}
				result := recorded()
				result.Actions[0].Evidence = strings.ReplaceAll(test.evidence, "Bild:", prefix)
				err := Validate(result, input)
				if (err == nil) != test.valid {
					t.Fatalf("valid=%v, got %v", test.valid, err)
				}
			})
		}
	}
}
