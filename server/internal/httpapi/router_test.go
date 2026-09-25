package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserOrigins(t *testing.T) {
	handler := New([]string{" http://localhost:8081 ", "*", "null", ""}, nil)

	for _, test := range []struct {
		name   string
		origin string
		status int
		allow  string
	}{
		{"native request", "", http.StatusOK, ""},
		{"configured web app", "http://localhost:8081", http.StatusOK, "http://localhost:8081"},
		{"other origin", "https://example.com", http.StatusForbidden, ""},
		{"similar hostname", "http://localhost:8081.example.com", http.StatusForbidden, ""},
		{"opaque origin", "null", http.StatusForbidden, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)

			if response.Code != test.status {
				t.Errorf("status = %d, want %d", response.Code, test.status)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.allow {
				t.Errorf("allowed origin = %q, want %q", got, test.allow)
			}
		})
	}
}
