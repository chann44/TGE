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

type GoPackageMetadata struct {
	Name          string
	LatestVersion string
	Creator       string
	RegistryURL   string
	LastUpdated   string
	Dependencies  []PackageDependency
}

type goProxyLatestResponse struct {
	Version string `json:"Version"`
	Time    string `json:"Time"`
}

func GetGoPackageMetadata(ctx context.Context, modulePath string) (*GoPackageMetadata, error) {
	if strings.TrimSpace(modulePath) == "" {
		return nil, fmt.Errorf("module path is required")
	}

	endpoint := fmt.Sprintf("https://proxy.golang.org/%s/@latest", url.PathEscape(modulePath))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create go metadata request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch go metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go metadata request failed with status %d", resp.StatusCode)
	}

	var payload goProxyLatestResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode go metadata response: %w", err)
	}

	dependencies, _ := getGoModuleDependencies(ctx, modulePath, strings.TrimSpace(payload.Version))

	return &GoPackageMetadata{
		Name:          modulePath,
		LatestVersion: strings.TrimSpace(payload.Version),
		Creator:       inferGoCreator(modulePath),
		RegistryURL:   fmt.Sprintf("https://pkg.go.dev/%s", modulePath),
		LastUpdated:   strings.TrimSpace(payload.Time),
		Dependencies:  dependencies,
	}, nil
}

func getGoModuleDependencies(ctx context.Context, modulePath, version string) ([]PackageDependency, error) {
	if strings.TrimSpace(modulePath) == "" || strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("module path and version are required")
	}

	endpoint := fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.mod", url.PathEscape(modulePath), url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create go module file request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch go module file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go module file request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read go module file: %w", err)
	}

	return parseGoModDependenciesFromContent(string(body)), nil
}

func parseGoModDependenciesFromContent(content string) []PackageDependency {
	deps := make([]PackageDependency, 0)
	lines := strings.Split(content, "\n")
	inRequireBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && trimmed == ")" {
			inRequireBlock = false
			continue
		}

		candidate := trimmed
		if strings.HasPrefix(candidate, "require ") {
			candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "require "))
		}

		if !inRequireBlock && !strings.HasPrefix(trimmed, "require ") {
			continue
		}

		fields := strings.Fields(candidate)
		if len(fields) < 2 {
			continue
		}

		module := strings.TrimSpace(fields[0])
		version := strings.TrimSpace(fields[1])
		if module == "" || strings.HasPrefix(module, "module") {
			continue
		}

		deps = append(deps, PackageDependency{
			Name:        module,
			VersionSpec: version,
			Manager:     "go",
			Registry:    "github",
		})
	}

	return deps
}

func inferGoCreator(modulePath string) string {
	parts := strings.Split(strings.TrimSpace(modulePath), "/")
	if len(parts) >= 3 && parts[0] == "github.com" {
		return parts[1]
	}
	if len(parts) >= 2 && parts[0] == "golang.org" {
		return "golang"
	}
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}
