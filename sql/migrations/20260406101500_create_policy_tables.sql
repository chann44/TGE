-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    CREATE TYPE trigger_type AS ENUM ('push', 'pull_request', 'schedule', 'manual');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE severity AS ENUM ('low', 'medium', 'high', 'critical');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE custom_source_format AS ENUM ('osv', 'nvd');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

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

CREATE INDEX IF NOT EXISTS policies_user_id_idx
ON policies (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS policy_repositories_policy_id_idx
ON policy_repositories (policy_id);

CREATE INDEX IF NOT EXISTS policy_triggers_policy_idx
ON policy_triggers (policy_id);

CREATE INDEX IF NOT EXISTS policy_source_custom_policy_idx
ON policy_source_custom (policy_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS policy_source_custom_policy_idx;
DROP INDEX IF EXISTS policy_triggers_policy_idx;
DROP INDEX IF EXISTS policy_repositories_policy_id_idx;
DROP INDEX IF EXISTS policies_user_id_idx;

DROP TABLE IF EXISTS policy_registry;
DROP TABLE IF EXISTS policy_sast;
DROP TABLE IF EXISTS policy_source_custom;
DROP TABLE IF EXISTS policy_sources;
DROP TABLE IF EXISTS policy_triggers;
DROP TABLE IF EXISTS policy_repositories;
DROP TABLE IF EXISTS policies;

DROP TYPE IF EXISTS custom_source_format;
DROP TYPE IF EXISTS severity;
DROP TYPE IF EXISTS trigger_type;
-- +goose StatementEnd
