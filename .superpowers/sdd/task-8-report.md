# Task 8 Report

## Status

Implemented current-workspace authorized Kubernetes cluster listing and per-cluster `plugin_kubernetes.cluster_settings` GET/PUT behavior.

## Key Changes

- Added neutral `platform/plugin.WriteAuthorizer` and injected a bootstrap adapter backed by `authz.PermissionChecker.HasPermission(..., authz.PermGovernanceAdmin)`.
- Added `GET /api/v1/plugins/kubernetes/clusters` to list only `Clusters.ListMeta(wsID)` authorized clusters and merge settings by `cluster_id`.
- Added `PUT /api/v1/plugins/kubernetes/clusters/:clusterId/settings` with workspace/member context lookup, governance-admin write guard, authorized cluster confirmation, URL validation, and settings upsert.
- Masked non-empty `PrometheusPassword`, `VelauxPassword`, and `LLMAPIKey` as `***` in responses; empty secrets remain empty.
- Treated request secret value `***` as a reserved placeholder that preserves the stored secret; empty string clears it.
- Added explicit 503 responses for missing plugin context dependencies instead of panics.

## TDD

RED:

- `go test ./internal/plugins/kubernetes/http/ -count=1`
- Expected failure observed: missing `PluginContext.Writer` and context-aware `RegisterRoutes` signature.

GREEN:

- Implemented platform context, bootstrap writer adapter, router/context wiring, and HTTP settings handlers.
- Fixed local error mapping so unauthorized cluster settings return 403 `PLUGIN_CLUSTER_FORBIDDEN`.

## Verification

- `go test ./internal/plugins/kubernetes/http/ -count=1`: pass.
- `go test ./internal/platform/plugin ./internal/bootstrap ./internal/plugins/kubernetes/schema -count=1`: pass.
- `go test ./internal/plugins/kubernetes -count=1`: pass.
- IDE lint on edited Go files: no diagnostics.

## Notes

- Kubernetes plugin HTTP code imports only platform/shared packages and its own schema; it does not import governance domain or infrastructure.
- Manifest metrics `read_only` entries were not changed.
- Full deployment SQLite failures were intentionally not chased.

## Review Fix Follow-up

Status: fixed all reported Critical/Important items and split the heavy HTTP file.

Changes:

- Added neutral `platform/plugin.WorkspaceMemberAuthorizer` and `PluginContext.Members`; bootstrap injects `c.WorkspaceMemberChecker`.
- Kubernetes plugin routes now install `httpx.RequireWorkspace()` and neutral membership middleware at the plugin group level, covering ready, current clusters/settings routes, and future routes under the same group.
- Missing `Members` returns 503; non-members return 403 `TENANT_NOT_WORKSPACE_MEMBER`; tests exercise the production middleware chain without externally adding `RequireWorkspace`.
- URL validation now rejects embedded userinfo, keeps the error message fixed without echoing the submitted URL, and still allows private/localhost hosts for trusted admin-configured internal service endpoints.
- Settings upsert moved into `settings_store.go`; PUT uses a DB transaction and `clause.Locking{Strength:"UPDATE"}` for existing rows before preserving `***` secrets.
- `clusters.go` now contains only handlers and view mapping; request merge/URL validation moved to `settings.go`, and DB access moved to `settings_store.go`.

TDD RED:

- `go test ./internal/plugins/kubernetes/http/ -count=1`
- Expected failure observed: `PluginContext` lacked `Members` for new production-chain tests.

GREEN / Verification:

- `go test ./internal/plugins/kubernetes/http/ -count=1`: pass.
- `go test ./internal/plugins/kubernetes/http/ ./internal/platform/plugin/ ./internal/bootstrap/ ./internal/plugins/kubernetes/ -count=1`: pass.
- IDE lint on edited Go files: no diagnostics.
