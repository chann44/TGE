CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    github_id BIGINT NOT NULL UNIQUE,
    login TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_oauth_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    access_token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider)
);

CREATE TABLE IF NOT EXISTS repositories (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_repo_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    private BOOLEAN NOT NULL DEFAULT FALSE,
    default_branch TEXT NOT NULL DEFAULT '',
    html_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, github_repo_id)
);

CREATE TABLE IF NOT EXISTS user_github_installations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL,
    app_slug TEXT NOT NULL DEFAULT '',
    account_login TEXT NOT NULL DEFAULT '',
    account_type TEXT NOT NULL DEFAULT '',
    html_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, installation_id)
);

CREATE TYPE trigger_type AS ENUM ('push', 'pull_request', 'schedule', 'manual');
CREATE TYPE severity AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE custom_source_format AS ENUM ('osv', 'nvd');

CREATE TABLE IF NOT EXISTS policies (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policy_repositories (
    repository_id BIGINT PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
    policy_id BIGINT NOT NULL REFERENCES policies(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policy_triggers (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    type trigger_type NOT NULL,
    branches TEXT[],
    cron TEXT,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT policy_trigger_schedule_check CHECK (
        type != 'schedule'::trigger_type OR NULLIF(BTRIM(COALESCE(cron, '')), '') IS NOT NULL
    ),
    CONSTRAINT policy_trigger_push_pr_branches_check CHECK (
        type NOT IN ('push'::trigger_type, 'pull_request'::trigger_type)
        OR branches IS NULL
        OR CARDINALITY(branches) > 0
    )
);

CREATE TABLE IF NOT EXISTS policy_sources (
    policy_id BIGINT PRIMARY KEY REFERENCES policies(id) ON DELETE CASCADE,
    registry_first BOOLEAN NOT NULL DEFAULT TRUE,
    registry_max_age_days INTEGER NOT NULL DEFAULT 7,
    registry_only BOOLEAN NOT NULL DEFAULT FALSE,
    osv_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ghsa_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ghsa_token_ref TEXT NOT NULL DEFAULT '',
    nvd_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    nvd_api_key_ref TEXT NOT NULL DEFAULT '',
    govulncheck_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    supply_chain_enabled BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS policy_source_custom (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    format custom_source_format NOT NULL,
    auth_header TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS policy_sast (
    policy_id BIGINT PRIMARY KEY REFERENCES policies(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    patterns_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    rulesets TEXT[] NOT NULL DEFAULT ARRAY['default'],
    min_severity severity NOT NULL DEFAULT 'medium',
    exclude_paths TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ai_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ai_max_files_per_scan INTEGER NOT NULL DEFAULT 50,
    ai_reachability BOOLEAN NOT NULL DEFAULT TRUE,
    ai_suggest_fix BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS policy_registry (
    policy_id BIGINT PRIMARY KEY REFERENCES policies(id) ON DELETE CASCADE,
    push_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    push_url TEXT NOT NULL DEFAULT '',
    push_signing_key_ref TEXT NOT NULL DEFAULT '',
    pull_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    pull_url TEXT NOT NULL DEFAULT '',
    pull_trusted_keys TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    pull_max_age_days INTEGER NOT NULL DEFAULT 7,
    CONSTRAINT policy_registry_push_fields_required CHECK (
        NOT push_enabled
        OR (
            NULLIF(BTRIM(COALESCE(push_url, '')), '') IS NOT NULL
            AND NULLIF(BTRIM(COALESCE(push_signing_key_ref, '')), '') IS NOT NULL
        )
    ),
    CONSTRAINT policy_registry_pull_fields_required CHECK (
        NOT pull_enabled OR NULLIF(BTRIM(COALESCE(pull_url, '')), '') IS NOT NULL
    )
);

CREATE TABLE IF NOT EXISTS dependency_packages (
    id BIGSERIAL PRIMARY KEY,
    manager TEXT NOT NULL,
    registry TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (manager, registry, normalized_name)
);

CREATE TABLE IF NOT EXISTS dependency_package_versions (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES dependency_packages(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    creator TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    homepage TEXT NOT NULL DEFAULT '',
    repository_url TEXT NOT NULL DEFAULT '',
    registry_url TEXT NOT NULL DEFAULT '',
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, version)
);

CREATE TABLE IF NOT EXISTS dependency_version_dependencies (
    id BIGSERIAL PRIMARY KEY,
    from_version_id BIGINT NOT NULL REFERENCES dependency_package_versions(id) ON DELETE CASCADE,
    to_version_id BIGINT NOT NULL REFERENCES dependency_package_versions(id) ON DELETE CASCADE,
    dependency_type TEXT NOT NULL DEFAULT 'default',
    version_spec TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (dependency_type IN ('prod', 'dev', 'peer', 'optional', 'default')),
    UNIQUE (from_version_id, to_version_id, dependency_type)
);

CREATE TABLE IF NOT EXISTS repository_dependency_files (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL,
    path TEXT NOT NULL,
    file TEXT NOT NULL,
    manager TEXT NOT NULL,
    registry TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repository_id, path)
);

CREATE TABLE IF NOT EXISTS repository_dependencies (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL,
    package_id BIGINT NOT NULL REFERENCES dependency_packages(id) ON DELETE CASCADE,
    source_file TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'default',
    version_spec TEXT NOT NULL DEFAULT '',
    resolved_version_id BIGINT REFERENCES dependency_package_versions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (scope IN ('prod', 'dev', 'peer', 'optional', 'default')),
    UNIQUE (repository_id, package_id, source_file, scope, version_spec)
);

CREATE TABLE IF NOT EXISTS repository_dependency_syncs (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    trigger TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('queued', 'running', 'success', 'failed')),
    CHECK (trigger IN ('connect', 'manual'))
);

CREATE TABLE IF NOT EXISTS repository_scan_runs (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL,
    policy_id BIGINT REFERENCES policies(id) ON DELETE SET NULL,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    findings_total INTEGER NOT NULL DEFAULT 0,
    findings_critical INTEGER NOT NULL DEFAULT 0,
    findings_high INTEGER NOT NULL DEFAULT 0,
    findings_medium INTEGER NOT NULL DEFAULT 0,
    findings_low INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('queued', 'running', 'success', 'failed')),
    CHECK (trigger IN ('manual', 'policy', 'schedule', 'connect', 'sync'))
);

CREATE TABLE IF NOT EXISTS repository_scan_findings (
    id BIGSERIAL PRIMARY KEY,
    scan_run_id BIGINT NOT NULL REFERENCES repository_scan_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL,
    policy_id BIGINT REFERENCES policies(id) ON DELETE SET NULL,
    package_id BIGINT,
    package_name TEXT NOT NULL,
    manager TEXT NOT NULL,
    registry TEXT NOT NULL,
    version_spec TEXT NOT NULL DEFAULT '',
    resolved_version TEXT NOT NULL DEFAULT '',
    advisory_id TEXT NOT NULL,
    aliases TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    severity severity NOT NULL DEFAULT 'medium',
    fixed_version TEXT NOT NULL DEFAULT '',
    reference_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('open', 'resolved')),
    UNIQUE (scan_run_id, package_name, advisory_id)
);

CREATE TABLE IF NOT EXISTS repository_scan_finding_sources (
    finding_id BIGINT NOT NULL REFERENCES repository_scan_findings(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    provider_record_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (finding_id, source),
    CHECK (source IN ('osv', 'ghsa', 'nvd', 'custom'))
);

CREATE TABLE IF NOT EXISTS repository_scan_logs (
    id BIGSERIAL PRIMARY KEY,
    scan_run_id BIGINT NOT NULL REFERENCES repository_scan_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    directory_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (level IN ('debug', 'info', 'warn', 'error'))
);

CREATE INDEX IF NOT EXISTS dependency_packages_lookup_idx
ON dependency_packages (manager, registry, normalized_name);

CREATE INDEX IF NOT EXISTS dependency_package_versions_package_id_idx
ON dependency_package_versions (package_id);

CREATE INDEX IF NOT EXISTS dependency_version_dependencies_from_version_idx
ON dependency_version_dependencies (from_version_id);

CREATE INDEX IF NOT EXISTS dependency_version_dependencies_to_version_idx
ON dependency_version_dependencies (to_version_id);

CREATE INDEX IF NOT EXISTS repository_dependency_files_repository_id_idx
ON repository_dependency_files (repository_id);

CREATE INDEX IF NOT EXISTS repository_dependencies_repository_id_idx
ON repository_dependencies (repository_id);

CREATE INDEX IF NOT EXISTS repository_dependencies_package_id_idx
ON repository_dependencies (package_id);

CREATE INDEX IF NOT EXISTS repository_dependency_syncs_repository_id_idx
ON repository_dependency_syncs (repository_id, started_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_runs_repository_id_idx
ON repository_scan_runs (repository_id, started_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_runs_status_idx
ON repository_scan_runs (status, started_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_findings_repository_id_idx
ON repository_scan_findings (repository_id, created_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_findings_scan_run_id_idx
ON repository_scan_findings (scan_run_id);

CREATE INDEX IF NOT EXISTS repository_scan_findings_advisory_idx
ON repository_scan_findings (advisory_id);

CREATE INDEX IF NOT EXISTS repository_scan_logs_run_id_idx
ON repository_scan_logs (scan_run_id, created_at ASC);

CREATE INDEX IF NOT EXISTS repository_scan_logs_repository_id_idx
ON repository_scan_logs (repository_id, created_at DESC);

CREATE INDEX IF NOT EXISTS policies_user_id_idx
ON policies (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS policy_repositories_policy_id_idx
ON policy_repositories (policy_id);

CREATE INDEX IF NOT EXISTS policy_triggers_policy_idx
ON policy_triggers (policy_id);

CREATE INDEX IF NOT EXISTS policy_source_custom_policy_idx
ON policy_source_custom (policy_id);

CREATE TABLE IF NOT EXISTS service_status_snapshots (
    id BIGSERIAL PRIMARY KEY,
    service TEXT NOT NULL,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    uptime_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('ok', 'degraded', 'down'))
);

CREATE INDEX IF NOT EXISTS service_status_snapshots_service_checked_at_idx
ON service_status_snapshots (service, checked_at DESC);
