package adapters

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubAppInstallation struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
	AppSlug string `json:"app_slug"`
}

type githubUserInstallationsResponse struct {
	Installations []GitHubAppInstallation `json:"installations"`
}

type githubInstallationAccessTokenResponse struct {
	Token string `json:"token"`
}

type GitHubRepositoryTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

type githubRepositoryTreeResponse struct {
	Tree []GitHubRepositoryTreeEntry `json:"tree"`
}

type githubRepositoryContentResponse struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

func ListUserAppInstallations(ctx context.Context, accessToken, appSlug string) ([]GitHubAppInstallation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBaseURL+"/user/installations", nil)
	if err != nil {
		return nil, fmt.Errorf("create github app installations request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github app installations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newGitHubAPIError(resp)
	}

	var payload githubUserInstallationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github app installations response: %w", err)
	}

	if appSlug == "" {
		return payload.Installations, nil
	}

	filtered := make([]GitHubAppInstallation, 0, len(payload.Installations))
	for _, installation := range payload.Installations {
		if installation.AppSlug == appSlug {
			filtered = append(filtered, installation)
		}
	}

	return filtered, nil
}

func CreateInstallationAccessToken(ctx context.Context, appIssuer, privateKeyPEM string, installationID int64) (string, error) {
	appJWT, err := createGitHubAppJWT(appIssuer, privateKeyPEM)
	if err != nil {
		return "", err
	}

	reqURL := fmt.Sprintf("%s/app/installations/%d/access_tokens", githubAPIBaseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("create installation token request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", newGitHubAPIError(resp)
	}

	var tokenResponse githubInstallationAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("decode installation token response: %w", err)
	}

	if strings.TrimSpace(tokenResponse.Token) == "" {
		return "", fmt.Errorf("installation token response missing token")
	}

	return tokenResponse.Token, nil
}

func GetRepositoryByID(ctx context.Context, accessToken string, repoID int64) (*GitHubRepository, error) {
	reqURL := fmt.Sprintf("%s/repositories/%d", githubAPIBaseURL, repoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create github repository request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newGitHubAPIError(resp)
	}

	var repository GitHubRepository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, fmt.Errorf("decode github repository response: %w", err)
	}

	return &repository, nil
}

func ListRepositoryTree(ctx context.Context, accessToken, owner, repo, ref string) ([]GitHubRepositoryTreeEntry, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	if strings.TrimSpace(ref) == "" {
		ref = "HEAD"
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", githubAPIBaseURL, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create github repository tree request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github repository tree: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newGitHubAPIError(resp)
	}

	var payload githubRepositoryTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github repository tree response: %w", err)
	}

	return payload.Tree, nil
}

func GetRepositoryFileContent(ctx context.Context, accessToken, owner, repo, filePath, ref string) (string, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" || strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("owner, repo and filePath are required")
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIBaseURL, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(filePath))
	reqURL = appendGitHubQuery(reqURL, "ref", strings.TrimSpace(ref))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("create github repository content request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch github repository content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", newGitHubAPIError(resp)
	}

	var payload githubRepositoryContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode github repository content response: %w", err)
	}

	if payload.Type != "file" {
		return "", fmt.Errorf("github content path is not a file")
	}
	if payload.Encoding != "base64" {
		return "", fmt.Errorf("unsupported github content encoding: %s", payload.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return "", fmt.Errorf("decode github file content: %w", err)
	}

	return string(decoded), nil
}

func appendGitHubQuery(rawURL, key, value string) string {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := parsed.Query()
	q.Set(key, value)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func createGitHubAppJWT(appIssuer, privateKeyPEM string) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(appIssuer) == "" {
		return "", fmt.Errorf("github app issuer is empty")
	}

	now := time.Now().Unix()
	headerPayload := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}
	claims := map[string]any{
		"iat": now - 60,
		"exp": now + 540,
		"iss": appIssuer,
	}

	headerRaw, err := json.Marshal(headerPayload)
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(claimsRaw)
	input := header + "." + payload

	sum := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}

	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(raw, `\n`, "\n"))
	if normalized == "" {
		return nil, fmt.Errorf("github app private key is empty")
	}

	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("invalid github app private key pem")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github app private key: %w", err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github app private key is not rsa")
	}

	return key, nil
}
