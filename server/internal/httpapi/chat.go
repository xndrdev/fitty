package httpapi

import (
	"encoding/json"
	"errors"
	"fitty/server/internal/intelligence"
	"fitty/server/internal/storage"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	DB      *pgxpool.Pool
	Auth    *Auth
	AI      intelligence.Provider
	Model   string
	Storage storage.Store
}
type Profile struct {
	DisplayName string `json:"display_name"`
	TimeZone    string `json:"time_zone"`
	Goals       string `json:"goals"`
	Preferences string `json:"preferences"`
}
type Message struct {
	ID               string       `json:"id"`
	ClientID         string       `json:"client_id"`
	Role             string       `json:"role"`
	Content          string       `json:"content"`
	CreatedAt        time.Time    `json:"created_at"`
	AnalysisStatus   string       `json:"analysis_status"`
	AnalysisError    string       `json:"analysis_error"`
	AnalysisAttempts int          `json:"analysis_attempts"`
	Attachments      []Attachment `json:"attachments"`
}
type Day struct {
	Date         string `json:"date"`
	MessageCount int    `json:"message_count"`
	Preview      string `json:"preview"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func serverError(w http.ResponseWriter, err error) {
	// Avoid logging database errors that can contain personal input values.
	slog.Error("chat request failed", "type", "database")
	fail(w, http.StatusServiceUnavailable, "Die Datenbank ist gerade nicht erreichbar. Bitte erneut versuchen.")
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		fail(w, 400, "Ungültige Eingabe.")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "Ungültige Eingabe.")
		return false
	}
	return true
}
func validDate(date string) bool {
	t, err := time.Parse("2006-01-02", date)
	return err == nil && t.Year() >= 1900 && t.Year() <= 9999 && t.Format("2006-01-02") == date
}
func (a *App) authenticated(handler func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.Auth.user(r.Context(), r.Header.Get("Authorization"))
		if errors.Is(err, errUnauthorized) {
			fail(w, 401, "Bitte erneut anmelden.")
			return
		}
		if err != nil {
			fail(w, 503, "Anmeldung kann gerade nicht geprüft werden.")
			return
		}
		handler(w, r, user)
	}
}
func (a *App) getProfile(w http.ResponseWriter, r *http.Request, user string) {
	p := Profile{TimeZone: "Europe/Berlin"}
	err := a.DB.QueryRow(r.Context(), `select display_name, time_zone, goals, preferences from fitty.profiles where user_id=$1`, user).Scan(&p.DisplayName, &p.TimeZone, &p.Goals, &p.Preferences)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, p)
}
func (a *App) putProfile(w http.ResponseWriter, r *http.Request, user string) {
	var p Profile
	if !decode(w, r, &p) {
		return
	}
	p.DisplayName, p.Goals, p.Preferences = strings.TrimSpace(p.DisplayName), strings.TrimSpace(p.Goals), strings.TrimSpace(p.Preferences)
	_, zoneErr := time.LoadLocation(p.TimeZone)
	if p.TimeZone == "" || p.TimeZone == "Local" || zoneErr != nil || utf8.RuneCountInString(p.DisplayName) > 80 || utf8.RuneCountInString(p.Goals) > 2000 || utf8.RuneCountInString(p.Preferences) > 2000 {
		fail(w, 400, "Bitte eine gültige Zeitzone und kürzere Profilangaben verwenden.")
		return
	}
	_, err := a.DB.Exec(r.Context(), `insert into fitty.profiles(user_id,display_name,time_zone,goals,preferences) values($1,$2,$3,$4,$5)
		on conflict(user_id) do update set display_name=excluded.display_name,time_zone=excluded.time_zone,goals=excluded.goals,preferences=excluded.preferences,updated_at=now()`, user, p.DisplayName, p.TimeZone, p.Goals, p.Preferences)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, p)
}
func (a *App) days(w http.ResponseWriter, r *http.Request, user string) {
	before := r.URL.Query().Get("before")
	if before != "" && !validDate(before) {
		fail(w, 400, "Ungültiges Datum.")
		return
	}
	rows, err := a.DB.Query(r.Context(), `select d.local_date::text,
		(select count(*) from fitty.messages m where m.day_id=d.id and m.user_id=$1),
		coalesce((select coalesce(nullif(left(content,120),''),'Foto') from fitty.messages m where m.day_id=d.id and m.user_id=$1 order by m.id desc limit 1),'')
		from fitty.days d where d.user_id=$1 and ($2='' or d.local_date < nullif($2,'')::date)
		order by d.local_date desc limit 31`, user, before)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	days := []Day{}
	for rows.Next() {
		var d Day
		if err = rows.Scan(&d.Date, &d.MessageCount, &d.Preview); err != nil {
			serverError(w, err)
			return
		}
		days = append(days, d)
	}
	if err = rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	var next *string
	if len(days) > 30 {
		days = days[:30]
		next = &days[29].Date
	}
	writeJSON(w, 200, map[string]any{"days": days, "next_before": next})
}
func (a *App) messages(w http.ResponseWriter, r *http.Request, user string) {
	date := r.PathValue("date")
	if !validDate(date) {
		fail(w, 400, "Ungültiges Datum.")
		return
	}
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		var err error
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			fail(w, 400, "Ungültige Seitenauswahl.")
			return
		}
	}
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var dayID *string
	err = tx.QueryRow(r.Context(), `select id::text from fitty.days where user_id=$1 and local_date=$2::date`, user, date).Scan(&dayID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	rows, err := tx.Query(r.Context(), `select m.id::text,m.client_id::text,m.role,m.content,m.created_at,coalesce(j.status,'saved'),coalesce(j.error_code,''),coalesce(j.attempts,0)
		from fitty.messages m join fitty.days d on d.id=m.day_id and d.user_id=m.user_id
		left join fitty.analysis_jobs j on j.message_id=m.id and j.user_id=m.user_id
		where m.user_id=$1 and d.local_date=$2::date and ($3::bigint=0 or m.id<$3)
		order by m.id desc limit 101`, user, date, before)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	messages := []Message{}
	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.ID, &m.ClientID, &m.Role, &m.Content, &m.CreatedAt, &m.AnalysisStatus, &m.AnalysisError, &m.AnalysisAttempts); err != nil {
			serverError(w, err)
			return
		}
		messages = append(messages, m)
	}
	if err = rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	var next *string
	if len(messages) > 100 {
		messages = messages[:100]
		cursor := messages[99].ID
		next = &cursor
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	rows.Close()
	if err = loadMessageAttachments(r.Context(), tx, user, messages); err != nil {
		serverError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"date": date, "day_id": dayID, "messages": messages, "next_before": next})
}
func (a *App) send(w http.ResponseWriter, r *http.Request, user string) {
	date := r.PathValue("date")
	if !validDate(date) {
		fail(w, 400, "Ungültiges Datum.")
		return
	}
	var input struct {
		ClientID      string   `json:"client_id"`
		Content       string   `json:"content"`
		Analyze       *bool    `json:"analyze"`
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Content = strings.TrimSpace(input.Content)
	if !uuidPattern.MatchString(input.ClientID) || (input.Content == "" && len(input.AttachmentIDs) == 0) || utf8.RuneCountInString(input.Content) > 8000 || strings.ContainsRune(input.Content, 0) || len(input.AttachmentIDs) > 4 {
		fail(w, 400, "Bitte Text oder bis zu vier Bilder, höchstens 8.000 Zeichen und eine gültige Nachrichten-ID senden.")
		return
	}
	seen := map[string]bool{}
	for i, id := range input.AttachmentIDs {
		id = strings.ToLower(id)
		if !uuidPattern.MatchString(id) || seen[id] {
			fail(w, 400, "Ungültige oder doppelte Bild-ID.")
			return
		}
		seen[id] = true
		input.AttachmentIDs[i] = id
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `select pg_advisory_xact_lock($1)`, analysisQueueLock); err != nil {
		serverError(w, err)
		return
	}
	var dayID int64
	err = tx.QueryRow(r.Context(), `insert into fitty.days(user_id,local_date,time_zone)
		values($1,$2::date,coalesce((select time_zone from fitty.profiles where user_id=$1),'Europe/Berlin'))
		on conflict(user_id,local_date) do update set local_date=excluded.local_date returning id`, user, date).Scan(&dayID)
	if err != nil {
		serverError(w, err)
		return
	}
	var retired bool
	err = tx.QueryRow(r.Context(), `select exists(select 1 from fitty.deleted_identifiers where user_id=$1 and kind='message' and id=$2)`, user, input.ClientID).Scan(&retired)
	if err != nil {
		serverError(w, err)
		return
	}
	if retired {
		fail(w, 409, "Diese Nachricht gehörte zu einem gelöschten Tageschat. Bitte eine neue Nachricht verfassen.")
		return
	}
	var m Message
	err = tx.QueryRow(r.Context(), `insert into fitty.messages(day_id,user_id,client_id,content) values($1,$2,$3,$4)
		on conflict(user_id,client_id) do nothing returning id::text,client_id::text,role,content,created_at`, dayID, user, input.ClientID, input.Content).Scan(&m.ID, &m.ClientID, &m.Role, &m.Content, &m.CreatedAt)
	status := http.StatusCreated
	if errors.Is(err, pgx.ErrNoRows) {
		var existingDay int64
		err = tx.QueryRow(r.Context(), `select id::text,client_id::text,role,content,created_at,day_id from fitty.messages where user_id=$1 and client_id=$2`, user, input.ClientID).Scan(&m.ID, &m.ClientID, &m.Role, &m.Content, &m.CreatedAt, &existingDay)
		if err == nil && (existingDay != dayID || m.Content != input.Content) {
			fail(w, 409, "Diese Nachrichten-ID gehört bereits zu einem anderen Inhalt oder Tag.")
			return
		}
		status = http.StatusOK
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if status == http.StatusCreated {
		for position, id := range input.AttachmentIDs {
			result, bindErr := tx.Exec(r.Context(), `update fitty.attachments set message_id=$1::bigint,position=$2
 where id=$3 and user_id=$4 and local_date=$5::date and message_id is null and state='ready' and delete_requested_at is null`, m.ID, position, id, user, date)
			if bindErr != nil {
				serverError(w, bindErr)
				return
			}
			if result.RowsAffected() != 1 {
				fail(w, 409, "Ein Bild ist noch nicht hochgeladen oder gehört bereits zu einer anderen Nachricht. Bitte Bildauswahl prüfen.")
				return
			}
		}
	}
	withAttachments := []Message{m}
	if err = loadMessageAttachments(r.Context(), tx, user, withAttachments); err != nil {
		serverError(w, err)
		return
	}
	m = withAttachments[0]
	if status == http.StatusOK {
		ids := make([]string, len(m.Attachments))
		for i, item := range m.Attachments {
			ids[i] = item.ID
		}
		if !slices.Equal(ids, input.AttachmentIDs) {
			fail(w, 409, "Diese Nachrichten-ID gehört bereits zu einer anderen Bildauswahl.")
			return
		}
	}
	if status == http.StatusCreated {
		_, err = tx.Exec(r.Context(), `update fitty.days set updated_at=now(),version=version+1 where id=$1 and user_id=$2`, dayID, user)
		if err == nil {
			state := "disabled"
			if a.AI != nil && (input.Analyze == nil || *input.Analyze) {
				state = "queued"
			}
			_, err = tx.Exec(r.Context(), `insert into fitty.analysis_jobs(message_id,day_id,user_id,status) values($1::bigint,$2,$3,$4)`, m.ID, dayID, user, state)
		}
	}
	if err != nil {
		serverError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), `select status,error_code,attempts from fitty.analysis_jobs where message_id=$1::bigint and user_id=$2`, m.ID, user).Scan(&m.AnalysisStatus, &m.AnalysisError, &m.AnalysisAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		m.AnalysisStatus = "saved"
		err = nil
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"message": m})
}
