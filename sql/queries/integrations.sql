-- name: UpsertIntegration :one
INSERT INTO integrations (
    user_id,
    provider,
    name,
    status,
    enabled,
    config,
    connected_at,
    last_error
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    NOW(),
    ''
)
ON CONFLICT (user_id, provider)
DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    enabled = EXCLUDED.enabled,
    config = EXCLUDED.config,
    connected_at = NOW(),
    last_error = '',
    updated_at = NOW()
RETURNING id, user_id, provider, name, status, enabled, config, connected_at, last_error, created_at, updated_at;

-- name: UpdateIntegrationStatus :exec
UPDATE integrations
SET status = $3,
    last_error = $4,
    updated_at = NOW()
WHERE user_id = $1
  AND provider = $2;

-- name: GetIntegrationByProviderAndUser :one
SELECT id, user_id, provider, name, status, enabled, config, connected_at, last_error, created_at, updated_at
FROM integrations
WHERE user_id = $1
  AND provider = $2
LIMIT 1;

-- name: ListIntegrationsByUser :many
SELECT id, user_id, provider, name, status, enabled, config, connected_at, last_error, created_at, updated_at
FROM integrations
WHERE user_id = $1
ORDER BY provider;

-- name: CreateIntegrationActivity :exec
INSERT INTO integration_activities (
    user_id,
    integration_id,
    provider,
    action,
    status,
    detail,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
);

-- name: ListIntegrationActivitiesByUser :many
SELECT id, user_id, integration_id, provider, action, status, detail, metadata, created_at
FROM integration_activities
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
OFFSET $3;
