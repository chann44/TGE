package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GitHubAdvisory struct {
	GHSAID       string
	Summary      string
	Description  string
	Severity     string
	ReferenceURL string
	Aliases      []string
}

type githubAdvisoryResponse struct {
	GHSAID      string `json:"ghsa_id"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	HTMLURL     string `json:"html_url"`
	CVEID       string `json:"cve_id"`
	Identifiers []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"identifiers"`
}

func GetGitHubSecurityAdvisory(ctx context.Context, ghsaID, token string) (*GitHubAdvisory, error) {
	id := strings.TrimSpace(ghsaID)
	if id == "" {
		return nil, fmt.Errorf("ghsa id is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/advisories/%s", id), nil)
	if err != nil {
		return nil, fmt.Errorf("create ghsa advisory request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute ghsa advisory request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ghsa advisory request failed with status %d", resp.StatusCode)
	}

	var payload githubAdvisoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode ghsa advisory response: %w", err)
	}

	aliases := make([]string, 0, len(payload.Identifiers)+1)
	if strings.TrimSpace(payload.CVEID) != "" {
		aliases = append(aliases, strings.TrimSpace(payload.CVEID))
	}
	for _, identifier := range payload.Identifiers {
		value := strings.TrimSpace(identifier.Value)
		if value != "" {
			aliases = append(aliases, value)
		}
	}

	return &GitHubAdvisory{
		GHSAID:       firstNonEmptyTrimmed(payload.GHSAID, id),
		Summary:      strings.TrimSpace(payload.Summary),
		Description:  strings.TrimSpace(payload.Description),
		Severity:     strings.ToLower(strings.TrimSpace(payload.Severity)),
		ReferenceURL: strings.TrimSpace(payload.HTMLURL),
		Aliases:      cleanStringList(aliases),
	}, nil
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
