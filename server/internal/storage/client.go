// Package storage accesses only the private Fitty image bucket.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const MaxBytes = 8 << 20
const bucket = "chat-attachments"

var ErrUnavailable = errors.New("private image storage unavailable")
var pathPattern = regexp.MustCompile(`^[0-9a-f-]{36}/(?:progress/)?[0-9a-f-]{36}\.jpg$`)

type Store interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
	Sign(context.Context, string) (string, error)
}
type Client struct {
	URL, Key   string
	HTTPClient *http.Client
}

func New(base, key string) *Client {
	return &Client{URL: strings.TrimRight(base, "/") + "/storage/v1", Key: key, HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}
func (c *Client) request(ctx context.Context, method, path, contentType string, body []byte, limit int64) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, method, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, ErrUnavailable
	}
	r.Header.Set("Authorization", "Bearer "+c.Key)
	r.Header.Set("apikey", c.Key)
	r.Header.Set("Content-Type", contentType)
	r.Header.Set("x-upsert", "true")
	r.Header.Set("Cache-Control", "no-store")
	client := *c.HTTPClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(r)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrUnavailable
	}
	return data, nil
}
func (c *Client) Put(ctx context.Context, path string, data []byte) error {
	if !pathPattern.MatchString(path) || len(data) == 0 || len(data) > MaxBytes {
		return ErrUnavailable
	}
	_, err := c.request(ctx, "POST", "/object/"+bucket+"/"+path, "image/jpeg", data, 64<<10)
	return err
}
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	if !pathPattern.MatchString(path) {
		return nil, ErrUnavailable
	}
	return c.request(ctx, "GET", "/object/"+bucket+"/"+path, "application/json", nil, MaxBytes)
}
func (c *Client) Delete(ctx context.Context, path string) error {
	if !pathPattern.MatchString(path) {
		return ErrUnavailable
	}
	body, _ := json.Marshal(map[string]any{"prefixes": []string{path}})
	_, err := c.request(ctx, "DELETE", "/object/"+bucket, "application/json", body, 64<<10)
	return err
}
func (c *Client) Sign(ctx context.Context, path string) (string, error) {
	if !pathPattern.MatchString(path) {
		return "", ErrUnavailable
	}
	data, err := c.request(ctx, "POST", "/object/sign/"+bucket+"/"+path, "application/json", []byte(`{"expiresIn":600}`), 64<<10)
	if err != nil {
		return "", err
	}
	var result struct {
		SignedURL string `json:"signedURL"`
	}
	if json.Unmarshal(data, &result) != nil {
		return "", ErrUnavailable
	}
	u, err := url.Parse(result.SignedURL)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path != "/object/sign/"+bucket+"/"+path || u.Query().Get("token") == "" {
		return "", ErrUnavailable
	}
	return "/storage/v1" + result.SignedURL, nil
}
