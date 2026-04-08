package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/chann44/TGE/adapters"
	db "github.com/chann44/TGE/internals/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var supportedIntegrationProviders = map[string]struct{}{
	"github":  {},
	"slack":   {},
	"jira":    {},
	"linear":  {},
	"discord": {},
}

type connectIntegrationRequest struct {
	Name         string `json:"name"`
	Enabled      *bool  `json:"enabled"`
	WebhookURL   string `json:"webhook_url"`
	JiraBaseURL  string `json:"jira_base_url"`
	JiraEmail    string `json:"jira_email"`
	JiraAPIToken string `json:"jira_api_token"`
	JiraProject  string `json:"jira_project_key"`
	LinearToken  string `json:"linear_api_token"`
	LinearTeamID string `json:"linear_team_id"`
}

type sendIntegrationMessageRequest struct {
	Title    string         `json:"title"`
	Text     string         `json:"text"`
	Severity string         `json:"severity"`
	Metadata map[string]any `json:"metadata"`
}

type createIntegrationIssueRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Labels      []string `json:"labels"`
	RepoID      int64    `json:"repo_id"`
	ProjectKey  string   `json:"project_key"`
}

type linearTeamResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type listLinearTeamsRequest struct {
	LinearToken string `json:"linear_api_token"`
}

type integrationResponse struct {
	Provider    string         `json:"provider"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Enabled     bool           `json:"enabled"`
	ConnectedAt string         `json:"connected_at,omitempty"`
	LastError   string         `json:"last_error,omitempty"`
	Config      map[string]any `json:"config"`
	UpdatedAt   string         `json:"updated_at"`
}

type integrationActivityResponse struct {
	ID        int64          `json:"id"`
	Provider  string         `json:"provider"`
	Action    string         `json:"action"`
	Status    string         `json:"status"`
	Detail    string         `json:"detail"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt string         `json:"created_at"`
}

func (h *Handler) listIntegrations(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.queries.ListIntegrationsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch integrations", http.StatusInternalServerError)
		return
	}

	items := make([]integrationResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapIntegrationRow(row))
	}

	writeJSON(w, http.StatusOK, map[string]any{"integrations": items})
}

func (h *Handler) getIntegration(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	provider, ok := integrationProviderFromPath(w, r)
	if !ok {
		return
	}

	row, err := h.queries.GetIntegrationByProviderAndUser(r.Context(), db.GetIntegrationByProviderAndUserParams{
		UserID:   userID,
		Provider: provider,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "integration not connected", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch integration", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"integration": mapIntegrationRow(row)})
}

func (h *Handler) connectIntegration(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	provider, ok := integrationProviderFromPath(w, r)
	if !ok {
		return
	}

	var req connectIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existingConfig := map[string]any{}
	if existing, err := h.queries.GetIntegrationByProviderAndUser(r.Context(), db.GetIntegrationByProviderAndUserParams{
		UserID:   userID,
		Provider: provider,
	}); err == nil {
		existingConfig = decodeIntegrationConfig(existing.Config)
	}

	config, err := integrationConfigFromConnectRequest(r.Context(), provider, req, existingConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		http.Error(w, "failed to encode integration config", http.StatusInternalServerError)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = providerDisplayName(provider)
	}
	h.requestLog(r, "integration connect requested user_id=%d provider=%s enabled=%t", userID, provider, enabled)

	row, err := h.queries.UpsertIntegration(r.Context(), db.UpsertIntegrationParams{
		UserID:   userID,
		Provider: provider,
		Name:     name,
		Status:   "connected",
		Enabled:  enabled,
		Config:   configBytes,
	})
	if err != nil {
		http.Error(w, "failed to save integration", http.StatusInternalServerError)
		return
	}

	h.logIntegrationActivity(r.Context(), userID, pgtype.Int8{Int64: row.ID, Valid: true}, provider, "connected", "success", "integration connected", map[string]any{})

	writeJSON(w, http.StatusOK, map[string]any{"integration": mapIntegrationRow(row)})
}

func (h *Handler) listLinearTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req listLinearTeamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	apiToken := strings.TrimSpace(req.LinearToken)
	if apiToken == "" {
		http.Error(w, "linear_api_token is required", http.StatusBadRequest)
		return
	}

	teams, err := adapters.ListLinearTeams(r.Context(), apiToken)
	if err != nil {
		h.requestLog(r, "linear team fetch failed user_id=%d error=%v", userID, err)
		http.Error(w, "failed to fetch linear teams", http.StatusBadGateway)
		return
	}

	result := make([]linearTeamResponse, 0, len(teams))
	for _, team := range teams {
		if strings.TrimSpace(team.ID) == "" {
			continue
		}
		result = append(result, linearTeamResponse{
			ID:   strings.TrimSpace(team.ID),
			Key:  strings.TrimSpace(team.Key),
			Name: strings.TrimSpace(team.Name),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"teams": result})
}

func (h *Handler) sendIntegrationMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	provider, ok := integrationProviderFromPath(w, r)
	if !ok {
		return
	}

	integration, ok := h.connectedIntegrationForProvider(w, r, userID, provider)
	if !ok {
		return
	}

	var req sendIntegrationMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	messageText := strings.TrimSpace(req.Text)
	if messageText == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	config := decodeIntegrationConfig(integration.Config)
	formatted := formatIntegrationMessage(req.Title, req.Severity, messageText)
	h.requestLog(r, "integration message test requested user_id=%d provider=%s integration_id=%d", userID, provider, integration.ID)

	var err error
	switch provider {
	case "slack":
		webhook := configString(config, "webhook_url")
		if webhook == "" {
			h.requestLog(r, "integration message missing slack webhook user_id=%d integration_id=%d", userID, integration.ID)
			http.Error(w, "slack webhook_url is not configured", http.StatusBadRequest)
			return
		}
		if !isLikelyWebhookURL(webhook) {
			h.requestLog(r, "integration message invalid slack webhook user_id=%d integration_id=%d webhook=%s", userID, integration.ID, maskSecret(webhook))
			http.Error(w, "slack webhook_url is invalid; paste full https://hooks.slack.com/services/... URL and save again", http.StatusBadRequest)
			return
		}
		h.requestLog(r, "integration message sending slack user_id=%d integration_id=%d webhook=%s", userID, integration.ID, maskSecret(webhook))
		err = adapters.SendSlackWebhookMessage(r.Context(), webhook, formatted)
	case "discord":
		webhook := configString(config, "webhook_url")
		if webhook == "" {
			h.requestLog(r, "integration message missing discord webhook user_id=%d integration_id=%d", userID, integration.ID)
			http.Error(w, "discord webhook_url is not configured", http.StatusBadRequest)
			return
		}
		if !isLikelyWebhookURL(webhook) {
			h.requestLog(r, "integration message invalid discord webhook user_id=%d integration_id=%d webhook=%s", userID, integration.ID, maskSecret(webhook))
			http.Error(w, "discord webhook_url is invalid; paste full webhook URL and save again", http.StatusBadRequest)
			return
		}
		h.requestLog(r, "integration message sending discord user_id=%d integration_id=%d webhook=%s", userID, integration.ID, maskSecret(webhook))
		err = adapters.SendDiscordWebhookMessage(r.Context(), webhook, formatted)
	default:
		http.Error(w, "provider does not support outbound messages", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.requestLog(r, "integration message send failed user_id=%d provider=%s integration_id=%d error=%v", userID, provider, integration.ID, err)
		_ = h.queries.UpdateIntegrationStatus(r.Context(), db.UpdateIntegrationStatusParams{
			UserID:    userID,
			Provider:  provider,
			Status:    "error",
			LastError: truncateForDB(err.Error(), 8000),
		})
		h.logIntegrationActivity(r.Context(), userID, pgtype.Int8{Int64: integration.ID, Valid: true}, provider, "message_sent", "failed", "failed to send message", map[string]any{"error": err.Error()})
		http.Error(w, "failed to send integration message", http.StatusBadGateway)
		return
	}

	_ = h.queries.UpdateIntegrationStatus(r.Context(), db.UpdateIntegrationStatusParams{
		UserID:    userID,
		Provider:  provider,
		Status:    "connected",
		LastError: "",
	})
	h.logIntegrationActivity(r.Context(), userID, pgtype.Int8{Int64: integration.ID, Valid: true}, provider, "message_sent", "success", "message sent", req.Metadata)
	h.requestLog(r, "integration message send success user_id=%d provider=%s integration_id=%d", userID, provider, integration.ID)

	writeJSON(w, http.StatusOK, map[string]any{"sent": true, "provider": provider})
}

func (h *Handler) createIntegrationIssue(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	provider, ok := integrationProviderFromPath(w, r)
	if !ok {
		return
	}

	integration, ok := h.connectedIntegrationForProvider(w, r, userID, provider)
	if !ok {
		return
	}

	var req createIntegrationIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	config := decodeIntegrationConfig(integration.Config)
	description := formatIntegrationMessage("", req.Severity, req.Description)

	result := map[string]any{"provider": provider}
	var createErr error

	switch provider {
	case "github":
		if req.RepoID <= 0 {
			http.Error(w, "repo_id is required for github issue creation", http.StatusBadRequest)
			return
		}
		token, repo, err := h.installationTokenForRepo(r.Context(), userID, req.RepoID)
		if err != nil {
			http.Error(w, "github repository is not connected through app installation", http.StatusBadRequest)
			return
		}
		issue, err := adapters.CreateGitHubIssue(r.Context(), token, repo.FullName, req.Title, description, sanitizeIssueLabels(req.Labels))
		if err != nil {
			createErr = err
			break
		}
		result["issue"] = map[string]any{"id": issue.ID, "number": issue.Number, "url": issue.HTMLURL, "repo": repo.FullName}
	case "jira":
		projectKey := firstNonEmpty(strings.TrimSpace(req.ProjectKey), configString(config, "project_key"))
		if projectKey == "" {
			http.Error(w, "project_key is required for jira issue creation", http.StatusBadRequest)
			return
		}
		issue, err := adapters.CreateJiraIssue(
			r.Context(),
			configString(config, "base_url"),
			configString(config, "email"),
			configString(config, "api_token"),
			projectKey,
			req.Title,
			description,
		)
		if err != nil {
			createErr = err
			break
		}
		result["issue"] = map[string]any{"id": issue.ID, "key": issue.Key, "url": issue.URL}
	case "linear":
		issue, err := adapters.CreateLinearIssue(
			r.Context(),
			configString(config, "api_token"),
			configString(config, "team_id"),
			req.Title,
			description,
		)
		if err != nil {
			createErr = err
			break
		}
		result["issue"] = map[string]any{"id": issue.ID, "identifier": issue.Identifier, "title": issue.Title, "url": issue.URL}
	default:
		http.Error(w, "provider does not support issue creation", http.StatusBadRequest)
		return
	}

	if createErr != nil {
		_ = h.queries.UpdateIntegrationStatus(r.Context(), db.UpdateIntegrationStatusParams{
			UserID:    userID,
			Provider:  provider,
			Status:    "error",
			LastError: truncateForDB(createErr.Error(), 8000),
		})
		h.logIntegrationActivity(r.Context(), userID, pgtype.Int8{Int64: integration.ID, Valid: true}, provider, "issue_created", "failed", "failed to create issue", map[string]any{"error": createErr.Error()})
		http.Error(w, "failed to create issue", http.StatusBadGateway)
		return
	}

	_ = h.queries.UpdateIntegrationStatus(r.Context(), db.UpdateIntegrationStatusParams{
		UserID:    userID,
		Provider:  provider,
		Status:    "connected",
		LastError: "",
	})
	h.logIntegrationActivity(r.Context(), userID, pgtype.Int8{Int64: integration.ID, Valid: true}, provider, "issue_created", "success", "issue created", map[string]any{})

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listIntegrationActivities(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := queryInt(r.URL.Query().Get("limit"), 50)
	if limit > 200 {
		limit = 200
	}
	offset := queryInt(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	rows, err := h.queries.ListIntegrationActivitiesByUser(r.Context(), db.ListIntegrationActivitiesByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		http.Error(w, "failed to fetch integration activities", http.StatusInternalServerError)
		return
	}

	activities := make([]integrationActivityResponse, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, integrationActivityResponse{
			ID:        row.ID,
			Provider:  row.Provider,
			Action:    row.Action,
			Status:    row.Status,
			Detail:    strings.TrimSpace(row.Detail),
			Metadata:  decodeIntegrationConfig(row.Metadata),
			CreatedAt: timestamptzToString(row.CreatedAt),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"activities": activities,
		"limit":      limit,
		"offset":     offset,
	})
}

func (h *Handler) connectedIntegrationForProvider(w http.ResponseWriter, r *http.Request, userID int64, provider string) (db.Integration, bool) {
	integration, err := h.queries.GetIntegrationByProviderAndUser(r.Context(), db.GetIntegrationByProviderAndUserParams{
		UserID:   userID,
		Provider: provider,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "integration not connected", http.StatusNotFound)
			return db.Integration{}, false
		}
		http.Error(w, "failed to fetch integration", http.StatusInternalServerError)
		return db.Integration{}, false
	}
	if !integration.Enabled || integration.Status == "disconnected" {
		http.Error(w, "integration is disabled", http.StatusBadRequest)
		return db.Integration{}, false
	}
	return integration, true
}

func (h *Handler) logIntegrationActivity(ctx context.Context, userID int64, integrationID pgtype.Int8, provider, action, status, detail string, metadata map[string]any) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		payload = []byte(`{}`)
	}
	_ = h.queries.CreateIntegrationActivity(ctx, db.CreateIntegrationActivityParams{
		UserID:        userID,
		IntegrationID: integrationID,
		Provider:      provider,
		Action:        action,
		Status:        status,
		Detail:        truncateForDB(strings.TrimSpace(detail), 2000),
		Metadata:      payload,
	})
}

func integrationProviderFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	provider := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "provider")))
	if _, ok := supportedIntegrationProviders[provider]; !ok {
		http.Error(w, "unsupported integration provider", http.StatusBadRequest)
		return "", false
	}
	return provider, true
}

func integrationConfigFromConnectRequest(ctx context.Context, provider string, req connectIntegrationRequest, existing map[string]any) (map[string]any, error) {
	config := map[string]any{}
	switch provider {
	case "github":
		return config, nil
	case "slack", "discord":
		webhook := resolveSecretInput(req.WebhookURL, configString(existing, "webhook_url"))
		if webhook == "" {
			return nil, fmt.Errorf("webhook_url is required")
		}
		config["webhook_url"] = webhook
		return config, nil
	case "jira":
		baseURL := firstNonEmpty(req.JiraBaseURL, configString(existing, "base_url"))
		email := firstNonEmpty(req.JiraEmail, configString(existing, "email"))
		apiToken := resolveSecretInput(req.JiraAPIToken, configString(existing, "api_token"))
		projectKey := firstNonEmpty(req.JiraProject, configString(existing, "project_key"))

		if baseURL == "" {
			return nil, fmt.Errorf("jira_base_url is required")
		}
		if email == "" {
			return nil, fmt.Errorf("jira_email is required")
		}
		if apiToken == "" {
			return nil, fmt.Errorf("jira_api_token is required")
		}
		if projectKey == "" {
			return nil, fmt.Errorf("jira_project_key is required")
		}
		config["base_url"] = baseURL
		config["email"] = email
		config["api_token"] = apiToken
		config["project_key"] = projectKey
		return config, nil
	case "linear":
		apiToken := resolveSecretInput(req.LinearToken, configString(existing, "api_token"))
		teamID := firstNonEmpty(req.LinearTeamID, configString(existing, "team_id"))
		if apiToken == "" {
			return nil, fmt.Errorf("linear_api_token is required")
		}
		if teamID == "" {
			teams, err := adapters.ListLinearTeams(ctx, apiToken)
			if err != nil {
				return nil, fmt.Errorf("linear_team_id is required or fetch failed: %w", err)
			}
			if len(teams) == 0 || strings.TrimSpace(teams[0].ID) == "" {
				return nil, fmt.Errorf("linear_team_id is required and no teams were found for this token")
			}
			teamID = strings.TrimSpace(teams[0].ID)
		}
		config["api_token"] = apiToken
		config["team_id"] = teamID
		return config, nil
	default:
		return nil, fmt.Errorf("unsupported integration provider")
	}
}

func resolveSecretInput(input, fallback string) string {
	v := strings.TrimSpace(input)
	if v == "" {
		return strings.TrimSpace(fallback)
	}
	if strings.Contains(v, "...") {
		return strings.TrimSpace(fallback)
	}
	return v
}

func isLikelyWebhookURL(value string) bool {
	v := strings.TrimSpace(value)
	return strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")
}

func mapIntegrationRow(row db.Integration) integrationResponse {
	return integrationResponse{
		Provider:    row.Provider,
		Name:        row.Name,
		Status:      row.Status,
		Enabled:     row.Enabled,
		ConnectedAt: timestamptzToString(row.ConnectedAt),
		LastError:   strings.TrimSpace(row.LastError),
		Config:      sanitizeIntegrationConfig(row.Provider, decodeIntegrationConfig(row.Config)),
		UpdatedAt:   timestamptzToString(row.UpdatedAt),
	}
}

func sanitizeIntegrationConfig(provider string, raw map[string]any) map[string]any {
	result := map[string]any{}
	switch provider {
	case "slack", "discord":
		result["webhook_url"] = configString(raw, "webhook_url")
	case "jira":
		result["base_url"] = configString(raw, "base_url")
		result["email"] = configString(raw, "email")
		result["project_key"] = configString(raw, "project_key")
		result["api_token"] = maskSecret(configString(raw, "api_token"))
	case "linear":
		result["team_id"] = configString(raw, "team_id")
		result["api_token"] = maskSecret(configString(raw, "api_token"))
	default:
		return result
	}
	return result
}

func decodeIntegrationConfig(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	item := map[string]any{}
	if err := json.Unmarshal(raw, &item); err != nil {
		return map[string]any{}
	}
	return item
}

func configString(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func maskSecret(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "********"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

func formatIntegrationMessage(title, severity, text string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(title) != "" {
		parts = append(parts, strings.TrimSpace(title))
	}
	if strings.TrimSpace(severity) != "" {
		parts = append(parts, "severity="+strings.ToLower(strings.TrimSpace(severity)))
	}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, strings.TrimSpace(text))
	}
	return strings.Join(parts, "\n")
}

func sanitizeIssueLabels(labels []string) []string {
	if len(labels) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, label := range labels {
		item := strings.TrimSpace(label)
		if item == "" {
			continue
		}
		lower := strings.ToLower(item)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, item)
	}
	return result
}

func providerDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "github":
		return "GitHub"
	case "slack":
		return "Slack"
	case "jira":
		return "Jira"
	case "linear":
		return "Linear"
	case "discord":
		return "Discord"
	default:
		return provider
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateForDB(value string, max int) string {
	v := strings.TrimSpace(value)
	if max <= 0 || len(v) <= max {
		return v
	}
	return v[:max]
}
