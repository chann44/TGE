package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chann44/TGE/adapters"
	internal "github.com/chann44/TGE/internals"
	db "github.com/chann44/TGE/internals/db"
	"github.com/chann44/TGE/internals/jobs"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := internal.GetConfig()
	postgresPool, err := adapters.NewPostgres(ctx, cfg)
	if err != nil {
		log.Fatalf("scheduler: failed to initialize postgres: %v", err)
	}
	defer postgresPool.Close()
	queries := db.New(postgresPool)

	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	redisClient, err := adapters.NewRedis(redisAddr, "", 0)
	if err != nil {
		log.Fatalf("scheduler: failed to initialize redis: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("scheduler: failed to close redis client: %v", err)
		}
	}()

	clickhouseClient, err := adapters.NewClickHouse(ctx, cfg)
	if err != nil {
		log.Printf("scheduler: clickhouse unavailable, log streaming degraded: %v", err)
	}
	centralLogger := adapters.NewCentralLogger(clickhouseClient, "scheduler")
	centralLogger.Log(ctx, "scheduler", "info", "scheduler process started", nil)

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer func() {
		if err := asynqClient.Close(); err != nil {
			log.Printf("scheduler: failed to close asynq client: %v", err)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("scheduler: running, checking scheduled scan policies")
	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler: shutdown signal received")
			return
		case <-ticker.C:
			if err := runScheduledScanTick(ctx, queries, redisClient, asynqClient, centralLogger); err != nil {
				log.Printf("scheduler: tick failed: %v", err)
			}
		}
	}
}

func runScheduledScanTick(ctx context.Context, queries *db.Queries, redisClient *adapters.Redis, asynqClient *asynq.Client, logger *adapters.CentralLogger) error {
	targets, err := queries.ListScheduledPolicyRepositoryTargets(ctx)
	if err != nil {
		return fmt.Errorf("list scheduled policy targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	now := time.Now().UTC().Truncate(time.Minute)
	enqueued := 0
	for _, target := range targets {
		if !target.Cron.Valid {
			continue
		}
		cronExpr := strings.TrimSpace(target.Cron.String)
		if cronExpr == "" {
			continue
		}

		tz := strings.TrimSpace(target.Timezone)
		if tz == "" {
			tz = "UTC"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}

		schedule, err := cron.ParseStandard(cronExpr)
		if err != nil {
			log.Printf("scheduler: invalid cron policy_id=%d trigger_id=%d cron=%q: %v", target.PolicyID, target.TriggerID, cronExpr, err)
			continue
		}

		nowLoc := now.In(loc)
		prevLoc := nowLoc.Add(-1 * time.Minute)
		next := schedule.Next(prevLoc)
		if next.After(nowLoc) {
			continue
		}

		slot := nowLoc.Format("200601021504")
		guardKey := "scan:schedule:" + strconv.FormatInt(target.PolicyID, 10) + ":" + strconv.FormatInt(target.GithubRepoID, 10) + ":" + slot
		acquired, err := redisClient.SetNX(ctx, guardKey, "1", 2*time.Minute)
		if err != nil {
			continue
		}
		if !acquired {
			continue
		}

		scanRun, err := queries.CreateRepositoryScanRun(ctx, db.CreateRepositoryScanRunParams{
			RepositoryID: target.GithubRepoID,
			PolicyID:     pgtype.Int8{Int64: target.PolicyID, Valid: true},
			Trigger:      "schedule",
		})
		if err != nil {
			_ = redisClient.Del(ctx, guardKey)
			continue
		}

		task, err := jobs.NewScanRunTask(jobs.ScanRunPayload{
			ScanRunID: scanRun.ID,
			UserID:    target.UserID,
			RepoID:    target.GithubRepoID,
			Trigger:   "schedule",
		})
		if err != nil {
			_ = queries.MarkRepositoryScanRunFailed(ctx, db.MarkRepositoryScanRunFailedParams{ID: scanRun.ID, ErrorMessage: "failed to build scan task payload"})
			continue
		}

		taskID := fmt.Sprintf("scan:%d:%d:%d", target.UserID, target.GithubRepoID, scanRun.ID)
		_, err = asynqClient.EnqueueContext(
			ctx,
			task,
			asynq.Queue("scans"),
			asynq.TaskID(taskID),
			asynq.Unique(2*time.Minute),
			asynq.MaxRetry(4),
			asynq.Timeout(20*time.Minute),
		)
		if err != nil {
			_ = queries.MarkRepositoryScanRunFailed(ctx, db.MarkRepositoryScanRunFailedParams{ID: scanRun.ID, ErrorMessage: "failed to enqueue scheduled scan task"})
			continue
		}

		enqueued++
		log.Printf("scheduler: queued scheduled scan run_id=%d user_id=%d repo_id=%d policy_id=%d", scanRun.ID, target.UserID, target.GithubRepoID, target.PolicyID)
		logger.Log(ctx, "scheduler", "info", "scheduled scan queued", map[string]any{
			"scan_run_id": scanRun.ID,
			"user_id":     target.UserID,
			"repo_id":     target.GithubRepoID,
			"policy_id":   target.PolicyID,
			"trigger_id":  target.TriggerID,
		})
	}

	if enqueued > 0 {
		log.Printf("scheduler: queued %d scheduled scan(s)", enqueued)
	}

	return nil
}
