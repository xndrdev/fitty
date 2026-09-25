// Package intelligence turns one daily chat message into a reply and validated
// tracking changes. It never writes tracking data itself.
package intelligence

import (
	"context"
	"errors"
)

type Profile struct {
	DisplayName string `json:"display_name"`
	TimeZone    string `json:"time_zone"`
	Goals       string `json:"goals"`
	Preferences string `json:"preferences"`
}

type Message struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Image contains a validated upload from the current message or one earlier
// user message supplied in History. Data is sent only as an image content part.
type Image struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	MIMEType  string `json:"mime_type"`
	Data      []byte `json:"-"`
}

type Values struct {
	Kind            string   `json:"kind"`
	Label           string   `json:"label"`
	Amount          string   `json:"amount"`
	Calories        *float64 `json:"calories"`
	ProteinG        *float64 `json:"protein_g"`
	CarbsG          *float64 `json:"carbs_g"`
	FatG            *float64 `json:"fat_g"`
	DurationMinutes *float64 `json:"duration_minutes"`
	DistanceKM      *float64 `json:"distance_km"`
	Source          string   `json:"source"`
	Notes           string   `json:"notes"`
}

type Entry struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	Values
}

type Input struct {
	DailyTargets DailyTargets       `json:"daily_targets"`
	Date         string             `json:"date"`
	Profile      Profile            `json:"profile"`
	History      []Message          `json:"history"`
	Message      Message            `json:"message"`
	Entries      []Entry            `json:"entries"`
	DailyTotals  map[string]float64 `json:"daily_totals"`
	Images       []Image            `json:"images"`
}

type Action struct {
	Operation string  `json:"operation"`
	EntryID   *string `json:"entry_id"`
	Entry     *Values `json:"entry"`
	Evidence  string  `json:"evidence"`
}

type Result struct {
	Reply   string   `json:"reply"`
	Intent  string   `json:"intent"`
	Actions []Action `json:"actions"`
}

type Provider interface {
	Analyze(context.Context, Input) (Result, error)
}

// These errors deliberately exclude provider response bodies and personal data.
var (
	ErrUnavailable   = errors.New("Die KI ist gerade nicht erreichbar. Bitte später erneut versuchen.")
	ErrRefused       = errors.New("Die KI konnte diese Nachricht nicht auswerten. Es wurde nichts gebucht.")
	ErrIncomplete    = errors.New("Die KI-Antwort war unvollständig. Bitte erneut versuchen.")
	ErrInvalidResult = errors.New("Die KI-Antwort konnte nicht sicher übernommen werden. Es wurde nichts gebucht.")
	ErrInvalidInput  = errors.New("Die Nachricht konnte nicht zur Auswertung vorbereitet werden.")
	ErrConfiguration = errors.New("Die KI-Verbindung ist noch nicht vollständig eingerichtet.")
)
