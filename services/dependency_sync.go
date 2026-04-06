package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chann44/TGE/adapters"
	internal "github.com/chann44/TGE/internals"
	db "github.com/chann44/TGE/internals/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type manifestFile struct {
	Path     string
	File     string
	Manager  string
	Registry string
}

type extractedDependency struct {
	Name       string
	Version    string
	Manager    string
	Registry   string
	Scope      string
	SourceFile string
}

func SyncRepositoryDependencies(ctx context.Context, queries *db.Queries, cfg *internal.Config, userID, repoID, syncID int64, trigger string, force bool) (retErr error) {
	if trigger == "" {
		trigger = "manual"
	}

	if syncID == 0 {
		syncRow, err := queries.CreateRepositoryDependencySync(ctx, db.CreateRepositoryDependencySyncParams{
			RepositoryID: repoID,
			Status:       "running",
			Trigger:      trigger,
		})
		if err != nil {
			return fmt.Errorf("create dependency sync row: %w", err)
		}
		syncID = syncRow.ID
	} else {
		if err := queries.MarkRepositoryDependencySyncRunning(ctx, syncID); err != nil {
			return fmt.Errorf("mark dependency sync running: %w", err)
		}
	}

	defer func() {
		if retErr != nil {
			_ = queries.MarkRepositoryDependencySyncFailed(ctx, db.MarkRepositoryDependencySyncFailedParams{
				ID:           syncID,
				ErrorMessage: truncateForDB(retErr.Error(), 8000),
			})
			return
		}
		_ = queries.MarkRepositoryDependencySyncSuccess(ctx, syncID)
	}()

	repo, err := findConnectedRepository(ctx, queries, userID, repoID)
	if err != nil {
		return err
	}

	installationToken, repository, err := installationTokenForRepo(ctx, queries, cfg, userID, repoID)
	if err != nil {
		return fmt.Errorf("resolve installation token: %w", err)
	}

	owner, repoName, ok := strings.Cut(repository.FullName, "/")
	if !ok || strings.TrimSpace(owner) == "" || strings.TrimSpace(repoName) == "" {
		return fmt.Errorf("invalid repository full name: %s", repository.FullName)
	}

	branch := strings.TrimSpace(repo.DefaultBranch)
	if branch == "" {
		branch = strings.TrimSpace(repository.DefaultBranch)
	}

	tree, err := adapters.ListRepositoryTree(ctx, installationToken, owner, repoName, branch)
	if err != nil {
		return fmt.Errorf("list repository tree: %w", err)
	}

	files := make([]manifestFile, 0)
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		fileName := path.Base(entry.Path)
		manager := dependencyManagerForFile(fileName)
		if manager == "" {
			continue
		}
		files = append(files, manifestFile{
			Path:     entry.Path,
			File:     fileName,
			Manager:  manager,
			Registry: registryForManager(manager),
		})
	}

	if err := queries.DeleteRepositoryDependencyFilesByRepo(ctx, repoID); err != nil {
		return fmt.Errorf("clear dependency files: %w", err)
	}
	for _, file := range files {
		if err := queries.UpsertRepositoryDependencyFile(ctx, db.UpsertRepositoryDependencyFileParams{
			RepositoryID: repoID,
			Path:         file.Path,
			File:         file.File,
			Manager:      file.Manager,
			Registry:     file.Registry,
		}); err != nil {
			return fmt.Errorf("upsert dependency file %s: %w", file.Path, err)
		}
	}

	deps := make([]extractedDependency, 0)
	for _, file := range files {
		content, fileErr := adapters.GetRepositoryFileContent(ctx, installationToken, owner, repoName, file.Path, branch)
		if fileErr != nil {
			continue
		}
		deps = append(deps, parseDependenciesFromManifest(file, content)...)
	}

	if err := queries.DeleteRepositoryDependenciesByRepo(ctx, repoID); err != nil {
		return fmt.Errorf("clear repository dependencies: %w", err)
	}

	resolver := newMetadataResolver(ctx)
	graphState := &graphPersistState{visitedEdges: make(map[string]struct{})}
	for _, dep := range deps {
		if strings.TrimSpace(dep.Name) == "" {
			continue
		}

		pkg, err := queries.UpsertDependencyPackage(ctx, db.UpsertDependencyPackageParams{
			Manager:        dep.Manager,
			Registry:       dep.Registry,
			NormalizedName: normalizeDependencyName(dep.Manager, dep.Name),
			DisplayName:    dep.Name,
		})
		if err != nil {
			return fmt.Errorf("upsert package %s: %w", dep.Name, err)
		}

		resolvedVersion := sanitizeVersion(dep.Version)
		meta := resolvedMetadata{}
		var resolvedVersionRow db.DependencyPackageVersion
		usedExisting := false

		if !force && resolvedVersion != "" {
			existingVersion, found, findErr := findDependencyPackageVersion(ctx, queries, dep.Manager, dep.Registry, dep.Name, resolvedVersion)
			if findErr != nil {
				return findErr
			}
			if found {
				resolvedVersionRow = existingVersion
				usedExisting = true
			}
		}

		if !usedExisting {
			meta = resolver.Resolve(dep)
			if resolvedVersion == "" {
				resolvedVersion = sanitizeVersion(meta.Version)
			}
			if resolvedVersion == "" {
				resolvedVersion = "unknown"
			}

			resolvedVersionRow, err = queries.UpsertDependencyPackageVersion(ctx, db.UpsertDependencyPackageVersionParams{
				PackageID:     pkg.ID,
				Version:       resolvedVersion,
				Creator:       meta.Creator,
				Description:   meta.Description,
				License:       meta.License,
				Homepage:      meta.Homepage,
				RepositoryUrl: meta.RepositoryURL,
				RegistryUrl:   meta.RegistryURL,
				ReleasedAt:    parseTimestamptz(meta.LastUpdated),
			})
			if err != nil {
				return fmt.Errorf("upsert package version %s@%s: %w", dep.Name, resolvedVersion, err)
			}
		}

		if err := queries.UpsertRepositoryDependency(ctx, db.UpsertRepositoryDependencyParams{
			RepositoryID: repoID,
			PackageID:    pkg.ID,
			SourceFile:   dep.SourceFile,
			Scope:        normalizeScope(dep.Scope),
			VersionSpec:  dep.Version,
			ResolvedVersionID: pgtype.Int8{
				Int64: resolvedVersionRow.ID,
				Valid: true,
			},
		}); err != nil {
			return fmt.Errorf("upsert repository dependency %s: %w", dep.Name, err)
		}

		if err := persistTransitiveDependencies(ctx, queries, resolver, graphState, resolvedVersionRow.ID, dep.Manager, meta.Dependencies, 1, force); err != nil {
			return err
		}
	}

	return nil
}

func findConnectedRepository(ctx context.Context, queries *db.Queries, userID, repoID int64) (*db.Repository, error) {
	repos, err := queries.ListUserRepositories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list connected repositories: %w", err)
	}
	for _, repo := range repos {
		if repo.GithubRepoID == repoID {
			copy := repo
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("repository %d is not connected", repoID)
}

func installationTokenForRepo(ctx context.Context, queries *db.Queries, cfg *internal.Config, userID, repoID int64) (string, *adapters.GitHubRepository, error) {
	issuer := strings.TrimSpace(cfg.GithubAppID)
	if issuer == "" {
		issuer = strings.TrimSpace(cfg.GithubClientID)
	}
	if issuer == "" {
		return "", nil, fmt.Errorf("github app issuer is not configured")
	}

	installations, err := queries.ListUserGitHubInstallations(ctx, userID)
	if err != nil {
		return "", nil, fmt.Errorf("list github installations: %w", err)
	}

	for _, installation := range installations {
		token, tokenErr := adapters.CreateInstallationAccessToken(ctx, issuer, cfg.GithubAppPrivateKey, installation.InstallationID)
		if tokenErr != nil {
			continue
		}
		repo, repoErr := adapters.GetRepositoryByID(ctx, token, repoID)
		if repoErr == nil {
			return token, repo, nil
		}
	}

	return "", nil, fmt.Errorf("no installation access for repository")
}

func dependencyManagerForFile(fileName string) string {
	switch strings.ToLower(fileName) {
	case "package.json":
		return "npm"
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml":
		return "npm"
	case "requirements.txt":
		return "pip"
	case "pyproject.toml", "poetry.lock", "pipfile.lock":
		return "pip"
	case "go.mod":
		return "go"
	case "go.sum":
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

func parseDependenciesFromManifest(manifest manifestFile, content string) []extractedDependency {
	if !isDependencyManifest(manifest.File) {
		return nil
	}
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

func isDependencyManifest(fileName string) bool {
	switch strings.ToLower(strings.TrimSpace(fileName)) {
	case "package.json", "requirements.txt", "go.mod":
		return true
	default:
		return false
	}
}

func parsePackageJSONDependencies(sourcePath, content string) []extractedDependency {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil
	}

	deps := make([]extractedDependency, 0)
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
			deps = append(deps, extractedDependency{
				Name:       strings.TrimSpace(name),
				Version:    strings.TrimSpace(version),
				Manager:    "npm",
				Registry:   "npm",
				Scope:      scope,
				SourceFile: sourcePath,
			})
		}
	}

	appendFrom("dependencies", "prod")
	appendFrom("devDependencies", "dev")
	appendFrom("peerDependencies", "peer")
	return deps
}

func parseRequirementsDependencies(sourcePath, content string) []extractedDependency {
	deps := make([]extractedDependency, 0)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}

		name, version := splitDependencySpec(trimmed)
		if name == "" {
			continue
		}

		deps = append(deps, extractedDependency{
			Name:       name,
			Version:    version,
			Manager:    "pip",
			Registry:   "pypi",
			Scope:      "default",
			SourceFile: sourcePath,
		})
	}
	return deps
}

func parseGoModDependencies(sourcePath, content string) []extractedDependency {
	deps := make([]extractedDependency, 0)
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

		if inRequireBlock || strings.HasPrefix(line, "require ") || strings.Contains(trimmed, ".") {
			parts := strings.Fields(trimmed)
			if len(parts) < 2 {
				continue
			}
			module := strings.TrimSpace(parts[0])
			version := strings.TrimSpace(parts[1])
			if module == "" || strings.HasPrefix(module, "module") {
				continue
			}
			deps = append(deps, extractedDependency{
				Name:       module,
				Version:    version,
				Manager:    "go",
				Registry:   "github",
				Scope:      "default",
				SourceFile: sourcePath,
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

type resolvedMetadata struct {
	Version       string
	Creator       string
	Description   string
	License       string
	Homepage      string
	RepositoryURL string
	RegistryURL   string
	LastUpdated   string
	Dependencies  []adapters.PackageDependency
}

type metadataResolver struct {
	ctx   context.Context
	mu    sync.Mutex
	cache map[string]resolvedMetadata
}

type graphPersistState struct {
	visitedEdges map[string]struct{}
	nodeCount    int
}

const (
	maxPersistGraphDepth = 5
	maxPersistGraphNodes = 1500
)

func newMetadataResolver(ctx context.Context) *metadataResolver {
	return &metadataResolver{ctx: ctx, cache: make(map[string]resolvedMetadata)}
}

func (r *metadataResolver) Resolve(dep extractedDependency) resolvedMetadata {
	key := strings.ToLower(dep.Manager + "|" + dep.Name)
	r.mu.Lock()
	if cached, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()

	meta := resolvedMetadata{}
	switch dep.Manager {
	case "npm":
		payload, err := adapters.GetNPMPackageMetadata(r.ctx, dep.Name)
		if err == nil && payload != nil {
			meta = resolvedMetadata{
				Version:       payload.LatestVersion,
				Creator:       payload.Creator,
				Description:   payload.Description,
				License:       payload.License,
				Homepage:      payload.Homepage,
				RepositoryURL: payload.RepositoryURL,
				RegistryURL:   payload.RegistryURL,
				LastUpdated:   payload.LastUpdated,
				Dependencies:  payload.Dependencies,
			}
		}
	case "pip":
		payload, err := adapters.GetPyPIPackageMetadata(r.ctx, dep.Name)
		if err == nil && payload != nil {
			meta = resolvedMetadata{
				Version:       payload.LatestVersion,
				Creator:       payload.Creator,
				Description:   payload.Description,
				License:       payload.License,
				Homepage:      payload.Homepage,
				RepositoryURL: payload.RepositoryURL,
				RegistryURL:   payload.RegistryURL,
				LastUpdated:   payload.LastUpdated,
				Dependencies:  payload.Dependencies,
			}
		}
	case "go":
		payload, err := adapters.GetGoPackageMetadata(r.ctx, dep.Name)
		if err == nil && payload != nil {
			meta = resolvedMetadata{
				Version:      payload.LatestVersion,
				Creator:      payload.Creator,
				RegistryURL:  payload.RegistryURL,
				LastUpdated:  payload.LastUpdated,
				Dependencies: payload.Dependencies,
			}
		}
	}

	r.mu.Lock()
	r.cache[key] = meta
	r.mu.Unlock()
	return meta
}

func persistTransitiveDependencies(
	ctx context.Context,
	queries *db.Queries,
	resolver *metadataResolver,
	state *graphPersistState,
	fromVersionID int64,
	defaultManager string,
	children []adapters.PackageDependency,
	depth int,
	force bool,
) error {
	if depth > maxPersistGraphDepth || len(children) == 0 {
		return nil
	}
	if !force {
		edgeCount, err := queries.CountDependencyEdgesByFromVersion(ctx, fromVersionID)
		if err == nil && edgeCount > 0 {
			return nil
		}
	}

	for _, child := range children {
		if state.nodeCount >= maxPersistGraphNodes {
			return nil
		}
		childName := strings.TrimSpace(child.Name)
		if childName == "" {
			continue
		}

		manager := strings.TrimSpace(child.Manager)
		if manager == "" {
			manager = defaultManager
		}
		registry := strings.TrimSpace(child.Registry)
		if registry == "" {
			registry = registryForManager(manager)
		}

		seed := extractedDependency{
			Name:     childName,
			Version:  strings.TrimSpace(child.VersionSpec),
			Manager:  manager,
			Registry: registry,
			Scope:    normalizeScope(child.Scope),
		}
		childMeta := resolver.Resolve(seed)

		childPackage, err := queries.UpsertDependencyPackage(ctx, db.UpsertDependencyPackageParams{
			Manager:        manager,
			Registry:       registry,
			NormalizedName: normalizeDependencyName(manager, childName),
			DisplayName:    childName,
		})
		if err != nil {
			return fmt.Errorf("upsert transitive package %s: %w", childName, err)
		}

		concreteVersion := resolveConcreteVersion(child.VersionSpec, childMeta.Version)
		childVersionRow, err := queries.UpsertDependencyPackageVersion(ctx, db.UpsertDependencyPackageVersionParams{
			PackageID:     childPackage.ID,
			Version:       concreteVersion,
			Creator:       strings.TrimSpace(childMeta.Creator),
			Description:   strings.TrimSpace(childMeta.Description),
			License:       strings.TrimSpace(childMeta.License),
			Homepage:      strings.TrimSpace(childMeta.Homepage),
			RepositoryUrl: strings.TrimSpace(childMeta.RepositoryURL),
			RegistryUrl:   strings.TrimSpace(childMeta.RegistryURL),
			ReleasedAt:    parseTimestamptz(childMeta.LastUpdated),
		})
		if err != nil {
			return fmt.Errorf("upsert transitive package version %s@%s: %w", childName, concreteVersion, err)
		}

		edgeKey := fmt.Sprintf("%d:%d:%s", fromVersionID, childVersionRow.ID, normalizeScope(child.Scope))
		if _, exists := state.visitedEdges[edgeKey]; exists {
			continue
		}
		state.visitedEdges[edgeKey] = struct{}{}

		if err := queries.UpsertDependencyVersionDependency(ctx, db.UpsertDependencyVersionDependencyParams{
			FromVersionID:  fromVersionID,
			ToVersionID:    childVersionRow.ID,
			DependencyType: normalizeScope(child.Scope),
			VersionSpec:    strings.TrimSpace(child.VersionSpec),
		}); err != nil {
			return fmt.Errorf("upsert transitive edge %d -> %d: %w", fromVersionID, childVersionRow.ID, err)
		}

		state.nodeCount++
		if err := persistTransitiveDependencies(
			ctx,
			queries,
			resolver,
			state,
			childVersionRow.ID,
			manager,
			childMeta.Dependencies,
			depth+1,
			force,
		); err != nil {
			return err
		}
	}

	return nil
}

func findDependencyPackageVersion(
	ctx context.Context,
	queries *db.Queries,
	manager, registry, name, version string,
) (db.DependencyPackageVersion, bool, error) {
	pkg, err := queries.GetDependencyPackageByKey(ctx, db.GetDependencyPackageByKeyParams{
		Manager:        manager,
		Registry:       registry,
		NormalizedName: normalizeDependencyName(manager, name),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.DependencyPackageVersion{}, false, nil
		}
		return db.DependencyPackageVersion{}, false, fmt.Errorf("query package cache %s: %w", name, err)
	}

	versionRow, err := queries.GetDependencyPackageVersionByPackageAndVersion(ctx, db.GetDependencyPackageVersionByPackageAndVersionParams{
		PackageID: pkg.ID,
		Version:   version,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.DependencyPackageVersion{}, false, nil
		}
		return db.DependencyPackageVersion{}, false, fmt.Errorf("query package version cache %s@%s: %w", name, version, err)
	}

	return versionRow, true, nil
}

func resolveConcreteVersion(versionSpec, fallbackLatest string) string {
	resolved := sanitizeVersion(versionSpec)
	if resolved != "" {
		return resolved
	}
	resolved = sanitizeVersion(fallbackLatest)
	if resolved != "" {
		return resolved
	}
	return "unknown"
}

func parseTimestamptz(raw string) pgtype.Timestamptz {
	value := strings.TrimSpace(raw)
	if value == "" {
		return pgtype.Timestamptz{}
	}

	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999Z", "2006-01-02T15:04:05Z"}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return pgtype.Timestamptz{Time: parsed, Valid: true}
		}
	}

	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		return pgtype.Timestamptz{Time: time.Unix(unix, 0), Valid: true}
	}

	return pgtype.Timestamptz{}
}

func normalizeDependencyName(manager, name string) string {
	v := strings.TrimSpace(name)
	if strings.EqualFold(manager, "go") {
		return strings.ToLower(v)
	}
	return strings.ToLower(v)
}

func sanitizeVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "=")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, ">") || strings.HasPrefix(v, "<") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, "^") || strings.Contains(v, " ") || strings.Contains(v, ",") {
		return ""
	}
	return v
}

func normalizeScope(scope string) string {
	v := strings.ToLower(strings.TrimSpace(scope))
	switch v {
	case "prod", "dev", "peer", "optional", "default":
		return v
	default:
		return "default"
	}
}

func truncateForDB(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
