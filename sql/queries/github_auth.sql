-- name: UpsertGitHubUser :one
INSERT INTO users (
    github_id,
    login,
    name,
    email,
    avatar_url
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
ON CONFLICT (github_id)
DO UPDATE SET
    login = EXCLUDED.login,
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = NOW()
RETURNING id, github_id, login, name, email, avatar_url, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, github_id, login, name, email, avatar_url, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;

-- name: UpsertUserOAuthToken :exec
INSERT INTO user_oauth_tokens (
    user_id,
    provider,
    access_token
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (user_id, provider)
DO UPDATE SET
    access_token = EXCLUDED.access_token,
    updated_at = NOW();

-- name: GetUserOAuthToken :one
SELECT id, user_id, provider, access_token, created_at, updated_at
FROM user_oauth_tokens
WHERE user_id = $1
  AND provider = $2
LIMIT 1;

-- name: DeleteUserRepositories :exec
DELETE FROM repositories
WHERE user_id = $1;

-- name: UpsertRepository :exec
INSERT INTO repositories (
    user_id,
    github_repo_id,
    name,
    full_name,
    private,
    default_branch,
    html_url
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
ON CONFLICT (user_id, github_repo_id)
DO UPDATE SET
    name = EXCLUDED.name,
    full_name = EXCLUDED.full_name,
    private = EXCLUDED.private,
    default_branch = EXCLUDED.default_branch,
    html_url = EXCLUDED.html_url,
    updated_at = NOW();

-- name: ListUserRepositories :many
SELECT id, user_id, github_repo_id, name, full_name, private, default_branch, html_url, created_at, updated_at
FROM repositories
WHERE user_id = $1
ORDER BY name;
