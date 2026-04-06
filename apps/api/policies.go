package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	db "github.com/chann44/TGE/internals/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type policyTriggerInput struct {
	Type     string   `json:"type"`
	Branches []string `json:"branches"`
	Cron     string   `json:"cron"`
	Timezone string   `json:"timezone"`
}

type policySourcesInput struct {
	RegistryFirst      bool   `json:"registry_first"`
	RegistryMaxAgeDays int32  `json:"registry_max_age_days"`
	RegistryOnly       bool   `json:"registry_only"`
	OsvEnabled         bool   `json:"osv_enabled"`
	GhsaEnabled        bool   `json:"ghsa_enabled"`
	GhsaTokenRef       string `json:"ghsa_token_ref"`
	NvdEnabled         bool   `json:"nvd_enabled"`
	NvdApiKeyRef       string `json:"nvd_api_key_ref"`
	GovulncheckEnabled bool   `json:"govulncheck_enabled"`
	SupplyChainEnabled bool   `json:"supply_chain_enabled"`
}

type policyCustomSourceInput struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Format     string `json:"format"`
	AuthHeader string `json:"auth_header"`
}

type policySASTInput struct {
	Enabled           bool     `json:"enabled"`
	PatternsEnabled   bool     `json:"patterns_enabled"`
	Rulesets          []string `json:"rulesets"`
	MinSeverity       string   `json:"min_severity"`
	ExcludePaths      []string `json:"exclude_paths"`
	AiEnabled         bool     `json:"ai_enabled"`
	AiMaxFilesPerScan int32    `json:"ai_max_files_per_scan"`
	AiReachability    bool     `json:"ai_reachability"`
	AiSuggestFix      bool     `json:"ai_suggest_fix"`
}

type policyRegistryInput struct {
	PushEnabled       bool     `json:"push_enabled"`
	PushURL           string   `json:"push_url"`
	PushSigningKeyRef string   `json:"push_signing_key_ref"`
	PullEnabled       bool     `json:"pull_enabled"`
	PullURL           string   `json:"pull_url"`
	PullTrustedKeys   []string `json:"pull_trusted_keys"`
	PullMaxAgeDays    int32    `json:"pull_max_age_days"`
}

type policyUpsertRequest struct {
	Name          string                    `json:"name"`
	Enabled       *bool                     `json:"enabled"`
	Triggers      []policyTriggerInput      `json:"triggers"`
	Sources       *policySourcesInput       `json:"sources"`
	CustomSources []policyCustomSourceInput `json:"custom_sources"`
	SAST          *policySASTInput          `json:"sast"`
	Registry      *policyRegistryInput      `json:"registry"`
}

type policyEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type assignRepositoryPolicyRequest struct {
	PolicyID int64 `json:"policy_id"`
}

type policySummaryResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	RepositoryCount int64  `json:"repository_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type policyTriggerResponse struct {
	ID        int64    `json:"id"`
	Type      string   `json:"type"`
	Branches  []string `json:"branches"`
	Cron      string   `json:"cron"`
	Timezone  string   `json:"timezone"`
	CreatedAt string   `json:"created_at"`
}

type policySourcesResponse struct {
	RegistryFirst      bool   `json:"registry_first"`
	RegistryMaxAgeDays int32  `json:"registry_max_age_days"`
	RegistryOnly       bool   `json:"registry_only"`
	OsvEnabled         bool   `json:"osv_enabled"`
	GhsaEnabled        bool   `json:"ghsa_enabled"`
	GhsaTokenRef       string `json:"ghsa_token_ref"`
	NvdEnabled         bool   `json:"nvd_enabled"`
	NvdApiKeyRef       string `json:"nvd_api_key_ref"`
	GovulncheckEnabled bool   `json:"govulncheck_enabled"`
}

type policyCustomSourceResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Format     string `json:"format"`
	AuthHeader string `json:"auth_header"`
}

type policySASTResponse struct {
	Enabled           bool     `json:"enabled"`
	PatternsEnabled   bool     `json:"patterns_enabled"`
	Rulesets          []string `json:"rulesets"`
	MinSeverity       string   `json:"min_severity"`
	ExcludePaths      []string `json:"exclude_paths"`
	AiEnabled         bool     `json:"ai_enabled"`
	AiMaxFilesPerScan int32    `json:"ai_max_files_per_scan"`
	AiReachability    bool     `json:"ai_reachability"`
	AiSuggestFix      bool     `json:"ai_suggest_fix"`
}

type policyRegistryResponse struct {
	PushEnabled       bool     `json:"push_enabled"`
	PushURL           string   `json:"push_url"`
	PushSigningKeyRef string   `json:"push_signing_key_ref"`
	PullEnabled       bool     `json:"pull_enabled"`
	PullURL           string   `json:"pull_url"`
	PullTrustedKeys   []string `json:"pull_trusted_keys"`
	PullMaxAgeDays    int32    `json:"pull_max_age_days"`
}

type policyRepositoryResponse struct {
	RepositoryID int64  `json:"repository_id"`
	FullName     string `json:"full_name"`
	AssignedAt   string `json:"assigned_at"`
}

type policyDetailResponse struct {
	ID            int64                        `json:"id"`
	Name          string                       `json:"name"`
	Enabled       bool                         `json:"enabled"`
	CreatedAt     string                       `json:"created_at"`
	UpdatedAt     string                       `json:"updated_at"`
	Triggers      []policyTriggerResponse      `json:"triggers"`
	Sources       policySourcesResponse        `json:"sources"`
	CustomSources []policyCustomSourceResponse `json:"custom_sources"`
	SAST          policySASTResponse           `json:"sast"`
	Registry      policyRegistryResponse       `json:"registry"`
	Repositories  []policyRepositoryResponse   `json:"repositories"`
}

func (h *Handler) listPolicies(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.queries.ListPoliciesByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to list policies", http.StatusInternalServerError)
		return
	}

	items := make([]policySummaryResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, policySummaryResponse{
			ID:              row.ID,
			Name:            row.Name,
			Enabled:         row.Enabled,
			RepositoryCount: row.RepositoryCount,
			CreatedAt:       timestamptzToString(row.CreatedAt),
			UpdatedAt:       timestamptzToString(row.UpdatedAt),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req policyUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	policy, err := h.queries.CreatePolicy(r.Context(), db.CreatePolicyParams{
		UserID:  userID,
		Name:    name,
		Enabled: enabled,
	})
	if err != nil {
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}

	if err := h.applyPolicyConfig(r.Context(), policy.ID, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := h.policyDetailResponse(r.Context(), userID, policy, true)
	if err != nil {
		http.Error(w, "failed to load created policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) getPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID, err := policyIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	policy, err := h.queries.GetPolicyByIDAndUser(r.Context(), db.GetPolicyByIDAndUserParams{ID: policyID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	response, err := h.policyDetailResponse(r.Context(), userID, policy, true)
	if err != nil {
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updatePolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID, err := policyIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existing, err := h.queries.GetPolicyByIDAndUser(r.Context(), db.GetPolicyByIDAndUserParams{ID: policyID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	var req policyUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(existing.Name)
	}
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	policy, err := h.queries.UpdatePolicyByIDAndUser(r.Context(), db.UpdatePolicyByIDAndUserParams{
		ID:      policyID,
		UserID:  userID,
		Name:    name,
		Enabled: enabled,
	})
	if err != nil {
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}

	if err := h.applyPolicyConfig(r.Context(), policyID, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := h.policyDetailResponse(r.Context(), userID, policy, true)
	if err != nil {
		http.Error(w, "failed to fetch updated policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) setPolicyEnabled(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID, err := policyIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req policyEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	policy, err := h.queries.SetPolicyEnabledByIDAndUser(r.Context(), db.SetPolicyEnabledByIDAndUserParams{
		ID:      policyID,
		UserID:  userID,
		Enabled: req.Enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         policy.ID,
		"enabled":    policy.Enabled,
		"updated_at": timestamptzToString(policy.UpdatedAt),
	})
}

func (h *Handler) deletePolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID, err := policyIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	err = h.queries.DeletePolicyByIDAndUser(r.Context(), db.DeletePolicyByIDAndUserParams{ID: policyID, UserID: userID})
	tx, err := h.postgres.Begin(r.Context())
	if err != nil {
		http.Error(w, "failed to delete policy", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	txQueries := h.queries.WithTx(tx)
	if err := txQueries.DeletePolicyRepositoryAssignmentsByPolicy(r.Context(), policyID); err != nil {
		http.Error(w, "failed to delete policy", http.StatusInternalServerError)
		return
	}

	err = txQueries.DeletePolicyByIDAndUser(r.Context(), db.DeletePolicyByIDAndUserParams{ID: policyID, UserID: userID})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			http.Error(w, "policy is assigned to at least one repository", http.StatusConflict)
			return
		}
		http.Error(w, "failed to delete policy", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to delete policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getRepositoryPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := repoIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	policy, err := h.queries.GetRepositoryPolicyByGitHubRepoIDAndUser(r.Context(), db.GetRepositoryPolicyByGitHubRepoIDAndUserParams{UserID: userID, GithubRepoID: repoID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not assigned", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch repository policy", http.StatusInternalServerError)
		return
	}

	response, err := h.policyDetailResponse(r.Context(), userID, policy, false)
	if err != nil {
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) assignRepositoryPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := repoIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req assignRepositoryPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.PolicyID == 0 {
		http.Error(w, "policy_id is required", http.StatusBadRequest)
		return
	}

	if _, err := h.queries.GetPolicyByIDAndUser(r.Context(), db.GetPolicyByIDAndUserParams{ID: req.PolicyID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch policy", http.StatusInternalServerError)
		return
	}

	repo, err := h.queries.GetUserRepositoryByGitHubRepoID(r.Context(), db.GetUserRepositoryByGitHubRepoIDParams{UserID: userID, GithubRepoID: repoID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "repository is not connected", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch repository", http.StatusInternalServerError)
		return
	}

	if err := h.queries.AssignPolicyToRepository(r.Context(), db.AssignPolicyToRepositoryParams{RepositoryID: repo.ID, PolicyID: req.PolicyID}); err != nil {
		http.Error(w, "failed to assign policy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repository_id": repo.GithubRepoID,
		"policy_id":     req.PolicyID,
		"assigned":      true,
	})
}

func (h *Handler) unassignRepositoryPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoID, err := repoIDFromURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repo, err := h.queries.GetUserRepositoryByGitHubRepoID(r.Context(), db.GetUserRepositoryByGitHubRepoIDParams{UserID: userID, GithubRepoID: repoID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "repository is not connected", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch repository", http.StatusInternalServerError)
		return
	}

	if err := h.queries.UnassignPolicyFromRepository(r.Context(), repo.ID); err != nil {
		http.Error(w, "failed to unassign policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) applyPolicyConfig(ctx context.Context, policyID int64, req policyUpsertRequest) error {
	sources := defaultPolicySourcesInput()
	if req.Sources != nil {
		sources = *req.Sources
	}
	if sources.RegistryMaxAgeDays <= 0 {
		sources.RegistryMaxAgeDays = 7
	}

	if err := h.queries.UpsertPolicySources(ctx, db.UpsertPolicySourcesParams{
		PolicyID:           policyID,
		RegistryFirst:      sources.RegistryFirst,
		RegistryMaxAgeDays: sources.RegistryMaxAgeDays,
		RegistryOnly:       sources.RegistryOnly,
		OsvEnabled:         sources.OsvEnabled,
		GhsaEnabled:        sources.GhsaEnabled,
		GhsaTokenRef:       strings.TrimSpace(sources.GhsaTokenRef),
		NvdEnabled:         sources.NvdEnabled,
		NvdApiKeyRef:       strings.TrimSpace(sources.NvdApiKeyRef),
		GovulncheckEnabled: sources.GovulncheckEnabled,
		SupplyChainEnabled: sources.SupplyChainEnabled,
	}); err != nil {
		return fmt.Errorf("failed to upsert policy sources")
	}

	sast := defaultPolicySASTInput()
	if req.SAST != nil {
		sast = *req.SAST
	}
	if sast.AiMaxFilesPerScan <= 0 {
		sast.AiMaxFilesPerScan = 50
	}
	if len(sast.Rulesets) == 0 {
		sast.Rulesets = []string{"default"}
	}
	severity, err := parseSeverity(sast.MinSeverity)
	if err != nil {
		return err
	}

	if err := h.queries.UpsertPolicySast(ctx, db.UpsertPolicySastParams{
		PolicyID:          policyID,
		Enabled:           sast.Enabled,
		PatternsEnabled:   sast.PatternsEnabled,
		Rulesets:          cleanStringList(sast.Rulesets),
		MinSeverity:       severity,
		ExcludePaths:      cleanStringListNonNil(sast.ExcludePaths),
		AiEnabled:         sast.AiEnabled,
		AiMaxFilesPerScan: sast.AiMaxFilesPerScan,
		AiReachability:    sast.AiReachability,
		AiSuggestFix:      sast.AiSuggestFix,
	}); err != nil {
		return fmt.Errorf("failed to upsert policy sast")
	}

	registry := defaultPolicyRegistryInput()
	if req.Registry != nil {
		registry = *req.Registry
	}
	if registry.PullMaxAgeDays <= 0 {
		registry.PullMaxAgeDays = 7
	}

	if err := h.queries.UpsertPolicyRegistry(ctx, db.UpsertPolicyRegistryParams{
		PolicyID:          policyID,
		PushEnabled:       registry.PushEnabled,
		PushUrl:           strings.TrimSpace(registry.PushURL),
		PushSigningKeyRef: strings.TrimSpace(registry.PushSigningKeyRef),
		PullEnabled:       registry.PullEnabled,
		PullUrl:           strings.TrimSpace(registry.PullURL),
		PullTrustedKeys:   cleanStringListNonNil(registry.PullTrustedKeys),
		PullMaxAgeDays:    registry.PullMaxAgeDays,
	}); err != nil {
		return fmt.Errorf("failed to upsert policy registry")
	}

	if err := h.queries.DeletePolicyCustomSourcesByPolicy(ctx, policyID); err != nil {
		return fmt.Errorf("failed to reset custom sources")
	}
	for _, custom := range req.CustomSources {
		name := strings.TrimSpace(custom.Name)
		url := strings.TrimSpace(custom.URL)
		if name == "" || url == "" {
			continue
		}
		format, err := parseCustomSourceFormat(custom.Format)
		if err != nil {
			return err
		}
		if err := h.queries.CreatePolicyCustomSource(ctx, db.CreatePolicyCustomSourceParams{
			PolicyID:   policyID,
			Name:       name,
			Url:        url,
			Format:     format,
			AuthHeader: strings.TrimSpace(custom.AuthHeader),
		}); err != nil {
			return fmt.Errorf("failed to create custom source")
		}
	}

	triggers := req.Triggers
	if len(triggers) == 0 {
		triggers = []policyTriggerInput{{Type: string(db.TriggerTypeManual)}}
	}

	if err := h.queries.DeletePolicyTriggersByPolicy(ctx, policyID); err != nil {
		return fmt.Errorf("failed to reset triggers")
	}

	for _, trigger := range triggers {
		triggerType, err := parseTriggerType(trigger.Type)
		if err != nil {
			return err
		}
		cron := pgtype.Text{}
		cronValue := strings.TrimSpace(trigger.Cron)
		if cronValue != "" {
			cron = pgtype.Text{String: cronValue, Valid: true}
		}
		timezone := strings.TrimSpace(trigger.Timezone)
		if timezone == "" {
			timezone = "UTC"
		}
		if err := h.queries.CreatePolicyTrigger(ctx, db.CreatePolicyTriggerParams{
			PolicyID: policyID,
			Type:     triggerType,
			Branches: cleanStringList(trigger.Branches),
			Cron:     cron,
			Timezone: timezone,
		}); err != nil {
			return fmt.Errorf("failed to create policy trigger")
		}
	}

	return nil
}

func (h *Handler) policyDetailResponse(ctx context.Context, userID int64, policy db.Policy, includeRepositories bool) (policyDetailResponse, error) {
	triggers, err := h.queries.ListPolicyTriggersByPolicy(ctx, policy.ID)
	if err != nil {
		return policyDetailResponse{}, err
	}

	sources, err := h.queries.GetPolicySourcesByPolicy(ctx, policy.ID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return policyDetailResponse{}, err
		}
		sources = db.PolicySource{
			PolicyID:           policy.ID,
			RegistryFirst:      true,
			RegistryMaxAgeDays: 7,
			RegistryOnly:       false,
			OsvEnabled:         true,
			GhsaEnabled:        true,
			GhsaTokenRef:       "",
			NvdEnabled:         true,
			NvdApiKeyRef:       "",
			GovulncheckEnabled: true,
			SupplyChainEnabled: false,
		}
	}

	sast, err := h.queries.GetPolicySastByPolicy(ctx, policy.ID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return policyDetailResponse{}, err
		}
		sast = db.PolicySast{
			PolicyID:          policy.ID,
			Enabled:           true,
			PatternsEnabled:   true,
			Rulesets:          []string{"default"},
			MinSeverity:       db.SeverityMedium,
			ExcludePaths:      []string{},
			AiEnabled:         false,
			AiMaxFilesPerScan: 50,
			AiReachability:    true,
			AiSuggestFix:      true,
		}
	}

	registry, err := h.queries.GetPolicyRegistryByPolicy(ctx, policy.ID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return policyDetailResponse{}, err
		}
		registry = db.PolicyRegistry{
			PolicyID:          policy.ID,
			PushEnabled:       false,
			PushUrl:           "",
			PushSigningKeyRef: "",
			PullEnabled:       false,
			PullUrl:           "",
			PullTrustedKeys:   []string{},
			PullMaxAgeDays:    7,
		}
	}

	customSources, err := h.queries.ListPolicyCustomSourcesByPolicy(ctx, policy.ID)
	if err != nil {
		return policyDetailResponse{}, err
	}

	repos := make([]policyRepositoryResponse, 0)
	if includeRepositories {
		repoRows, repoErr := h.queries.ListPolicyRepositoriesByPolicyAndUser(ctx, db.ListPolicyRepositoriesByPolicyAndUserParams{PolicyID: policy.ID, UserID: userID})
		if repoErr != nil {
			return policyDetailResponse{}, repoErr
		}
		for _, row := range repoRows {
			repos = append(repos, policyRepositoryResponse{
				RepositoryID: row.GithubRepoID,
				FullName:     row.FullName,
				AssignedAt:   timestamptzToString(row.AssignedAt),
			})
		}
	}

	triggerItems := make([]policyTriggerResponse, 0, len(triggers))
	for _, trigger := range triggers {
		triggerItems = append(triggerItems, policyTriggerResponse{
			ID:        trigger.ID,
			Type:      string(trigger.Type),
			Branches:  trigger.Branches,
			Cron:      textToString(trigger.Cron),
			Timezone:  trigger.Timezone,
			CreatedAt: timestamptzToString(trigger.CreatedAt),
		})
	}

	customItems := make([]policyCustomSourceResponse, 0, len(customSources))
	for _, source := range customSources {
		customItems = append(customItems, policyCustomSourceResponse{
			ID:         source.ID,
			Name:       source.Name,
			URL:        source.Url,
			Format:     string(source.Format),
			AuthHeader: source.AuthHeader,
		})
	}

	return policyDetailResponse{
		ID:        policy.ID,
		Name:      policy.Name,
		Enabled:   policy.Enabled,
		CreatedAt: timestamptzToString(policy.CreatedAt),
		UpdatedAt: timestamptzToString(policy.UpdatedAt),
		Triggers:  triggerItems,
		Sources: policySourcesResponse{
			RegistryFirst:      sources.RegistryFirst,
			RegistryMaxAgeDays: sources.RegistryMaxAgeDays,
			RegistryOnly:       sources.RegistryOnly,
			OsvEnabled:         sources.OsvEnabled,
			GhsaEnabled:        sources.GhsaEnabled,
			GhsaTokenRef:       sources.GhsaTokenRef,
			NvdEnabled:         sources.NvdEnabled,
			NvdApiKeyRef:       sources.NvdApiKeyRef,
			GovulncheckEnabled: sources.GovulncheckEnabled,
		},
		CustomSources: customItems,
		SAST: policySASTResponse{
			Enabled:           sast.Enabled,
			PatternsEnabled:   sast.PatternsEnabled,
			Rulesets:          sast.Rulesets,
			MinSeverity:       string(sast.MinSeverity),
			ExcludePaths:      sast.ExcludePaths,
			AiEnabled:         sast.AiEnabled,
			AiMaxFilesPerScan: sast.AiMaxFilesPerScan,
			AiReachability:    sast.AiReachability,
			AiSuggestFix:      sast.AiSuggestFix,
		},
		Registry: policyRegistryResponse{
			PushEnabled:       registry.PushEnabled,
			PushURL:           registry.PushUrl,
			PushSigningKeyRef: registry.PushSigningKeyRef,
			PullEnabled:       registry.PullEnabled,
			PullURL:           registry.PullUrl,
			PullTrustedKeys:   registry.PullTrustedKeys,
			PullMaxAgeDays:    registry.PullMaxAgeDays,
		},
		Repositories: repos,
	}, nil
}

func parseTriggerType(value string) (db.TriggerType, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", string(db.TriggerTypeManual):
		return db.TriggerTypeManual, nil
	case string(db.TriggerTypePush):
		return db.TriggerTypePush, nil
	case string(db.TriggerTypePullRequest):
		return db.TriggerTypePullRequest, nil
	case string(db.TriggerTypeSchedule):
		return db.TriggerTypeSchedule, nil
	default:
		return "", fmt.Errorf("invalid trigger type %q", value)
	}
}

func parseSeverity(value string) (db.Severity, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", string(db.SeverityMedium):
		return db.SeverityMedium, nil
	case string(db.SeverityLow):
		return db.SeverityLow, nil
	case string(db.SeverityHigh):
		return db.SeverityHigh, nil
	case string(db.SeverityCritical):
		return db.SeverityCritical, nil
	default:
		return "", fmt.Errorf("invalid severity %q", value)
	}
}

func parseCustomSourceFormat(value string) (db.CustomSourceFormat, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", string(db.CustomSourceFormatOsv):
		return db.CustomSourceFormatOsv, nil
	case string(db.CustomSourceFormatNvd):
		return db.CustomSourceFormatNvd, nil
	default:
		return "", fmt.Errorf("invalid custom source format %q", value)
	}
}

func cleanStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	clean := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		clean = append(clean, v)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func cleanStringListNonNil(items []string) []string {
	clean := cleanStringList(items)
	if clean == nil {
		return []string{}
	}
	return clean
}

func defaultPolicySourcesInput() policySourcesInput {
	return policySourcesInput{
		RegistryFirst:      true,
		RegistryMaxAgeDays: 7,
		RegistryOnly:       false,
		OsvEnabled:         true,
		GhsaEnabled:        true,
		GhsaTokenRef:       "",
		NvdEnabled:         true,
		NvdApiKeyRef:       "",
		GovulncheckEnabled: true,
		SupplyChainEnabled: false,
	}
}

func defaultPolicySASTInput() policySASTInput {
	return policySASTInput{
		Enabled:           true,
		PatternsEnabled:   true,
		Rulesets:          []string{"default"},
		MinSeverity:       string(db.SeverityMedium),
		ExcludePaths:      nil,
		AiEnabled:         false,
		AiMaxFilesPerScan: 50,
		AiReachability:    true,
		AiSuggestFix:      true,
	}
}

func defaultPolicyRegistryInput() policyRegistryInput {
	return policyRegistryInput{
		PushEnabled:       false,
		PushURL:           "",
		PushSigningKeyRef: "",
		PullEnabled:       false,
		PullURL:           "",
		PullTrustedKeys:   nil,
		PullMaxAgeDays:    7,
	}
}

func policyIDFromURL(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, "policyID"))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid policy id")
	}
	return value, nil
}

func repoIDFromURL(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, "repoID"))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid repository id")
	}
	return value, nil
}
