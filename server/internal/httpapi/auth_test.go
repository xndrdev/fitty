package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthentication(t *testing.T) {
	const user = "5bc70572-cd76-44e6-b123-129bc35b8d51"
	for _, tc := range []struct {
		name, header, body string
		status             int
		wantErr            error
	}{
		{"verified account", "Bearer test-session", `{"id":"` + user + `"}`, 200, nil},
		{"missing session", "", "", 200, errUnauthorized},
		{"expired session", "Bearer expired", "{}", 401, errUnauthorized},
		{"auth unavailable", "Bearer test-session", "{}", 503, errAuthUnavailable},
		{"invalid auth response", "Bearer test-session", `{"id":""}`, 200, errAuthUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &Auth{URL: "https://auth.example.test", Key: "public-test-key", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/auth/v1/user" || r.Header.Get("apikey") != "public-test-key" || r.Header.Get("Authorization") != tc.header {
					t.Fatal("authentication request not forwarded correctly")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			got, err := auth.user(context.Background(), tc.header)
			if err != tc.wantErr {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if err == nil && got != user {
				t.Fatalf("wrong user: %s", got)
			}
		})
	}
}

func TestCalendarDates(t *testing.T) {
	for date, want := range map[string]bool{"2026-09-16": true, "2024-02-29": true, "2026-02-29": false, "2026-2-01": false, "2026-13-01": false, "1899-12-31": false, "": false} {
		if got := validDate(date); got != want {
			t.Errorf("validDate(%q)=%v, want %v", date, got, want)
		}
	}
}
