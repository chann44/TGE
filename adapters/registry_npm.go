package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NPMPackageMetadata struct {
	Name             string
	LatestVersion    string
	Creator          string
	Description      string
	License          string
	Homepage         string
	RepositoryURL    string
	RegistryURL      string
	LastUpdated      string
	HasInstallScript bool
	Dependencies     []PackageDependency
}

type npmRegistryResponse struct {
	Name     string `json:"name"`
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Time   map[string]string `json:"time"`
	Author struct {
		Name string `json:"name"`
	} `json:"author"`
	Maintains []struct {
		Name string `json:"name"`
	} `json:"maintainers"`
	Versions map[string]struct {
		Description          string            `json:"description"`
		License              string            `json:"license"`
		Homepage             string            `json:"homepage"`
		Scripts              map[string]string `json:"scripts"`
		Dependencies         map[string]string `json:"dependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
		Repository           struct {
			URL string `json:"url"`
		} `json:"repository"`
	} `json:"versions"`
}

func GetNPMPackageMetadata(ctx context.Context, packageName string) (*NPMPackageMetadata, error) {
	if strings.TrimSpace(packageName) == "" {
		return nil, fmt.Errorf("package name is required")
	}

	endpoint := fmt.Sprintf("https://registry.npmjs.org/%s", url.PathEscape(packageName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create npm metadata request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch npm metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm metadata request failed with status %d", resp.StatusCode)
	}

	var payload npmRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode npm metadata response: %w", err)
	}

	latest := strings.TrimSpace(payload.DistTags.Latest)
	versionPayload, ok := payload.Versions[latest]
	if !ok {
		for version, current := range payload.Versions {
			latest = version
			versionPayload = current
			break
		}
	}

	creator := strings.TrimSpace(payload.Author.Name)
	if creator == "" && len(payload.Maintains) > 0 {
		creator = strings.TrimSpace(payload.Maintains[0].Name)
	}

	metadata := &NPMPackageMetadata{
		Name:             payload.Name,
		LatestVersion:    latest,
		Creator:          creator,
		Description:      strings.TrimSpace(versionPayload.Description),
		License:          strings.TrimSpace(versionPayload.License),
		Homepage:         strings.TrimSpace(versionPayload.Homepage),
		RepositoryURL:    normalizeRepositoryURL(strings.TrimSpace(versionPayload.Repository.URL)),
		RegistryURL:      fmt.Sprintf("https://www.npmjs.com/package/%s", payload.Name),
		LastUpdated:      strings.TrimSpace(payload.Time[latest]),
		HasInstallScript: hasNPMInstallScript(versionPayload.Scripts),
		Dependencies:     make([]PackageDependency, 0),
	}

	for depName, depVersion := range versionPayload.Dependencies {
		metadata.Dependencies = append(metadata.Dependencies, PackageDependency{
			Name:        strings.TrimSpace(depName),
			VersionSpec: strings.TrimSpace(depVersion),
			Manager:     "npm",
			Registry:    "npm",
			Scope:       "prod",
		})
	}

	for depName, depVersion := range versionPayload.PeerDependencies {
		metadata.Dependencies = append(metadata.Dependencies, PackageDependency{
			Name:        strings.TrimSpace(depName),
			VersionSpec: strings.TrimSpace(depVersion),
			Manager:     "npm",
			Registry:    "npm",
			Scope:       "peer",
		})
	}

	for depName, depVersion := range versionPayload.OptionalDependencies {
		metadata.Dependencies = append(metadata.Dependencies, PackageDependency{
			Name:        strings.TrimSpace(depName),
			VersionSpec: strings.TrimSpace(depVersion),
			Manager:     "npm",
			Registry:    "npm",
			Scope:       "optional",
		})
	}

	if metadata.LastUpdated == "" {
		metadata.LastUpdated = strings.TrimSpace(payload.Time["modified"])
	}

	return metadata, nil
}

func hasNPMInstallScript(scripts map[string]string) bool {
	if len(scripts) == 0 {
		return false
	}
	for key := range scripts {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "preinstall", "install", "postinstall":
			return true
		}
	}
	return false
}

func normalizeRepositoryURL(raw string) string {
	if raw == "" {
		return ""
	}
	normalized := strings.TrimPrefix(raw, "git+")
	normalized = strings.TrimSuffix(normalized, ".git")
	normalized = strings.TrimPrefix(normalized, "git://")
	if strings.HasPrefix(normalized, "github.com/") {
		normalized = "https://" + normalized
	}
	return normalized
}
