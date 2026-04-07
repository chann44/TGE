# AGENTS.md

Guidance for coding agents working in `TEG`.
This file documents practical commands and code conventions observed in this repo.

## Quick Facts
- Monorepo with Go backend/services and SvelteKit web app.
- Root module: `github.com/chann44/TGE`.
- Web app lives in `apps/web`.
- API lives in `apps/api`.
- SQL schema/queries live under `sql/` and generate `internals/db` via sqlc.

## Build, Lint, Test Commands

### Root Make Targets (preferred shortcuts)
- `make help` - list available targets.
- `make api-dev` - run API locally (`go run ./apps/api`).
- `make worker-dev` - run worker locally.
- `make scheduler-dev` - run scheduler locally.
- `make web-dev` - run Svelte dev server.
- `make dev` - run API + worker + scheduler + web together.
- `make test` - run all Go tests (`go test ./...`).
- `make fmt` - format Go code (`go fmt ./...`).
- `make codegen` - regenerate sqlc output from `sql/queries`.

### Backend (Go) Commands
- `go test ./...` - run all backend tests.
- `go test ./path/to/pkg` - run tests for one package.
- `go test ./path/to/pkg -run TestName` - run a single test function.
- `go test ./path/to/pkg -run TestName/Subcase` - run one subtest.
- `go test ./path/to/pkg -run TestName -count=1` - bypass test cache.
- `go run ./apps/api` - run API directly.
- `go run ./apps/worker` - run worker directly.
- `go run ./apps/scheduler` - run scheduler directly.
- `go fmt ./...` - format Go files.

### Frontend (SvelteKit) Commands
- `npm --prefix apps/web run dev` - local dev server.
- `npm --prefix apps/web run build` - production build.
- `npm --prefix apps/web run preview` - preview production build.
- `npm --prefix apps/web run check` - svelte-kit sync + svelte-check.
- `npm --prefix apps/web run lint` - Prettier check + ESLint.
- `npm --prefix apps/web run format` - Prettier write.
- `npm --prefix apps/web run test` - run Vitest once.
- `npm --prefix apps/web run test:unit` - watch mode Vitest.

### Run a Single Frontend Test
- Single file:
  - `npm --prefix apps/web run test -- src/lib/vitest-examples/greet.spec.ts`
- Single test by name:
  - `npm --prefix apps/web run test -- -t "greet"`
- Single project (from `vite.config.ts`):
  - `npm --prefix apps/web run test -- --project server src/lib/vitest-examples/greet.spec.ts`
  - `npm --prefix apps/web run test -- --project client src/lib/vitest-examples/Welcome.svelte.spec.ts`

### Database and Codegen
- `make migrate-up` - apply migrations via goose.
- `make migrate-down` - rollback one migration.
- `make migrate-status` - show migration status.
- `make migrate-reset` - rollback all migrations.
- `make migrate-create NAME=create_feature_name` - create a migration file.
- `make codegen` - regenerate `internals/db/*.go` from sqlc config.

### Docker
- `make docker-build` - build backend + web images.
- `make docker-push` - push backend + web images.
- Self-host compose: `docker compose -f deployments/selfhost.compose.yml up -d`.
- Dev dependencies compose: `docker compose -f deployments/dev.compose.yml up -d`.

## Required Agent Workflow
- Before finishing code changes, run relevant checks for touched areas.
- Minimum for backend edits: `go test ./...`.
- Minimum for frontend edits: `npm --prefix apps/web run check`.
- If formatting is needed, run formatter instead of manual whitespace fixes.
- Do not manually edit generated sqlc files in `internals/db`.

## Code Style Guidelines

### General
- Follow existing patterns in neighboring files before introducing new ones.
- Keep functions focused and small; prefer clear helpers over dense logic.
- Avoid broad refactors unless required by the task.
- Use descriptive names; avoid single-letter names except short loop indexes.

### Go Style
- Use standard Go formatting (`go fmt`).
- Keep imports grouped by `go fmt` default order.
- Package names are lowercase, no underscores.
- Exported symbols use `PascalCase`; unexported use `camelCase`.
- Prefer early returns for validation and error paths.
- Wrap errors with context (`fmt.Errorf("context: %w", err)`) when propagating.
- Use `errors.Is` for sentinel checks (e.g., `pgx.ErrNoRows`).
- Use explicit HTTP status codes and concise error messages in handlers.
- Use typed request/response structs for JSON payloads.
- `context.Context` should be the first parameter when passed explicitly.
- Keep handler methods on `*Handler` and route wiring in `apps/api/routes.go`.

### Error Handling Conventions (Go)
- Validate auth/user context early and return `401` when missing.
- Return `400` for invalid params/body, `404` for missing resources, `409` for conflicts.
- Return `500` for unexpected failures.
- Prefer `writeJSON(...)` for JSON responses, `http.Error(...)` for simple text errors.
- Log operational failures with enough context but do not leak secrets/tokens.

### Frontend Style (Svelte + TS)
- Use TypeScript in script blocks (`<script lang="ts">`).
- Prefer typed load/actions with generated `$types` imports.
- Keep component-local helpers (`statusClass`, mappers) near the top of the file.
- Use existing utility classes/components (`soc-*`, sidebar/ui primitives) consistently.
- Keep forms server-driven via `+page.server.ts` actions when pattern already exists.
- Use `fail(...)` and `redirect(...)` from `@sveltejs/kit` for server action control flow.
- Keep text and labels concise; preserve existing UI tone.

### Formatting and Linting
- Prettier config (web): tabs, single quotes, no trailing commas, print width 100.
- ESLint uses JS + TS + Svelte recommended configs and Prettier compatibility.
- Run `npm --prefix apps/web run lint` after significant web changes.

### SQL and Data Layer
- Put SQL in `sql/queries/*.sql` with sqlc `-- name: ...` annotations.
- Keep SQL column naming consistent with existing snake_case schema.
- After query/schema changes, run `make codegen`.
- Prefer adding migrations under `sql/migrations` instead of editing DB state manually.

### Naming Conventions
- Go files: feature-oriented (`domains.go`, `policies.go`, `system_health.go`).
- Svelte routes: standard SvelteKit naming (`+page.svelte`, `+page.server.ts`).
- Keep acronyms readable (`API`, `URL`, `ID`) and consistent with local code.

## Config and Environment Notes
- Env parsing is centralized in `internals/config.go`.
- Reasonable defaults are used for local dev (for example frontend/clickhouse defaults).
- Domain/Traefik sync behavior is controlled by env flags; do not assume enabled locally.

## Cursor/Copilot Rules
- No `.cursorrules` file found.
- No files found under `.cursor/rules/`.
- No `.github/copilot-instructions.md` found.
- If these files are added later, update this AGENTS.md accordingly.

## Paths You Will Touch Often
- API routes/handlers: `apps/api/*.go`
- Web routes: `apps/web/src/routes/**`
- Shared web styles: `apps/web/src/routes/layout.css`
- SQL schema and queries: `sql/schema.sql`, `sql/queries/*.sql`
- Generated DB access: `internals/db/*.go`
- Deployment: `deployments/*.yml`

## Practical PR Checklist for Agents
- Run relevant tests/checks for changed areas.
- Regenerate sqlc output if SQL changed.
- Keep diffs minimal and task-focused.
- Do not commit secrets or `.env` values.
- Update docs when behavior or required env vars change.
