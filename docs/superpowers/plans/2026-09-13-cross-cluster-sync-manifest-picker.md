# Cross-Cluster Sync Manifest Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let operators pick and edit Kubernetes manifests in the cross-cluster sync modal; apply only selected manifests to sync target clusters (not the build cluster).

**Architecture:** Persist selected manifests on `deployment_runs.sync_manifests`. Add `GET .../deployments/sync-preview` to list related live resources from the build cluster. Extend the k8s deployer to support ConfigMap/Secret/Ingress and optional workloads. Orchestrator uses user manifests for sync_only target clusters.

**Tech Stack:** Go, GORM/Postgres, Gin, Next.js, Zod, Vitest

## Global Constraints

- Only the **cross-cluster sync** entry gets the picker (publish unchanged).
- Edited YAML applies **only to this sync run’s target clusters**.
- Kind whitelist: Deployment, Service, ConfigMap, Secret, Ingress.
- Managed image-pull Secret: no dockerconfigjson in UI; ensure via Harbor on target.
- Do not delete unchecked resources on target clusters.

## File map

| Path | Responsibility |
|------|----------------|
| `backend/migrations/000044_deployment_sync_manifests.up.sql` | Add `sync_manifests JSONB` |
| `backend/.../domain/model/sync_manifest.go` | SyncManifest type + validation |
| `backend/.../domain/model/deployment_run.go` | `SyncManifests` field |
| `backend/.../infrastructure/persistence/deployment_run_repo.go` | Persist/load JSONB |
| `backend/.../application/command/create_deployment.go` | Accept + validate manifests |
| `backend/.../application/query/sync_preview.go` | Build-cluster related resource preview |
| `backend/.../interface/http/handler.go` + module routes | Wire sync-preview |
| `backend/.../runtime/.../deployer.go` | Extra kinds + optional workload mode |
| `backend/.../deployrun/.../orchestrator.go` | Apply user manifests to sync targets only |
| `fronted/packages/contracts/src/delivery/index.ts` | Preview + manifests schemas |
| `fronted/packages/{web-sdk,upstream,bff-modules}/src/delivery.ts` | API clients |
| `fronted/apps/web/.../CrossClusterConfirmModal.tsx` | Checklist + YAML editor |
| `fronted/apps/web/.../ProjectWorkbenchPage.tsx` | Pass manifests on sync |

---

### Task 1: Migration + domain SyncManifest

**Files:**
- Create: `backend/migrations/000044_deployment_sync_manifests.up.sql`
- Create: `backend/migrations/000044_deployment_sync_manifests.down.sql`
- Create: `backend/internal/modules/delivery/deployrun/domain/model/sync_manifest.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/model/sync_manifest_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go`

- [ ] Add column `sync_manifests JSONB`
- [ ] Define `SyncManifest` with Kind/Name/YAML/ManagedPullSecret
- [ ] Validate whitelist, identity lock (kind/name/namespace), non-empty selection for sync_only when provided
- [ ] Add `SyncManifests []SyncManifest` on `DeploymentRun`
- [ ] Unit tests for validation
- [ ] Commit

### Task 2: Persist SyncManifests on create

**Files:**
- Modify: `deployment_run_repo.go` (row + toDomain/toRow)
- Modify: `create_deployment.go` + tests

- [ ] Bind `manifests` on CreateInput when `sync_only`
- [ ] Reject empty/invalid manifests for sync_only when client sends them (require ≥1)
- [ ] Persist JSONB
- [ ] Tests
- [ ] Commit

### Task 3: Deployer supports ConfigMap/Secret/Ingress + optional workloads

**Files:**
- Modify: `backend/internal/modules/runtime/domain/port/ports.go` (`AllowPartialWorkloads bool`)
- Modify: `backend/internal/modules/runtime/infrastructure/k8s/deployer.go`
- Modify: deployer tests

- [ ] `resourceFor` allow ConfigMap, Secret, Ingress
- [ ] When `AllowPartialWorkloads`: do not require both Deployment and Service; skip service get / rollout wait if absent
- [ ] Apply order: ConfigMap → Secret → Service → Ingress → Deployment
- [ ] Tests
- [ ] Commit

### Task 4: Orchestrator applies sync manifests to targets only

**Files:**
- Modify: `orchestrator.go` + tests

- [ ] If `run.IsSyncOnly() && len(SyncManifests)>0`: skip finalize on build cluster; for each other cluster apply user manifests (managed pull secret → ensure; Deployment present → RolloutRestart)
- [ ] Append `applied: ...` event
- [ ] Tests
- [ ] Commit

### Task 5: Sync preview API

**Files:**
- Create: `application/query/sync_preview.go` + test
- Modify: HTTP handler + module wiring
- Reuse runtime inspector / k8s list where possible

- [ ] GET `.../projects/:id/deployments/sync-preview`
- [ ] Filter related ConfigMap/Secret/Ingress; default-select Deployment/Service/pull-secret
- [ ] Managed pull secret has empty yaml + note
- [ ] Tests
- [ ] Commit

### Task 6: Frontend contracts + SDK/BFF

**Files:**
- Modify contracts, web-sdk, upstream, bff-modules, BFF route

- [ ] Schemas for preview + manifests on create
- [ ] `getSyncPreview` + create body `manifests`
- [ ] Contract tests
- [ ] Commit

### Task 7: Sync modal UI + workbench wiring

**Files:**
- Modify: `CrossClusterConfirmModal.tsx`, `ProjectWorkbenchPage.tsx`, CSS if needed

- [ ] xl modal: list + YAML editor for sync mode
- [ ] Load preview on open; default selection; edit constraints client-side
- [ ] Submit selected manifests
- [ ] Commit

---

## Spec coverage check

- Picker only on sync ✅ Task 7
- Default Deployment/Service/pull-secret ✅ Task 5/7
- Related ConfigMap/Secret/Ingress ✅ Task 5
- YAML edit this-run only ✅ Task 4/7
- sync_manifests persistence ✅ Task 1/2
- Deployer kinds + partial workloads ✅ Task 3
- Skip build-cluster Apply for sync_only user manifests ✅ Task 4
- Preview API ✅ Task 5
