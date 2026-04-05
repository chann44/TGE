package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chann44/TGE/adapters"
	internal "github.com/chann44/TGE/internals"
	db "github.com/chann44/TGE/internals/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
)

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type Handler struct {
	cfg     *internal.Config
	redis   *adapters.Redis
	queries *db.Queries
}

type githubRepositoryResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	Stargazers    int    `json:"stargazers_count"`
	Forks         int    `json:"forks_count"`
	OpenIssues    int    `json:"open_issues_count"`
	UpdatedAt     string `json:"updated_at"`
	Connected     bool   `json:"connected"`
}

type githubRepositoriesResponse struct {
	Repositories []githubRepositoryResponse `json:"repositories"`
	Page         int                        `json:"page"`
	PageSize     int                        `json:"page_size"`
	Total        int                        `json:"total"`
	TotalPages   int                        `json:"total_pages"`
}

type meResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubAppConnectState struct {
	UserID int64 `json:"user_id"`
	RepoID int64 `json:"repo_id"`
}

type connectGitHubRepositoryResponse struct {
	Connected     bool   `json:"connected"`
	RepoID        int64  `json:"repo_id"`
	InstallNeeded bool   `json:"install_needed,omitempty"`
	RedirectURL   string `json:"redirect_url,omitempty"`
}

type dependencyFileResponse struct {
	Path     string `json:"path"`
	File     string `json:"file"`
	Manager  string `json:"manager"`
	Registry string `json:"registry"`
}

type dependencyFilesResponse struct {
	RepositoryID int64                    `json:"repository_id"`
	FullName     string                   `json:"full_name"`
	Files        []dependencyFileResponse `json:"files"`
}

func NewHandler(cfg *internal.Config, redisClient *adapters.Redis, queries *db.Queries) *Handler {
	return &Handler{cfg: cfg, redis: redisClient, queries: queries}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) requestLog(r *http.Request, format string, args ...any) {
	prefix := "api"
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		prefix = fmt.Sprintf("api request_id=%s", requestID)
	}
	log.Printf("%s %s", prefix, fmt.Sprintf(format, args...))
}

func (h *Handler) getGitHubAccessToken(ctx context.Context, userID int64) (string, error) {
	tokenRow, err := h.queries.GetUserOAuthToken(ctx, db.GetUserOAuthTokenParams{
		UserID:   userID,
		Provider: "github",
	})
	if err != nil {
		return "", err
	}

	if tokenRow.AccessToken == "" {
		return "", fmt.Errorf("github access token is empty")
	}

	return tokenRow.AccessToken, nil
}

func (h *Handler) githubAppInstallURL() (string, error) {
	installURL := strings.TrimSpace(h.cfg.GithubAppInstallURL)
	if installURL != "" {
		return installURL, nil
	}

	appSlug := strings.TrimSpace(h.cfg.GithubAppSlug)
	if appSlug == "" {
		return "", fmt.Errorf("github app install is not configured")
	}

	return fmt.Sprintf("https://github.com/apps/%s/installations/new", url.PathEscape(appSlug)), nil
}

func appendQueryParam(rawURL, key, value string) string {
	if rawURL == "" || key == "" || value == "" {
		return rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	query.Set(key, value)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func (h *Handler) frontendRedirect(path string, query map[string]string) string {
	base := strings.TrimRight(h.cfg.FrontendURL, "/")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	u, err := url.Parse(base + path)
	if err != nil {
		return base + path
	}

	q := u.Query()
	for k, v := range query {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (h *Handler) storeGitHubAppConnectState(ctx context.Context, userID, repoID int64) (string, error) {
	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("generate github app connect state: %w", err)
	}

	payload, err := json.Marshal(githubAppConnectState{UserID: userID, RepoID: repoID})
	if err != nil {
		return "", fmt.Errorf("marshal github app connect state: %w", err)
	}

	stateKey := fmt.Sprintf("github_app_connect_state:%s", state)
	if err := h.redis.Set(ctx, stateKey, string(payload), 15*time.Minute); err != nil {
		return "", fmt.Errorf("store github app connect state: %w", err)
	}

	return state, nil
}

func (h *Handler) appIssuer() (string, error) {
	if value := strings.TrimSpace(h.cfg.GithubAppID); value != "" {
		return value, nil
	}

	if value := strings.TrimSpace(h.cfg.GithubClientID); value != "" {
		return value, nil
	}

	return "", fmt.Errorf("github app issuer is not configured")
}

func (h *Handler) upsertConnectedRepository(ctx context.Context, userID int64, repo *adapters.GitHubRepository) error {
	return h.queries.UpsertRepository(ctx, db.UpsertRepositoryParams{
		UserID:        userID,
		GithubRepoID:  repo.ID,
		Name:          repo.Name,
		FullName:      repo.FullName,
		Private:       repo.Private,
		DefaultBranch: repo.DefaultBranch,
		HtmlUrl:       repo.HTMLURL,
	})
}

func (h *Handler) installationTokenForRepo(ctx context.Context, userID, repoID int64) (string, *adapters.GitHubRepository, error) {
	appIssuer, err := h.appIssuer()
	if err != nil {
		return "", nil, err
	}

	installations, err := h.queries.ListUserGitHubInstallations(ctx, userID)
	if err != nil {
		return "", nil, err
	}

	for _, installation := range installations {
		token, tokenErr := adapters.CreateInstallationAccessToken(ctx, appIssuer, h.cfg.GithubAppPrivateKey, installation.InstallationID)
		if tokenErr != nil {
			continue
		}

		repository, repoErr := adapters.GetRepositoryByID(ctx, token, repoID)
		if repoErr == nil {
			return token, repository, nil
		}
	}

	return "", nil, fmt.Errorf("no installation access for repository")
}

func dependencyManagerForFile(fileName string) string {
	switch strings.ToLower(fileName) {
	case "package.json":
		return "npm"
	case "requirements.txt":
		return "pip"
	case "go.mod":
		return "go"
	default:
		return ""
	}
}

func registryForManager(manager string) string {
	switch strings.ToLower(strings.TrimSpace(manager)) {
	case "npm":
		return "npm"
	case "pip":
		return "pypi"
	case "go":
		return "github"
	default:
		return "unknown"
	}
}

func queryInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func (h *Handler) githubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}
	stateKey := fmt.Sprintf("github_state:%s", state)
	if err := h.redis.Set(r.Context(), stateKey, "1", 10*time.Minute); err != nil {
		http.Error(w, "failed to set state", http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email",
		h.cfg.GithubClientID,
		h.cfg.GithubRedirectURI,
		state,
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *Handler) githubAppInstall(w http.ResponseWriter, r *http.Request) {
	installURL, err := h.githubAppInstallURL()
	if err != nil {
		h.requestLog(r, "github app install requested but not configured")
		http.Error(w, "github app install is not configured", http.StatusServiceUnavailable)
		return
	}
	h.requestLog(r, "redirecting to github app install url=%s", installURL)

	http.Redirect(w, r, installURL, http.StatusTemporaryRedirect)
}

func (h *Handler) githubCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	stateKey := fmt.Sprintf("github_state:%s", state)
	if _, err := h.redis.Get(r.Context(), stateKey); err != nil {
		http.Error(w, "invalid or expired state", http.StatusUnauthorized)
		return
	}
	_ = h.redis.Del(r.Context(), stateKey)

	token, err := adapters.ExchangeGitHubCode(
		r.Context(),
		h.cfg.GithubClientID,
		h.cfg.GithubClientSecret,
		code,
		h.cfg.GithubRedirectURI,
	)
	if err != nil {
		http.Error(w, "failed to exchange github code", http.StatusBadGateway)
		return
	}

	user, err := adapters.GetGitHubUser(r.Context(), token)
	if err != nil {
		http.Error(w, "failed to fetch github user", http.StatusBadGateway)
		return
	}

	dbUser, err := h.queries.UpsertGitHubUser(r.Context(), db.UpsertGitHubUserParams{
		GithubID:  user.ID,
		Login:     user.Login,
		Name:      user.Name,
		Email:     user.Email,
		AvatarUrl: user.AvatarURL,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to store github user: %v", err), http.StatusInternalServerError)
		return
	}

	if err := h.queries.UpsertUserOAuthToken(r.Context(), db.UpsertUserOAuthTokenParams{
		UserID:      dbUser.ID,
		Provider:    "github",
		AccessToken: token,
	}); err != nil {
		http.Error(w, "failed to store oauth token", http.StatusInternalServerError)
		return
	}

	sessionToken, err := internal.CreateSessionToken(
		strconv.FormatInt(dbUser.ID, 10),
		dbUser.Login,
		dbUser.Name,
		dbUser.Email,
		dbUser.AvatarUrl,
		24*time.Hour,
	)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	frontendURL := strings.TrimRight(h.cfg.FrontendURL, "/")
	redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", frontendURL, url.QueryEscape(sessionToken))
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *Handler) githubAppSetup(w http.ResponseWriter, r *http.Request) {
	h.requestLog(r, "github app setup callback setup_action=%s state_present=%t installation_id=%s", r.URL.Query().Get("setup_action"), strings.TrimSpace(r.URL.Query().Get("state")) != "", strings.TrimSpace(r.URL.Query().Get("installation_id")))

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	installationIDText := strings.TrimSpace(r.URL.Query().Get("installation_id"))
	installationID, err := strconv.ParseInt(installationIDText, 10, 64)
	if err != nil || installationID == 0 {
		h.requestLog(r, "github app setup invalid installation id raw=%q", installationIDText)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "invalid_installation"}), http.StatusTemporaryRedirect)
		return
	}

	if state == "" {
		h.requestLog(r, "github app setup completed without state installation_id=%d", installationID)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "installed"}), http.StatusTemporaryRedirect)
		return
	}

	stateKey := fmt.Sprintf("github_app_connect_state:%s", state)
	payload, err := h.redis.Get(r.Context(), stateKey)
	if err != nil {
		h.requestLog(r, "github app setup state expired state=%s installation_id=%d", state, installationID)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "state_expired"}), http.StatusTemporaryRedirect)
		return
	}
	_ = h.redis.Del(r.Context(), stateKey)

	var connectState githubAppConnectState
	if err := json.Unmarshal([]byte(payload), &connectState); err != nil {
		h.requestLog(r, "github app setup invalid state payload state=%s error=%v", state, err)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "invalid_state"}), http.StatusTemporaryRedirect)
		return
	}

	if connectState.UserID == 0 || connectState.RepoID == 0 {
		h.requestLog(r, "github app setup state missing fields state=%s user_id=%d repo_id=%d", state, connectState.UserID, connectState.RepoID)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "invalid_state"}), http.StatusTemporaryRedirect)
		return
	}
	h.requestLog(r, "github app setup resolved state user_id=%d repo_id=%d installation_id=%d", connectState.UserID, connectState.RepoID, installationID)

	appIssuer, err := h.appIssuer()
	if err != nil {
		h.requestLog(r, "github app setup missing app issuer user_id=%d installation_id=%d", connectState.UserID, installationID)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "app_not_configured"}), http.StatusTemporaryRedirect)
		return
	}

	installationToken, err := adapters.CreateInstallationAccessToken(
		r.Context(),
		appIssuer,
		h.cfg.GithubAppPrivateKey,
		installationID,
	)
	if err != nil {
		h.requestLog(r, "github app setup failed creating installation token installation_id=%d error=%v", installationID, err)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "token_failed"}), http.StatusTemporaryRedirect)
		return
	}

	repository, err := adapters.GetRepositoryByID(r.Context(), installationToken, connectState.RepoID)
	if err != nil {
		var githubErr *adapters.GitHubAPIError
		if errors.As(err, &githubErr) && githubErr.StatusCode == http.StatusNotFound {
			h.requestLog(r, "github app setup installation has no repo access installation_id=%d repo_id=%d", installationID, connectState.RepoID)
			http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "repo_not_granted"}), http.StatusTemporaryRedirect)
			return
		}
		h.requestLog(r, "github app setup repo verification failed installation_id=%d repo_id=%d error=%v", installationID, connectState.RepoID, err)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "repo_verify_failed"}), http.StatusTemporaryRedirect)
		return
	}

	htmlURL := fmt.Sprintf("https://github.com/settings/installations/%d", installationID)
	accessToken, tokenErr := h.getGitHubAccessToken(r.Context(), connectState.UserID)
	if tokenErr == nil {
		installations, listErr := adapters.ListUserAppInstallations(r.Context(), accessToken, strings.TrimSpace(h.cfg.GithubAppSlug))
		if listErr == nil {
			for _, installation := range installations {
				if installation.ID == installationID {
					htmlURL = installation.HTMLURL
					if err := h.queries.UpsertUserGitHubInstallation(r.Context(), db.UpsertUserGitHubInstallationParams{
						UserID:         connectState.UserID,
						InstallationID: installationID,
						AppSlug:        strings.TrimSpace(h.cfg.GithubAppSlug),
						AccountLogin:   installation.Account.Login,
						AccountType:    installation.Account.Type,
						HtmlUrl:        htmlURL,
					}); err != nil {
						http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "store_failed"}), http.StatusTemporaryRedirect)
						return
					}
					goto storeDone
				}
			}
		}
	}

	if err := h.queries.UpsertUserGitHubInstallation(r.Context(), db.UpsertUserGitHubInstallationParams{
		UserID:         connectState.UserID,
		InstallationID: installationID,
		AppSlug:        strings.TrimSpace(h.cfg.GithubAppSlug),
		AccountLogin:   "",
		AccountType:    "",
		HtmlUrl:        htmlURL,
	}); err != nil {
		h.requestLog(r, "github app setup failed storing installation user_id=%d installation_id=%d error=%v", connectState.UserID, installationID, err)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "store_failed"}), http.StatusTemporaryRedirect)
		return
	}

storeDone:

	if err := h.upsertConnectedRepository(r.Context(), connectState.UserID, repository); err != nil {
		h.requestLog(r, "github app setup failed connecting repo user_id=%d repo_id=%d error=%v", connectState.UserID, repository.ID, err)
		http.Redirect(w, r, h.frontendRedirect("/repos", map[string]string{"app_setup": "connect_failed"}), http.StatusTemporaryRedirect)
		return
	}
	h.requestLog(r, "github app setup connected repo user_id=%d repo_id=%d installation_id=%d", connectState.UserID, repository.ID, installationID)

	redirectURL := h.frontendRedirect(fmt.Sprintf("/repos/%d", repository.ID), map[string]string{"connected": "1"})
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, meResponse{
		ID:        user.ID,
		Login:     user.Login,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarUrl,
	})
}

func (h *Handler) githubRepositories(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var err error

	accessToken, err := h.getGitHubAccessToken(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "github account not connected", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to fetch oauth token", http.StatusInternalServerError)
		return
	}

	repositories, err := adapters.ListGitHubUserRepositories(r.Context(), accessToken)
	if err != nil {
		var githubErr *adapters.GitHubAPIError
		if errors.As(err, &githubErr) {
			switch githubErr.StatusCode {
			case http.StatusUnauthorized:
				http.Error(w, "github token expired, please reconnect github", http.StatusUnauthorized)
				return
			case http.StatusForbidden:
				http.Error(w, "github access denied for repositories", http.StatusForbidden)
				return
			}
		}
		http.Error(w, "failed to fetch github repositories", http.StatusBadGateway)
		return
	}

	connectedRepos, err := h.queries.ListUserRepositories(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch connected repositories", http.StatusInternalServerError)
		return
	}

	connectedByRepoID := make(map[int64]struct{}, len(connectedRepos))
	for _, repo := range connectedRepos {
		connectedByRepoID[repo.GithubRepoID] = struct{}{}
	}

	responseRepos := make([]githubRepositoryResponse, 0, len(repositories))
	for _, repo := range repositories {
		_, connected := connectedByRepoID[repo.ID]
		responseRepos = append(responseRepos, githubRepositoryResponse{
			ID:            repo.ID,
			Name:          repo.Name,
			FullName:      repo.FullName,
			Private:       repo.Private,
			DefaultBranch: repo.DefaultBranch,
			HTMLURL:       repo.HTMLURL,
			Description:   repo.Description,
			Language:      repo.Language,
			Stargazers:    repo.StargazersCount,
			Forks:         repo.ForksCount,
			OpenIssues:    repo.OpenIssuesCount,
			UpdatedAt:     repo.UpdatedAt,
			Connected:     connected,
		})
	}

	page := queryInt(r.URL.Query().Get("page"), 1)
	pageSize := queryInt(r.URL.Query().Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	total := len(responseRepos)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, githubRepositoriesResponse{
		Repositories: responseRepos[start:end],
		Page:         page,
		PageSize:     pageSize,
		Total:        total,
		TotalPages:   totalPages,
	})
}

func (h *Handler) githubRepositoryByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoIDText := chi.URLParam(r, "repoID")
	repoID, err := strconv.ParseInt(repoIDText, 10, 64)
	if err != nil {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}

	accessToken, err := h.getGitHubAccessToken(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "github account not connected", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to fetch oauth token", http.StatusInternalServerError)
		return
	}

	repositories, err := adapters.ListGitHubUserRepositories(r.Context(), accessToken)
	if err != nil {
		var githubErr *adapters.GitHubAPIError
		if errors.As(err, &githubErr) {
			switch githubErr.StatusCode {
			case http.StatusUnauthorized:
				http.Error(w, "github token expired, please reconnect github", http.StatusUnauthorized)
				return
			case http.StatusForbidden:
				http.Error(w, "github access denied for repositories", http.StatusForbidden)
				return
			}
		}
		http.Error(w, "failed to fetch github repository", http.StatusBadGateway)
		return
	}

	connectedRepos, err := h.queries.ListUserRepositories(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch connected repositories", http.StatusInternalServerError)
		return
	}

	connectedByRepoID := make(map[int64]struct{}, len(connectedRepos))
	for _, repo := range connectedRepos {
		connectedByRepoID[repo.GithubRepoID] = struct{}{}
	}

	for _, repo := range repositories {
		if repo.ID != repoID {
			continue
		}

		_, connected := connectedByRepoID[repo.ID]
		writeJSON(w, http.StatusOK, githubRepositoryResponse{
			ID:            repo.ID,
			Name:          repo.Name,
			FullName:      repo.FullName,
			Private:       repo.Private,
			DefaultBranch: repo.DefaultBranch,
			HTMLURL:       repo.HTMLURL,
			Description:   repo.Description,
			Language:      repo.Language,
			Stargazers:    repo.StargazersCount,
			Forks:         repo.ForksCount,
			OpenIssues:    repo.OpenIssuesCount,
			UpdatedAt:     repo.UpdatedAt,
			Connected:     connected,
		})
		return
	}

	http.Error(w, "repository not found", http.StatusNotFound)
}

func (h *Handler) githubRepositoryDependencyFiles(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repoIDText := chi.URLParam(r, "repoID")
	repoID, err := strconv.ParseInt(repoIDText, 10, 64)
	if err != nil {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}

	connectedRepos, err := h.queries.ListUserRepositories(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch connected repositories", http.StatusInternalServerError)
		return
	}

	connected := false
	for _, repo := range connectedRepos {
		if repo.GithubRepoID == repoID {
			connected = true
			break
		}
	}
	if !connected {
		http.Error(w, "repository is not connected", http.StatusNotFound)
		return
	}

	installationToken, repository, err := h.installationTokenForRepo(r.Context(), userID, repoID)
	if err != nil {
		http.Error(w, "github app installation access not found for repository", http.StatusForbidden)
		return
	}

	owner, repoName, ok := strings.Cut(repository.FullName, "/")
	if !ok || strings.TrimSpace(owner) == "" || strings.TrimSpace(repoName) == "" {
		http.Error(w, "invalid repository full name", http.StatusBadGateway)
		return
	}

	tree, err := adapters.ListRepositoryTree(r.Context(), installationToken, owner, repoName, repository.DefaultBranch)
	if err != nil {
		http.Error(w, "failed to fetch repository tree", http.StatusBadGateway)
		return
	}

	dependencyFiles := make([]dependencyFileResponse, 0)
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}

		name := path.Base(entry.Path)
		manager := dependencyManagerForFile(name)
		if manager == "" {
			continue
		}

		dependencyFiles = append(dependencyFiles, dependencyFileResponse{
			Path:     entry.Path,
			File:     name,
			Manager:  manager,
			Registry: registryForManager(manager),
		})
	}

	sort.Slice(dependencyFiles, func(i, j int) bool {
		return dependencyFiles[i].Path < dependencyFiles[j].Path
	})

	writeJSON(w, http.StatusOK, dependencyFilesResponse{
		RepositoryID: repoID,
		FullName:     repository.FullName,
		Files:        dependencyFiles,
	})
}

func (h *Handler) connectGitHubRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var err error

	repoIDText := chi.URLParam(r, "repoID")
	repoID, err := strconv.ParseInt(repoIDText, 10, 64)
	if err != nil {
		h.requestLog(r, "connect repo invalid repo id user_id=%d raw=%q", userID, repoIDText)
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return
	}
	h.requestLog(r, "connect repo requested user_id=%d repo_id=%d", userID, repoID)

	accessToken, err := h.getGitHubAccessToken(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.requestLog(r, "connect repo no github oauth token user_id=%d", userID)
			http.Error(w, "github account not connected", http.StatusUnauthorized)
			return
		}
		h.requestLog(r, "connect repo failed loading oauth token user_id=%d error=%v", userID, err)
		http.Error(w, "failed to fetch oauth token", http.StatusInternalServerError)
		return
	}

	selectedRepo, err := adapters.GetRepositoryByID(r.Context(), accessToken, repoID)
	if err != nil {
		var githubErr *adapters.GitHubAPIError
		if errors.As(err, &githubErr) {
			switch githubErr.StatusCode {
			case http.StatusUnauthorized:
				http.Error(w, "github token expired, please reconnect github", http.StatusUnauthorized)
				return
			case http.StatusForbidden:
				http.Error(w, "github access denied for repository", http.StatusForbidden)
				return
			case http.StatusNotFound:
				http.Error(w, "repository not found", http.StatusNotFound)
				return
			}
		}
		h.requestLog(r, "connect repo failed fetching selected github repo user_id=%d repo_id=%d error=%v", userID, repoID, err)
		http.Error(w, "failed to fetch github repository", http.StatusBadGateway)
		return
	}

	githubInstallations, listErr := adapters.ListUserAppInstallations(
		r.Context(),
		accessToken,
		strings.TrimSpace(h.cfg.GithubAppSlug),
	)
	if listErr == nil {
		h.requestLog(r, "connect repo discovered github installations user_id=%d count=%d", userID, len(githubInstallations))
		for _, installation := range githubInstallations {
			if err := h.queries.UpsertUserGitHubInstallation(r.Context(), db.UpsertUserGitHubInstallationParams{
				UserID:         userID,
				InstallationID: installation.ID,
				AppSlug:        strings.TrimSpace(h.cfg.GithubAppSlug),
				AccountLogin:   installation.Account.Login,
				AccountType:    installation.Account.Type,
				HtmlUrl:        installation.HTMLURL,
			}); err != nil {
				h.requestLog(r, "connect repo failed storing discovered installation user_id=%d installation_id=%d error=%v", userID, installation.ID, err)
				http.Error(w, "failed to store github installation", http.StatusInternalServerError)
				return
			}
		}
	}

	state, err := h.storeGitHubAppConnectState(r.Context(), userID, repoID)
	if err != nil {
		h.requestLog(r, "connect repo failed creating setup state user_id=%d repo_id=%d error=%v", userID, repoID, err)
		http.Error(w, "failed to initialize github app connect flow", http.StatusInternalServerError)
		return
	}

	redirectURL := ""
	if len(githubInstallations) > 0 {
		repoOwner, _, hasOwner := strings.Cut(selectedRepo.FullName, "/")
		if hasOwner {
			for _, installation := range githubInstallations {
				if strings.EqualFold(strings.TrimSpace(installation.Account.Login), strings.TrimSpace(repoOwner)) && strings.TrimSpace(installation.HTMLURL) != "" {
					redirectURL = installation.HTMLURL
					break
				}
			}
		}

		if redirectURL == "" && strings.TrimSpace(githubInstallations[0].HTMLURL) != "" {
			redirectURL = githubInstallations[0].HTMLURL
		}
	}

	if redirectURL == "" {
		redirectURL, err = h.githubAppInstallURL()
		if err != nil {
			h.requestLog(r, "connect repo app install URL not configured user_id=%d", userID)
			http.Error(w, "github app install is not configured", http.StatusServiceUnavailable)
			return
		}
	}

	if strings.Contains(redirectURL, "/installations/new") && selectedRepo.Owner.ID > 0 {
		redirectURL = appendQueryParam(redirectURL, "target_id", strconv.FormatInt(selectedRepo.Owner.ID, 10))
		if strings.TrimSpace(selectedRepo.Owner.Type) != "" {
			redirectURL = appendQueryParam(redirectURL, "target_type", selectedRepo.Owner.Type)
		}
	}

	redirectURL = appendQueryParam(redirectURL, "state", state)
	h.requestLog(r, "connect repo requires install/config user_id=%d repo_id=%d redirect_url=%s", userID, selectedRepo.ID, redirectURL)

	writeJSON(w, http.StatusOK, connectGitHubRepositoryResponse{
		Connected:     false,
		RepoID:        selectedRepo.ID,
		InstallNeeded: true,
		RedirectURL:   redirectURL,
	})
}
