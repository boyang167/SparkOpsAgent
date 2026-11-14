# Task 6 Report: Grant write API and sharing approval workflow

## Status

Task 6 review findings fixed on `master`; implementation remains fail-closed.

## Review fixes

- Every governed and legacy submission now passes through one command entry that authorizes authoritative `SHARE` access to the source node.
- Node classification is authoritative persisted state. Migration 85 backfills existing nodes to `INTERNAL`; transfer commit writes the session classification; copied files preserve it.
- Classification lookup is a primary-key query and rejects missing, empty, or illegal values. Inherited directory sharing uses one recursive CTE to determine the subtree maximum.
- Every pending request enforces separation of duties. Approval still uses status CAS and transactionally writes grants, state, command result, and Outbox.
- Legacy `ALLOW` and `DENY` are accepted without requiring or trusting `grantee_organization_id`; effect is persisted and applied to generated grants.
- Rollback deletes unrepresentable `DOWNLOAD` grants before restoring the old constraint.
- Reject reasons are trimmed and validated as 1..1000 in HTTP and command layers.
- PostgreSQL migration, SQLite fallback schema, persistence models, runtime behavior, and OpenAPI are synchronized.

## TDD evidence

RED:

- Command tests failed because source authorization, subtree classification, and universal pending self-approval protection were absent.
- Persistence tests failed because Node had no classification and no recursive subtree query.
- HTTP tests failed because legacy `DENY` was rejected and reject reason bounds were not enforced.
- Migration/OpenAPI tests failed because Node classification, safe DOWNLOAD rollback, optional compatibility organization, and schema bounds were absent.

GREEN:

- `go test ./migrations -count=1`
- `go test ./internal/modules/filesystem/application/command -run "Grant|ShareRequest" -count=1`
- `go test ./internal/modules/filesystem/infrastructure/persistence -count=1`
- `go test ./internal/modules/filesystem/interface/http -run "Grant|ShareRequest|GovernedShare" -count=1`
- `go test ./internal/modules/filesystem/... -count=1`
- `go test ./api/openapi ./internal/modules/tenant/... ./internal/modules/governance/... -count=1`
- `git diff --check`

## Self-review

- Client organization and classification fields never drive authorization.
- Resolver and source-authorization failures deny before mutation.
- Batch scope resolution and subtree classification complete before transaction writes.
- Missing/invalid classification cannot fall back to `INTERNAL`.
- Reject never creates grants; replay and non-pending conflicts retain their prior semantics.

## Concerns

- PostgreSQL subtree coverage runs when the repository test DSN is configured; SQLite coverage always runs.
