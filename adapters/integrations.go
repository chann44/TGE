package adapters

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GitHubIssue struct {
	ID      int64  `json:"id"`
	Number  int64  `json:"number"`
	HTMLURL string `json:"html_url"`
}

type JiraIssue struct {
	ID  string `json:"id"`
	Key string `json:"key"`
	URL string `json:"url"`
}

type LinearIssue struct {
	ID  string `json:"id"`
	URL string `json:"url"`

	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

type LinearTeam struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func SendSlackWebhookMessage(ctx context.Context, webhookURL, text string) error {
	payload := map[string]string{"text": text}
	return postIntegrationJSON(ctx, webhookURL, "", payload, http.StatusOK, http.StatusNoContent)
}

func SendDiscordWebhookMessage(ctx context.Context, webhookURL, text string) error {
	payload := map[string]string{"content": text}
	return postIntegrationJSON(ctx, webhookURL, "", payload, http.StatusNoContent, http.StatusOK)
}

func CreateGitHubIssue(ctx context.Context, accessToken, fullName, title, body string, labels []string) (*GitHubIssue, error) {
	owner, repo, err := splitGitHubRepoFullName(fullName)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/issues", githubAPIBaseURL, url.PathEscape(owner), url.PathEscape(repo))
	payload := map[string]any{
		"title":  strings.TrimSpace(title),
		"body":   strings.TrimSpace(body),
		"labels": labels,
	}

	var issue GitHubIssue
	if err := postIntegrationJSONDecode(ctx, reqURL, "Bearer "+accessToken, payload, &issue, http.StatusCreated); err != nil {
		return nil, err
	}
	return &issue, nil
}

func CreateJiraIssue(ctx context.Context, baseURL, email, apiToken, projectKey, title, description string) (*JiraIssue, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("jira base url is required")
	}
	reqURL := base + "/rest/api/3/issue"
	auth := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(email) + ":" + strings.TrimSpace(apiToken)))

	payload := map[string]any{
		"fields": map[string]any{
			"project": map[string]any{"key": strings.TrimSpace(projectKey)},
			"summary": strings.TrimSpace(title),
			"description": map[string]any{
				"type":    "doc",
				"version": 1,
				"content": []map[string]any{{
					"type": "paragraph",
					"content": []map[string]any{{
						"type": "text",
						"text": strings.TrimSpace(description),
					}},
				}},
			},
			"issuetype": map[string]any{"name": "Task"},
		},
	}

	var response struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := postIntegrationJSONDecode(ctx, reqURL, "Basic "+auth, payload, &response, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}

	item := &JiraIssue{
		ID:  strings.TrimSpace(response.ID),
		Key: strings.TrimSpace(response.Key),
	}
	if item.Key != "" {
		item.URL = base + "/browse/" + url.PathEscape(item.Key)
	}
	return item, nil
}

func CreateLinearIssue(ctx context.Context, apiToken, teamID, title, description string) (*LinearIssue, error) {
	reqURL := "https://api.linear.app/graphql"
	payload := map[string]any{
		"query": `mutation CreateIssue($input: IssueCreateInput!) { issueCreate(input: $input) { success issue { id identifier title url } } }`,
		"variables": map[string]any{
			"input": map[string]any{
				"teamId":      strings.TrimSpace(teamID),
				"title":       strings.TrimSpace(title),
				"description": strings.TrimSpace(description),
			},
		},
	}

	var response struct {
		Data struct {
			IssueCreate struct {
				Success bool        `json:"success"`
				Issue   LinearIssue `json:"issue"`
			} `json:"issueCreate"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := postIntegrationJSONDecode(ctx, reqURL, strings.TrimSpace(apiToken), payload, &response, http.StatusOK); err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("linear issue create failed: %s", strings.TrimSpace(response.Errors[0].Message))
	}
	if !response.Data.IssueCreate.Success {
		return nil, fmt.Errorf("linear issue create was not successful")
	}
	return &response.Data.IssueCreate.Issue, nil
}

func ListLinearTeams(ctx context.Context, apiToken string) ([]LinearTeam, error) {
	reqURL := "https://api.linear.app/graphql"
	payload := map[string]any{
		"query": `query Teams { teams { nodes { id key name } } }`,
	}

	var response struct {
		Data struct {
			Teams struct {
				Nodes []LinearTeam `json:"nodes"`
			} `json:"teams"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := postIntegrationJSONDecode(ctx, reqURL, strings.TrimSpace(apiToken), payload, &response, http.StatusOK); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("linear teams query failed: %s", strings.TrimSpace(response.Errors[0].Message))
	}

	teams := make([]LinearTeam, 0, len(response.Data.Teams.Nodes))
	for _, team := range response.Data.Teams.Nodes {
		id := strings.TrimSpace(team.ID)
		if id == "" {
			continue
		}
		teams = append(teams, LinearTeam{
			ID:   id,
			Key:  strings.TrimSpace(team.Key),
			Name: strings.TrimSpace(team.Name),
		})
	}

	return teams, nil
}

func postIntegrationJSON(ctx context.Context, endpoint, authHeader string, payload any, expectedStatus ...int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(endpoint), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(authHeader) != "" {
		if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
			req.Header.Set("Authorization", authHeader)
		} else if strings.Contains(endpoint, "api.linear.app") {
			req.Header.Set("Authorization", authHeader)
		} else {
			req.Header.Set("Authorization", "Bearer "+authHeader)
		}
	}
	if strings.Contains(endpoint, "api.github.com") {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, status := range expectedStatus {
		if resp.StatusCode == status {
			return nil
		}
	}
	return newGitHubAPIError(resp)
}

func postIntegrationJSONDecode(ctx context.Context, endpoint, authHeader string, payload any, target any, expectedStatus ...int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(endpoint), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(authHeader) != "" {
		if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
			req.Header.Set("Authorization", authHeader)
		} else if strings.Contains(endpoint, "api.linear.app") {
			req.Header.Set("Authorization", authHeader)
		} else {
			req.Header.Set("Authorization", "Bearer "+authHeader)
		}
	}
	if strings.Contains(endpoint, "api.github.com") {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	allowed := false
	for _, status := range expectedStatus {
		if resp.StatusCode == status {
			allowed = true
			break
		}
	}
	if !allowed {
		return newGitHubAPIError(resp)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func splitGitHubRepoFullName(fullName string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(fullName), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid github repository full name")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid github repository full name")
	}
	return owner, repo, nil
}
