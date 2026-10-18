# Environment Deployment Sync Task Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an official `环境部署同步` workflow task that previews and copies selected projects' live image/YAML state from one product environment to another through auditable deployment runs.

**Architecture:** Delivery owns live Kubernetes capture, immutable preview snapshots, manifest merge, and `environment-sync` deployment runs. Product workflow owns task configuration, preview binding at run creation, per-project orchestration, and aggregate status. The frontend adds a dedicated editor drawer and requires a confirmed preview before starting a workflow that includes the task.

**Tech Stack:** Go 1.x, Gin, GORM/PostgreSQL, Kubernetes client-go, React 19, Next.js 15, TypeScript 5.8, Zod, Vitest, pnpm.

## Global Constraints

- Source and target environments must belong to the same product and must be different.
- Default project behavior is `sync_image=true`, `sync_yaml=false`.
- An unassociated target project forces both options to `true`; associate it only after successful deployment.
- Image synchronization means repository plus current tag.
- YAML comes from the source environment's live Kubernetes resources.
- YAML sync only creates or updates; it never deletes target-only resources.
- Never copy Kubernetes Secret data; only preserve managed pull-secret references.
- A confirmed preview is mandatory and immutable for the workflow run.
- One failed project does not stop the remaining projects; any project failure makes the job fail.
- Reuse existing design tokens and UI components; add no frontend library.
- Preserve unrelated uncommitted work and commit only files listed by each task.

---

## File Structure

Backend additions:

- `backend/internal/modules/delivery/deployrun/domain/model/environment_sync.go` — snapshot, project option, resource diff, and immutable captured project types.
- `backend/internal/modules/delivery/deployrun/domain/repository/environment_sync_snapshot_repository.go` — snapshot persistence contract.
- `backend/internal/modules/delivery/deployrun/infrastructure/persistence/environment_sync_snapshot_repo.go` — PostgreSQL implementation.
- `backend/internal/modules/delivery/deployrun/application/service/environment_sync_planner.go` — pure image/YAML merge and sanitization.
- `backend/internal/modules/delivery/deployrun/application/query/environment_sync_preview.go` — authorization, live capture, target diff, snapshot creation.
- `backend/internal/modules/delivery/deployrun/application/command/create_environment_sync.go` — create one target deployment run from one captured project.
- `backend/internal/modules/delivery/deployrun/interface/http/environment_sync_handler.go` — preview endpoint.
- `backend/internal/modules/product_workflow/domain/engine/environment_sync_executor.go` — fan out projects, save deploy refs, and aggregate completion.
- `backend/migrations/000076_environment_sync_snapshots.up.sql` and `.down.sql` — snapshot tables and deployment provenance.

Frontend additions:

- `fronted/apps/web/src/modules/product/components/environment-sync-job-spec.ts` — typed spec parser/serializer.
- `fronted/apps/web/src/modules/product/components/EnvironmentSyncJobConfigDrawer.tsx` — workflow editor configuration.
- `fronted/apps/web/src/modules/product/components/EnvironmentSyncRunFields.tsx` — runtime overrides and preview UI.
- `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/products/[productId]/environment-sync-previews/route.ts` — BFF route.

Existing files retain their current responsibilities; do not move unrelated code.

---

### Task 1: Register the task and define a stable spec

**Files:**
- Modify: `backend/internal/modules/product_workflow/domain/model/workflow_definition.go`
- Modify: `backend/internal/modules/product_workflow/domain/model/job_catalog.go`
- Modify: `backend/internal/modules/product_workflow/domain/model/job_catalog_test.go`
- Modify: `fronted/packages/contracts/src/product/product-workflow.ts`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-job-spec.ts`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-job-spec.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-editor-utils.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-editor-utils.test.ts`

**Interfaces:**
- Produces: `model.JobTypeSparkopsEnvironmentSync`.
- Produces: `EnvironmentSyncJobSpec`, `parseEnvironmentSyncJobSpec()`, `environmentSyncJobSpecToRecord()`.
- Produces task spec shape `{ sourceEnvironmentId, targetEnvironmentId, projects[] }`.

- [ ] **Step 1: Add failing backend catalog assertions**

```go
func TestJobCatalogIncludesEnvironmentSync(t *testing.T) {
	got := map[string]model.JobCatalogItem{}
	for _, item := range model.JobCatalog() {
		got[item.Type] = item
	}
	item := got[model.JobTypeSparkopsEnvironmentSync]
	require.Equal(t, "deploy", item.Category)
	require.Equal(t, "环境部署同步", item.Title)
	require.True(t, model.IsKnownJobType(model.JobTypeSparkopsEnvironmentSync))
}
```

- [ ] **Step 2: Run the backend test and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/product_workflow/domain/model -run TestJobCatalogIncludesEnvironmentSync -count=1
```

Expected: compile failure because `JobTypeSparkopsEnvironmentSync` is undefined.

- [ ] **Step 3: Add the backend constant and catalog item**

```go
const (
	JobTypeSparkopsEnvironmentSync = "sparkops-environment-sync"
)

// JobCatalog:
{Type: JobTypeSparkopsEnvironmentSync, Category: "deploy", Title: "环境部署同步",
	Description: "将源环境中的项目部署同步到目标环境", Executable: true},
```

- [ ] **Step 4: Add failing spec/default-job tests**

```ts
it("normalizes environment sync defaults", () => {
  expect(parseEnvironmentSyncJobSpec({
    source_environment_id: "source",
    target_environment_id: "target",
    projects: [{ project_id: "p1" }],
  })).toEqual({
    sourceEnvironmentId: "source",
    targetEnvironmentId: "target",
    projects: [{ projectId: "p1", syncImage: true, syncYaml: false }],
  });
});

it("creates an environment sync job", () => {
  expect(defaultJob("sparkops-environment-sync", 3)).toMatchObject({
    name: "environment-sync-3",
    type: "sparkops-environment-sync",
    spec: { source_environment_id: "", target_environment_id: "", projects: [] },
  });
});
```

- [ ] **Step 5: Run frontend tests and verify RED**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/environment-sync-job-spec.test.ts apps/web/src/modules/product/components/workflow-editor-utils.test.ts
```

Expected: missing module/default case failures.

- [ ] **Step 6: Implement the contracts entry and spec parser**

```ts
export interface EnvironmentSyncProjectSpec {
  projectId: string;
  syncImage: boolean;
  syncYaml: boolean;
}

export interface EnvironmentSyncJobSpec {
  sourceEnvironmentId: string;
  targetEnvironmentId: string;
  projects: EnvironmentSyncProjectSpec[];
}

export function parseEnvironmentSyncJobSpec(raw?: Record<string, unknown>): EnvironmentSyncJobSpec {
  const projects = Array.isArray(raw?.projects) ? raw.projects : [];
  return {
    sourceEnvironmentId: typeof raw?.source_environment_id === "string" ? raw.source_environment_id : "",
    targetEnvironmentId: typeof raw?.target_environment_id === "string" ? raw.target_environment_id : "",
    projects: projects.flatMap((value) => {
      if (!value || typeof value !== "object") return [];
      const item = value as Record<string, unknown>;
      const projectId = typeof item.project_id === "string" ? item.project_id.trim() : "";
      return projectId ? [{
        projectId,
        syncImage: item.sync_image !== false,
        syncYaml: item.sync_yaml === true,
      }] : [];
    }),
  };
}
```

Add the contracts catalog item:

```ts
{
  type: "sparkops-environment-sync",
  category: "deploy",
  label: "环境同步",
  title: "环境部署同步",
  description: "将源环境中的项目部署同步到目标环境",
  official: true,
},
```

- [ ] **Step 7: Run task tests**

Run:

```powershell
cd backend
go test ./internal/modules/product_workflow/domain/model -count=1
cd ../fronted
pnpm vitest run apps/web/src/modules/product/components/environment-sync-job-spec.test.ts apps/web/src/modules/product/components/workflow-editor-utils.test.ts
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add backend/internal/modules/product_workflow/domain/model/workflow_definition.go backend/internal/modules/product_workflow/domain/model/job_catalog.go backend/internal/modules/product_workflow/domain/model/job_catalog_test.go fronted/packages/contracts/src/product/product-workflow.ts fronted/apps/web/src/modules/product/components/environment-sync-job-spec.ts fronted/apps/web/src/modules/product/components/environment-sync-job-spec.test.ts fronted/apps/web/src/modules/product/components/workflow-editor-utils.ts fronted/apps/web/src/modules/product/components/workflow-editor-utils.test.ts
git commit -m "feat(workflow): register environment sync task"
```

---

### Task 2: Implement deterministic live-resource merge rules

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/domain/model/environment_sync.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/environment_sync_planner.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/environment_sync_planner_test.go`

**Interfaces:**
- Produces: `EnvironmentSyncProjectOption`, `EnvironmentSyncResource`, `EnvironmentSyncProjectSnapshot`.
- Produces: `PlanEnvironmentSync(source, target []EnvironmentSyncResource, option EnvironmentSyncProjectOption, targetNamespace string)`.
- Produces: `EnvironmentSyncPlan{Manifests, Diffs, ImageRepository, ImageTag}`.

- [ ] **Step 1: Write failing planner tests for the four merge paths**

```go
func TestPlanEnvironmentSyncImageOnlyPreservesTargetSpec(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 5)
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 2)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: true, SyncYAML: false},
		"target-ns",
	)

	require.NoError(t, err)
	require.Contains(t, plan.Manifests[0].YAML, "registry/source/app:v2")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 2")
}

func TestPlanEnvironmentSyncYAMLOnlyPreservesTargetImage(t *testing.T) {
	// Source replicas are copied, target image is retained.
}

func TestPlanEnvironmentSyncNewWorkloadUsesSourceImage(t *testing.T) {
	// There is no target image to preserve.
}

func TestPlanEnvironmentSyncNeverCopiesSecretDataOrDeletesTargetOnlyResources(t *testing.T) {
	// Secret data is absent and target-only resources do not appear as delete operations.
}
```

- [ ] **Step 2: Run the planner tests and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/application/service -run TestPlanEnvironmentSync -count=1
```

Expected: compile failure because planner types/functions do not exist.

- [ ] **Step 3: Define snapshot and diff types**

```go
type EnvironmentSyncProjectOption struct {
	ProjectID  kernel.ProjectID `json:"project_id"`
	SyncImage  bool             `json:"sync_image"`
	SyncYAML   bool             `json:"sync_yaml"`
	FirstSync  bool             `json:"first_sync"`
}

type EnvironmentSyncResource struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	YAML              string `json:"yaml,omitempty"`
	ManagedPullSecret bool   `json:"managed_pull_secret,omitempty"`
}

type EnvironmentSyncDiff struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Action string `json:"action"` // create | update | unchanged | excluded
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Note   string `json:"note,omitempty"`
}
```

- [ ] **Step 4: Implement minimal pure planner**

The implementation must:

```go
func PlanEnvironmentSync(
	source, target []model.EnvironmentSyncResource,
	option model.EnvironmentSyncProjectOption,
	targetNamespace string,
) (model.EnvironmentSyncPlan, error) {
	if option.FirstSync {
		option.SyncImage, option.SyncYAML = true, true
	}
	// Index by kind/name, sanitize source metadata, then:
	// image only -> clone target workload and replace matched container images
	// YAML only -> clone source resource and restore target workload images
	// both -> use sanitized source
	// never emit a delete diff or Secret.data/stringData
	return plan, nil
}
```

Use `model.SyncManifest` and `model.SyncApplyOrder()` for output ordering. Match containers by workload `kind/name` plus container name; return a domain error if source and target container identity is ambiguous.

- [ ] **Step 5: Add sanitization tests**

```go
func TestPlanEnvironmentSyncSanitizesClusterMetadata(t *testing.T) {
	// Assert target namespace is used and status, uid, resourceVersion,
	// generation, managedFields and timestamps are absent.
}

func TestPlanEnvironmentSyncRejectsAmbiguousContainerMatch(t *testing.T) {
	// Assert an explicit error instead of choosing a container.
}
```

- [ ] **Step 6: Run planner tests**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/application/service -run "TestPlanEnvironmentSync" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/modules/delivery/deployrun/domain/model/environment_sync.go backend/internal/modules/delivery/deployrun/application/service/environment_sync_planner.go backend/internal/modules/delivery/deployrun/application/service/environment_sync_planner_test.go
git commit -m "feat(delivery): add environment sync planner"
```

---

### Task 3: Persist immutable preview snapshots and deployment provenance

**Files:**
- Modify: `backend/internal/shared/kernel/ids.go`
- Modify: `backend/internal/shared/kernel/ids_test.go`
- Create: `backend/migrations/000076_environment_sync_snapshots.up.sql`
- Create: `backend/migrations/000076_environment_sync_snapshots.down.sql`
- Create: `backend/internal/modules/delivery/deployrun/domain/repository/environment_sync_snapshot_repository.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/environment_sync_snapshot_repo.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/environment_sync_snapshot_repo_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go`

**Interfaces:**
- Produces: `kernel.EnvironmentSyncSnapshotID`.
- Produces: `EnvironmentSyncSnapshotRepository.Create`, `FindByID`, `BindToRun`.
- Adds deployment provenance columns `source_environment_id` and `environment_sync_snapshot_id`.

- [ ] **Step 1: Write the repository contract and failing round-trip test**

```go
type EnvironmentSyncSnapshotRepository interface {
	Create(ctx context.Context, snapshot *model.EnvironmentSyncSnapshot) error
	FindByID(ctx context.Context, id kernel.EnvironmentSyncSnapshotID) (*model.EnvironmentSyncSnapshot, error)
	BindToRun(ctx context.Context, id kernel.EnvironmentSyncSnapshotID, runID kernel.ProductWorkflowRunID) error
}

func TestEnvironmentSyncSnapshotRepositoryRoundTripAndSingleBind(t *testing.T) {
	// Create a snapshot with two projects, reload it, bind it once,
	// and assert a second bind returns ErrEnvironmentSyncSnapshotAlreadyBound.
}
```

- [ ] **Step 2: Run the persistence test and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/infrastructure/persistence -run TestEnvironmentSyncSnapshotRepository -count=1
```

Expected: compile failure for missing repository/model/ID.

- [ ] **Step 3: Add the migration**

```sql
CREATE TABLE delivery.environment_sync_snapshots (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    product_id UUID NOT NULL,
    workflow_id UUID NOT NULL,
    job_name VARCHAR(255) NOT NULL,
    source_environment_id UUID NOT NULL,
    target_environment_id UUID NOT NULL,
    projects JSONB NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    created_by UUID NOT NULL,
    bound_run_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bound_at TIMESTAMPTZ
);

ALTER TABLE delivery.deployment_runs
    ADD COLUMN source_environment_id UUID,
    ADD COLUMN environment_sync_snapshot_id UUID;
```

The down migration drops the two deployment columns, then the snapshot table and its product/created indexes.

- [ ] **Step 4: Implement typed ID and repository**

```go
type EnvironmentSyncSnapshotID struct{ uuid.UUID }

func NewEnvironmentSyncSnapshotID() EnvironmentSyncSnapshotID {
	return EnvironmentSyncSnapshotID{UUID: uuid.New()}
}

func ParseEnvironmentSyncSnapshotID(raw string) (EnvironmentSyncSnapshotID, error) {
	id, err := uuid.Parse(raw)
	return EnvironmentSyncSnapshotID{UUID: id}, err
}
```

Use one JSONB `projects` column so the captured YAML remains immutable and is loaded atomically. Add the snapshot row to the existing `AutoMigrate` call in `deployment_run_repo.go`. `BindToRun` must execute:

```sql
UPDATE delivery.environment_sync_snapshots
SET bound_run_id = ?, bound_at = NOW()
WHERE id = ? AND bound_run_id IS NULL
```

and require exactly one affected row.

- [ ] **Step 5: Run ID and repository tests**

Run:

```powershell
cd backend
go test ./internal/shared/kernel ./internal/modules/delivery/deployrun/infrastructure/persistence -run "EnvironmentSync" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/shared/kernel/ids.go backend/internal/shared/kernel/ids_test.go backend/migrations/000076_environment_sync_snapshots.up.sql backend/migrations/000076_environment_sync_snapshots.down.sql backend/internal/modules/delivery/deployrun/domain/repository/environment_sync_snapshot_repository.go backend/internal/modules/delivery/deployrun/infrastructure/persistence/environment_sync_snapshot_repo.go backend/internal/modules/delivery/deployrun/infrastructure/persistence/environment_sync_snapshot_repo_test.go backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go
git commit -m "feat(delivery): persist environment sync previews"
```

---

### Task 4: Build and expose the environment sync preview

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/application/query/environment_sync_preview.go`
- Create: `backend/internal/modules/delivery/deployrun/application/query/environment_sync_preview_test.go`
- Create: `backend/internal/modules/delivery/deployrun/interface/http/environment_sync_handler.go`
- Create: `backend/internal/modules/delivery/deployrun/interface/http/environment_sync_handler_test.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `fronted/packages/contracts/src/product/environment.ts`
- Modify: `fronted/packages/upstream/src/product.ts`
- Modify: `fronted/packages/bff-modules/src/product/index.ts`
- Modify: `fronted/packages/web-sdk/src/product.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/products/[productId]/environment-sync-previews/route.ts`

**Interfaces:**
- Produces backend `CreateEnvironmentSyncPreviewHandler.Handle(ctx, wsID, productID, workflowID, jobName, actor, input)`.
- Produces `POST /api/v1/products/:productId/environment-sync-previews`.
- Produces frontend `product.createEnvironmentSyncPreview(...)`.

- [ ] **Step 1: Write failing preview service tests**

```go
func TestCreateEnvironmentSyncPreviewNormalizesFirstSync(t *testing.T) {
	input := query.CreateEnvironmentSyncPreviewInput{
		WorkflowID: workflowID,
		JobName: "promote",
		SourceEnvironmentID: sourceID,
		TargetEnvironmentID: targetID,
		Projects: []model.EnvironmentSyncProjectOption{{
			ProjectID: projectID, SyncImage: true, SyncYAML: false,
		}},
	}
	preview, err := handler.Handle(ctx, wsID, productID, actorID, input)
	require.NoError(t, err)
	require.True(t, preview.Projects[0].Option.FirstSync)
	require.True(t, preview.Projects[0].Option.SyncYAML)
}

func TestCreateEnvironmentSyncPreviewRejectsSameEnvironment(t *testing.T) {}
func TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData(t *testing.T) {}
```

- [ ] **Step 2: Run preview tests and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/application/query -run TestCreateEnvironmentSyncPreview -count=1
```

Expected: compile failure for missing handler.

- [ ] **Step 3: Implement live capture and snapshot creation**

Refactor only reusable live-read helpers from `sync_preview.go`; keep its public response unchanged. The new handler must:

```go
func (h *CreateEnvironmentSyncPreviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	actor kernel.MemberID,
	input CreateEnvironmentSyncPreviewInput,
) (*model.EnvironmentSyncSnapshot, error)
```

For each project:

1. Resolve source and target with `port.EnvironmentResolver`.
2. Verify both resolved environments belong to `productID`.
3. Load source/target cluster credentials through `ProjectDeploymentReader`.
4. Read supported resources from the environments' namespaces.
5. Detect first sync from target `environment_projects`.
6. Call `service.PlanEnvironmentSync`.
7. Store source resources, planned manifests, diffs, and image repository/tag in the immutable snapshot.

- [ ] **Step 4: Add HTTP request/response and route**

```go
type createEnvironmentSyncPreviewRequest struct {
	WorkflowID             string                               `json:"workflow_id" binding:"required"`
	JobName                string                               `json:"job_name" binding:"required"`
	SourceEnvironmentID    string                               `json:"source_environment_id" binding:"required"`
	TargetEnvironmentID    string                               `json:"target_environment_id" binding:"required"`
	Projects               []environmentSyncProjectRequest      `json:"projects" binding:"required"`
}
```

Register with workspace auth and product write permission:

```go
c.Engine.POST("/api/v1/products/:productId/environment-sync-previews",
	auth, ws, wsMember, writePerm, environmentSyncHandler.CreatePreview)
```

- [ ] **Step 5: Add matching Zod and SDK types**

```ts
export const EnvironmentSyncPreviewCommandSchema = z.object({
  workflow_id: z.string().min(1),
  job_name: z.string().min(1),
  source_environment_id: z.string().min(1),
  target_environment_id: z.string().min(1),
  projects: z.array(z.object({
    project_id: z.string().min(1),
    sync_image: z.boolean(),
    sync_yaml: z.boolean(),
  })).min(1),
});
```

The VM includes `preview_id`, normalized projects, image before/after, resource diffs, and warnings. The BFF route must parse the command schema before calling `createEnvironmentSyncPreview`.

- [ ] **Step 6: Run backend tests and frontend typecheck**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/application/query ./internal/modules/delivery/deployrun/interface/http -run EnvironmentSync -count=1
cd ../fronted
pnpm --filter @sparkops/web typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/modules/delivery/deployrun/application/query/environment_sync_preview.go backend/internal/modules/delivery/deployrun/application/query/environment_sync_preview_test.go backend/internal/modules/delivery/deployrun/interface/http/environment_sync_handler.go backend/internal/modules/delivery/deployrun/interface/http/environment_sync_handler_test.go backend/internal/modules/delivery/module.go fronted/packages/contracts/src/product/environment.ts fronted/packages/upstream/src/product.ts fronted/packages/bff-modules/src/product/index.ts fronted/packages/web-sdk/src/product.ts fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/products/[productId]/environment-sync-previews/route.ts
git commit -m "feat(delivery): add environment sync preview API"
```

---

### Task 5: Execute a captured project through DeploymentRun

**Files:**
- Modify: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go`
- Modify: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run_test.go`
- Create: `backend/internal/modules/delivery/deployrun/application/command/create_environment_sync.go`
- Create: `backend/internal/modules/delivery/deployrun/application/command/create_environment_sync_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/service/orchestrator.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/service/orchestrator_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo_test.go`
- Modify: `backend/internal/modules/delivery/domain/event/deploy_completed.go`
- Modify: `backend/internal/modules/delivery/module.go`

**Interfaces:**
- Produces: `RunTypeEnvironmentSync = "environment-sync"`.
- Produces: `CreateEnvironmentSyncDeploymentHandler.Handle(...)`.
- Persists source environment and snapshot IDs and emits them in completion events.

- [ ] **Step 1: Add failing domain and command tests**

```go
func TestNewEnvironmentSyncDeploymentRunSkipsBuildAndTargetsEnvironment(t *testing.T) {
	run := model.NewEnvironmentSyncDeploymentRun(wsID, projectID, targetClusterID, actorID)
	require.Equal(t, model.RunTypeEnvironmentSync, run.RunType)
	require.False(t, run.CrossCluster)
	require.Equal(t, model.StatusQueued, run.Status)
}

func TestCreateEnvironmentSyncDeploymentUsesCapturedManifests(t *testing.T) {
	// Load the bound snapshot project, create a run with its planned manifests,
	// source/target IDs and image repository/tag, then queue it.
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/domain/model ./internal/modules/delivery/deployrun/application/command -run EnvironmentSync -count=1
```

Expected: compile failure for run type/handler.

- [ ] **Step 3: Implement the deployment constructor and command**

```go
const RunTypeEnvironmentSync RunType = "environment-sync"

func NewEnvironmentSyncDeploymentRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	createdBy kernel.MemberID,
) *DeploymentRun {
	run := NewDeploymentRun(wsID, projectID, clusterID, createdBy, "branch", "", "")
	run.RunType = RunTypeEnvironmentSync
	run.ImageTag = ""
	return run
}
```

The command input is snapshot-based, not arbitrary YAML:

```go
type CreateEnvironmentSyncDeploymentInput struct {
	SnapshotID kernel.EnvironmentSyncSnapshotID
	ProjectID  kernel.ProjectID
}
```

It must load a bound snapshot, locate the project, re-resolve the target environment, enforce `HasActiveByProject`, copy only the captured planned manifests, create the run, and publish the standard deployment operation.

- [ ] **Step 4: Add failing orchestrator tests**

```go
func TestEnvironmentSyncRunAppliesToTargetWithoutBuildOrCrossCluster(t *testing.T) {
	// queued -> preparing -> deploying -> succeeded;
	// builder calls == 0, deployer calls == 1, target namespace is target env.
}

func TestEnvironmentSyncRunReportsPartialApplyFailure(t *testing.T) {
	// The run fails and keeps resource-level event messages.
}
```

- [ ] **Step 5: Implement orchestrator branching**

In preparing, transition an environment-sync run directly to deploying. In deploying, call a focused `applyEnvironmentSyncManifests` against the resolved target project context and do not call `syncCrossClusterTargets`.

`applyEnvironmentSyncManifests` must use `SyncApplyOrder`, ensure managed pull secrets first, then call the deployer once per remaining manifest. Append `applied: Kind/name` after each success and `failed: Kind/name: reason` on failure. Stop that project at the first failed resource while retaining prior success events; this provides the resource-level partial-apply record required for safe retry. After all manifests succeed, restart each changed workload once and emit normal completion.

- [ ] **Step 6: Persist and emit provenance**

Map `SourceEnvironmentID` and `EnvironmentSyncSnapshotID` in the GORM model. Add both optional values to `DeployCompleted` so subscribers and UI can distinguish first sync without reading mutable task input.

- [ ] **Step 7: Run delivery tests**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/domain/model ./internal/modules/delivery/deployrun/application/command ./internal/modules/delivery/deployrun/application/service ./internal/modules/delivery/deployrun/infrastructure/persistence -run EnvironmentSync -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go backend/internal/modules/delivery/deployrun/domain/model/deployment_run_test.go backend/internal/modules/delivery/deployrun/application/command/create_environment_sync.go backend/internal/modules/delivery/deployrun/application/command/create_environment_sync_test.go backend/internal/modules/delivery/deployrun/application/service/orchestrator.go backend/internal/modules/delivery/deployrun/application/service/orchestrator_test.go backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo_test.go backend/internal/modules/delivery/domain/event/deploy_completed.go backend/internal/modules/delivery/module.go
git commit -m "feat(delivery): execute environment sync runs"
```

---

### Task 6: Bind previews and orchestrate multi-project workflow jobs

**Files:**
- Modify: `backend/internal/platform/appctx/ports.go`
- Modify: `backend/internal/platform/appctx/container.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Create: `backend/internal/modules/product_workflow/domain/engine/environment_sync_executor.go`
- Create: `backend/internal/modules/product_workflow/domain/engine/environment_sync_executor_test.go`
- Modify: `backend/internal/modules/product_workflow/domain/model/product_workflow.go`
- Modify: `backend/internal/modules/product_workflow/domain/engine/scheduler.go`
- Modify: `backend/internal/modules/product_workflow/domain/engine/scheduler_build_completed_test.go`
- Modify: `backend/internal/modules/product_workflow/application/command/workflow_commands.go`
- Create: `backend/internal/modules/product_workflow/application/command/workflow_commands_test.go`
- Modify: `backend/internal/modules/product_workflow/module.go`

**Interfaces:**
- Produces app ports `EnvironmentSyncSnapshots` and `EnvironmentSyncTrigger`.
- Consumes preview IDs from run input key `environment_sync_preview_ids[jobName]`.
- Produces project result fields `SourceEnvironmentID`, `SyncImage`, `SyncYAML`, and `FirstSync`.

- [ ] **Step 1: Define ports and write failing start-handler tests**

```go
type EnvironmentSyncSnapshots interface {
	Validate(
		ctx context.Context,
		snapshotID kernel.EnvironmentSyncSnapshotID,
		productID kernel.ProductID,
		workflowID kernel.ProductWorkflowID,
		jobName string,
	) error
	BindMany(ctx context.Context, bindings []EnvironmentSyncSnapshotBinding) error
	LoadBound(ctx context.Context, snapshotID kernel.EnvironmentSyncSnapshotID, runID kernel.ProductWorkflowRunID) (*EnvironmentSyncSnapshot, error)
}

type EnvironmentSyncTrigger interface {
	TriggerEnvironmentSync(ctx context.Context, snapshotID kernel.EnvironmentSyncSnapshotID, projectID kernel.ProjectID) (kernel.DeploymentRunID, error)
}
```

Test that a workflow containing two environment-sync jobs requires one valid preview ID per selected job and rejects a snapshot whose workflow/job/options do not match. Add a second test proving that a Git/cron/webhook launch without confirmed previews returns `ErrInvalidWorkflowInput`; the task must never silently capture live state for an automatic trigger.

- [ ] **Step 2: Run start-handler tests and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/product_workflow/application/command -run EnvironmentSyncPreview -count=1
```

Expected: compile/test failure because the port is not wired.

- [ ] **Step 3: Bind snapshots before saving the run**

After `PrepareRun` creates `runID`, inspect only stages selected by `stage_indices`. For each selected `sparkops-environment-sync` job, parse:

```go
previewIDs, _ := stored["environment_sync_preview_ids"].(map[string]any)
rawID, _ := previewIDs[job.Name].(string)
```

Call `Validate` for every snapshot before writing. Then use `postgres.Transactor` to execute `SaveRun` and one atomic `BindMany` in the same database transaction. If validation, run persistence, or any binding fails, roll back all writes and return `ErrInvalidWorkflowInput`; do not start the scheduler. Start the scheduler only after the transaction commits.

- [ ] **Step 4: Write failing executor tests**

```go
func TestEnvironmentSyncExecutorContinuesAfterProjectFailure(t *testing.T) {
	// Three captured projects: p1 succeeds to trigger, p2 conflicts,
	// p3 succeeds. Assert three result rows, two running and one failed.
}

func TestEnvironmentSyncJobFailsAfterAllProjectsFinishWhenAnyFailed(t *testing.T) {
	// Completion of successful children does not erase the failed child.
}
```

- [ ] **Step 5: Implement executor and scheduler integration**

```go
type EnvironmentSyncExecutor struct {
	snapshots appctx.EnvironmentSyncSnapshots
	trigger   appctx.EnvironmentSyncTrigger
	repo      repository.ProductWorkflowRepository
}
```

`RunJob` loads the bound snapshot, attempts every project, appends one `ProjectDeployRun` per project, saves a `WorkflowDeployRef` for each started deployment, and returns no early error after a project-level failure. Set the job to:

- `running` while any child is running;
- `failed` after all children are terminal and at least one failed;
- `success` only when every project succeeded.

Add the executor to `Scheduler`, its constructor, `executeJob`, stuck-run recovery, and deployment-completion handling.

- [ ] **Step 6: Extend project result JSON**

```go
type ProjectDeployRun struct {
	// existing fields...
	SourceEnvironmentID string `json:"source_environment_id,omitempty"`
	SyncImage           bool   `json:"sync_image,omitempty"`
	SyncYAML            bool   `json:"sync_yaml,omitempty"`
	FirstSync           bool   `json:"first_sync,omitempty"`
}
```

- [ ] **Step 7: Wire delivery ports and product-workflow module**

Delivery provides adapters backed by the snapshot repository and create command. Product workflow requires both ports at module registration and constructs `NewEnvironmentSyncExecutor`.

- [ ] **Step 8: Run workflow tests**

Run:

```powershell
cd backend
go test ./internal/modules/product_workflow/... -run EnvironmentSync -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```powershell
git add backend/internal/platform/appctx/ports.go backend/internal/platform/appctx/container.go backend/internal/modules/delivery/module.go backend/internal/modules/product_workflow/domain/engine/environment_sync_executor.go backend/internal/modules/product_workflow/domain/engine/environment_sync_executor_test.go backend/internal/modules/product_workflow/domain/model/product_workflow.go backend/internal/modules/product_workflow/domain/engine/scheduler.go backend/internal/modules/product_workflow/domain/engine/scheduler_build_completed_test.go backend/internal/modules/product_workflow/application/command/workflow_commands.go backend/internal/modules/product_workflow/application/command/workflow_commands_test.go backend/internal/modules/product_workflow/module.go
git commit -m "feat(workflow): execute environment sync jobs"
```

---

### Task 7: Add the workflow editor configuration drawer

**Files:**
- Create: `fronted/apps/web/src/modules/product/components/EnvironmentSyncJobConfigDrawer.tsx`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-job-validation.ts`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-job-validation.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/ProductWorkflowEditor.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-stage-validation.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-stage-validation.test.ts`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes `EnvironmentVM[]`, `ProductProjectVM[]`, and `EnvironmentSyncJobSpec`.
- Produces a normalized job spec for `ProductWorkflowEditor`.

- [ ] **Step 1: Write failing validation tests**

```ts
it("requires distinct environments and one project", () => {
  expect(validateEnvironmentSyncJob({
    sourceEnvironmentId: "env",
    targetEnvironmentId: "env",
    projects: [],
  })).toEqual({
    sourceEnvironmentId: "源环境和目标环境不能相同",
    projects: "请至少选择一个项目",
  });
});

it("forces first-sync projects to image and yaml", () => {
  expect(normalizeEnvironmentSyncProjects(
    [{ projectId: "p1", syncImage: true, syncYaml: false }],
    new Set<string>(),
  )[0]).toMatchObject({ syncImage: true, syncYaml: true });
});
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/environment-sync-job-validation.test.ts apps/web/src/modules/product/components/workflow-stage-validation.test.ts
```

Expected: missing module/task validation failures.

- [ ] **Step 3: Implement the drawer**

Use the existing `Drawer`, `InlineAlert`, `stack-*`, form and button classes. The drawer must:

- filter source and target to active environments;
- exclude the selected source from target choices;
- fetch both source and target status with `product.getEnvironmentStatus`;
- default-select every enabled project returned by source status;
- render per-project image and YAML checkboxes;
- lock both checked for projects absent from target status;
- disable Save until validation passes;
- expose loading, empty, and error states separately.

Core props:

```ts
interface EnvironmentSyncJobConfigDrawerProps {
  open: boolean;
  workspaceId: string;
  productId: string;
  job: WorkflowJobVM;
  environments: EnvironmentVM[];
  projects: ProductProjectVM[];
  onClose: () => void;
  onConfirm: (job: WorkflowJobVM) => void;
}
```

- [ ] **Step 4: Register the dedicated drawer and save validation**

Add `sparkops-environment-sync` to `hasDedicatedJobConfigDrawer`. Mount the new drawer beside `DeployJobConfigDrawer`. Extend `validateWorkflowStages` to call the new validator and prefix errors with stage/job names.

- [ ] **Step 5: Add restrained styles**

Add only task-specific layout classes for the project list, option row, first-sync badge, loading/empty/error areas, and narrow-screen wrapping. Use existing CSS variables and `150ms ease`; no hard-coded colors.

- [ ] **Step 6: Run tests and typecheck**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/environment-sync-job-validation.test.ts apps/web/src/modules/product/components/workflow-stage-validation.test.ts apps/web/src/modules/product/components/workflow-editor-utils.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add fronted/apps/web/src/modules/product/components/EnvironmentSyncJobConfigDrawer.tsx fronted/apps/web/src/modules/product/components/environment-sync-job-validation.ts fronted/apps/web/src/modules/product/components/environment-sync-job-validation.test.ts fronted/apps/web/src/modules/product/components/ProductWorkflowEditor.tsx fronted/apps/web/src/modules/product/components/workflow-stage-validation.ts fronted/apps/web/src/modules/product/components/workflow-stage-validation.test.ts fronted/apps/web/app/globals.css
git commit -m "feat(web): configure environment sync jobs"
```

---

### Task 8: Require runtime preview confirmation

**Files:**
- Modify: `fronted/packages/contracts/src/product/product-workflow.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-run-form.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-run-form.test.ts`
- Create: `fronted/apps/web/src/modules/product/components/EnvironmentSyncRunFields.tsx`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-run-state.ts`
- Create: `fronted/apps/web/src/modules/product/components/environment-sync-run-state.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/SprintWorkItemExecuteModal.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/release-plan-workflow-params.ts`
- Modify: `fronted/apps/web/src/modules/product/components/release-plan-workflow-params.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/ReleasePlanJobEditor.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Produces run input `environment_sync_preview_ids: Record<string, string>`.
- Produces `RunFormEnvironmentSyncJob`.
- Calls `product.createEnvironmentSyncPreview`.

- [ ] **Step 1: Add failing run-form and confirmation-state tests**

```ts
it("maps environment sync jobs into the run form", () => {
  const form = buildWorkflowRunForm(stages, projects, []);
  expect(form.stages[0].environmentSyncJobs[0]).toMatchObject({
    jobName: "promote",
    sourceEnvironmentId: "dev",
    targetEnvironmentId: "prod",
  });
});

it("invalidates confirmation when any option changes", () => {
  const confirmed = confirmEnvironmentSyncPreview(state, preview);
  expect(updateEnvironmentSyncProject(confirmed, "p1", { syncYaml: true }).previewId).toBe("");
});
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/workflow-run-form.test.ts apps/web/src/modules/product/components/environment-sync-run-state.test.ts
```

Expected: missing environment-sync form/state failures.

- [ ] **Step 3: Extend the run input contract**

```ts
environment_sync_preview_ids: z.record(z.string()).optional(),
```

No raw YAML or unconfirmed override is sent in start input; the immutable preview is the effective override.

- [ ] **Step 4: Implement runtime state and fields**

`EnvironmentSyncRunFields` must allow source, target, project and option overrides; every mutation clears `previewId`. Its “生成同步预览” action calls the SDK, then renders:

- image before/after;
- resource create/update/unchanged/excluded groups;
- first-sync warning;
- explicit “不会删除目标多余资源” and Secret exclusion messages.

The confirm action stores `preview.preview_id`. Keep the confirmed summary visible and provide “重新预览”.

- [ ] **Step 5: Gate workflow submission**

In `StartWorkflowRunModal`, compute selected environment-sync jobs after stage filtering. Disable Submit if any selected job lacks a preview ID. Submit:

```ts
onSubmit({
  // existing fields...
  environment_sync_preview_ids: Object.fromEntries(
    selectedEnvironmentSyncJobs.map((job) => [job.jobName, state[job.jobName].previewId]),
  ),
});
```

Skipped stages do not require a preview.

- [ ] **Step 6: Cover release-plan and sprint launch surfaces**

Reuse `EnvironmentSyncRunFields` in `SprintWorkItemExecuteModal`. Extend `release-plan-workflow-params.ts` so a workflow job containing environment sync produces a required preview parameter keyed by job name, and render the same preview control in `ReleasePlanJobEditor`. Persist only the resulting `preview_id` in release-plan parameters; execution passes it through `environment_sync_preview_ids`.

Add tests proving a release-plan workflow with one sync job reports one missing required preview and becomes valid after the preview ID is supplied.

- [ ] **Step 7: Run tests and typecheck**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/workflow-run-form.test.ts apps/web/src/modules/product/components/environment-sync-run-state.test.ts apps/web/src/modules/product/components/release-plan-workflow-params.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add fronted/packages/contracts/src/product/product-workflow.ts fronted/apps/web/src/modules/product/components/workflow-run-form.ts fronted/apps/web/src/modules/product/components/workflow-run-form.test.ts fronted/apps/web/src/modules/product/components/EnvironmentSyncRunFields.tsx fronted/apps/web/src/modules/product/components/environment-sync-run-state.ts fronted/apps/web/src/modules/product/components/environment-sync-run-state.test.ts fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx fronted/apps/web/src/modules/product/components/SprintWorkItemExecuteModal.tsx fronted/apps/web/src/modules/product/components/release-plan-workflow-params.ts fronted/apps/web/src/modules/product/components/release-plan-workflow-params.test.ts fronted/apps/web/src/modules/product/components/ReleasePlanJobEditor.tsx fronted/apps/web/app/globals.css
git commit -m "feat(web): confirm environment sync previews"
```

---

### Task 9: Surface per-project results and verify the full slice

**Files:**
- Modify: `fronted/packages/contracts/src/product/product-workflow.ts`
- Modify: `fronted/packages/contracts/src/delivery/index.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-run-canvas-utils.ts`
- Modify: `fronted/apps/web/src/modules/product/components/workflow-run-canvas-utils.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/WorkflowRunTaskPanel.tsx`
- Create: `backend/internal/modules/product_workflow/application/environment_sync_integration_test.go`
- Modify: `backend/internal/modules/product/application/subscriber/on_deploy_completed_test.go`

**Interfaces:**
- Consumes extended `ProjectDeployRun` and `DeploymentRunVM` provenance.
- Produces one selectable deploy task per synchronized project.

- [ ] **Step 1: Write failing canvas/result tests**

```ts
it("renders every environment sync project as a deploy task", () => {
  const tasks = workflowRunToPipelineTasks(run);
  expect(tasks.filter((task) => task.jobType === "sparkops-environment-sync")).toMatchObject([
    { projectId: "p1", kind: "deploy", sourceEnvironmentId: "dev", environmentId: "prod" },
    { projectId: "p2", kind: "deploy", status: "failed" },
  ]);
});
```

- [ ] **Step 2: Run the frontend test and verify RED**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/product/components/workflow-run-canvas-utils.test.ts
```

Expected: missing task fields/mapping failure.

- [ ] **Step 3: Extend contracts and task detail**

Add optional source environment, sync flags, first-sync and snapshot fields to the VM schemas. In `WorkflowRunTaskPanel`, environment-sync deploys show:

- source → target environment;
- image/YAML scopes;
- first-sync marker;
- image reference;
- deployment link, live log, and resource-level error.

Use existing metadata rows and status classes; do not add a second primary action.

- [ ] **Step 4: Add backend integration coverage**

```go
func TestEnvironmentSyncWorkflowContinuesProjectsAndAssociatesOnlySuccess(t *testing.T) {
	// Create source/target envs and two projects.
	// Bind a preview where one trigger succeeds and one conflicts.
	// Assert both were attempted, workflow job failed after completion,
	// successful first-sync project was upserted into environment_projects,
	// failed project was not associated.
}
```

Also extend `OnDeployCompleted` tests to assert an `environment-sync` success upserts a missing environment project and stale events do not overwrite newer state.

- [ ] **Step 5: Run focused tests**

Run:

```powershell
cd backend
go test ./internal/modules/product_workflow/application ./internal/modules/product/application/subscriber -run EnvironmentSync -count=1
cd ../fronted
pnpm vitest run apps/web/src/modules/product/components/workflow-run-canvas-utils.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: PASS.

- [ ] **Step 6: Run regression suites**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/deployrun/... ./internal/modules/product_workflow/... ./internal/modules/product/...
cd ../fronted
pnpm vitest run apps/web/src/modules/product/components
pnpm --filter @sparkops/web typecheck
```

Expected: all commands exit 0.

- [ ] **Step 7: Restart and smoke-test the backend**

Run:

```powershell
cd A:\21-Agent\SparkOpsAgent
.\scripts\dev-restart.ps1 -Backend
```

Expected: backend PID changes and `http://localhost:8080` responds (a root 404 is acceptable).

Manual smoke test:

1. Add “环境部署同步” under the workflow Deploy category.
2. Choose source/target and deselect YAML for an existing target project.
3. Confirm a missing target project forces image + YAML.
4. Start the workflow only after preview confirmation.
5. Verify per-project deployment links and aggregate failure behavior.
6. Verify no target-only resource was deleted and no Secret data was copied.

- [ ] **Step 8: Commit**

```powershell
git add fronted/packages/contracts/src/product/product-workflow.ts fronted/packages/contracts/src/delivery/index.ts fronted/apps/web/src/modules/product/components/workflow-run-canvas-utils.ts fronted/apps/web/src/modules/product/components/workflow-run-canvas-utils.test.ts fronted/apps/web/src/modules/product/components/WorkflowRunTaskPanel.tsx backend/internal/modules/product_workflow/application/environment_sync_integration_test.go backend/internal/modules/product/application/subscriber/on_deploy_completed_test.go
git commit -m "feat(workflow): expose environment sync results"
```

---

## Final Verification

- [ ] Confirm every requirement in `docs/superpowers/specs/2026-10-18-environment-deployment-sync-task-design.md` maps to Tasks 1–9.
- [ ] Run `git diff --check`.
- [ ] Run backend regression tests from Task 9.
- [ ] Run frontend Vitest and typecheck from Task 9.
- [ ] Confirm the backend restart succeeded.
- [ ] Inspect `git status --short` and verify unrelated pre-existing changes remain untouched.
