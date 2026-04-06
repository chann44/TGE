package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chann44/TGE/adapters"
	internal "github.com/chann44/TGE/internals"
	db "github.com/chann44/TGE/internals/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type policySourceConfig struct {
	OSVEnabled  bool
	GHSAEnabled bool
	NVDEnabled  bool
	GHSAToken   string
	NVDAPIKey   string
}

type normalizedFinding struct {
	PackageID       pgtype.Int8
	PackageName     string
	Manager         string
	Registry        string
	VersionSpec     string
	ResolvedVersion string
	AdvisoryID      string
	Aliases         []string
	Title           string
	Summary         string
	Severity        db.Severity
	FixedVersion    string
	ReferenceURL    string
	Sources         []string
}

type findingCounts struct {
	Total    int32
	Critical int32
	High     int32
	Medium   int32
	Low      int32
}

func RunRepositoryScan(ctx context.Context, queries *db.Queries, cfg *internal.Config, logger *adapters.CentralLogger, userID, repoID, scanRunID int64, trigger string) (retErr error) {
	if trigger == "" {
		trigger = "manual"
	}

	policyID, sourceCfg := resolvePolicyForScan(ctx, queries, cfg, userID, repoID)

	if scanRunID == 0 {
		run, err := queries.CreateRepositoryScanRun(ctx, db.CreateRepositoryScanRunParams{
			RepositoryID: repoID,
			PolicyID:     policyID,
			Trigger:      normalizeScanTrigger(trigger),
		})
		if err != nil {
			return fmt.Errorf("create scan run row: %w", err)
		}
		scanRunID = run.ID
	}

	if err := queries.MarkRepositoryScanRunRunning(ctx, scanRunID); err != nil {
		return fmt.Errorf("mark scan run running: %w", err)
	}
	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", "scan run started", "")

	defer func() {
		if retErr != nil {
			_ = queries.MarkRepositoryScanRunFailed(ctx, db.MarkRepositoryScanRunFailedParams{
				ID:           scanRunID,
				ErrorMessage: truncateForDB(retErr.Error(), 8000),
			})
			return
		}

		counts, countErr := countFindingsBySeverity(ctx, queries, scanRunID, userID)
		if countErr != nil {
			_ = queries.MarkRepositoryScanRunFailed(ctx, db.MarkRepositoryScanRunFailedParams{
				ID:           scanRunID,
				ErrorMessage: truncateForDB(countErr.Error(), 8000),
			})
			retErr = countErr
			return
		}

		_ = queries.MarkRepositoryScanRunSuccess(ctx, db.MarkRepositoryScanRunSuccessParams{
			ID:               scanRunID,
			FindingsTotal:    counts.Total,
			FindingsCritical: counts.Critical,
			FindingsHigh:     counts.High,
			FindingsMedium:   counts.Medium,
			FindingsLow:      counts.Low,
		})
	}()

	deps, err := queries.ListRepositoryDependenciesDetailed(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list repository dependencies: %w", err)
	}
	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", fmt.Sprintf("loaded %d direct dependency records", len(deps)), "")
	if len(deps) == 0 {
		logScanEvent(ctx, queries, logger, scanRunID, repoID, "warn", "no dependencies found; nothing to scan", "")
		return nil
	}

	allDeps := expandDependenciesWithGraph(ctx, queries, deps)
	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", fmt.Sprintf("expanded to %d dependency nodes including graph", len(allDeps)), "")

	for _, directory := range uniqueDependencyDirectories(allDeps) {
		logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", "scanning directory", directory)
	}

	findingMap := make(map[string]*normalizedFinding)
	if sourceCfg.OSVEnabled {
		logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", "querying OSV advisories", "")
		if err := collectOSVFindings(ctx, allDeps, sourceCfg, findingMap); err != nil {
			logScanEvent(ctx, queries, logger, scanRunID, repoID, "error", "OSV query failed: "+truncateForDB(err.Error(), 2000), "")
			return err
		}
	}

	addSupplyChainFlags(ctx, queries, logger, cfg, scanRunID, repoID, allDeps, findingMap)

	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", "enriching findings with GHSA/NVD", "")
	if err := enrichFindingsFromProviders(ctx, sourceCfg, findingMap); err != nil {
		logScanEvent(ctx, queries, logger, scanRunID, repoID, "error", "provider enrichment failed: "+truncateForDB(err.Error(), 2000), "")
		return err
	}

	items := make([]*normalizedFinding, 0, len(findingMap))
	for _, item := range findingMap {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Severity != items[j].Severity {
			return items[i].Severity > items[j].Severity
		}
		if items[i].PackageName != items[j].PackageName {
			return items[i].PackageName < items[j].PackageName
		}
		return items[i].AdvisoryID < items[j].AdvisoryID
	})

	for _, finding := range items {
		row, err := queries.CreateRepositoryScanFinding(ctx, db.CreateRepositoryScanFindingParams{
			ScanRunID:       scanRunID,
			RepositoryID:    repoID,
			PolicyID:        policyID,
			PackageID:       finding.PackageID,
			PackageName:     finding.PackageName,
			Manager:         finding.Manager,
			Registry:        finding.Registry,
			VersionSpec:     finding.VersionSpec,
			ResolvedVersion: finding.ResolvedVersion,
			AdvisoryID:      finding.AdvisoryID,
			Aliases:         finding.Aliases,
			Title:           finding.Title,
			Summary:         finding.Summary,
			Severity:        finding.Severity,
			FixedVersion:    finding.FixedVersion,
			ReferenceUrl:    finding.ReferenceURL,
		})
		if err != nil {
			return fmt.Errorf("create scan finding: %w", err)
		}

		for _, source := range finding.Sources {
			if err := queries.AddRepositoryScanFindingSource(ctx, db.AddRepositoryScanFindingSourceParams{
				FindingID:        row.ID,
				Source:           source,
				ProviderRecordID: finding.AdvisoryID,
			}); err != nil {
				return fmt.Errorf("attach finding source: %w", err)
			}
		}
	}

	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", fmt.Sprintf("scan completed with %d finding(s)", len(items)), "")

	return nil
}

func logScanEvent(ctx context.Context, queries *db.Queries, logger *adapters.CentralLogger, scanRunID, repoID int64, level, message, directory string) {
	msg := truncateForDB(strings.TrimSpace(message), 8000)
	dir := truncateForDB(strings.TrimSpace(directory), 1024)
	_ = queries.CreateRepositoryScanLog(ctx, db.CreateRepositoryScanLogParams{
		ScanRunID:     scanRunID,
		RepositoryID:  repoID,
		Level:         sanitizeLogLevel(level),
		Message:       msg,
		DirectoryPath: dir,
	})
	if logger != nil {
		logger.Log(ctx, "scan", sanitizeLogLevel(level), msg, map[string]any{
			"scan_run_id":    scanRunID,
			"repository_id":  repoID,
			"directory_path": dir,
		})
	}
}

func sanitizeLogLevel(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "debug", "info", "warn", "error":
		return v
	default:
		return "info"
	}
}

func uniqueDependencyDirectories(deps []db.ListRepositoryDependenciesDetailedRow) []string {
	if len(deps) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	items := make([]string, 0)
	for _, dep := range deps {
		directory := normalizeDependencyDirectory(dep.SourceFile)
		if directory == "" {
			continue
		}
		if _, ok := seen[directory]; ok {
			continue
		}
		seen[directory] = struct{}{}
		items = append(items, directory)
	}
	sort.Strings(items)
	return items
}

func normalizeDependencyDirectory(sourceFile string) string {
	v := strings.TrimSpace(sourceFile)
	if v == "" {
		return ""
	}
	v = strings.ReplaceAll(v, "\\", "/")
	v = path.Clean(v)
	if v == "." || v == "/" {
		return "/"
	}
	directory := path.Dir(v)
	if directory == "." {
		return "/"
	}
	if strings.HasPrefix(directory, "../") {
		return "/"
	}
	return directory
}

func expandDependenciesWithGraph(ctx context.Context, queries *db.Queries, deps []db.ListRepositoryDependenciesDetailedRow) []db.ListRepositoryDependenciesDetailedRow {
	if len(deps) == 0 {
		return deps
	}

	items := make([]db.ListRepositoryDependenciesDetailedRow, 0, len(deps)*2)
	seen := make(map[string]struct{})
	queue := make([]int64, 0, len(deps))
	visitedVersion := make(map[int64]struct{})

	appendDep := func(dep db.ListRepositoryDependenciesDetailedRow) {
		key := strings.ToLower(strings.TrimSpace(dep.Manager) + "|" + strings.TrimSpace(dep.Registry) + "|" + strings.TrimSpace(dep.DisplayName) + "|" + resolvedVersionForDependency(dep))
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, dep)
	}

	for _, dep := range deps {
		appendDep(dep)
		if dep.ResolvedVersionID.Valid {
			queue = append(queue, dep.ResolvedVersionID.Int64)
		}
	}

	const maxDepth = 4
	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		nextQueue := make([]int64, 0, len(queue)*2)
		for _, versionID := range queue {
			if _, ok := visitedVersion[versionID]; ok {
				continue
			}
			visitedVersion[versionID] = struct{}{}
			edges, err := queries.ListDependencyEdgesByFromVersion(ctx, versionID)
			if err != nil {
				continue
			}
			for _, edge := range edges {
				dep := db.ListRepositoryDependenciesDetailedRow{
					PackageID:       0,
					SourceFile:      "",
					Scope:           strings.TrimSpace(edge.DependencyType),
					VersionSpec:     strings.TrimSpace(edge.VersionSpec),
					ResolvedVersion: pgtype.Text{String: strings.TrimSpace(edge.ChildVersion), Valid: strings.TrimSpace(edge.ChildVersion) != ""},
					Manager:         strings.TrimSpace(edge.ChildManager),
					Registry:        strings.TrimSpace(edge.ChildRegistry),
					DisplayName:     strings.TrimSpace(edge.ChildName),
					Description:     pgtype.Text{String: strings.TrimSpace(edge.ChildDescription), Valid: strings.TrimSpace(edge.ChildDescription) != ""},
					RepositoryUrl:   pgtype.Text{String: strings.TrimSpace(edge.ChildRepositoryUrl), Valid: strings.TrimSpace(edge.ChildRepositoryUrl) != ""},
					RegistryUrl:     pgtype.Text{String: strings.TrimSpace(edge.ChildRegistryUrl), Valid: strings.TrimSpace(edge.ChildRegistryUrl) != ""},
					ReleasedAt:      edge.ChildReleasedAt,
				}
				appendDep(dep)
				nextQueue = append(nextQueue, edge.ToVersionID)
			}
		}
		queue = nextQueue
	}

	return items
}

type supplyChainSignal struct {
	SignalID string
	Weight   int
	Category string
	Detail   string
}

type supplyChainFlag struct {
	FlagID          string
	Title           string
	Summary         string
	Severity        db.Severity
	PackageName     string
	Manager         string
	Registry        string
	VersionSpec     string
	ResolvedVersion string
	ReferenceURL    string
	Remediation     string
	Category        string
	IsRepoLevel     bool
	Signals         []string
}

const (
	scoreTyposquat        = 30
	scoreInstallScript    = 25
	scoreRecentHighVer    = 20
	scoreMissingRepo      = 10
	scoreDepConfusion     = 35
	scoreKnownMalware     = 50
	scoreAbandonedVuln    = 15
	scoreVersionAnomaly   = 30
	scoreSingleMaintainer = 5
	scoreMissingLockfile  = 20
)

const (
	thresholdCritical = 50
	thresholdHigh     = 30
	thresholdMedium   = 15
)

func addSupplyChainFlags(ctx context.Context, queries *db.Queries, logger *adapters.CentralLogger, cfg *internal.Config, scanRunID, repoID int64, deps []db.ListRepositoryDependenciesDetailedRow, findingMap map[string]*normalizedFinding) {
	if len(deps) == 0 {
		return
	}

	directDeps := filterDirectDeps(deps)
	depFiles, _ := queries.ListRepositoryDependencyFiles(ctx, repoID)

	flags := make([]supplyChainFlag, 0)
	flags = append(flags, checkLockfiles(depFiles)...)

	packageScores := map[string]*packageRiskProfile{}
	for _, dep := range directDeps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(dep.Manager) + "|" + name)
		if _, ok := packageScores[key]; !ok {
			packageScores[key] = &packageRiskProfile{
				Name:     name,
				Manager:  strings.TrimSpace(dep.Manager),
				Registry: strings.TrimSpace(dep.Registry),
				Dep:      dep,
			}
		}
	}

	collectTyposquatSignals(directDeps, packageScores)
	collectInstallScriptSignals(ctx, directDeps, packageScores)
	collectDependencyConfusionSignals(directDeps, packageScores)
	collectVersionAnomalySignals(ctx, directDeps, packageScores)
	collectMissingRepositorySignals(directDeps, packageScores)
	collectAbandonedWithVulnSignals(directDeps, findingMap, packageScores)
	collectGoModuleSignals(directDeps, depFiles, packageScores)
	collectKnownMalwareSignals(directDeps, packageScores)

	for _, profile := range packageScores {
		if profile.Score == 0 {
			continue
		}
		flag := profile.ToFlag()
		if flag.FlagID != "" {
			flags = append(flags, flag)
		}
	}

	for _, flag := range flags {
		advisoryID := flag.FlagID
		if flag.IsRepoLevel {
			key := strings.ToLower("supplychain|" + advisoryID)
			if _, exists := findingMap[key]; exists {
				continue
			}
			findingMap[key] = &normalizedFinding{
				PackageID:       pgtype.Int8{},
				PackageName:     "repository",
				Manager:         "repository",
				Registry:        "repository",
				VersionSpec:     "",
				ResolvedVersion: "",
				AdvisoryID:      advisoryID,
				Aliases:         []string{},
				Title:           flag.Title,
				Summary:         flag.Summary + "\n\nRemediation: " + flag.Remediation,
				Severity:        flag.Severity,
				FixedVersion:    "",
				ReferenceURL:    flag.ReferenceURL,
				Sources:         []string{"custom"},
			}
			continue
		}

		packageName := strings.TrimSpace(flag.PackageName)
		if packageName == "" {
			continue
		}
		key := strings.ToLower(packageName + "|" + advisoryID)
		if _, exists := findingMap[key]; exists {
			continue
		}
		findingMap[key] = &normalizedFinding{
			PackageID:       pgtype.Int8{},
			PackageName:     packageName,
			Manager:         flag.Manager,
			Registry:        flag.Registry,
			VersionSpec:     flag.VersionSpec,
			ResolvedVersion: flag.ResolvedVersion,
			AdvisoryID:      advisoryID,
			Aliases:         []string{},
			Title:           flag.Title,
			Summary:         flag.Summary + "\n\nRemediation: " + flag.Remediation,
			Severity:        flag.Severity,
			FixedVersion:    "",
			ReferenceURL:    flag.ReferenceURL,
			Sources:         []string{"custom"},
		}
	}

	logScanEvent(ctx, queries, logger, scanRunID, repoID, "info", fmt.Sprintf("supply-chain analysis produced %d flag(s)", len(flags)), "")
}

type packageRiskProfile struct {
	Name     string
	Manager  string
	Registry string
	Dep      db.ListRepositoryDependenciesDetailedRow
	Score    int
	Signals  []supplyChainSignal
}

func (p *packageRiskProfile) AddSignal(signalID string, weight int, category, detail string) {
	p.Score += weight
	p.Signals = append(p.Signals, supplyChainSignal{
		SignalID: signalID,
		Weight:   weight,
		Category: category,
		Detail:   detail,
	})
}

func (p *packageRiskProfile) ToFlag() supplyChainFlag {
	if p.Score < thresholdMedium {
		return supplyChainFlag{}
	}

	var severity db.Severity
	var title string
	var summary strings.Builder
	var remediation strings.Builder
	signalNames := make([]string, 0, len(p.Signals))

	for _, sig := range p.Signals {
		signalNames = append(signalNames, sig.SignalID)
		summary.WriteString(sig.Detail)
		summary.WriteString("\n")
	}

	switch {
	case p.Score >= thresholdCritical:
		severity = db.SeverityCritical
		title = fmt.Sprintf("Supply chain risk: %s (score: %d)", p.Name, p.Score)
	case p.Score >= thresholdHigh:
		severity = db.SeverityHigh
		title = fmt.Sprintf("Supply chain warning: %s (score: %d)", p.Name, p.Score)
	default:
		severity = db.SeverityMedium
		title = fmt.Sprintf("Supply chain advisory: %s (score: %d)", p.Name, p.Score)
	}

	remediation.WriteString("Review the following signals and take action:\n")
	for _, sig := range p.Signals {
		remediation.WriteString("- ")
		remediation.WriteString(sig.Detail)
		remediation.WriteString("\n")
	}

	return supplyChainFlag{
		FlagID:          "SUPPLY-CHAIN|" + strings.ToUpper(strings.Join(signalNames, "+")),
		Title:           title,
		Summary:         strings.TrimSpace(summary.String()),
		Severity:        severity,
		PackageName:     p.Name,
		Manager:         p.Manager,
		Registry:        p.Registry,
		VersionSpec:     strings.TrimSpace(p.Dep.VersionSpec),
		ResolvedVersion: resolvedVersionForDependency(p.Dep),
		ReferenceURL:    strings.TrimSpace(textValue(p.Dep.RegistryUrl)),
		Remediation:     strings.TrimSpace(remediation.String()),
		Category:        "supply-chain",
		Signals:         signalNames,
	}
}

func collectTyposquatSignals(deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		manager := strings.TrimSpace(dep.Manager)
		match, distance := findTyposquatMatch(manager, name)
		if match == "" {
			continue
		}
		key := strings.ToLower(manager + "|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		weight := scoreTyposquat
		if distance == 1 {
			weight = scoreTyposquat + 10
		}
		profile.AddSignal("typosquat", weight, "typosquat", fmt.Sprintf("Name '%s' is very close to popular package '%s' (distance: %d).", name, match, distance))
	}
}

func collectInstallScriptSignals(ctx context.Context, deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	metaCache := map[string]*adapters.NPMPackageMetadata{}
	for _, dep := range deps {
		if !strings.EqualFold(strings.TrimSpace(dep.Manager), "npm") {
			continue
		}
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		meta, ok := metaCache[name]
		if !ok {
			resolved, err := adapters.GetNPMPackageMetadata(ctx, name)
			if err != nil {
				metaCache[name] = nil
				continue
			}
			metaCache[name] = resolved
			meta = resolved
		}
		if meta == nil || !meta.HasInstallScript {
			continue
		}
		key := strings.ToLower("npm|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		profile.AddSignal("install-script", scoreInstallScript, "execution", fmt.Sprintf("Package '%s' defines preinstall/install/postinstall scripts that execute during installation.", name))
	}
}

func collectDependencyConfusionSignals(deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		registry := strings.TrimSpace(dep.Registry)
		if !isPotentialDependencyConfusion(name, registry) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(dep.Manager) + "|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		profile.AddSignal("dep-confusion", scoreDepConfusion, "confusion", fmt.Sprintf("Package '%s' uses an internal-sounding name but resolves from a public registry.", name))
	}
}

func collectVersionAnomalySignals(ctx context.Context, deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		manager := strings.TrimSpace(dep.Manager)
		if manager != "npm" && manager != "pip" {
			continue
		}
		resolved := resolvedVersionForDependency(dep)
		if resolved == "" || resolved == "unknown" {
			continue
		}

		versions, err := fetchPackageVersionHistory(ctx, manager, name)
		if err != nil || len(versions) < 2 {
			continue
		}

		anomalies := detectVersionAnomalies(versions, resolved)
		if len(anomalies) == 0 {
			continue
		}

		key := strings.ToLower(manager + "|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}

		for _, anomaly := range anomalies {
			profile.AddSignal(anomaly.SignalID, anomaly.Weight, "version", anomaly.Detail)
		}
	}
}

type versionAnomaly struct {
	SignalID string
	Weight   int
	Detail   string
}

func detectVersionAnomalies(versions []packageVersionEntry, resolved string) []versionAnomaly {
	if len(versions) < 2 {
		return nil
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].PublishedAt.Before(versions[j].PublishedAt)
	})

	anomalies := make([]versionAnomaly, 0)
	resolvedClean := strings.TrimPrefix(resolved, "v")

	for i, entry := range versions {
		if entry.Version != resolvedClean && entry.Version != resolved {
			continue
		}

		if i == 0 {
			continue
		}
		prevVersion := versions[i-1].Version
		prevTime := versions[i-1].PublishedAt
		currTime := entry.PublishedAt

		prevSemver := parseSemver(prevVersion)
		currSemver := parseSemver(entry.Version)

		if currSemver.Major > prevSemver.Major+1 {
			anomalies = append(anomalies, versionAnomaly{
				SignalID: "version-major-skip",
				Weight:   scoreVersionAnomaly,
				Detail:   fmt.Sprintf("Version jumped from %s to %s (skipped major).", prevVersion, entry.Version),
			})
		}

		if currSemver.Major < prevSemver.Major && currSemver.Major > 0 {
			anomalies = append(anomalies, versionAnomaly{
				SignalID: "version-major-downgrade",
				Weight:   scoreVersionAnomaly + 10,
				Detail:   fmt.Sprintf("Version downgraded from %s to %s (major decreased).", prevVersion, entry.Version),
			})
		}

		if currTime.Sub(prevTime) < 24*time.Hour && currSemver.Major > prevSemver.Major {
			anomalies = append(anomalies, versionAnomaly{
				SignalID: "version-rapid-major",
				Weight:   scoreVersionAnomaly,
				Detail:   fmt.Sprintf("Major version bump (%s -> %s) within 24 hours.", prevVersion, entry.Version),
			})
		}

		if isHighVersion(entry.Version) && currTime.Sub(prevTime) < 7*24*time.Hour && prevSemver.Major < 2 {
			anomalies = append(anomalies, versionAnomaly{
				SignalID: "version-sudden-high",
				Weight:   scoreRecentHighVer,
				Detail:   fmt.Sprintf("Sudden jump to high version %s from %s within a week.", entry.Version, prevVersion),
			})
		}

		break
	}

	return anomalies
}

type packageVersionEntry struct {
	Version     string
	PublishedAt time.Time
}

func fetchPackageVersionHistory(ctx context.Context, manager, name string) ([]packageVersionEntry, error) {
	if manager == "npm" {
		return fetchNPMVersionHistory(ctx, name)
	}
	if manager == "pip" {
		return fetchPyPIVersionHistory(ctx, name)
	}
	return nil, fmt.Errorf("unsupported manager for version history: %s", manager)
}

func fetchNPMVersionHistory(ctx context.Context, name string) ([]packageVersionEntry, error) {
	endpoint := fmt.Sprintf("https://registry.npmjs.org/%s", url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm registry returned %d", resp.StatusCode)
	}

	var payload struct {
		Time map[string]string `json:"time"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	entries := make([]packageVersionEntry, 0)
	for version, timestamp := range payload.Time {
		if version == "created" || version == "modified" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			continue
		}
		entries = append(entries, packageVersionEntry{
			Version:     version,
			PublishedAt: parsed,
		})
	}

	return entries, nil
}

func fetchPyPIVersionHistory(ctx context.Context, name string) ([]packageVersionEntry, error) {
	endpoint := fmt.Sprintf("https://pypi.org/pypi/%s/json", url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi returned %d", resp.StatusCode)
	}

	var payload struct {
		Releases map[string][]struct {
			UploadTime string `json:"upload_time"`
		} `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	entries := make([]packageVersionEntry, 0)
	for version, releases := range payload.Releases {
		if len(releases) == 0 {
			continue
		}
		parsed, err := time.Parse("2006-01-02T15:04:05", releases[0].UploadTime)
		if err != nil {
			continue
		}
		entries = append(entries, packageVersionEntry{
			Version:     version,
			PublishedAt: parsed,
		})
	}

	return entries, nil
}

type semver struct {
	Major int
	Minor int
	Patch int
}

func parseSemver(version string) semver {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.SplitN(v, ".", 3)
	s := semver{}
	if len(parts) >= 1 {
		s.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		s.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		patchStr := strings.Split(parts[2], "-")[0]
		s.Patch, _ = strconv.Atoi(patchStr)
	}
	return s
}

func collectMissingRepositorySignals(deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		repoURL := strings.TrimSpace(textValue(dep.RepositoryUrl))
		registryURL := strings.TrimSpace(textValue(dep.RegistryUrl))
		if repoURL != "" || registryURL != "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(dep.Manager) + "|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		profile.AddSignal("missing-repo", scoreMissingRepo, "provenance", fmt.Sprintf("Package '%s' has no repository or source URL.", name))
	}
}

func collectAbandonedWithVulnSignals(deps []db.ListRepositoryDependenciesDetailedRow, findingMap map[string]*normalizedFinding, profiles map[string]*packageRiskProfile) {
	now := time.Now().UTC()
	vulnNoFixByPackage := map[string]bool{}
	for _, finding := range findingMap {
		if strings.TrimSpace(finding.FixedVersion) != "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(finding.Manager) + "|" + strings.TrimSpace(finding.PackageName))
		vulnNoFixByPackage[key] = true
	}

	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		manager := strings.TrimSpace(dep.Manager)
		if !dep.ReleasedAt.Valid {
			continue
		}
		if now.Sub(dep.ReleasedAt.Time) <= 2*365*24*time.Hour {
			continue
		}
		pkgKey := strings.ToLower(manager + "|" + name)
		if !vulnNoFixByPackage[pkgKey] {
			continue
		}
		profile := profiles[pkgKey]
		if profile == nil {
			continue
		}
		profile.AddSignal("abandoned-vuln", scoreAbandonedVuln, "maintenance", fmt.Sprintf("Package '%s' is >2 years old and has unresolved vulnerabilities.", name))
	}
}

func collectGoModuleSignals(deps []db.ListRepositoryDependenciesDetailedRow, files []db.RepositoryDependencyFile, profiles map[string]*packageRiskProfile) {
	hasGoMod := false
	hasGoSum := false
	for _, file := range files {
		name := strings.ToLower(strings.TrimSpace(file.File))
		if name == "go.mod" {
			hasGoMod = true
		}
		if name == "go.sum" {
			hasGoSum = true
		}
	}

	for _, dep := range deps {
		if !strings.EqualFold(strings.TrimSpace(dep.Manager), "go") {
			continue
		}
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		key := strings.ToLower("go|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		if hasGoMod && !hasGoSum {
			profile.AddSignal("go-sum-missing", scoreVersionAnomaly, "lockfile", fmt.Sprintf("go.sum missing; checksum verification disabled for '%s'.", name))
		}
		registryURL := strings.TrimSpace(textValue(dep.RegistryUrl))
		if registryURL != "" && !strings.Contains(strings.ToLower(registryURL), "pkg.go.dev") {
			profile.AddSignal("go-not-indexed", scoreMissingRepo, "provenance", fmt.Sprintf("Go module '%s' not indexed on pkg.go.dev.", name))
		}
	}
}

func collectKnownMalwareSignals(deps []db.ListRepositoryDependenciesDetailedRow, profiles map[string]*packageRiskProfile) {
	for _, dep := range deps {
		name := strings.TrimSpace(dep.DisplayName)
		if name == "" {
			continue
		}
		manager := strings.TrimSpace(dep.Manager)
		if !isKnownMaliciousPackage(manager, name) {
			continue
		}
		key := strings.ToLower(manager + "|" + name)
		profile := profiles[key]
		if profile == nil {
			continue
		}
		profile.AddSignal("known-malware", scoreKnownMalware, "malware", fmt.Sprintf("Package '%s' matches a known malware campaign.", name))
	}
}

func filterDirectDeps(deps []db.ListRepositoryDependenciesDetailedRow) []db.ListRepositoryDependenciesDetailedRow {
	items := make([]db.ListRepositoryDependenciesDetailedRow, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep.SourceFile) != "" {
			items = append(items, dep)
		}
	}
	return items
}

func checkLockfiles(files []db.RepositoryDependencyFile) []supplyChainFlag {
	hasNPMManifest := false
	hasPipManifest := false
	hasGoMod := false
	hasNPMOrYarnLock := false
	hasPipLock := false
	hasGoSum := false

	for _, file := range files {
		name := strings.ToLower(strings.TrimSpace(file.File))
		switch name {
		case "package.json":
			hasNPMManifest = true
		case "requirements.txt", "pyproject.toml":
			hasPipManifest = true
		case "go.mod":
			hasGoMod = true
		case "package-lock.json", "yarn.lock", "pnpm-lock.yaml":
			hasNPMOrYarnLock = true
		case "poetry.lock", "pipfile.lock", "requirements.lock":
			hasPipLock = true
		case "go.sum":
			hasGoSum = true
		}
	}

	flags := make([]supplyChainFlag, 0, 3)
	if hasNPMManifest && !hasNPMOrYarnLock {
		flags = append(flags, supplyChainFlag{
			FlagID:      "FLAG-MISSING-LOCKFILE-NPM",
			Title:       "Missing npm lockfile",
			Summary:     "package.json is present but no lockfile (package-lock.json, yarn.lock, or pnpm-lock.yaml) was found.",
			Severity:    db.SeverityHigh,
			Remediation: "Run `npm install` or `pnpm install` to generate a lockfile and commit it.",
			Category:    "lockfile",
			IsRepoLevel: true,
		})
	}
	if hasPipManifest && !hasPipLock {
		flags = append(flags, supplyChainFlag{
			FlagID:      "FLAG-MISSING-LOCKFILE-PIP",
			Title:       "Missing Python lockfile",
			Summary:     "requirements.txt or pyproject.toml is present but no lockfile (poetry.lock or pipfile.lock) was found.",
			Severity:    db.SeverityHigh,
			Remediation: "Run `poetry lock` or `pip-compile` to generate a lockfile and commit it.",
			Category:    "lockfile",
			IsRepoLevel: true,
		})
	}
	if hasGoMod && !hasGoSum {
		flags = append(flags, supplyChainFlag{
			FlagID:      "FLAG-GO-SUM-MISSING",
			Title:       "go.sum missing",
			Summary:     "go.mod is present but go.sum is missing. Module checksum verification can be bypassed.",
			Severity:    db.SeverityCritical,
			Remediation: "Run `go mod tidy` to generate go.sum and commit it.",
			Category:    "lockfile",
			IsRepoLevel: true,
		})
	}
	return flags
}

func isPotentialDependencyConfusion(name, registry string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	publicRegistry := strings.EqualFold(registry, "npm") || strings.EqualFold(registry, "pypi") || strings.EqualFold(registry, "github")
	if !publicRegistry {
		return false
	}
	keywords := []string{"internal", "private", "corp", "company", "intranet", "local"}
	for _, key := range keywords {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

func isKnownMaliciousPackage(manager, name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	known := map[string]struct{}{
		"eslint-scope": {},
		"event-stream": {},
		"ua-parser-js": {},
		"ctx":          {},
		"rc":           {},
	}
	_, ok := known[lower]
	_ = manager
	return ok
}

func findTyposquatMatch(manager, name string) (string, int) {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		return "", 0
	}
	var popular []string
	switch strings.ToLower(strings.TrimSpace(manager)) {
	case "npm":
		popular = []string{"react", "lodash", "axios", "express", "typescript", "webpack", "eslint", "vite", "next", "vue"}
	case "pip":
		popular = []string{"requests", "numpy", "pandas", "django", "flask", "pytest", "setuptools", "urllib3", "scipy", "pydantic"}
	case "go":
		popular = []string{"github.com/gin-gonic/gin", "github.com/sirupsen/logrus", "github.com/spf13/cobra", "golang.org/x/crypto", "github.com/stretchr/testify"}
	default:
		return "", 0
	}
	bestMatch := ""
	bestDist := 999
	for _, candidate := range popular {
		cand := strings.ToLower(candidate)
		if base == cand {
			return "", 0
		}
		dist := levenshteinDistance(base, cand)
		if dist <= 2 && dist < bestDist {
			bestDist = dist
			bestMatch = candidate
			continue
		}
		if strings.Contains(base, cand) || strings.Contains(cand, base) {
			if len(base) > len(cand) && dist < bestDist {
				bestDist = dist
				bestMatch = candidate
			}
		}
	}
	if bestMatch == "" || bestDist > 2 {
		return "", 0
	}
	return bestMatch, bestDist
}

func isHighVersion(version string) bool {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return false
	}
	parts := strings.Split(v, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	if major >= 5 {
		return true
	}
	if major < 1 {
		return false
	}
	if len(parts) >= 2 {
		minor, err := strconv.Atoi(parts[1])
		if err == nil {
			return minor >= 0
		}
	}
	return true
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len([]rune(b))
	}
	if b == "" {
		return len([]rune(a))
	}
	ar := []rune(a)
	br := []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr := make([]int, len(br)+1)
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = minInt(ins, del, sub)
		}
		prev = curr
	}
	return prev[len(br)]
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func collectOSVFindings(ctx context.Context, deps []db.ListRepositoryDependenciesDetailedRow, sourceCfg policySourceConfig, findingMap map[string]*normalizedFinding) error {
	queries := make([]adapters.OSVPackageQuery, 0, len(deps))
	depByKey := make(map[string]db.ListRepositoryDependenciesDetailedRow, len(deps))

	for _, dep := range deps {
		ecosystem := osvEcosystemForManager(dep.Manager)
		if ecosystem == "" {
			continue
		}
		version := resolvedVersionForDependency(dep)
		if version == "" {
			continue
		}

		query := adapters.OSVPackageQuery{
			Name:      strings.TrimSpace(dep.DisplayName),
			Ecosystem: ecosystem,
			Version:   version,
		}
		key := adapters.OSVPackageKey(query.Ecosystem, query.Name, query.Version)
		queries = append(queries, query)
		depByKey[key] = dep
	}

	osvResults, err := adapters.QueryOSVBatch(ctx, queries)
	if err != nil {
		return fmt.Errorf("query osv advisories: %w", err)
	}

	for key, advisories := range osvResults {
		dep, ok := depByKey[key]
		if !ok {
			continue
		}

		for _, advisory := range advisories {
			advisoryID := strings.TrimSpace(advisory.ID)
			if advisoryID == "" {
				continue
			}

			findingKey := strings.ToLower(dep.DisplayName + "|" + advisoryID)
			if _, exists := findingMap[findingKey]; exists {
				continue
			}

			aliases := dedupeStrings(advisory.Aliases)
			sources := []string{"osv"}
			if sourceCfg.GHSAEnabled && hasGHSAAlias(advisoryID, aliases) {
				sources = append(sources, "ghsa")
			}
			if sourceCfg.NVDEnabled && hasCVEAlias(advisoryID, aliases) {
				sources = append(sources, "nvd")
			}

			title := strings.TrimSpace(advisory.Summary)
			if title == "" {
				title = advisoryID
			}

			findingMap[findingKey] = &normalizedFinding{
				PackageID:       pgtype.Int8{Int64: dep.PackageID, Valid: dep.PackageID > 0},
				PackageName:     strings.TrimSpace(dep.DisplayName),
				Manager:         strings.TrimSpace(dep.Manager),
				Registry:        strings.TrimSpace(dep.Registry),
				VersionSpec:     strings.TrimSpace(dep.VersionSpec),
				ResolvedVersion: resolvedVersionForDependency(dep),
				AdvisoryID:      advisoryID,
				Aliases:         aliases,
				Title:           title,
				Summary:         firstNonEmpty(strings.TrimSpace(advisory.Details), strings.TrimSpace(advisory.Summary)),
				Severity:        mapSeverityFromOSV(advisory.Severity, advisoryID, aliases),
				FixedVersion:    strings.TrimSpace(advisory.FixedVersion),
				ReferenceURL:    strings.TrimSpace(advisory.ReferenceURL),
				Sources:         dedupeStrings(sources),
			}
		}
	}

	return nil
}

func resolvePolicyForScan(ctx context.Context, queries *db.Queries, cfg *internal.Config, userID, repoID int64) (pgtype.Int8, policySourceConfig) {
	defaultConfig := policySourceConfig{
		OSVEnabled:  true,
		GHSAEnabled: true,
		NVDEnabled:  true,
		GHSAToken:   strings.TrimSpace(cfg.GHSAAPIToken),
		NVDAPIKey:   strings.TrimSpace(cfg.NVDAPIKey),
	}

	policy, err := queries.GetRepositoryPolicyByGitHubRepoIDAndUser(ctx, db.GetRepositoryPolicyByGitHubRepoIDAndUserParams{
		UserID:       userID,
		GithubRepoID: repoID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return pgtype.Int8{}, defaultConfig
		}
		return pgtype.Int8{}, defaultConfig
	}

	policyID := pgtype.Int8{Int64: policy.ID, Valid: true}
	sources, err := queries.GetPolicySourcesByPolicy(ctx, policy.ID)
	if err != nil {
		return policyID, defaultConfig
	}

	return policyID, policySourceConfig{
		OSVEnabled:  sources.OsvEnabled,
		GHSAEnabled: sources.GhsaEnabled,
		NVDEnabled:  sources.NvdEnabled,
		GHSAToken:   firstNonEmpty(strings.TrimSpace(sources.GhsaTokenRef), strings.TrimSpace(cfg.GHSAAPIToken)),
		NVDAPIKey:   firstNonEmpty(strings.TrimSpace(sources.NvdApiKeyRef), strings.TrimSpace(cfg.NVDAPIKey)),
	}
}

func enrichFindingsFromProviders(ctx context.Context, sourceCfg policySourceConfig, findingMap map[string]*normalizedFinding) error {
	if len(findingMap) == 0 {
		return nil
	}

	for _, finding := range findingMap {
		if sourceCfg.GHSAEnabled {
			ghsaID := firstAliasByPrefix(finding.AdvisoryID, finding.Aliases, "GHSA-")
			if ghsaID != "" {
				advisory, err := adapters.GetGitHubSecurityAdvisory(ctx, ghsaID, sourceCfg.GHSAToken)
				if err == nil && advisory != nil {
					finding.Sources = dedupeStrings(append(finding.Sources, "ghsa"))
					if strings.TrimSpace(finding.Title) == "" || strings.EqualFold(strings.TrimSpace(finding.Title), strings.TrimSpace(finding.AdvisoryID)) {
						finding.Title = firstNonEmpty(strings.TrimSpace(advisory.Summary), finding.Title)
					}
					finding.Summary = firstNonEmpty(strings.TrimSpace(advisory.Description), finding.Summary)
					finding.ReferenceURL = firstNonEmpty(strings.TrimSpace(advisory.ReferenceURL), finding.ReferenceURL)
					finding.Aliases = dedupeStrings(append(finding.Aliases, advisory.Aliases...))
					finding.Severity = mergeSeverity(finding.Severity, mapSeverityValue(advisory.Severity))
				}
			}
		}

		if sourceCfg.NVDEnabled {
			cveID := firstAliasByPrefix(finding.AdvisoryID, finding.Aliases, "CVE-")
			if cveID != "" {
				advisory, err := adapters.GetNVDAdvisory(ctx, cveID, sourceCfg.NVDAPIKey)
				if err == nil && advisory != nil {
					finding.Sources = dedupeStrings(append(finding.Sources, "nvd"))
					finding.Summary = firstNonEmpty(strings.TrimSpace(advisory.Summary), finding.Summary)
					finding.ReferenceURL = firstNonEmpty(strings.TrimSpace(advisory.ReferenceURL), finding.ReferenceURL)
					finding.Severity = mergeSeverity(finding.Severity, mapSeverityValue(advisory.Severity))
				}
			}
		}
	}

	return nil
}

func countFindingsBySeverity(ctx context.Context, queries *db.Queries, scanRunID, userID int64) (findingCounts, error) {
	rows, err := queries.ListRepositoryScanFindingsByRunAndUser(ctx, db.ListRepositoryScanFindingsByRunAndUserParams{
		UserID:    userID,
		ScanRunID: scanRunID,
	})
	if err != nil {
		return findingCounts{}, fmt.Errorf("list findings by scan run: %w", err)
	}

	counts := findingCounts{Total: int32(len(rows))}
	for _, row := range rows {
		switch row.Severity {
		case db.SeverityCritical:
			counts.Critical++
		case db.SeverityHigh:
			counts.High++
		case db.SeverityLow:
			counts.Low++
		default:
			counts.Medium++
		}
	}

	return counts, nil
}

func normalizeScanTrigger(trigger string) string {
	v := strings.ToLower(strings.TrimSpace(trigger))
	switch v {
	case "manual", "policy", "schedule", "connect", "sync":
		return v
	default:
		return "manual"
	}
}

func resolvedVersionForDependency(dep db.ListRepositoryDependenciesDetailedRow) string {
	if dep.ResolvedVersion.Valid {
		value := strings.TrimSpace(dep.ResolvedVersion.String)
		if value != "" {
			return value
		}
	}
	return normalizeVersionSpec(dep.VersionSpec)
}

func normalizeVersionSpec(versionSpec string) string {
	v := strings.TrimSpace(versionSpec)
	v = strings.TrimPrefix(v, "=")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, ">") || strings.HasPrefix(v, "<") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, "^") {
		return ""
	}
	if strings.Contains(v, " ") || strings.Contains(v, ",") {
		return ""
	}
	return v
}

func osvEcosystemForManager(manager string) string {
	switch strings.ToLower(strings.TrimSpace(manager)) {
	case "npm":
		return "npm"
	case "pip":
		return "PyPI"
	case "go":
		return "Go"
	default:
		return ""
	}
}

func hasGHSAAlias(advisoryID string, aliases []string) bool {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(advisoryID)), "GHSA-") {
		return true
	}
	for _, alias := range aliases {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(alias)), "GHSA-") {
			return true
		}
	}
	return false
}

func hasCVEAlias(advisoryID string, aliases []string) bool {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(advisoryID)), "CVE-") {
		return true
	}
	for _, alias := range aliases {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(alias)), "CVE-") {
			return true
		}
	}
	return false
}

func mapSeverityFromOSV(raw, advisoryID string, aliases []string) db.Severity {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(v, "critical"):
		return db.SeverityCritical
	case strings.Contains(v, "high"):
		return db.SeverityHigh
	case strings.Contains(v, "medium"), strings.Contains(v, "moderate"):
		return db.SeverityMedium
	case strings.Contains(v, "low"):
		return db.SeverityLow
	}

	if hasCVEAlias(advisoryID, aliases) {
		return db.SeverityHigh
	}
	if hasGHSAAlias(advisoryID, aliases) {
		return db.SeverityMedium
	}
	return db.SeverityMedium
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return []string{}
	}
	sort.Strings(items)
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstAliasByPrefix(advisoryID string, aliases []string, prefix string) string {
	target := strings.ToUpper(strings.TrimSpace(prefix))
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(advisoryID)), target) {
		return strings.TrimSpace(advisoryID)
	}
	for _, alias := range aliases {
		item := strings.TrimSpace(alias)
		if strings.HasPrefix(strings.ToUpper(item), target) {
			return item
		}
	}
	return ""
}

func mapSeverityValue(raw string) db.Severity {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(v, "critical"):
		return db.SeverityCritical
	case strings.Contains(v, "high"):
		return db.SeverityHigh
	case strings.Contains(v, "medium"), strings.Contains(v, "moderate"):
		return db.SeverityMedium
	case strings.Contains(v, "low"):
		return db.SeverityLow
	default:
		return db.SeverityMedium
	}
}

func mergeSeverity(current, incoming db.Severity) db.Severity {
	if severityRank(incoming) > severityRank(current) {
		return incoming
	}
	return current
}

func severityRank(value db.Severity) int {
	switch value {
	case db.SeverityCritical:
		return 4
	case db.SeverityHigh:
		return 3
	case db.SeverityMedium:
		return 2
	case db.SeverityLow:
		return 1
	default:
		return 0
	}
}
