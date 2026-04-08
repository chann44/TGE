-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS integrations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'connected',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB NOT NULL DEFAULT '{}'::JSONB,
    connected_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider),
    CHECK (provider IN ('github', 'slack', 'jira', 'linear', 'discord')),
    CHECK (status IN ('connected', 'error', 'disconnected'))
);

CREATE TABLE IF NOT EXISTS integration_activities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    integration_id BIGINT REFERENCES integrations(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider IN ('github', 'slack', 'jira', 'linear', 'discord')),
    CHECK (status IN ('success', 'failed'))
);

CREATE INDEX IF NOT EXISTS integrations_user_id_idx
ON integrations (user_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS integration_activities_user_id_idx
ON integration_activities (user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS integration_activities_user_id_idx;
DROP INDEX IF EXISTS integrations_user_id_idx;
DROP TABLE IF EXISTS integration_activities;
DROP TABLE IF EXISTS integrations;
-- +goose StatementEnd
