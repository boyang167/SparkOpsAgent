# Task 6 Report: Grant write API and sharing approval workflow

## Status

Task 6 review findings fixed on `master`; implementation remains fail-closed.

## Review fixes

- Every governed and legacy submission now passes through one command entry that authorizes authoritative `SHARE` access to the source node.
- Node classification is authoritative persisted state. Migration 85 marks historical directories `INTERNAL`, unknown files `RESTRICTED`, and derives known files from completed transfers using the highest observed sensitivity; transfer commit records both Node classification and the authoritative transfer-to-node link.
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

## Second security review

- RED evidence:
  - Migration/fallback tests failed because historical files were all `INTERNAL` and export/transfer classification columns were absent.
  - Export/import tests failed because `ExportTask` had no classification and import used the default.
  - Command/HTTP/OpenAPI tests failed because DENY still entered ALLOW policy, views omitted effect, and byte length rejected valid Unicode reasons.
- GREEN evidence:
  - `go test ./migrations -count=1`
  - `go test ./internal/modules/filesystem/application/command -run "Grant|ShareRequest|ControlledExport|Copy|CommitTransfer" -count=1`
  - `go test ./internal/modules/filesystem/application/operation -run Import -count=1`
  - `go test ./internal/modules/filesystem/infrastructure/persistence -count=1`
  - `go test ./internal/modules/filesystem/interface/http -run "Grant|ShareRequest|GovernedShare|Export|Reject" -count=1`
  - `go test ./internal/modules/filesystem/... -count=1`
  - `go test ./api/openapi ./internal/modules/tenant/... ./internal/modules/governance/... -count=1`
  - `git diff --check`
- Controlled export snapshots the source Node classification; persistence and HTTP expose it, and import preserves it. Legacy or in-flight exports without valid classification import as `RESTRICTED`.
- `DENY` requests still require source `SHARE` authorization and authoritative same-organization principal resolution, then immediately create DENY grants without expansion approval or ALLOW-only organization restrictions.
- Share request views and OpenAPI now expose `effect`.
- Reject reason limits use trimmed Unicode code-point counts, with 1000/1001 Chinese-character boundaries covered in HTTP and command tests.
- Derived `NewFile` paths were audited: transfer commit, copy, and import each have classification-preservation tests; ordinary local file creation remains the intentional `INTERNAL` default.

## Task 13 handoff

- Migration 85 down is intentionally lossy: `DOWNLOAD` grants are deleted before restoring the old permission constraint because the prior schema cannot represent them. Task 13 rollout/rollback procedures must surface this deletion and require an explicit backup or acceptance before rollback.
