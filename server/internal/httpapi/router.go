package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// The health endpoint reports liveness; application routes require a verified session.
func New(allowedOrigins []string, app *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(struct {
			Status    string `json:"status"`
			Service   string `json:"service"`
			AIEnabled bool   `json:"ai_enabled"`
		}{Status: "ok", Service: "fitty-api", AIEnabled: app != nil && app.AI != nil})
	})
	if app != nil {
		mux.HandleFunc("GET /v1/profile", app.authenticated(app.getProfile))
		mux.HandleFunc("PUT /v1/profile", app.authenticated(app.putProfile))
		mux.HandleFunc("GET /v1/targets", app.authenticated(app.getTargets))
		mux.HandleFunc("PUT /v1/targets", app.authenticated(app.putTargets))
		mux.HandleFunc("GET /v1/progress-photos", app.authenticated(app.progressPhotos))
		mux.HandleFunc("PUT /v1/progress-photos/{id}", app.authenticated(app.uploadProgressPhoto))
		mux.HandleFunc("GET /v1/progress-photos/{id}/url", app.authenticated(app.progressPhotoURL))
		mux.HandleFunc("DELETE /v1/progress-photos/{id}", app.authenticated(app.removeProgressPhoto))
		mux.HandleFunc("GET /v1/days", app.authenticated(app.days))
		mux.HandleFunc("GET /v1/days/{date}/messages", app.authenticated(app.messages))
		mux.HandleFunc("POST /v1/days/{date}/messages", app.authenticated(app.send))
		mux.HandleFunc("GET /v1/days/{date}/summary", app.authenticated(app.summary))
		mux.HandleFunc("GET /v1/days/{date}/deletion", app.authenticated(app.dayDeletionPreview))
		mux.HandleFunc("DELETE /v1/days/{date}", app.authenticated(app.deleteDay))
		mux.HandleFunc("PUT /v1/days/{date}/entries/{id}", app.authenticated(app.changeEntry))
		mux.HandleFunc("DELETE /v1/days/{date}/entries/{id}", app.authenticated(app.changeEntry))
		mux.HandleFunc("POST /v1/messages/{id}/analysis/{action}", app.authenticated(app.changeAnalysis))
		mux.HandleFunc("PUT /v1/days/{date}/attachments/{id}", app.authenticated(app.uploadAttachment))
		mux.HandleFunc("GET /v1/attachments/{id}/url", app.authenticated(app.attachmentURL))
		mux.HandleFunc("DELETE /v1/attachments/{id}", app.authenticated(app.removeAttachment))
	}

	origins := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" && origin != "*" && origin != "null" {
			origins[origin] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Add("Vary", "Origin")
		if origin := r.Header.Get("Origin"); origin != "" {
			if !origins[origin] {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
