package main

import (
	"encoding/json"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/chann44/TGE/adapters"
	"github.com/go-chi/chi/v5"
)

type repositoryDependency struct {
	Name            string                `json:"name"`
	VersionSpec     string                `json:"version_spec"`
	VersionSpecs    []string              `json:"version_specs,omitempty"`
	LatestVersion   string                `json:"latest_version"`
	Manager         string                `json:"manager"`
	Registry        string                `json:"registry"`
	Scope           string                `json:"scope"`
	Scopes          []string              `json:"scopes,omitempty"`
	SourceFile      string                `json:"source_file"`
	UsedInFiles     []string              `json:"used_in_files,omitempty"`
	UsageCount      int                   `json:"usage_count"`
	Creator         string                `json:"creator"`
	Description     string                `json:"description"`
	License         string                `json:"license"`
	Homepage        string                `json:"homepage"`
	RepositoryURL   string                `json:"repository_url"`
	RegistryURL     string                `json:"registry_url"`
	LastUpdated     string                `json:"last_updated"`
	DependencyGraph []dependencyGraphNode `json:"dependency_graph,omitempty"`
}

type dependencyGraphNode struct {
	Name          string `json:"name"`
	VersionSpec   string `json:"version_spec"`
	LatestVersion string `json:"latest_version"`
	Manager       string `json:"manager"`
	Registry      string `json:"registry"`
	Parent        string `json:"parent,omitempty"`
	Depth         int    `json:"depth"`
	Creator       string `json:"creator"`
	Description   string `json:"description"`
	License       string `json:"license"`
	Homepage      string `json:"homepage"`
	RepositoryURL string `json:"repository_url"`
	RegistryURL   string `json:"registry_url"`
	LastUpdated   string `json:"last_updated"`
}

type repositoryDependenciesResponse struct {
	RepositoryID int64                  `json:"repository_id"`
	FullName     string                 `json:"full_name"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"page_size"`
	Total        int                    `json:"total"`
	TotalPages   int                    `json:"total_pages"`
	Dependencies []repositoryDependency `json:"dependencies"`
}

func (h *Handler) githubRepositoryDependencies(w http.ResponseWriter, r *http.Request) {
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

	manifestFiles := make([]dependencyFileResponse, 0)
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		fileName := path.Base(entry.Path)
		manager := dependencyManagerForFile(fileName)
		if manager == "" {
			continue
		}
		manifestFiles = append(manifestFiles, dependencyFileResponse{
			Path:     entry.Path,
			File:     fileName,
			Manager:  manager,
			Registry: registryForManager(manager),
		})
	}

	dependencies := make([]repositoryDependency, 0)
	for _, manifest := range manifestFiles {
		content, err := adapters.GetRepositoryFileContent(r.Context(), installationToken, owner, repoName, manifest.Path, repository.DefaultBranch)
		if err != nil {
			continue
		}

		parsed := parseDependenciesFromManifest(manifest, content)
		dependencies = append(dependencies, parsed...)
	}

	includePeer := strings.TrimSpace(r.URL.Query().Get("include_peer"))
	if strings.EqualFold(includePeer, "false") || includePeer == "0" {
		filtered := make([]repositoryDependency, 0, len(dependencies))
		for _, dep := range dependencies {
			if dep.Scope == "peer" {
				continue
			}
			filtered = append(filtered, dep)
		}
		dependencies = filtered
	}

	managerFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("manager")))
	if managerFilter != "" && managerFilter != "all" {
		filtered := make([]repositoryDependency, 0, len(dependencies))
		for _, dep := range dependencies {
			if strings.ToLower(dep.Manager) == managerFilter {
				filtered = append(filtered, dep)
			}
		}
		dependencies = filtered
	}

	scopeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scopeFilter != "" && scopeFilter != "all" {
		filtered := make([]repositoryDependency, 0, len(dependencies))
		for _, dep := range dependencies {
			if strings.ToLower(dep.Scope) == scopeFilter {
				filtered = append(filtered, dep)
			}
		}
		dependencies = filtered
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" {
		filtered := make([]repositoryDependency, 0, len(dependencies))
		for _, dep := range dependencies {
			if strings.Contains(strings.ToLower(dep.Name), q) || strings.Contains(strings.ToLower(dep.SourceFile), q) {
				filtered = append(filtered, dep)
			}
		}
		dependencies = filtered
	}

	enrichDependenciesWithMetadata(r, dependencies)
	dependencies = groupDependencies(dependencies)

	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Manager != dependencies[j].Manager {
			return dependencies[i].Manager < dependencies[j].Manager
		}
		return dependencies[i].Name < dependencies[j].Name
	})

	page := queryInt(r.URL.Query().Get("page"), 1)
	pageSize := queryInt(r.URL.Query().Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}

	total := len(dependencies)
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

	writeJSON(w, http.StatusOK, repositoryDependenciesResponse{
		RepositoryID: repoID,
		FullName:     repository.FullName,
		Page:         page,
		PageSize:     pageSize,
		Total:        total,
		TotalPages:   totalPages,
		Dependencies: dependencies[start:end],
	})
}

func parseDependenciesFromManifest(manifest dependencyFileResponse, content string) []repositoryDependency {
	switch manifest.Manager {
	case "npm":
		return parsePackageJSONDependencies(manifest.Path, content)
	case "pip":
		return parseRequirementsDependencies(manifest.Path, content)
	case "go":
		return parseGoModDependencies(manifest.Path, content)
	default:
		return nil
	}
}

func parsePackageJSONDependencies(sourcePath, content string) []repositoryDependency {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil
	}

	deps := make([]repositoryDependency, 0)
	appendFrom := func(key, scope string) {
		raw, ok := payload[key]
		if !ok {
			return
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return
		}
		for name, versionRaw := range m {
			version, _ := versionRaw.(string)
			deps = append(deps, repositoryDependency{
				Name:        strings.TrimSpace(name),
				VersionSpec: strings.TrimSpace(version),
				Manager:     "npm",
				Registry:    "npm",
				Scope:       scope,
				SourceFile:  sourcePath,
			})
		}
	}

	appendFrom("dependencies", "prod")
	appendFrom("devDependencies", "dev")
	appendFrom("peerDependencies", "peer")
	return deps
}

func parseRequirementsDependencies(sourcePath, content string) []repositoryDependency {
	deps := make([]repositoryDependency, 0)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}

		name, version := splitDependencySpec(trimmed)
		if name == "" {
			continue
		}

		deps = append(deps, repositoryDependency{
			Name:        name,
			VersionSpec: version,
			Manager:     "pip",
			Registry:    "pypi",
			Scope:       "default",
			SourceFile:  sourcePath,
		})
	}
	return deps
}

func parseGoModDependencies(sourcePath, content string) []repositoryDependency {
	deps := make([]repositoryDependency, 0)
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

		if strings.HasPrefix(trimmed, "require ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "require "))
		}

		if inRequireBlock || strings.HasPrefix(line, "require ") || strings.HasPrefix(trimmed, "github.com") || strings.Contains(trimmed, ".") {
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				continue
			}
			module := fields[0]
			version := fields[1]
			if strings.HasPrefix(module, "(") || strings.HasPrefix(module, "module") {
				continue
			}
			deps = append(deps, repositoryDependency{
				Name:        module,
				VersionSpec: version,
				Manager:     "go",
				Registry:    "github",
				Scope:       "default",
				SourceFile:  sourcePath,
			})
		}
	}

	return deps
}

func splitDependencySpec(line string) (string, string) {
	operators := []string{"==", ">=", "<=", "~=", "!=", ">", "<", "="}
	for _, op := range operators {
		idx := strings.Index(line, op)
		if idx > 0 {
			return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx:])
		}
	}
	return strings.TrimSpace(line), ""
}

func groupDependencies(deps []repositoryDependency) []repositoryDependency {
	if len(deps) == 0 {
		return deps
	}

	groupedByKey := make(map[string]*repositoryDependency)
	order := make([]string, 0, len(deps))

	for _, dep := range deps {
		key := strings.ToLower(dep.Manager + "|" + dep.Registry + "|" + dep.Name)
		group, exists := groupedByKey[key]
		if !exists {
			clone := dep
			clone.VersionSpecs = nil
			clone.Scopes = nil
			clone.UsedInFiles = nil
			clone.UsageCount = 0
			clone.DependencyGraph = nil
			groupedByKey[key] = &clone
			group = &clone
			order = append(order, key)
		}

		group.UsageCount++
		group.VersionSpecs = appendUnique(group.VersionSpecs, dep.VersionSpec)
		group.Scopes = appendUnique(group.Scopes, dep.Scope)
		group.UsedInFiles = appendUnique(group.UsedInFiles, dep.SourceFile)

		if group.VersionSpec == "" && dep.VersionSpec != "" {
			group.VersionSpec = dep.VersionSpec
		}
		if group.Scope == "" && dep.Scope != "" {
			group.Scope = dep.Scope
		}
		if group.SourceFile == "" && dep.SourceFile != "" {
			group.SourceFile = dep.SourceFile
		}

		if group.LatestVersion == "" {
			group.LatestVersion = dep.LatestVersion
		}
		if group.Creator == "" {
			group.Creator = dep.Creator
		}
		if group.Description == "" {
			group.Description = dep.Description
		}
		if group.License == "" {
			group.License = dep.License
		}
		if group.Homepage == "" {
			group.Homepage = dep.Homepage
		}
		if group.RepositoryURL == "" {
			group.RepositoryURL = dep.RepositoryURL
		}
		if group.RegistryURL == "" {
			group.RegistryURL = dep.RegistryURL
		}
		if group.LastUpdated == "" {
			group.LastUpdated = dep.LastUpdated
		}
		if len(group.DependencyGraph) == 0 && len(dep.DependencyGraph) > 0 {
			group.DependencyGraph = dep.DependencyGraph
		}
	}

	grouped := make([]repositoryDependency, 0, len(order))
	for _, key := range order {
		grouped = append(grouped, *groupedByKey[key])
	}

	return grouped
}

func appendUnique(items []string, value string) []string {
	v := strings.TrimSpace(value)
	if v == "" {
		return items
	}
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
}

func enrichDependenciesWithMetadata(r *http.Request, deps []repositoryDependency) {
	if len(deps) == 0 {
		return
	}

	workerCount := 6
	if len(deps) < workerCount {
		workerCount = len(deps)
	}

	ch := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range ch {
				dep := &deps[idx]
				switch dep.Manager {
				case "npm":
					meta, err := adapters.GetNPMPackageMetadata(r.Context(), dep.Name)
					if err == nil && meta != nil {
						dep.LatestVersion = meta.LatestVersion
						dep.Creator = meta.Creator
						dep.Description = meta.Description
						dep.License = meta.License
						dep.Homepage = meta.Homepage
						dep.RepositoryURL = meta.RepositoryURL
						dep.RegistryURL = meta.RegistryURL
						dep.LastUpdated = meta.LastUpdated
						dep.DependencyGraph = buildFullDependencyGraph(r, adapters.PackageDependency{
							Name:        dep.Name,
							VersionSpec: dep.VersionSpec,
							Manager:     dep.Manager,
							Registry:    dep.Registry,
						}, meta.Dependencies)
					}
				case "pip":
					meta, err := adapters.GetPyPIPackageMetadata(r.Context(), dep.Name)
					if err == nil && meta != nil {
						dep.LatestVersion = meta.LatestVersion
						dep.Creator = meta.Creator
						dep.Description = meta.Description
						dep.License = meta.License
						dep.Homepage = meta.Homepage
						dep.RepositoryURL = meta.RepositoryURL
						dep.RegistryURL = meta.RegistryURL
						dep.LastUpdated = meta.LastUpdated
						dep.DependencyGraph = buildFullDependencyGraph(r, adapters.PackageDependency{
							Name:        dep.Name,
							VersionSpec: dep.VersionSpec,
							Manager:     dep.Manager,
							Registry:    dep.Registry,
						}, meta.Dependencies)
					}
				case "go":
					meta, err := adapters.GetGoPackageMetadata(r.Context(), dep.Name)
					if err == nil && meta != nil {
						dep.LatestVersion = meta.LatestVersion
						dep.Creator = meta.Creator
						dep.RegistryURL = meta.RegistryURL
						dep.LastUpdated = meta.LastUpdated
						dep.DependencyGraph = buildFullDependencyGraph(r, adapters.PackageDependency{
							Name:        dep.Name,
							VersionSpec: dep.VersionSpec,
							Manager:     dep.Manager,
							Registry:    dep.Registry,
						}, meta.Dependencies)
					}
				}
			}
		}()
	}

	for idx := range deps {
		ch <- idx
	}
	close(ch)
	wg.Wait()
}

type graphQueueItem struct {
	Dep    adapters.PackageDependency
	Parent string
	Depth  int
}

type graphMetadata struct {
	Node     dependencyGraphNode
	Children []adapters.PackageDependency
}

func buildFullDependencyGraph(r *http.Request, root adapters.PackageDependency, roots []adapters.PackageDependency) []dependencyGraphNode {
	if len(roots) == 0 {
		return nil
	}

	const (
		maxGraphNodes = 400
		maxGraphDepth = 6
	)

	queue := make([]graphQueueItem, 0, len(roots))
	for _, dep := range roots {
		queue = append(queue, graphQueueItem{Dep: dep, Parent: root.Name, Depth: 1})
	}

	cache := make(map[string]graphMetadata)
	visited := make(map[string]struct{})
	graph := make([]dependencyGraphNode, 0)

	for len(queue) > 0 && len(graph) < maxGraphNodes {
		item := queue[0]
		queue = queue[1:]

		if item.Depth > maxGraphDepth || strings.TrimSpace(item.Dep.Name) == "" {
			continue
		}

		key := strings.ToLower(item.Dep.Manager + "|" + item.Dep.Name)
		if _, seen := visited[key]; seen {
			continue
		}
		visited[key] = struct{}{}

		metadata, ok := cache[key]
		if !ok {
			resolved := resolveGraphMetadata(r, item.Dep)
			cache[key] = resolved
			metadata = resolved
		}

		node := metadata.Node
		node.Parent = item.Parent
		node.Depth = item.Depth
		if node.VersionSpec == "" {
			node.VersionSpec = item.Dep.VersionSpec
		}
		if node.Manager == "" {
			node.Manager = item.Dep.Manager
		}
		if node.Registry == "" {
			node.Registry = item.Dep.Registry
		}

		graph = append(graph, node)

		for _, child := range metadata.Children {
			queue = append(queue, graphQueueItem{Dep: child, Parent: item.Dep.Name, Depth: item.Depth + 1})
		}
	}

	sort.Slice(graph, func(i, j int) bool {
		if graph[i].Depth != graph[j].Depth {
			return graph[i].Depth < graph[j].Depth
		}
		if graph[i].Manager != graph[j].Manager {
			return graph[i].Manager < graph[j].Manager
		}
		return graph[i].Name < graph[j].Name
	})

	return graph
}

func resolveGraphMetadata(r *http.Request, dep adapters.PackageDependency) graphMetadata {
	node := dependencyGraphNode{
		Name:        dep.Name,
		VersionSpec: dep.VersionSpec,
		Manager:     dep.Manager,
		Registry:    dep.Registry,
	}

	switch dep.Manager {
	case "npm":
		meta, err := adapters.GetNPMPackageMetadata(r.Context(), dep.Name)
		if err != nil || meta == nil {
			return graphMetadata{Node: node}
		}
		node.LatestVersion = meta.LatestVersion
		node.Creator = meta.Creator
		node.Description = meta.Description
		node.License = meta.License
		node.Homepage = meta.Homepage
		node.RepositoryURL = meta.RepositoryURL
		node.RegistryURL = meta.RegistryURL
		node.LastUpdated = meta.LastUpdated
		return graphMetadata{Node: node, Children: meta.Dependencies}
	case "pip":
		meta, err := adapters.GetPyPIPackageMetadata(r.Context(), dep.Name)
		if err != nil || meta == nil {
			return graphMetadata{Node: node}
		}
		node.LatestVersion = meta.LatestVersion
		node.Creator = meta.Creator
		node.Description = meta.Description
		node.License = meta.License
		node.Homepage = meta.Homepage
		node.RepositoryURL = meta.RepositoryURL
		node.RegistryURL = meta.RegistryURL
		node.LastUpdated = meta.LastUpdated
		return graphMetadata{Node: node, Children: meta.Dependencies}
	case "go":
		meta, err := adapters.GetGoPackageMetadata(r.Context(), dep.Name)
		if err != nil || meta == nil {
			return graphMetadata{Node: node}
		}
		node.LatestVersion = meta.LatestVersion
		node.Creator = meta.Creator
		node.RegistryURL = meta.RegistryURL
		node.LastUpdated = meta.LastUpdated
		return graphMetadata{Node: node, Children: meta.Dependencies}
	default:
		return graphMetadata{Node: node}
	}
}
