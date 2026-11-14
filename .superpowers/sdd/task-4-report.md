# Task 4 Report: Principal resolution and effective authorization

## Status

Completed on base commit `f047472`.

## Implementation

- Added an application-level `PrincipalResolver` that always resolves the actor's USER principal and adds WORKSPACE, ORGANIZATION, and authoritative ROLE principals only for an active member of the requested Workspace.
- Added shared `authz` ports for authoritative Workspace access and role IDs, exposed them through `appctx.Container`, and wired tenant/governance implementations without importing their infrastructure into filesystem.
- Reworked filesystem resource authorization to:
  - load the authoritative Namespace before evaluating access;
  - reject Workspace/Organization scope mismatches;
  - query grants for all resolved principals;
  - limit USER authority to the USER Namespace owner;
  - limit `file:admin` management authority to active members of the current WORKSPACE Namespace;
  - use `org:file:admin` for ORGANIZATION Namespace management;
  - preserve global Deny precedence across the complete ancestor chain.
- Added `AuthorizationCacheKey` with organization, workspace, actor, node, action, and Namespace policy version. No authorization cache was introduced.
- Added `org:file:admin` to the fixed Organization owner/admin permission map. Governance default Workspace role bootstrap assignments were not changed.

## TDD evidence

RED was observed before implementation:

```text
undefined: ResolvePrincipalsInput
undefined: NewPrincipalResolver
undefined: ErrPrincipalScopeMismatch
```

Coverage added for authoritative principal resolution, inactive membership, cross-Organization rejection, USER owner access, Tenant Owner non-bypass, Workspace and Organization grants, global Deny precedence, nearest inherited Allow selection, Namespace-aware management authority, and policy-version cache keys.

## Verification

- `go test ./internal/modules/filesystem/application/authorization -count=1` — PASS
- `go test ./internal/modules/filesystem/domain/service -run Decide -count=1` — PASS
- `go test ./internal/modules/filesystem/interface/http -run Access -count=1` — PASS (package compiled; no matching test names)
- `go test ./internal/modules/filesystem/... -count=1` — PASS
- `go test ./internal/modules/tenant/... ./internal/modules/governance/... -count=1` — PASS
- `go test ./internal/bootstrap/... -count=1` — PASS
- IDE lint diagnostics for changed areas — none
- `git diff --check` — PASS

## Self-review

- Filesystem depends only on application/shared ports; there are no tenant/governance infrastructure imports.
- Client-provided Organization or role claims are not used for authorization reads.
- Organization audience is absent without valid Workspace context.
- Inactive Workspace membership cannot gain Workspace, Organization, ROLE, or Workspace-admin authority.
- Deny relations are collected through the full ancestor chain before the domain decision.

## Concerns

No blocking concerns. Task 3's `GranteeOrganizationID` remains on grant mutation requests as planned, and Task 5 API work was intentionally not included. No cache exists yet; the cache-key contract is defined and tested for future use.
