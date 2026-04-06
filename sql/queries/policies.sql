-- name: CreatePolicy :one
INSERT INTO policies (
    user_id,
    name,
    enabled
) VALUES (
    $1,
    $2,
    $3
)
RETURNING id, user_id, name, enabled, created_at, updated_at;

-- name: ListPoliciesByUser :many
SELECT
    p.id,
    p.user_id,
    p.name,
    p.enabled,
    p.created_at,
    p.updated_at,
    COUNT(pr.repository_id)::BIGINT AS repository_count
FROM policies p
LEFT JOIN policy_repositories pr ON pr.policy_id = p.id
LEFT JOIN repositories r ON r.id = pr.repository_id AND r.user_id = p.user_id
WHERE p.user_id = $1
GROUP BY p.id
ORDER BY p.created_at DESC;

-- name: GetPolicyByIDAndUser :one
SELECT id, user_id, name, enabled, created_at, updated_at
FROM policies
WHERE id = $1
  AND user_id = $2;

-- name: UpdatePolicyByIDAndUser :one
UPDATE policies
SET name = $3,
    enabled = $4,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, name, enabled, created_at, updated_at;

-- name: SetPolicyEnabledByIDAndUser :one
UPDATE policies
SET enabled = $3,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, name, enabled, created_at, updated_at;

-- name: DeletePolicyByIDAndUser :exec
DELETE FROM policies
WHERE id = $1
  AND user_id = $2;

-- name: GetUserRepositoryByGitHubRepoID :one
SELECT id, user_id, github_repo_id, name, full_name, private, default_branch, html_url, created_at, updated_at
FROM repositories
WHERE user_id = $1
  AND github_repo_id = $2;

-- name: AssignPolicyToRepository :exec
INSERT INTO policy_repositories (
    repository_id,
    policy_id,
    assigned_at
) VALUES (
    $1,
    $2,
    NOW()
)
ON CONFLICT (repository_id)
DO UPDATE SET
    policy_id = EXCLUDED.policy_id,
    assigned_at = NOW();

-- name: UnassignPolicyFromRepository :exec
DELETE FROM policy_repositories
WHERE repository_id = $1;

-- name: DeletePolicyRepositoryAssignmentsByPolicy :exec
DELETE FROM policy_repositories
WHERE policy_id = $1;

-- name: GetRepositoryPolicyByGitHubRepoIDAndUser :one
SELECT p.id, p.user_id, p.name, p.enabled, p.created_at, p.updated_at
FROM policy_repositories pr
INNER JOIN repositories r ON r.id = pr.repository_id
INNER JOIN policies p ON p.id = pr.policy_id
WHERE r.user_id = $1
  AND r.github_repo_id = $2;

-- name: ListPolicyRepositoriesByPolicyAndUser :many
SELECT
    r.id,
    r.github_repo_id,
    r.full_name,
    pr.assigned_at
FROM policy_repositories pr
INNER JOIN repositories r ON r.id = pr.repository_id
WHERE pr.policy_id = $1
  AND r.user_id = $2
ORDER BY r.full_name;

-- name: ListPolicyTriggersByPolicy :many
SELECT id, policy_id, type, branches, cron, timezone, created_at
FROM policy_triggers
WHERE policy_id = $1
ORDER BY created_at, id;

-- name: DeletePolicyTriggersByPolicy :exec
DELETE FROM policy_triggers
WHERE policy_id = $1;

-- name: CreatePolicyTrigger :exec
INSERT INTO policy_triggers (
    policy_id,
    type,
    branches,
    cron,
    timezone
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: GetPolicySourcesByPolicy :one
SELECT
    policy_id,
    registry_first,
    registry_max_age_days,
    registry_only,
    osv_enabled,
    ghsa_enabled,
    ghsa_token_ref,
    nvd_enabled,
    nvd_api_key_ref,
    govulncheck_enabled
FROM policy_sources
WHERE policy_id = $1;

-- name: UpsertPolicySources :exec
INSERT INTO policy_sources (
    policy_id,
    registry_first,
    registry_max_age_days,
    registry_only,
    osv_enabled,
    ghsa_enabled,
    ghsa_token_ref,
    nvd_enabled,
    nvd_api_key_ref,
    govulncheck_enabled
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
ON CONFLICT (policy_id)
DO UPDATE SET
    registry_first = EXCLUDED.registry_first,
    registry_max_age_days = EXCLUDED.registry_max_age_days,
    registry_only = EXCLUDED.registry_only,
    osv_enabled = EXCLUDED.osv_enabled,
    ghsa_enabled = EXCLUDED.ghsa_enabled,
    ghsa_token_ref = EXCLUDED.ghsa_token_ref,
    nvd_enabled = EXCLUDED.nvd_enabled,
    nvd_api_key_ref = EXCLUDED.nvd_api_key_ref,
    govulncheck_enabled = EXCLUDED.govulncheck_enabled;

-- name: GetPolicySastByPolicy :one
SELECT
    policy_id,
    enabled,
    patterns_enabled,
    rulesets,
    min_severity,
    exclude_paths,
    ai_enabled,
    ai_max_files_per_scan,
    ai_reachability,
    ai_suggest_fix
FROM policy_sast
WHERE policy_id = $1;

-- name: UpsertPolicySast :exec
INSERT INTO policy_sast (
    policy_id,
    enabled,
    patterns_enabled,
    rulesets,
    min_severity,
    exclude_paths,
    ai_enabled,
    ai_max_files_per_scan,
    ai_reachability,
    ai_suggest_fix
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
ON CONFLICT (policy_id)
DO UPDATE SET
    enabled = EXCLUDED.enabled,
    patterns_enabled = EXCLUDED.patterns_enabled,
    rulesets = EXCLUDED.rulesets,
    min_severity = EXCLUDED.min_severity,
    exclude_paths = EXCLUDED.exclude_paths,
    ai_enabled = EXCLUDED.ai_enabled,
    ai_max_files_per_scan = EXCLUDED.ai_max_files_per_scan,
    ai_reachability = EXCLUDED.ai_reachability,
    ai_suggest_fix = EXCLUDED.ai_suggest_fix;

-- name: GetPolicyRegistryByPolicy :one
SELECT
    policy_id,
    push_enabled,
    push_url,
    push_signing_key_ref,
    pull_enabled,
    pull_url,
    pull_trusted_keys,
    pull_max_age_days
FROM policy_registry
WHERE policy_id = $1;

-- name: UpsertPolicyRegistry :exec
INSERT INTO policy_registry (
    policy_id,
    push_enabled,
    push_url,
    push_signing_key_ref,
    pull_enabled,
    pull_url,
    pull_trusted_keys,
    pull_max_age_days
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (policy_id)
DO UPDATE SET
    push_enabled = EXCLUDED.push_enabled,
    push_url = EXCLUDED.push_url,
    push_signing_key_ref = EXCLUDED.push_signing_key_ref,
    pull_enabled = EXCLUDED.pull_enabled,
    pull_url = EXCLUDED.pull_url,
    pull_trusted_keys = EXCLUDED.pull_trusted_keys,
    pull_max_age_days = EXCLUDED.pull_max_age_days;

-- name: ListPolicyCustomSourcesByPolicy :many
SELECT id, policy_id, name, url, format, auth_header
FROM policy_source_custom
WHERE policy_id = $1
ORDER BY id;

-- name: DeletePolicyCustomSourcesByPolicy :exec
DELETE FROM policy_source_custom
WHERE policy_id = $1;

-- name: CreatePolicyCustomSource :exec
INSERT INTO policy_source_custom (
    policy_id,
    name,
    url,
    format,
    auth_header
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: ListScheduledPolicyRepositoryTargets :many
SELECT
    p.user_id,
    p.id AS policy_id,
    pt.id AS trigger_id,
    pt.cron,
    pt.timezone,
    r.github_repo_id
FROM policies p
INNER JOIN policy_triggers pt ON pt.policy_id = p.id
INNER JOIN policy_repositories pr ON pr.policy_id = p.id
INNER JOIN repositories r ON r.id = pr.repository_id
WHERE p.enabled = TRUE
  AND pt.type = 'schedule'::trigger_type
ORDER BY p.id, pt.id, r.github_repo_id;
