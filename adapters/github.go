package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubAccessTokenURL = "https://github.com/login/oauth/access_token"
	githubAPIBaseURL     = "https://api.github.com"
)

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubRepository struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Private         bool   `json:"private"`
	DefaultBranch   string `json:"default_branch"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	UpdatedAt       string `json:"updated_at"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type githubAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type GitHubAPIError struct {
	StatusCode int
	Body       string
}

func (e *GitHubAPIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("github api request failed with status %d", e.StatusCode)
	}

	return fmt.Sprintf("github api request failed with status %d: %s", e.StatusCode, e.Body)
}

func newGitHubAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &GitHubAPIError{StatusCode: resp.StatusCode}
	}

	return &GitHubAPIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
}

func ExchangeGitHubCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAccessTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create github token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange github code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp githubAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode github token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("github token exchange error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("github token exchange returned empty access token")
	}

	return tokenResp.AccessToken, nil
}

func GetGitHubUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBaseURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("create github user request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newGitHubAPIError(resp)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode github user response: %w", err)
	}

	if user.Email == "" {
		email, err := GetGitHubPrimaryEmail(ctx, accessToken)
		if err == nil {
			user.Email = email
		}
	}

	return &user, nil
}

func GetGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBaseURL+"/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("create github email request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch github user emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", newGitHubAPIError(resp)
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode github emails response: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified github email found")
}

func ListGitHubUserRepositories(ctx context.Context, accessToken string) ([]GitHubRepository, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	repositories := make([]GitHubRepository, 0)

	for page := 1; ; page++ {
		reqURL := fmt.Sprintf("%s/user/repos?per_page=100&page=%d&sort=updated", githubAPIBaseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create github repositories request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch github repositories: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			err := newGitHubAPIError(resp)
			_ = resp.Body.Close()
			return nil, err
		}

		var pageRepos []GitHubRepository
		if err := json.NewDecoder(resp.Body).Decode(&pageRepos); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("decode github repositories response: %w", err)
		}
		_ = resp.Body.Close()

		repositories = append(repositories, pageRepos...)
		if len(pageRepos) < 100 {
			break
		}
	}

	return repositories, nil
}
