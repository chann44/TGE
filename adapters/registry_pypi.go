package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PyPIPackageMetadata struct {
	Name          string
	LatestVersion string
	Creator       string
	Description   string
	License       string
	Homepage      string
	RepositoryURL string
	RegistryURL   string
	LastUpdated   string
	Dependencies  []PackageDependency
}

type pypiRegistryResponse struct {
	Info struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Author       string            `json:"author"`
		Maintainer   string            `json:"maintainer"`
		Summary      string            `json:"summary"`
		License      string            `json:"license"`
		HomePage     string            `json:"home_page"`
		ProjectUrls  map[string]string `json:"project_urls"`
		RequiresDist []string          `json:"requires_dist"`
	} `json:"info"`
	Releases map[string][]struct {
		UploadTimeISO8601 string `json:"upload_time_iso_8601"`
	} `json:"releases"`
}

func GetPyPIPackageMetadata(ctx context.Context, packageName string) (*PyPIPackageMetadata, error) {
	if strings.TrimSpace(packageName) == "" {
		return nil, fmt.Errorf("package name is required")
	}

	endpoint := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create pypi metadata request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch pypi metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi metadata request failed with status %d", resp.StatusCode)
	}

	var payload pypiRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode pypi metadata response: %w", err)
	}

	creator := strings.TrimSpace(payload.Info.Author)
	if creator == "" {
		creator = strings.TrimSpace(payload.Info.Maintainer)
	}

	repoURL := ""
	for key, value := range payload.Info.ProjectUrls {
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(lower, "source") || strings.Contains(lower, "repository") || strings.Contains(lower, "github") {
			repoURL = strings.TrimSpace(value)
			break
		}
	}

	latest := strings.TrimSpace(payload.Info.Version)
	lastUpdated := ""
	for _, release := range payload.Releases[latest] {
		if strings.TrimSpace(release.UploadTimeISO8601) != "" {
			lastUpdated = strings.TrimSpace(release.UploadTimeISO8601)
			break
		}
	}

	deps := make([]PackageDependency, 0)
	for _, item := range payload.Info.RequiresDist {
		name, version := parsePyPIRequiresDist(item)
		if name == "" {
			continue
		}
		deps = append(deps, PackageDependency{
			Name:        name,
			VersionSpec: version,
			Manager:     "pip",
			Registry:    "pypi",
		})
	}

	return &PyPIPackageMetadata{
		Name:          payload.Info.Name,
		LatestVersion: latest,
		Creator:       creator,
		Description:   strings.TrimSpace(payload.Info.Summary),
		License:       strings.TrimSpace(payload.Info.License),
		Homepage:      strings.TrimSpace(payload.Info.HomePage),
		RepositoryURL: repoURL,
		RegistryURL:   fmt.Sprintf("https://pypi.org/project/%s/", payload.Info.Name),
		LastUpdated:   lastUpdated,
		Dependencies:  deps,
	}, nil
}

func parsePyPIRequiresDist(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ""
	}
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	if idx := strings.Index(value, "("); idx >= 0 {
		end := strings.Index(value[idx:], ")")
		if end > 0 {
			name := strings.TrimSpace(value[:idx])
			version := strings.TrimSpace(value[idx+1 : idx+end])
			return name, version
		}
	}
	return value, ""
}
