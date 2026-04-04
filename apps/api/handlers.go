package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chann44/TGE/adapters"
	internal "github.com/chann44/TGE/internals"
	db "github.com/chann44/TGE/internals/db"
	"github.com/go-chi/chi/v5"
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
}

type meResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func NewHandler(cfg *internal.Config, redisClient *adapters.Redis, queries *db.Queries) *Handler {
	return &Handler{cfg: cfg, redis: redisClient, queries: queries}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
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

	writeJSON(w, http.StatusOK, githubRepositoriesResponse{Repositories: responseRepos})
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
		http.Error(w, "failed to fetch github repositories", http.StatusBadGateway)
		return
	}

	var selectedRepo *adapters.GitHubRepository
	for _, repo := range repositories {
		if repo.ID == repoID {
			r := repo
			selectedRepo = &r
			break
		}
	}

	if selectedRepo == nil {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	if err := h.queries.UpsertRepository(r.Context(), db.UpsertRepositoryParams{
		UserID:        userID,
		GithubRepoID:  selectedRepo.ID,
		Name:          selectedRepo.Name,
		FullName:      selectedRepo.FullName,
		Private:       selectedRepo.Private,
		DefaultBranch: selectedRepo.DefaultBranch,
		HtmlUrl:       selectedRepo.HTMLURL,
	}); err != nil {
		http.Error(w, "failed to connect repository", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connected": true,
		"repo_id":   selectedRepo.ID,
	})
}
