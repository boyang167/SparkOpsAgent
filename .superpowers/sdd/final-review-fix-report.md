# Filesystem Final Review Fix Report

## Baseline and scope

- Branch: `master`
- Starting HEAD: `08cb015c2686127f19fa3b9079acf3d2be2d3180`
- Scope: only the four supplied filesystem governance findings, their tests,
  OpenAPI contract, migration release documentation, and this report.

## Fixes

### Migration 000085 destructive down guard

- The first executable down-migration statement is a PostgreSQL `DO` guard.
- Existing `DOWNLOAD` grants stop the migration before every `DELETE` or `DROP`
  unless the same transaction explicitly sets
  `sparkops.allow_destructive_filesystem_000085_down = 'on'`.
- Static tests verify statement order and guard text. Conditional PostgreSQL
  tests verify default refusal preserves the grant and `share_requests` table,
  explicit transaction-local opt-in completes the down migration, and an empty
  `DOWNLOAD` set needs no override.
- Release documentation now gives the one-time disposable-database transaction
  example and makes production forward-fix the default.

### Organization-only personal sharing

- Governance HTTP no longer copies `organization_id` into
  `SubmitShare.WorkspaceID`.
- `SubmitShare.ScopeID` carries command/idempotency scope independently; the
  canonical hash binds both scope and the real workspace authorization context.
- Personal namespace preview/submit works with only `X-Organization-ID` for the
  owner. Cross-organization and non-owner requests are concealed. Workspace
  namespaces still require a middleware-validated workspace context.
- Principal resolution with a nil workspace returns only `USER`; organization
  grants are available only after authoritative active workspace membership.
- Header contracts remain unchanged: organization required, workspace optional.

### Stable approve/reject replay

- Review hashes canonically bind operation, actor, organization, request, and
  normalized reason.
- A completed command is read, identity-checked, re-claimed, and its persisted
  request returned before mutable target-principal, permission, self-approval,
  or status checks.
- First execution still performs target resolution, current admin checks,
  separation of duties, and status CAS.
- Tests cover approve and reject replay after permission/target invalidation,
  first-execution denial, and conflicts for actor, organization, request,
  operation, and reason changes.

### Bounded children authorization

- Raw candidate scans are capped at 256 per response in at most four 64-node
  chunks.
- Each non-empty chunk uses one `AuthorizeMany` call; no per-child ancestor
  authorization loop remains.
- Budget exhaustion returns a safe encrypted continuation cursor instead of
  scanning indefinitely.
- The cursor is domain-separated and authenticated against namespace, parent,
  actor, workspace, node type, and normalized query.
- Tests cover bounded repository/authorization calls, hidden-candidate budget
  behavior, cursor context replay rejection, and duplicate-free normal paging.
- OpenAPI documents the bounded scan, opaque cursor, filters, and 503 response.

### Critical follow-up: organization grant workspace gate

- Removed the nil-workspace `ORGANIZATION` principal that could expose
  organization-granted shared metadata to an organization-only request.
- A real `PrincipalResolver` + authorization `Service` + SharedWithMe projection
  test proves organization grants are hidden without workspace context, visible
  with active same-organization membership, hidden for inactive membership, and
  rejected for a cross-organization workspace.
- Personal namespace owners continue to preview and submit shares with only
  `X-Organization-ID`; that path relies on namespace owner authority, not an
  organization principal.

## TDD evidence

Observed RED before implementation:

- Nil-workspace principal resolution incorrectly included the organization
  principal, and the real SharedWithMe projection returned the organization
  grant without workspace context.
- Personal-share tests did not compile because `ScopeID` did not exist.
- Completed review replay failed after target scope/authority invalidation.
- Migration static test found `DROP TABLE` before any destructive guard.
- Children tests did not compile without batch authorization, scan constants,
  and the context-bound children cursor.
- OpenAPI did not define the children cursor contract or scan budget.

All corresponding focused tests passed after the minimal implementation.

## Verification

Passed:

- `go test ./internal/modules/filesystem/... -count=1`
- `go test ./migrations -count=1`
- `go test ./api/openapi -count=1`
- `go vet ./internal/modules/filesystem/... ./api/openapi ./migrations`
- `pnpm typecheck` (6/6 frontend packages)
- IDE diagnostics for affected backend/OpenAPI/migration paths
- `git diff --check`

Environment-limited:

- `DATABASE_DSN` is not set. PostgreSQL-conditional migration tests explicitly
  reported `SKIP`; no real PostgreSQL execution is claimed.

Known unrelated blocker:

- Repository-wide `go vet ./...` reaches an existing failure in
  `internal/modules/product_workflow/domain/engine/environment_sync_executor_test.go`:
  `SaveDeployRef passes lock by value` through a test repository containing
  `sync.Mutex`. The affected-scope vet command above passes, and this unrelated
  file was not changed.
