package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5"
)

type analysisJob struct {
	MessageID, DayID int64
	UserID, Lease    string
}

// Direct entry edits briefly share the claim lock so no worker starts from an
// entry snapshot taken during a manual change. No provider call holds it.
const analysisQueueLock int64 = 714719203

// One bounded worker per Go process. Claims are coordinated in PostgreSQL;
// provider calls never hold a database transaction open.
func (a *App) RunAnalysis(ctx context.Context) {
	if a.AI == nil {
		return
	}
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := a.claimAnalysis(ctx)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("analysis claim failed")
		}
		if err == nil {
			a.processAnalysis(ctx, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (a *App) claimAnalysis(ctx context.Context) (analysisJob, error) {
	var job analysisJob
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return job, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	err = tx.QueryRow(ctx, `select pg_try_advisory_xact_lock($1)`, analysisQueueLock).Scan(&locked)
	if err != nil {
		return job, err
	}
	if !locked {
		return job, pgx.ErrNoRows
	}
	_, err = tx.Exec(ctx, `update fitty.analysis_jobs set status=case when attempts<3 then 'queued' else 'failed' end,
  lease=null,error_code='interrupted' where status='processing' and started_at<now()-interval '2 minutes'`)
	if err != nil {
		return job, err
	}
	err = tx.QueryRow(ctx, `select j.message_id,j.day_id,j.user_id::text from fitty.analysis_jobs j
  where j.status='queued' and j.attempts<3 and not exists (
   select 1 from fitty.analysis_jobs earlier where earlier.day_id=j.day_id and
    (earlier.status='processing' or (earlier.message_id<j.message_id and earlier.status in ('queued','failed'))))
		order by j.message_id limit 1 for update of j skip locked`).Scan(&job.MessageID, &job.DayID, &job.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return job, commitErr
		}
		return job, pgx.ErrNoRows
	}
	if err != nil {
		return job, err
	}
	err = tx.QueryRow(ctx, `update fitty.analysis_jobs set status='processing',attempts=attempts+1,
  started_at=now(),finished_at=null,error_code='',lease=gen_random_uuid(),model=$2 where message_id=$1 returning lease::text`, job.MessageID, a.Model).Scan(&job.Lease)
	if err != nil {
		return job, err
	}
	return job, tx.Commit(ctx)
}

func (a *App) analysisInput(ctx context.Context, job analysisJob) (intelligence.Input, error) {
	var input intelligence.Input
	input.Profile.TimeZone = "Europe/Berlin"
	err := a.DB.QueryRow(ctx, `select d.local_date::text,m.id::text,m.role,m.content from fitty.days d join fitty.messages m on m.day_id=d.id and m.user_id=d.user_id
  where m.id=$1 and m.day_id=$2 and m.user_id=$3 and m.role='user'`, job.MessageID, job.DayID, job.UserID).Scan(&input.Date, &input.Message.ID, &input.Message.Role, &input.Message.Content)
	if err != nil {
		return input, err
	}
	err = a.DB.QueryRow(ctx, `select display_name,time_zone,goals,preferences from fitty.profiles where user_id=$1`, job.UserID).Scan(&input.Profile.DisplayName, &input.Profile.TimeZone, &input.Profile.Goals, &input.Profile.Preferences)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return input, err
	}
	targets, err := loadTargetsForDate(ctx, a.DB, job.UserID, input.Date)
	if err != nil {
		return input, err
	}
	input.DailyTargets = targets.Targets
	totals, err := loadTotals(ctx, a.DB, job.UserID, job.DayID)
	if err != nil {
		return input, err
	}
	input.DailyTotals = map[string]float64{"food_calories": totals.Calories, "protein_g": totals.ProteinG, "carbs_g": totals.CarbsG, "fat_g": totals.FatG, "activity_calories": totals.ActivityCalories, "duration_minutes": totals.DurationMinutes, "distance_km": totals.DistanceKM, "estimated_entries": float64(totals.EstimatedEntries), "unknown_activity_calories": float64(totals.UnknownActivityCalories)}
	input.Entries, err = loadEntries(ctx, a.DB, job.UserID, job.DayID)
	if err != nil {
		return input, err
	}
	rows, err := a.DB.Query(ctx, `select id::text,role,content from fitty.messages where user_id=$1 and day_id=$2
  and (id<$3 or reply_to<$3) order by fitty.messages.id desc limit 30`, job.UserID, job.DayID, job.MessageID)
	if err != nil {
		return input, err
	}
	defer rows.Close()
	input.History = []intelligence.Message{}
	size := 0
	for rows.Next() {
		var m intelligence.Message
		if err = rows.Scan(&m.ID, &m.Role, &m.Content); err != nil {
			return input, err
		}
		size += len(m.Content)
		if size > 48000 {
			break
		}
		input.History = append(input.History, m)
	}
	if err = rows.Err(); err != nil {
		return input, err
	}
	for i, j := 0, len(input.History)-1; i < j; i, j = i+1, j-1 {
		input.History[i], input.History[j] = input.History[j], input.History[i]
	}
	err = a.analysisImages(ctx, job, &input)
	return input, err
}

func (a *App) processAnalysis(ctx context.Context, job analysisJob) {
	attemptCtx, cancel := context.WithTimeout(ctx, 95*time.Second)
	defer cancel()
	input, err := a.analysisInput(attemptCtx, job)
	var result intelligence.Result
	if err == nil {
		result, err = a.AI.Analyze(attemptCtx, input)
	}
	if err == nil {
		err = intelligence.Validate(result, input)
	}
	if err == nil {
		err = a.commitAnalysis(attemptCtx, job, input, result)
	}
	if err == nil {
		return
	}
	code := "unavailable"
	switch {
	case errors.Is(err, intelligence.ErrRefused):
		code = "refused"
	case errors.Is(err, intelligence.ErrIncomplete):
		code = "incomplete"
	case errors.Is(err, intelligence.ErrInvalidResult), errors.Is(err, intelligence.ErrInvalidInput):
		code = "invalid_result"
	case errors.Is(err, intelligence.ErrConfiguration):
		code = "configuration"
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	status := "failed"
	if ctx.Err() != nil {
		status = "queued"
		code = "interrupted"
	}
	_, finishErr := a.DB.Exec(finishCtx, `update fitty.analysis_jobs set status=case when $3='queued' and attempts>=3 then 'failed' else $3 end,
  error_code=$4,finished_at=now(),lease=null where message_id=$1 and lease=$2::uuid and status='processing'`, job.MessageID, job.Lease, status, code)
	if finishErr != nil {
		slog.Error("analysis status update failed")
	}
	slog.Warn("analysis not applied", "code", code)
}

func rounded(value *float64) *float64 {
	if value == nil {
		return nil
	}
	n := math.Round(*value*100) / 100
	return &n
}
func roundEntry(e intelligence.Values) intelligence.Values {
	e.Calories = rounded(e.Calories)
	e.ProteinG = rounded(e.ProteinG)
	e.CarbsG = rounded(e.CarbsG)
	e.FatG = rounded(e.FatG)
	e.DurationMinutes = rounded(e.DurationMinutes)
	e.DistanceKM = rounded(e.DistanceKM)
	return e
}

func (a *App) commitAnalysis(ctx context.Context, job analysisJob, input intelligence.Input, result intelligence.Result) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Deletion, sends and direct edits acquire this parent row first.
	var day int64
	if err = tx.QueryRow(ctx, `select id from fitty.days where id=$1 and user_id=$2 for update`, job.DayID, job.UserID).Scan(&day); err != nil {
		return err
	}
	var status, lease string
	err = tx.QueryRow(ctx, `select status,coalesce(lease::text,'') from fitty.analysis_jobs where message_id=$1 and user_id=$2 for update`, job.MessageID, job.UserID).Scan(&status, &lease)
	if err != nil {
		return err
	}
	if status != "processing" || lease != job.Lease {
		return errors.New("stale analysis lease")
	}
	// Revalidate ownership and current state while holding the job lock.
	current, err := loadEntries(ctx, tx, job.UserID, job.DayID)
	if err != nil {
		return err
	}
	input.Entries = current
	if err = intelligence.Validate(result, input); err != nil {
		return err
	}
	creates := 0
	deletes := 0
	for _, action := range result.Actions {
		if action.Operation == "create" {
			creates++
		}
		if action.Operation == "delete" {
			deletes++
		}
	}
	if len(current)+creates-deletes > 200 {
		return intelligence.ErrInvalidResult
	}
	for index, action := range result.Actions {
		var entryID string
		var before, after []byte
		if action.EntryID != nil {
			entryID = *action.EntryID
			for _, entry := range current {
				if entry.ID == entryID {
					before, _ = json.Marshal(entry.Values)
					break
				}
			}
		}
		if action.Operation == "delete" {
			tag, execErr := tx.Exec(ctx, `update fitty.tracking_entries set deleted_at=now(),updated_at=now(),updated_by_message_id=$4,version=version+1 where id=$1::bigint and day_id=$2 and user_id=$3 and deleted_at is null`, entryID, job.DayID, job.UserID, job.MessageID)
			if execErr != nil {
				return execErr
			}
			if tag.RowsAffected() != 1 {
				return intelligence.ErrInvalidResult
			}
		} else {
			e := roundEntry(*action.Entry)
			after, _ = json.Marshal(e)
			args := []any{job.UserID, job.DayID, job.MessageID, e.Kind, e.Label, e.Amount, e.Calories, e.ProteinG, e.CarbsG, e.FatG, e.DurationMinutes, e.DistanceKM, e.Source, e.Notes}
			if action.Operation == "create" {
				err = tx.QueryRow(ctx, `insert into fitty.tracking_entries(user_id,day_id,source_message_id,updated_by_message_id,kind,label,amount,calories,protein_g,carbs_g,fat_g,duration_minutes,distance_km,source,notes)
     values($1,$2,$3,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) returning id::text`, args...).Scan(&entryID)
			} else {
				args = append(args, entryID)
				tag, execErr := tx.Exec(ctx, `update fitty.tracking_entries set updated_by_message_id=$3,kind=$4,label=$5,amount=$6,calories=$7,protein_g=$8,carbs_g=$9,fat_g=$10,
     duration_minutes=$11,distance_km=$12,source=$13,notes=$14,updated_at=now(),version=version+1 where user_id=$1 and day_id=$2 and id=$15::bigint and deleted_at is null`, args...)
				err = execErr
				if err == nil && tag.RowsAffected() != 1 {
					return intelligence.ErrInvalidResult
				}
			}
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `insert into fitty.entry_changes(message_id,action_index,entry_id,operation,before_value,after_value,evidence) values($1,$2,$3::bigint,$4,$5::jsonb,$6::jsonb,$7)`, job.MessageID, index, entryID, action.Operation, before, after, action.Evidence)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `insert into fitty.messages(day_id,user_id,client_id,role,content,reply_to) values($1,$2,gen_random_uuid(),'assistant',$3,$4)`, job.DayID, job.UserID, result.Reply, job.MessageID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update fitty.analysis_jobs set status='completed',finished_at=now(),error_code='',lease=null where message_id=$1 and lease=$2::uuid`, job.MessageID, job.Lease)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update fitty.days set updated_at=now(),version=version+1 where id=$1 and user_id=$2`, job.DayID, job.UserID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
