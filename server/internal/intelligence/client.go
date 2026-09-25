package intelligence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEndpoint  = "https://api.openai.com/v1/responses"
	maxResponseBytes = 256 * 1024
	maxInputBytes    = 256 * 1024
	maxImageBytes    = 8 * 1024 * 1024
	maxImages        = 4
)

type Client struct {
	HTTPClient *http.Client
	Endpoint   string
	apiKey     string
	model      string
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey: strings.TrimSpace(apiKey), model: strings.TrimSpace(model),
		Endpoint: defaultEndpoint, HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (client *Client) Analyze(ctx context.Context, input Input) (Result, error) {
	if client.apiKey == "" || client.model == "" {
		return Result{}, ErrConfiguration
	}
	if !validInput(input) {
		return Result{}, ErrInvalidInput
	}
	data, err := json.Marshal(input)
	if err != nil || len(data) > maxInputBytes {
		return Result{}, ErrInvalidInput
	}
	content := []map[string]string{{"type": "input_text", "text": string(data)}}
	for _, image := range input.Images {
		// The JSON label binds each image to its upload and originating message.
		// Data is excluded by Image's JSON tag and appears only in input_image.
		var evidenceRef *string
		if image.MessageID == input.Message.ID {
			ref := "image:" + image.ID
			evidenceRef = &ref
		}
		label, _ := json.Marshal(struct {
			Image
			EvidenceRef *string `json:"evidence_ref"`
		}{Image: image, EvidenceRef: evidenceRef})
		content = append(content,
			map[string]string{"type": "input_text", "text": "Bildmetadaten: " + string(label)},
			map[string]string{"type": "input_image", "image_url": "data:" + image.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(image.Data), "detail": "high"},
		)
	}
	body, err := json.Marshal(map[string]any{
		"model": client.model, "instructions": instructions,
		"input": []map[string]any{{"role": "user", "content": content}},
		"store": false, "max_output_tokens": 8192, "reasoning": map[string]string{"effort": "low"},
		"text": map[string]any{"format": map[string]any{
			"type": "json_schema", "name": "fitty_daily_tracking", "strict": true, "schema": resultSchema(),
		}},
	})
	if err != nil {
		return Result{}, ErrConfiguration
	}
	endpoint := client.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, ErrConfiguration
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	// A redirect is not a valid Responses API reply. Never forward credentials or
	// personal context to a location provided by an upstream response.
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	configured := *httpClient
	configured.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := configured.Do(request)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, ErrUnavailable
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if len(data) > maxResponseBytes {
		return Result{}, ErrInvalidResult
	}
	result, err := parseResponse(data)
	if err != nil {
		return Result{}, err
	}
	// Keep technical evidence identifiers in the structured action, not the
	// user-facing answer. Only known image references are replaced.
	for index, image := range input.Images {
		label := "Foto " + strconv.Itoa(index+1)
		result.Reply = strings.ReplaceAll(result.Reply, "image:"+image.ID, label)
		result.Reply = strings.ReplaceAll(result.Reply, "Bild:"+image.ID, label)
	}
	if err := Validate(result, input); err != nil {
		return Result{}, err
	}
	return result, nil
}

type responseEnvelope struct {
	Status string          `json:"status"`
	Error  json.RawMessage `json:"error"`
	Output []struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Status  string `json:"status"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func parseResponse(data []byte) (Result, error) {
	var response responseEnvelope
	if uniqueJSON(data) != nil || json.Unmarshal(data, &response) != nil {
		return Result{}, ErrInvalidResult
	}
	if response.Status != "completed" {
		return Result{}, ErrIncomplete
	}
	if len(response.Error) != 0 && !bytes.Equal(response.Error, []byte("null")) {
		return Result{}, ErrUnavailable
	}
	var output string
	for _, item := range response.Output {
		if item.Type == "reasoning" {
			continue
		}
		if item.Type != "message" || item.Role != "assistant" || item.Status != "completed" {
			return Result{}, ErrInvalidResult
		}
		for _, content := range item.Content {
			if content.Type == "refusal" {
				return Result{}, ErrRefused
			}
			// One final structured result is expected. Reject unexpected additional
			// output rather than arbitrarily choosing which result to persist.
			if content.Type != "output_text" || content.Text == "" || output != "" {
				return Result{}, ErrInvalidResult
			}
			output = content.Text
		}
	}
	if output == "" {
		return Result{}, ErrInvalidResult
	}
	return decodeResult([]byte(output))
}
