# Backend Secret Preview Repair Report

## Scope

- Worktree: `A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- Scope: Cluster 3 only, `TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData`
- Unrelated failure clusters were not changed.

## Root-cause evidence

The exact baseline command:

```text
cd backend
go test ./internal/modules/delivery/deployrun/application/query -run '^TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData$' -count=1
```

failed because the Secret diff note was `resource type not selected for sync`, not
`secret data is not copied`.

The data flow was traced through:

1. `CreateEnvironmentSyncPreviewHandler.Handle` normalizes each project option and
   passes captured source/target resources to `service.PlanEnvironmentSync`.
2. `NormalizeEnvironmentSyncOption` filtered explicit kinds against
   `environmentSyncSelectableKinds`.
3. That list contained Deployment, Service, ConfigMap, and Ingress, but omitted
   Secret, so an explicitly selected Secret was removed before planning.
4. `PlanEnvironmentSync` already contains the security-sensitive Secret branch:
   it removes both `data` and `stringData` via `stripSecretData` and sets the note
   `secret data is not copied`.

Product semantics were checked against callers, tests, the sync manifest whitelist,
the source-resource endpoint, frontend defaults, design specification, and git
history. The design says Secret is selectable but not default-selected; the final
manifest whitelist already permits Secret, and managed pull Secrets are a separate
credential-backed path. Therefore the repair preserves the four existing default
resource kinds while allowing explicit Secret selection.

## TDD evidence

### RED

Added the minimal domain contract test:

```text
go test ./internal/modules/delivery/deployrun/domain/model -run '^TestNormalizeEnvironmentSyncOptionAllowsExplicitSecretSelection$' -count=1
```

Result: RED. The expected explicit `Secret` selection normalized to an empty
selection because Secret was not in the selectable-kind list.

### GREEN

After the minimal production change and explicit-selection preview contract:

```text
go test ./internal/modules/delivery/deployrun/domain/model -run '^TestNormalizeEnvironmentSyncOptionAllowsExplicitSecretSelection$' -count=1
```

Result: PASS.

```text
go test ./internal/modules/delivery/deployrun/application/query -run '^TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData$' -count=1
```

Result: PASS. The test explicitly selects Secret, requires the Secret diff to be
present, requires the note, and asserts neither `data` nor `stringData` appears in
the generated preview.

## Changes

- Added Secret to backend selectable resource kinds.
- Kept the default resource-kind set unchanged: Deployment, Service, ConfigMap,
  and Ingress.
- Updated the preview test to express the documented explicit-Secret contract and
  to fail if the Secret diff is silently absent.
- Added the domain normalization contract test.
- No changes were made to `stripSecretData` or any secret-redaction assertion.

## Verification

Passed:

```text
go test ./internal/modules/delivery/deployrun/domain/model -count=1
go test ./internal/modules/delivery/deployrun/application/service -count=1
go test ./internal/modules/delivery/deployrun/application/query -count=1
```

`git diff --check` passed and IDE lint reported no errors for the changed files.

## Commit

This report is included in the single atomic commit created with the changes;
the exact commit identifier is returned in the handoff.

## Concerns

- The current frontend resource-kind list still exposes only Deployment, Service,
  ConfigMap, and Ingress, so the backend now supports explicit Secret selection
  but the existing frontend does not offer that selection yet. This was left
  untouched because the requested scope is backend Cluster 3 only.
- The full backend baseline still contains the other diagnosis-report failure
  clusters; they were intentionally not addressed.

## Follow-up: target Secret `Before` redaction

### Finding and root cause

Review found that `mergeResource` assigned `tgt.YAML` directly to
`diff.Before` before the existing Secret redaction branch. The prior query test
had no target Secret, so it could not observe this leak. For a target Secret
containing `data` or `stringData`, the preview view exposed those fields in
`Before`.

### TDD RED

The query regression test was extended with a target Secret containing both
`data` and `stringData`, while retaining the existing assertions for both
`Before` and `After`:

```text
go test ./internal/modules/delivery/deployrun/application/query -run '^TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData$' -count=1
```

Result: RED. The failure showed the target Secret `Before` still contained
`stringData`/`data`.

### Minimal fix and GREEN

The planner now:

- redacts Secret `diff.Before` immediately after reading target YAML;
- uses the redacted `diff.Before` for unchanged/unsupported Secret `After`;
- reuses the same `redactSecretYAML` helper for merged Secret `After`;
- leaves non-Secret `Before`/`After` behavior unchanged.

The exact regression test then passed:

```text
go test ./internal/modules/delivery/deployrun/application/query -run '^TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData$' -count=1
```

### Follow-up verification

Passed:

```text
go test ./internal/modules/delivery/deployrun/domain/model -count=1
go test ./internal/modules/delivery/deployrun/application/service -count=1
go test ./internal/modules/delivery/deployrun/application/query -count=1
```

`git diff --check` passed and IDE lint reported no errors for the changed files.

### Follow-up commit

The test, planner fix, and this report append are included in the new atomic
follow-up commit; its exact identifier is returned in the handoff.

### Follow-up concerns

No new concerns. The existing scope concerns remain: the frontend does not yet
offer an explicit Secret kind selector, and other baseline failure clusters
remain intentionally untouched.
