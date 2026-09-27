package httpapi

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFavoriteRequestsRequireAuthentication(t *testing.T) {
	router := New(nil, &App{Auth: &Auth{}})
	for _, request := range []struct{ method, path string }{
		{"GET", "/v1/favorites"},
		{"PUT", "/v1/favorites/1"},
		{"DELETE", "/v1/favorites/1"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(request.method, request.path, strings.NewReader(`{"entry_version":1}`)))
		if w.Code != 401 {
			t.Errorf("%s %s: status %d, want 401", request.method, request.path, w.Code)
		}
	}
}

func TestFavoriteRequestValidation(t *testing.T) {
	app := &App{} // Invalid input must be rejected before any database access.
	for _, id := range []string{"", "0", "-1", "+1", "01", "1.0", "1e2", " 1", "9223372036854775808"} {
		for _, method := range []string{"PUT", "DELETE"} {
			t.Run(method+"/"+strconv.Quote(id), func(t *testing.T) {
				r := httptest.NewRequest(method, "/v1/favorites/invalid", strings.NewReader(`{"entry_version":1}`))
				r.SetPathValue("id", id)
				w := httptest.NewRecorder()
				if method == "PUT" {
					app.putFavorite(w, r, "test-user")
				} else {
					app.deleteFavorite(w, r, "test-user")
				}
				if w.Code != 400 {
					t.Fatalf("status %d, want 400", w.Code)
				}
			})
		}
	}
	for _, body := range []string{"", "null", "{}", `{"entry_version":0}`, `{"entry_version":-1}`, `{"entry_version":1.5}`, `{"entry_version":"1"}`, `{"entry_version":9223372036854775808}`, `{"entry_version":1,"user_id":"other"}`, `{"entry_version":1} {}`} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest("PUT", "/v1/favorites/1", strings.NewReader(body))
			r.SetPathValue("id", "1")
			w := httptest.NewRecorder()
			app.putFavorite(w, r, "test-user")
			if w.Code != 400 {
				t.Fatalf("status %d, want 400", w.Code)
			}
		})
	}
}
