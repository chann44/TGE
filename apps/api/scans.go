package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	db "github.com/chann44/TGE/internals/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type runScanRequest struct {
	RepoID int64 `json:"repo_id"`
}

type scanRunResponse struct {
	ID               int64  `json:"id"`
	RepositoryID     int64  `json:"repository_id"`
	Repository       string `json:"repository"`
	PolicyID         *int64 `json:"policy_id,omitempty"`
	Policy           string `json:"policy"`
	Trigger          string `json:"trigger"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message,omitempty"`
	FindingsTotal    int32  `json:"findings_total"`
	FindingsCritical int32  `json:"findings_critical"`
	FindingsHigh     int32  `json:"findings_high"`
	FindingsMedium   int32  `json:"findings_medium"`
	FindingsLow      int32  `json:"findings_low"`
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
	Duration         string `json:"duration,omitempty"`
}

type scanFindingResponse struct {
	ID              int64                `json:"id"`
	ScanRunID       int64                `json:"scan_run_id"`
	RepositoryID    int64                `json:"repository_id,omitempty"`
	Repository      string               `json:"repository,omitempty"`
	Policy          string               `json:"policy,omitempty"`
	PackageName     string               `json:"package_name"`
	Manager         string               `json:"manager"`
	Registry        string               `json:"registry"`
	VersionSpec     string               `json:"version_spec"`
	ResolvedVersion string               `json:"resolved_version"`
	AdvisoryID      string               `json:"advisory_id"`
	Aliases         []string             `json:"aliases"`
	Title           string               `json:"title"`
	Summary         string               `json:"summary"`
	Severity        string               `json:"severity"`
	FixedVersion    string               `json:"fixed_version"`
	ReferenceURL    string               `json:"reference_url"`
	Status          string               `json:"status"`
	Sources         []string             `json:"sources"`
	SourceLinks     []sourceLinkResponse `json:"source_links,omitempty"`
	CreatedAt       string               `json:"created_at"`
}

type sourceLinkResponse struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

type scanLogResponse struct {
	ID            int64  `json:"id"`
	Level         string `json:"level"`
	Message       string `json:"message"`
	DirectoryPath string `json:"directory_path,omitempty"`
	CreatedAt     string `json:"created_at"`
}

func (h *Handler) runRepositoryScan(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := strconv.ParseInt(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}

	if _, connected := connectedRepositoryForUser(r.Context(), h.queries, userID, repoID); !connected {
		http.Error(w, "repository is not connected", http.StatusNotFound)
		return
	}

	runID, err := h.enqueueRepositoryScan(r.Context(), userID, repoID, "manual")
	if err != nil {
		http.Error(w, "failed to enqueue scan", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":  true,
		"repo_id": repoID,
		"scan_id": runID,
		"status":  "queued",
	})
}

func (h *Handler) runPolicyScan(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID, err := strconv.ParseInt(chi.URLParam(r, "policyID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid policy id", http.StatusBadRequest)
		return
	}

	if _, err := h.queries.GetPolicyByIDAndUser(r.Context(), db.GetPolicyByIDAndUserParams{ID: policyID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	var req runScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RepoID <= 0 {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return
	}

	policy, err := h.queries.GetRepositoryPolicyByGitHubRepoIDAndUser(r.Context(), db.GetRepositoryPolicyByGitHubRepoIDAndUserParams{
		UserID:       userID,
		GithubRepoID: req.RepoID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "repository is not assigned to this policy", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to validate repository policy", http.StatusInternalServerError)
		return
	}
	if policy.ID != policyID {
		http.Error(w, "repository is not assigned to this policy", http.StatusBadRequest)
		return
	}

	runID, err := h.enqueueRepositoryScan(r.Context(), userID, req.RepoID, "policy")
	if err != nil {
		http.Error(w, "failed to enqueue scan", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":    true,
		"repo_id":   req.RepoID,
		"policy_id": policyID,
		"scan_id":   runID,
		"status":    "queued",
	})
}

func (h *Handler) listScans(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.queries.ListRepositoryScanRunsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch scans", http.StatusInternalServerError)
		return
	}

	items := make([]scanRunResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapScanRunRow(row))
	}

	writeJSON(w, http.StatusOK, map[string]any{"scans": items})
}

func (h *Handler) getScan(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	scanID, err := strconv.ParseInt(chi.URLParam(r, "scanID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid scan id", http.StatusBadRequest)
		return
	}

	run, err := h.queries.GetRepositoryScanRunByIDAndUser(r.Context(), db.GetRepositoryScanRunByIDAndUserParams{UserID: userID, ID: scanID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "scan not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch scan", http.StatusInternalServerError)
		return
	}

	findingsRows, err := h.queries.ListRepositoryScanFindingsByRunAndUser(r.Context(), db.ListRepositoryScanFindingsByRunAndUserParams{UserID: userID, ScanRunID: scanID})
	if err != nil {
		http.Error(w, "failed to fetch scan findings", http.StatusInternalServerError)
		return
	}

	sourceRows, err := h.queries.ListRepositoryScanFindingSourcesByRunAndUser(r.Context(), db.ListRepositoryScanFindingSourcesByRunAndUserParams{UserID: userID, ScanRunID: scanID})
	if err != nil {
		http.Error(w, "failed to fetch scan finding sources", http.StatusInternalServerError)
		return
	}

	sourcesByFinding := make(map[int64][]string)
	for _, source := range sourceRows {
		sourcesByFinding[source.FindingID] = appendUniqueScanSource(sourcesByFinding[source.FindingID], source.Source)
	}

	findings := make([]scanFindingResponse, 0, len(findingsRows))
	for _, finding := range findingsRows {
		findings = append(findings, mapScanFindingByRunRow(finding, sourcesByFinding[finding.ID]))
	}

	logRows, err := h.queries.ListRepositoryScanLogsByRunAndUser(r.Context(), db.ListRepositoryScanLogsByRunAndUserParams{UserID: userID, ScanRunID: scanID})
	if err != nil {
		http.Error(w, "failed to fetch scan logs", http.StatusInternalServerError)
		return
	}
	logs := make([]scanLogResponse, 0, len(logRows))
	for _, row := range logRows {
		logs = append(logs, scanLogResponse{
			ID:            row.ID,
			Level:         row.Level,
			Message:       row.Message,
			DirectoryPath: strings.TrimSpace(row.DirectoryPath),
			CreatedAt:     timestamptzToString(row.CreatedAt),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scan":     mapScanRunDetailRow(run),
		"findings": findings,
		"logs":     logs,
	})
}

func (h *Handler) listFindings(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.queries.ListFindingsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch findings", http.StatusInternalServerError)
		return
	}

	items := make([]scanFindingResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapFindingListRow(row))
	}

	writeJSON(w, http.StatusOK, map[string]any{"findings": items})
}

func (h *Handler) getFinding(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	findingID, err := strconv.ParseInt(chi.URLParam(r, "findingID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid finding id", http.StatusBadRequest)
		return
	}

	row, err := h.queries.GetFindingByIDAndUser(r.Context(), db.GetFindingByIDAndUserParams{UserID: userID, ID: findingID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "finding not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch finding", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"finding": mapFindingDetailRow(row)})
}

func (h *Handler) repositoryScans(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := strconv.ParseInt(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}

	if _, connected := connectedRepositoryForUser(r.Context(), h.queries, userID, repoID); !connected {
		http.Error(w, "repository is not connected", http.StatusNotFound)
		return
	}

	rows, err := h.queries.ListRepositoryScanRunsByRepoAndUser(r.Context(), db.ListRepositoryScanRunsByRepoAndUserParams{UserID: userID, RepositoryID: repoID})
	if err != nil {
		http.Error(w, "failed to fetch repository scans", http.StatusInternalServerError)
		return
	}

	items := make([]scanRunResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapScanRunRepoRow(row))
	}

	writeJSON(w, http.StatusOK, map[string]any{"repository_id": repoID, "scans": items})
}

func (h *Handler) repositoryFindings(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := strconv.ParseInt(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}

	if _, connected := connectedRepositoryForUser(r.Context(), h.queries, userID, repoID); !connected {
		http.Error(w, "repository is not connected", http.StatusNotFound)
		return
	}

	findingsRows, err := h.queries.ListLatestRepositoryFindingsByRepoAndUser(r.Context(), db.ListLatestRepositoryFindingsByRepoAndUserParams{UserID: userID, RepositoryID: repoID})
	if err != nil {
		http.Error(w, "failed to fetch repository findings", http.StatusInternalServerError)
		return
	}

	sourceRows, err := h.queries.ListLatestRepositoryFindingSourcesByRepoAndUser(r.Context(), db.ListLatestRepositoryFindingSourcesByRepoAndUserParams{UserID: userID, RepositoryID: repoID})
	if err != nil {
		http.Error(w, "failed to fetch repository finding sources", http.StatusInternalServerError)
		return
	}

	sourcesByFinding := make(map[int64][]string)
	for _, source := range sourceRows {
		sourcesByFinding[source.FindingID] = appendUniqueScanSource(sourcesByFinding[source.FindingID], source.Source)
	}

	findings := make([]scanFindingResponse, 0, len(findingsRows))
	for _, finding := range findingsRows {
		findings = append(findings, mapLatestRepositoryFindingRow(finding, sourcesByFinding[finding.ID]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repository_id": repoID,
		"findings":      findings,
	})
}

func mapScanRunRow(row db.ListRepositoryScanRunsByUserRow) scanRunResponse {
	return scanRunResponse{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Repository:       row.RepositoryFullName,
		PolicyID:         int64PtrFromPg(row.PolicyID),
		Policy:           firstNonEmptyString(strings.TrimSpace(row.PolicyName), "Unassigned"),
		Trigger:          row.Trigger,
		Status:           row.Status,
		ErrorMessage:     strings.TrimSpace(row.ErrorMessage),
		FindingsTotal:    row.FindingsTotal,
		FindingsCritical: row.FindingsCritical,
		FindingsHigh:     row.FindingsHigh,
		FindingsMedium:   row.FindingsMedium,
		FindingsLow:      row.FindingsLow,
		StartedAt:        timestamptzToString(row.StartedAt),
		FinishedAt:       timestamptzToString(row.FinishedAt),
		Duration:         durationBetweenTimestamps(row.StartedAt, row.FinishedAt),
	}
}

func mapScanRunRepoRow(row db.ListRepositoryScanRunsByRepoAndUserRow) scanRunResponse {
	return scanRunResponse{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Repository:       row.RepositoryFullName,
		PolicyID:         int64PtrFromPg(row.PolicyID),
		Policy:           firstNonEmptyString(strings.TrimSpace(row.PolicyName), "Unassigned"),
		Trigger:          row.Trigger,
		Status:           row.Status,
		ErrorMessage:     strings.TrimSpace(row.ErrorMessage),
		FindingsTotal:    row.FindingsTotal,
		FindingsCritical: row.FindingsCritical,
		FindingsHigh:     row.FindingsHigh,
		FindingsMedium:   row.FindingsMedium,
		FindingsLow:      row.FindingsLow,
		StartedAt:        timestamptzToString(row.StartedAt),
		FinishedAt:       timestamptzToString(row.FinishedAt),
		Duration:         durationBetweenTimestamps(row.StartedAt, row.FinishedAt),
	}
}

func mapScanRunDetailRow(row db.GetRepositoryScanRunByIDAndUserRow) scanRunResponse {
	return scanRunResponse{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Repository:       row.RepositoryFullName,
		PolicyID:         int64PtrFromPg(row.PolicyID),
		Policy:           firstNonEmptyString(strings.TrimSpace(row.PolicyName), "Unassigned"),
		Trigger:          row.Trigger,
		Status:           row.Status,
		ErrorMessage:     strings.TrimSpace(row.ErrorMessage),
		FindingsTotal:    row.FindingsTotal,
		FindingsCritical: row.FindingsCritical,
		FindingsHigh:     row.FindingsHigh,
		FindingsMedium:   row.FindingsMedium,
		FindingsLow:      row.FindingsLow,
		StartedAt:        timestamptzToString(row.StartedAt),
		FinishedAt:       timestamptzToString(row.FinishedAt),
		Duration:         durationBetweenTimestamps(row.StartedAt, row.FinishedAt),
	}
}

func mapScanFinding(row db.RepositoryScanFinding, sources []string) scanFindingResponse {
	sort.Strings(sources)
	return scanFindingResponse{
		ID:              row.ID,
		ScanRunID:       row.ScanRunID,
		RepositoryID:    row.RepositoryID,
		PackageName:     row.PackageName,
		Manager:         row.Manager,
		Registry:        row.Registry,
		VersionSpec:     row.VersionSpec,
		ResolvedVersion: row.ResolvedVersion,
		AdvisoryID:      row.AdvisoryID,
		Aliases:         row.Aliases,
		Title:           row.Title,
		Summary:         row.Summary,
		Severity:        string(row.Severity),
		FixedVersion:    row.FixedVersion,
		ReferenceURL:    row.ReferenceUrl,
		Status:          row.Status,
		Sources:         sources,
		SourceLinks:     buildSourceLinks(row.AdvisoryID, row.Aliases, row.ReferenceUrl, sources),
		CreatedAt:       timestamptzToString(row.CreatedAt),
	}
}

func mapScanFindingByRunRow(row db.ListRepositoryScanFindingsByRunAndUserRow, sources []string) scanFindingResponse {
	sort.Strings(sources)
	return scanFindingResponse{
		ID:              row.ID,
		ScanRunID:       row.ScanRunID,
		RepositoryID:    row.RepositoryID,
		PackageName:     row.PackageName,
		Manager:         row.Manager,
		Registry:        row.Registry,
		VersionSpec:     row.VersionSpec,
		ResolvedVersion: row.ResolvedVersion,
		AdvisoryID:      row.AdvisoryID,
		Aliases:         row.Aliases,
		Title:           row.Title,
		Summary:         row.Summary,
		Severity:        string(row.Severity),
		FixedVersion:    row.FixedVersion,
		ReferenceURL:    row.ReferenceUrl,
		Status:          row.Status,
		Sources:         sources,
		SourceLinks:     buildSourceLinks(row.AdvisoryID, row.Aliases, row.ReferenceUrl, sources),
		CreatedAt:       timestamptzToString(row.CreatedAt),
	}
}

func mapLatestRepositoryFindingRow(row db.ListLatestRepositoryFindingsByRepoAndUserRow, sources []string) scanFindingResponse {
	sort.Strings(sources)
	return scanFindingResponse{
		ID:              row.ID,
		ScanRunID:       row.ScanRunID,
		RepositoryID:    row.RepositoryID,
		PackageName:     row.PackageName,
		Manager:         row.Manager,
		Registry:        row.Registry,
		VersionSpec:     row.VersionSpec,
		ResolvedVersion: row.ResolvedVersion,
		AdvisoryID:      row.AdvisoryID,
		Aliases:         row.Aliases,
		Title:           row.Title,
		Summary:         row.Summary,
		Severity:        string(row.Severity),
		FixedVersion:    row.FixedVersion,
		ReferenceURL:    row.ReferenceUrl,
		Status:          row.Status,
		Sources:         sources,
		SourceLinks:     buildSourceLinks(row.AdvisoryID, row.Aliases, row.ReferenceUrl, sources),
		CreatedAt:       timestamptzToString(row.CreatedAt),
	}
}

func mapFindingListRow(row db.ListFindingsByUserRow) scanFindingResponse {
	sources := dedupeAndSortSources(row.Sources)
	return scanFindingResponse{
		ID:              row.ID,
		ScanRunID:       row.ScanRunID,
		RepositoryID:    row.RepositoryID,
		Repository:      row.RepositoryFullName,
		Policy:          firstNonEmptyString(row.PolicyName, "Unassigned"),
		PackageName:     row.PackageName,
		Manager:         row.Manager,
		Registry:        row.Registry,
		VersionSpec:     row.VersionSpec,
		ResolvedVersion: row.ResolvedVersion,
		AdvisoryID:      row.AdvisoryID,
		Aliases:         row.Aliases,
		Title:           row.Title,
		Summary:         row.Summary,
		Severity:        string(row.Severity),
		FixedVersion:    row.FixedVersion,
		ReferenceURL:    row.ReferenceUrl,
		Status:          row.Status,
		Sources:         sources,
		SourceLinks:     buildSourceLinks(row.AdvisoryID, row.Aliases, row.ReferenceUrl, sources),
		CreatedAt:       timestamptzToString(row.CreatedAt),
	}
}

func mapFindingDetailRow(row db.GetFindingByIDAndUserRow) scanFindingResponse {
	sources := dedupeAndSortSources(row.Sources)
	return scanFindingResponse{
		ID:              row.ID,
		ScanRunID:       row.ScanRunID,
		RepositoryID:    row.RepositoryID,
		Repository:      row.RepositoryFullName,
		Policy:          firstNonEmptyString(row.PolicyName, "Unassigned"),
		PackageName:     row.PackageName,
		Manager:         row.Manager,
		Registry:        row.Registry,
		VersionSpec:     row.VersionSpec,
		ResolvedVersion: row.ResolvedVersion,
		AdvisoryID:      row.AdvisoryID,
		Aliases:         row.Aliases,
		Title:           row.Title,
		Summary:         row.Summary,
		Severity:        string(row.Severity),
		FixedVersion:    row.FixedVersion,
		ReferenceURL:    row.ReferenceUrl,
		Status:          row.Status,
		Sources:         sources,
		SourceLinks:     buildSourceLinks(row.AdvisoryID, row.Aliases, row.ReferenceUrl, sources),
		CreatedAt:       timestamptzToString(row.CreatedAt),
	}
}

func int64PtrFromPg(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func durationBetweenTimestamps(startedAt, finishedAt pgtype.Timestamptz) string {
	if !startedAt.Valid || !finishedAt.Valid {
		return ""
	}
	delta := finishedAt.Time.Sub(startedAt.Time)
	if delta < 0 {
		return ""
	}
	if delta < time.Minute {
		return strconv.FormatInt(int64(delta.Seconds()), 10) + "s"
	}
	minutes := int64(delta / time.Minute)
	seconds := int64((delta % time.Minute) / time.Second)
	if minutes < 60 {
		return strconv.FormatInt(minutes, 10) + "m " + strconv.FormatInt(seconds, 10) + "s"
	}
	hours := minutes / 60
	remainMin := minutes % 60
	return strconv.FormatInt(hours, 10) + "h " + strconv.FormatInt(remainMin, 10) + "m"
}

func appendUniqueScanSource(items []string, source string) []string {
	v := strings.TrimSpace(strings.ToLower(source))
	if v == "" {
		return items
	}
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
}

func firstNonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func dedupeAndSortSources(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(strings.ToLower(value))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		items = append(items, v)
	}
	sort.Strings(items)
	return items
}

func buildSourceLinks(advisoryID string, aliases []string, referenceURL string, sources []string) []sourceLinkResponse {
	items := make([]sourceLinkResponse, 0, 6)
	seen := make(map[string]struct{})

	add := func(source, url string) {
		src := strings.TrimSpace(strings.ToLower(source))
		link := strings.TrimSpace(url)
		if src == "" || link == "" {
			return
		}
		key := src + "|" + link
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, sourceLinkResponse{Source: src, URL: link})
	}

	advisory := strings.TrimSpace(advisoryID)
	allAliases := append([]string{advisory}, aliases...)
	for _, alias := range allAliases {
		value := strings.TrimSpace(alias)
		upper := strings.ToUpper(value)
		switch {
		case strings.HasPrefix(upper, "OSV-"):
			add("osv", "https://osv.dev/vulnerability/"+value)
		case strings.HasPrefix(upper, "GHSA-"):
			add("ghsa", "https://github.com/advisories/"+value)
		case strings.HasPrefix(upper, "CVE-"):
			add("nvd", "https://nvd.nist.gov/vuln/detail/"+value)
		}
	}

	for _, source := range sources {
		src := strings.ToLower(strings.TrimSpace(source))
		if src == "custom" && strings.TrimSpace(referenceURL) != "" {
			add("custom", referenceURL)
		}
	}

	if strings.TrimSpace(referenceURL) != "" {
		add("reference", referenceURL)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		return items[i].URL < items[j].URL
	})

	return items
}
