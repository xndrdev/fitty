package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const testObjectPath = "45e57230-a4f0-41dc-974d-042fc07c7bf0/8a6b16ad-23b0-40b3-b034-8aabec33cd06.jpg"
const testSignedPath = "/object/sign/chat-attachments/" + testObjectPath

func TestClientUsesPrivateBucketAndOperationContracts(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0xff, 0xd9}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer private-test-key" || r.Header.Get("apikey") != "private-test-key" || r.Header.Get("Cache-Control") != "no-store" {
			t.Error("private server credentials or no-store header missing")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/storage/v1/object/chat-attachments/"+testObjectPath:
			if r.Header.Get("Content-Type") != "image/jpeg" || r.Header.Get("x-upsert") != "true" || !bytes.Equal(body, image) {
				t.Error("upload must send the exact JPEG with upsert enabled")
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Key":"chat-attachments/test"}`))
		case r.Method == "GET" && r.URL.Path == "/storage/v1/object/chat-attachments/"+testObjectPath:
			if len(body) != 0 {
				t.Error("image read must not send a request body")
			}
			_, _ = w.Write(image)
		case r.Method == "DELETE" && r.URL.Path == "/storage/v1/object/chat-attachments":
			var request struct {
				Prefixes []string `json:"prefixes"`
			}
			if r.Header.Get("Content-Type") != "application/json" || json.Unmarshal(body, &request) != nil || len(request.Prefixes) != 1 || request.Prefixes[0] != testObjectPath {
				t.Error("delete must name only the requested private object")
			}
			_, _ = w.Write([]byte(`[]`))
		case r.Method == "POST" && r.URL.Path == "/storage/v1"+testSignedPath:
			var request struct {
				ExpiresIn int `json:"expiresIn"`
			}
			if r.Header.Get("Content-Type") != "application/json" || json.Unmarshal(body, &request) != nil || request.ExpiresIn != 600 {
				t.Error("signed previews must expire after 600 seconds")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"signedURL": testSignedPath + "?token=preview-token"})
		default:
			t.Errorf("unexpected storage request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := New(server.URL+"/", "private-test-key")
	ctx := context.Background()
	if err := client.Put(ctx, testObjectPath, image); err != nil {
		t.Fatal(err)
	}
	got, err := client.Get(ctx, testObjectPath)
	if err != nil || !bytes.Equal(got, image) {
		t.Fatalf("image read changed data: %v", err)
	}
	if err := client.Delete(ctx, testObjectPath); err != nil {
		t.Fatal(err)
	}
	url, err := client.Sign(ctx, testObjectPath)
	if err != nil || url != "/storage/v1"+testSignedPath+"?token=preview-token" {
		t.Fatalf("wrong preview URL: %q, %v", url, err)
	}
	if calls.Load() != 4 {
		t.Fatalf("unexpected request count %d", calls.Load())
	}
}

func storageOperations() map[string]func(*Client, context.Context, string) error {
	return map[string]func(*Client, context.Context, string) error{
		"put":    func(c *Client, ctx context.Context, path string) error { return c.Put(ctx, path, []byte("jpeg-data")) },
		"get":    func(c *Client, ctx context.Context, path string) error { _, err := c.Get(ctx, path); return err },
		"delete": func(c *Client, ctx context.Context, path string) error { return c.Delete(ctx, path) },
		"sign":   func(c *Client, ctx context.Context, path string) error { _, err := c.Sign(ctx, path); return err },
	}
}

func TestClientRejectsInvalidPathsAndUploadSizesBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := New(server.URL, "private-test-key")
	for name, operation := range storageOperations() {
		for _, path := range []string{"", "photo.jpg", "/" + testObjectPath, "../" + testObjectPath, strings.Replace(testObjectPath, ".jpg", ".png", 1), testObjectPath + "?token=other"} {
			if err := operation(client, context.Background(), path); !errors.Is(err, ErrUnavailable) {
				t.Errorf("%s accepted invalid object path %q: %v", name, path, err)
			}
		}
	}
	for _, data := range [][]byte{nil, {}, make([]byte, MaxBytes+1)} {
		if err := client.Put(context.Background(), testObjectPath, data); !errors.Is(err, ErrUnavailable) {
			t.Errorf("accepted invalid upload size %d: %v", len(data), err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid inputs reached storage")
	}
}

func TestClientSanitizesStorageFailures(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"private storage diagnostics"}`))
		}))
		client := New(server.URL, "private-test-key")
		for name, operation := range storageOperations() {
			if err := operation(client, context.Background(), testObjectPath); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "diagnostics") {
				t.Errorf("%s status %d should return only the safe error: %v", name, status, err)
			}
		}
		server.Close()
	}
}

func TestClientDoesNotFollowStorageRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client := New(server.URL, "private-test-key")
	for name, operation := range storageOperations() {
		if err := operation(client, context.Background(), testObjectPath); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s accepted a redirect: %v", name, err)
		}
	}
	if redirected.Load() != 0 {
		t.Fatal("storage followed a redirect and exposed the private request")
	}
}

func TestClientBoundsResponseBodies(t *testing.T) {
	for name, operation := range storageOperations() {
		t.Run(name, func(t *testing.T) {
			limit := 64 << 10
			if name == "get" {
				limit = MaxBytes
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(bytes.Repeat([]byte("x"), limit+1))
			}))
			defer server.Close()
			if err := operation(New(server.URL, "private-test-key"), context.Background(), testObjectPath); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("oversize response should be refused: %v", err)
			}
		})
	}
}

func TestClientAcceptsExactImageByteLimit(t *testing.T) {
	data := bytes.Repeat([]byte{0x5a}, MaxBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			received, err := io.ReadAll(r.Body)
			if err != nil || !bytes.Equal(received, data) {
				t.Error("maximum upload was truncated")
			}
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := New(server.URL, "private-test-key")
	if err := client.Put(context.Background(), testObjectPath, data); err != nil {
		t.Fatal(err)
	}
	received, err := client.Get(context.Background(), testObjectPath)
	if err != nil || !bytes.Equal(received, data) {
		t.Fatalf("maximum image read failed: length=%d err=%v", len(received), err)
	}
}

func TestClientRejectsInterruptedResponsesAndCanceledRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()
	client := New(server.URL, "private-test-key")
	for name, operation := range storageOperations() {
		if err := operation(client, context.Background(), testObjectPath); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s accepted an incomplete response: %v", name, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := operation(client, ctx, testObjectPath); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s accepted a canceled request: %v", name, err)
		}
	}
}

func TestSignedURLMustReferToExpectedPrivateObject(t *testing.T) {
	for _, signed := range []string{
		"", testSignedPath, testSignedPath + "?token=", "https://storage.example.test" + testSignedPath + "?token=preview",
		"//storage.example.test" + testSignedPath + "?token=preview", "/object/public/chat-attachments/" + testObjectPath + "?token=preview",
		strings.Replace(testSignedPath, "chat-attachments", "another-bucket", 1) + "?token=preview",
		strings.Replace(testSignedPath, "8a6b16ad", "00000000", 1) + "?token=preview", testSignedPath + "/extra?token=preview",
		"%invalid?token=preview",
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"signedURL": signed})
		}))
		url, err := New(server.URL, "private-test-key").Sign(context.Background(), testObjectPath)
		server.Close()
		if !errors.Is(err, ErrUnavailable) || url != "" {
			t.Errorf("accepted unexpected signed URL %q: returned %q, %v", signed, url, err)
		}
	}
	for _, body := range []string{`{}`, `{"signedURL":false}`, `{"signedURL":`, `<html>storage error</html>`, `{"signedURL":"` + testSignedPath + `?token=preview"}{}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		url, err := New(server.URL, "private-test-key").Sign(context.Background(), testObjectPath)
		server.Close()
		if !errors.Is(err, ErrUnavailable) || url != "" {
			t.Errorf("accepted malformed sign response: returned %q, %v", url, err)
		}
	}
}
