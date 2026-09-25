package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fitty/server/internal/httpapi"
	"fitty/server/internal/intelligence"
	"fitty/server/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	databaseURL := os.Getenv("FITTY_DATABASE_URL")
	authURL, authKey := os.Getenv("FITTY_SUPABASE_URL"), os.Getenv("FITTY_SUPABASE_KEY")
	if databaseURL == "" || authURL == "" || authKey == "" {
		return errors.New("FITTY_DATABASE_URL, FITTY_SUPABASE_URL and FITTY_SUPABASE_KEY are required; run npm run setup:local")
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.Ping(pingCtx); err != nil {
		return errors.New("database unavailable; check local Supabase and server/.env")
	}
	app := &httpapi.App{DB: db, Auth: &httpapi.Auth{URL: authURL, Key: authKey, Client: &http.Client{Timeout: 5 * time.Second}}}
	if key := strings.TrimSpace(os.Getenv("FITTY_SUPABASE_SERVICE_ROLE_KEY")); key != "" {
		app.Storage = storage.New(authURL, key)
	}
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		app.Model = envOrDefault("FITTY_OPENAI_MODEL", "gpt-5-mini")
		app.AI = intelligence.NewClient(key, app.Model)
	}
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); app.RunAnalysis(ctx) }()
	defer func() { stop(); <-workerDone }()
	cleanupDone := make(chan struct{})
	go func() { defer close(cleanupDone); app.RunAttachmentCleanup(ctx) }()
	defer func() { stop(); <-cleanupDone }()

	server := &http.Server{
		Addr:              envOrDefault("FITTY_ADDR", "127.0.0.1:8787"),
		Handler:           httpapi.New(strings.Split(envOrDefault("FITTY_ALLOWED_ORIGINS", "http://localhost:8788,http://127.0.0.1:8788"), ","), app),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      75 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	result := make(chan error, 1)
	go func() {
		slog.Info("API listening", "address", server.Addr)
		result <- server.ListenAndServe()
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
	}

	if err := <-result; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
