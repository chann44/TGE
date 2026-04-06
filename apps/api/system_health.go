package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	db "github.com/chann44/TGE/internals/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const appVersion = "v0.1.0"

type serviceDescriptor struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type serviceStatus struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	LatencyMS     int     `json:"latency_ms"`
	UptimePct     float64 `json:"uptime_pct"`
	LastCheckedAt string  `json:"last_checked_at"`
	Note          string  `json:"note"`
}

var knownServices = []serviceDescriptor{
	{Key: "api", Name: "API Server"},
	{Key: "worker", Name: "Worker"},
	{Key: "scheduler", Name: "Scheduler"},
	{Key: "postgres", Name: "PostgreSQL"},
	{Key: "redis", Name: "Redis"},
	{Key: "github_webhook", Name: "GitHub Webhook"},
	{Key: "osv_scanner", Name: "OSV Scanner"},
	{Key: "scan_worker", Name: "Scan Worker"},
}

func (h *Handler) systemHealthSummary(w http.ResponseWriter, r *http.Request) {
	statuses := h.collectServiceStatuses(r.Context())
	servicesUp := 0
	for _, status := range statuses {
		if status.Status == "ok" {
			servicesUp++
		}
	}

	queued, _ := h.queries.CountRepositoryDependencySyncByStatus(r.Context(), "queued")
	running, _ := h.queries.CountRepositoryDependencySyncByStatus(r.Context(), "running")
	throughput, _ := h.queries.CountRepositoryDependencySyncSuccessSince(r.Context(), toPgTimestamptz(time.Now().Add(-1*time.Hour)))
	scanQueued, _ := h.queries.CountRepositoryScansByStatus(r.Context(), "queued")
	scanRunning, _ := h.queries.CountRepositoryScansByStatus(r.Context(), "running")
	scanThroughput, _ := h.queries.CountRepositoryScansSuccessSince(r.Context(), toPgTimestamptz(time.Now().Add(-1*time.Hour)))

	writeJSON(w, http.StatusOK, map[string]any{
		"services_up":                   servicesUp,
		"services_total":                len(statuses),
		"queue_backlog":                 queued + running + scanQueued + scanRunning,
		"dependency_sync_throughput_1h": throughput,
		"scan_throughput_1h":            scanThroughput,
		"version":                       appVersion,
		"generated_at":                  time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) systemHealthServices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"services": h.collectServiceStatuses(r.Context()),
	})
}

func (h *Handler) systemHealthQueues(w http.ResponseWriter, r *http.Request) {
	queued, _ := h.queries.CountRepositoryDependencySyncByStatus(r.Context(), "queued")
	running, _ := h.queries.CountRepositoryDependencySyncByStatus(r.Context(), "running")
	failedSince, _ := h.queries.CountRepositoryDependencySyncFailedSince(r.Context(), toPgTimestamptz(time.Now().Add(-24*time.Hour)))
	scanQueued, _ := h.queries.CountRepositoryScansByStatus(r.Context(), "queued")
	scanRunning, _ := h.queries.CountRepositoryScansByStatus(r.Context(), "running")
	scanFailedSince, _ := h.queries.CountRepositoryScansFailedSince(r.Context(), toPgTimestamptz(time.Now().Add(-24*time.Hour)))

	writeJSON(w, http.StatusOK, map[string]any{
		"queues": []map[string]any{
			{
				"queue":      "dependencies",
				"job_type":   "dependency_sync",
				"pending":    queued,
				"running":    running,
				"failed":     failedSince,
				"sampled_at": time.Now().UTC().Format(time.RFC3339),
			},
			{
				"queue":      "scans",
				"job_type":   "scan_run",
				"pending":    scanQueued,
				"running":    scanRunning,
				"failed":     scanFailedSince,
				"sampled_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
}

func (h *Handler) systemHealthLogs(w http.ResponseWriter, r *http.Request) {
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	level := normalizeLogLevel(strings.TrimSpace(r.URL.Query().Get("level")))
	cursor := parseInt64Default(r.URL.Query().Get("cursor"), 0)
	limit := queryInt(r.URL.Query().Get("limit"), 50)
	if limit > 200 {
		limit = 200
	}
	if h.clickhouse == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "next_cursor": 0})
		return
	}

	rows, err := h.clickhouse.ListServiceLogs(r.Context(), service, level, cursor, limit)
	if err != nil {
		http.Error(w, "failed to fetch service logs", http.StatusInternalServerError)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	nextCursor := int64(0)
	for _, row := range rows {
		metadata := map[string]any{}
		if strings.TrimSpace(row.Metadata) != "" {
			_ = json.Unmarshal([]byte(row.Metadata), &metadata)
		}
		items = append(items, map[string]any{
			"id":         row.Cursor,
			"service":    row.Service,
			"level":      row.Level,
			"message":    row.Message,
			"metadata":   metadata,
			"created_at": row.Timestamp.UTC().Format(time.RFC3339),
		})
		nextCursor = row.Cursor
	}

	if len(rows) < limit {
		nextCursor = 0
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) systemHealthLogServices(w http.ResponseWriter, r *http.Request) {
	rows := make([]string, 0)
	if h.clickhouse != nil {
		values, err := h.clickhouse.ListDistinctLogServices(r.Context())
		if err != nil {
			http.Error(w, "failed to fetch log services", http.StatusInternalServerError)
			return
		}
		rows = values
	}

	serviceByKey := map[string]string{}
	for _, service := range knownServices {
		serviceByKey[service.Key] = service.Name
	}
	for _, key := range rows {
		if _, exists := serviceByKey[key]; exists {
			continue
		}
		serviceByKey[key] = humanizeServiceKey(key)
	}

	services := make([]serviceDescriptor, 0, len(serviceByKey))
	for key, name := range serviceByKey {
		services = append(services, serviceDescriptor{Key: key, Name: name})
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
	})
}

func (h *Handler) systemHealthLogsStream(w http.ResponseWriter, r *http.Request) {
	if h.clickhouse == nil {
		http.Error(w, "log streaming unavailable", http.StatusServiceUnavailable)
		return
	}

	service := strings.TrimSpace(r.URL.Query().Get("service"))
	level := normalizeLogLevel(strings.TrimSpace(r.URL.Query().Get("level")))
	cursor := parseInt64Default(r.URL.Query().Get("cursor"), 0)
	if cursor == 0 {
		cursor = time.Now().Add(-5 * time.Second).UnixNano()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	pollTicker := time.NewTicker(1 * time.Second)
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer pollTicker.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeatTicker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().Unix())
			flusher.Flush()
		case <-pollTicker.C:
			rows, err := h.clickhouse.ListServiceLogsAfter(r.Context(), service, level, cursor, 200)
			if err != nil {
				_, _ = fmt.Fprint(w, "event: error\ndata: {\"message\":\"failed to read logs\"}\n\n")
				flusher.Flush()
				continue
			}

			for _, row := range rows {
				metadata := map[string]any{}
				if strings.TrimSpace(row.Metadata) != "" {
					_ = json.Unmarshal([]byte(row.Metadata), &metadata)
				}
				payload, _ := json.Marshal(map[string]any{
					"id":         row.Cursor,
					"service":    row.Service,
					"level":      row.Level,
					"message":    row.Message,
					"metadata":   metadata,
					"created_at": row.Timestamp.UTC().Format(time.RFC3339),
				})
				_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
				cursor = row.Cursor
			}
			flusher.Flush()
		}
	}
}

func (h *Handler) collectServiceStatuses(ctx context.Context) []serviceStatus {
	now := time.Now().UTC()
	statuses := make([]serviceStatus, 0, len(knownServices))

	statuses = append(statuses, serviceStatus{
		Key:           "api",
		Name:          "API Server",
		Status:        "ok",
		LatencyMS:     1,
		UptimePct:     99.9,
		LastCheckedAt: now.Format(time.RFC3339),
		Note:          "process alive",
	})

	pgStatus := serviceStatus{Key: "postgres", Name: "PostgreSQL", Status: "ok", UptimePct: 100, Note: "ready"}
	pgStart := time.Now()
	if h.postgres == nil {
		pgStatus.Status = "down"
		pgStatus.UptimePct = 0
		pgStatus.Note = "not configured"
	} else if err := h.postgres.Ping(ctx); err != nil {
		pgStatus.Status = "down"
		pgStatus.UptimePct = 0
		pgStatus.Note = truncateMessage(err.Error(), 250)
		h.writeServiceLog(ctx, "postgres", "error", "postgres ping failed", map[string]any{"error": err.Error()})
	}
	pgStatus.LatencyMS = int(time.Since(pgStart).Milliseconds())
	pgStatus.LastCheckedAt = now.Format(time.RFC3339)
	statuses = append(statuses, pgStatus)

	redisStatus := serviceStatus{Key: "redis", Name: "Redis", Status: "ok", UptimePct: 100, Note: "ready"}
	redisStart := time.Now()
	if err := h.redis.Ping(ctx); err != nil {
		redisStatus.Status = "down"
		redisStatus.UptimePct = 0
		redisStatus.Note = truncateMessage(err.Error(), 250)
		h.writeServiceLog(ctx, "redis", "error", "redis ping failed", map[string]any{"error": err.Error()})
	}
	redisStatus.LatencyMS = int(time.Since(redisStart).Milliseconds())
	redisStatus.LastCheckedAt = now.Format(time.RFC3339)
	statuses = append(statuses, redisStatus)

	workerStatus := serviceStatus{Key: "worker", Name: "Worker", Status: "ok", UptimePct: 99.7, Note: "processing jobs"}
	running, runErr := h.queries.CountRepositoryDependencySyncByStatus(ctx, "running")
	queued, queueErr := h.queries.CountRepositoryDependencySyncByStatus(ctx, "queued")
	if runErr != nil || queueErr != nil {
		workerStatus.Status = "degraded"
		workerStatus.UptimePct = 95
		workerStatus.Note = "unable to read queue stats"
	} else if running == 0 && queued > 20 {
		workerStatus.Status = "degraded"
		workerStatus.UptimePct = 96
		workerStatus.Note = "queue backlog increasing"
	}
	workerStatus.LastCheckedAt = now.Format(time.RFC3339)
	statuses = append(statuses, workerStatus)

	scanWorkerStatus := serviceStatus{Key: "scan_worker", Name: "Scan Worker", Status: "ok", UptimePct: 99.5, Note: "processing scan jobs"}
	scanRunning, scanRunErr := h.queries.CountRepositoryScansByStatus(ctx, "running")
	scanQueued, scanQueueErr := h.queries.CountRepositoryScansByStatus(ctx, "queued")
	if scanRunErr != nil || scanQueueErr != nil {
		scanWorkerStatus.Status = "degraded"
		scanWorkerStatus.UptimePct = 95
		scanWorkerStatus.Note = "unable to read scan queue stats"
	} else if scanRunning == 0 && scanQueued > 20 {
		scanWorkerStatus.Status = "degraded"
		scanWorkerStatus.UptimePct = 96
		scanWorkerStatus.Note = "scan queue backlog increasing"
	}
	scanWorkerStatus.LastCheckedAt = now.Format(time.RFC3339)
	statuses = append(statuses, scanWorkerStatus)

	statuses = append(statuses, serviceStatus{
		Key:           "scheduler",
		Name:          "Scheduler",
		Status:        "degraded",
		LatencyMS:     0,
		UptimePct:     98,
		LastCheckedAt: now.Format(time.RFC3339),
		Note:          "active heartbeat probe not configured",
	})

	statuses = append(statuses, serviceStatus{
		Key:           "github_webhook",
		Name:          "GitHub Webhook",
		Status:        "degraded",
		LatencyMS:     0,
		UptimePct:     98,
		LastCheckedAt: now.Format(time.RFC3339),
		Note:          "active probe not configured",
	})

	statuses = append(statuses, serviceStatus{
		Key:           "osv_scanner",
		Name:          "OSV Scanner",
		Status:        "degraded",
		LatencyMS:     0,
		UptimePct:     98,
		LastCheckedAt: now.Format(time.RFC3339),
		Note:          "active probe not configured",
	})

	for _, status := range statuses {
		_ = h.queries.CreateServiceStatusSnapshot(ctx, db.CreateServiceStatusSnapshotParams{
			Service:   status.Key,
			Status:    status.Status,
			LatencyMs: int32(status.LatencyMS),
			UptimePct: status.UptimePct,
			Note:      status.Note,
			CheckedAt: toPgTimestamptz(now),
		})
	}

	return statuses
}

func (h *Handler) writeServiceLog(ctx context.Context, service, level, message string, metadata map[string]any) {
	if h == nil {
		return
	}
	h.logger.Log(ctx, service, nonEmpty(normalizeLogLevel(level), "info"), message, metadata)
}

func normalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "error":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return ""
	}
}

func parseInt64Default(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func humanizeServiceKey(key string) string {
	parts := strings.Split(strings.ReplaceAll(key, "-", "_"), "_")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func truncateMessage(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func toPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
