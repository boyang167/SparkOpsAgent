# 项目环境下线与归档联动 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为项目提供按集群「下线」与「下线全部环境并归档」能力，下线作为 `DeploymentRun` 的 `undeploy` 类型执行。

**Architecture:** 后端在 delivery/deployrun 新增 `RunTypeUndeploy`，编排器在 `deploying` 阶段调用 runtime 新端口 `ManifestRemover` 删除 K8s 资源；delivery 通过 `appctx.ProjectTeardown` 向 workspace_app 暴露在线集群查询 / 发起下线 / 禁用 Jenkins；workspace_app 新增项目状态 `archiving`，由 `ArchiveCoordinator` 订阅 `delivery.DeployCompleted` 串行推进归档。前端在环境状态行与详情页提供下线入口，并以 `ArchiveProjectModal` 替换旧确认框。

**Tech Stack:** Go 1.x + gin + gorm + client-go；Next.js + React + zod + vitest；pnpm/turbo monorepo。

**Spec:** `docs/superpowers/specs/2026-10-11-project-undeploy-archive-design.md`

## Global Constraints

- 下线删除 Deployment / Service / Ingress；ConfigMap / Secret / PVC 默认保留，`delete_config` / `delete_storage` 勾选才删；**不删 Namespace，不删 imagePullSecret**。
- 「在线」= 某集群最近一次 run `status == succeeded && run_type != undeploy`（前后端同规则）。
- 下线与归档请求必须携带 `confirm_name`，与项目 `name` 完全一致。
- 同一项目同一时刻只允许一个活跃 run（沿用 `HasActiveByProject` + 部分唯一索引）。
- 归档：禁用 Jenkins Job（`POST /job/<folder>/job/<name>/disable`，404 视为成功），Harbor 不动；禁用失败只记 warning，不阻塞归档。
- 前端禁止 `window.confirm`；危险操作用 `Modal`，样式只用现有 CSS 类与设计 token。
- 后端修改后执行 `.\scripts\dev-restart.ps1 -Backend`（仓库规则）。
- Go 测试命令在 `backend/` 下执行：`go test ./internal/... -run <Name> -count=1`。
- 前端测试命令在 `fronted/` 下执行：`pnpm vitest run <path>`；类型检查：`pnpm --filter web typecheck`（若 filter 名不同，以 `fronted/apps/web/package.json` 的 `name` 为准）。

## 已确认的偏差（相对 spec 的收敛）

- 归档中下线失败时「发通知」收敛为：项目恢复 `active` + `zap` warning 日志 + 失败原因留在 undeploy run 的 `error_message`（归档弹窗与部署历史可见）。不新建通知通道。
- 出站 webhook 不新增事件类型：undeploy 仍以 `deploy` 事件发出，`data.run_type = "undeploy"` 已可区分。
- 「服务重启续推」由 workspace_app 内的定时器调用 `ArchiveCoordinator.ResumeAll` 实现，周期复用 `Config.Deploy.ReconcileInterval`。

## 文件结构

后端（新增 ★ / 修改 ✎）：

| 文件 | 职责 |
|---|---|
| ✎ `backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go` | `RunTypeUndeploy`、`UndeployOptions`、`NewUndeployRun`、`IsUndeploy` |
| ★ `backend/internal/modules/delivery/deployrun/domain/model/online.go` | `IsOnlineRun` / `LatestRunByCluster` / `OnlineClusters` |
| ✎ `backend/internal/modules/delivery/deployrun/domain/errors.go` | 新错误码 |
| ✎ `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go` | `undeploy_options` 列映射 |
| ★ `backend/migrations/000063_deployment_undeploy.{up,down}.sql` | 加列 |
| ✎ `backend/internal/modules/runtime/domain/port/ports.go` | `RemoveSpec` / `ManifestRemover` |
| ★ `backend/internal/modules/runtime/infrastructure/k8s/remover.go` | K8s 删除实现 |
| ✎ `backend/internal/modules/runtime/infrastructure/k8s/mock_deployer.go` | mock 实现 `Remove` |
| ✎ `backend/internal/modules/delivery/deployrun/application/service/orchestrator.go` | undeploy 分支 |
| ★ `backend/internal/modules/delivery/deployrun/domain/port/project_lifecycle.go` | `ProjectLifecycleReader` |
| ✎ `backend/internal/modules/delivery/deployrun/infrastructure/acl/workspace_app_acl.go` | 实现 `ProjectLifecycle` |
| ★ `backend/internal/modules/delivery/deployrun/application/command/undeploy.go` | `UndeployHandler` |
| ✎ `backend/internal/modules/delivery/deployrun/application/command/create_deployment.go` / `rollback_deployment.go` | archiving 拒绝、undeploy 过滤 |
| ✎ `backend/internal/modules/delivery/deployrun/interface/http/handler.go` | `Undeploy` 路由处理 |
| ✎ `backend/internal/modules/delivery/domain/port/ports.go` + `infrastructure/jenkins/{executor,mock_executor}.go` | `JenkinsJobDisabler` |
| ✎ `backend/internal/modules/delivery/deployrun/application/service/prepare.go` | `DisableJob` |
| ✎ `backend/internal/platform/appctx/{ports,container}.go` | `ProjectTeardown` 端口 |
| ★ `backend/internal/modules/delivery/teardown_port.go` | 端口实现 |
| ✎ `backend/internal/modules/delivery/module.go` | 接线 |
| ✎ `backend/internal/modules/delivery/domain/event/deploy_completed.go` | `CreatedBy` 字段 |
| ✎ `backend/internal/modules/workspace_app/domain/{model/project.go,errors.go}` | `archiving` 状态 |
| ✎ `backend/internal/modules/workspace_app/domain/repository/project_repository.go` + `infrastructure/persistence/project_repo.go` | 列表包含 archiving、`ListByStatus` |
| ✎ `backend/internal/modules/workspace_app/application/query/project_queries.go` | Get 允许 archiving |
| ✎ `backend/internal/modules/workspace_app/application/command/archive_project.go` | 新归档流程 |
| ★ `backend/internal/modules/workspace_app/application/command/archive_coordinator.go` | 订阅推进 + 续推 |
| ✎ `backend/internal/modules/workspace_app/{interface/http/handler.go,module.go}` | 路由与接线 |
| ✎ `backend/internal/modules/product/application/subscriber/on_deploy_completed.go` | undeploy 清空当前版本 |
| ✎ `backend/internal/platform/httpx/errors.go` | HTTP 状态映射 |

前端：

| 文件 | 职责 |
|---|---|
| ✎ `fronted/packages/contracts/src/delivery/index.ts` | `undeploy` run_type、`UndeployCommandSchema`、`deploy_namespace` |
| ✎ `fronted/packages/contracts/src/workspace/index.ts` | `ArchiveProjectCommandSchema`、`ArchiveProjectResultVM`、状态选项 |
| ✎ `fronted/packages/upstream/src/{delivery,workspace}.ts` | 上游调用 |
| ✎ `fronted/packages/bff-modules/src/{delivery/index.ts,workspace/*}` | BFF 透传 |
| ★ `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/undeployments/route.ts` | BFF 路由 |
| ★ `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/archive/route.ts` | BFF 路由 |
| ✎ `fronted/packages/web-sdk/src/{delivery,workspace}.ts` | SDK |
| ✎ `fronted/apps/web/src/modules/workspace/components/project/project-environment-status.ts` (+test) | 在线判定与文案 |
| ✎ `.../project/deployment-console-utils.ts` (+test) | 「下线」标题 |
| ✎ `.../project/useProjectRunSummary.ts` | `reload()`、统计排除 undeploy |
| ✎ `.../project/ProjectEnvironmentStatus.tsx` | hover 下线按钮 |
| ★ `.../project/UndeployModal.tsx` | 下线弹窗 |
| ★ `.../project/ArchiveProjectModal.tsx` | 归档弹窗 |
| ★ `.../project/archive-progress.ts` (+test) | 归档进度纯函数 |
| ✎ `.../project/{ProjectOverviewCard,ProjectCard,ProjectDeploymentConsolePanel,ProjectServiceDetailPage}.tsx` | 接线 |
| ✎ `.../components/{ProjectPanel,ProjectSettings}.tsx` | 替换归档确认 |
| ✎ `fronted/apps/web/app/globals.css` | 少量样式 |

---

### Task 1: 领域模型 —— undeploy 类型与在线判定

**Files:**
- Modify: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/model/online.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/command/rollback_deployment.go:90-99`
- Modify: `backend/internal/modules/delivery/deployrun/application/command/create_deployment.go:126-131`
- Test: `backend/internal/modules/delivery/deployrun/domain/model/undeploy_test.go`
- Test: `backend/internal/modules/delivery/deployrun/application/command/undeploy_filters_test.go`

**Interfaces:**
- Produces:
  - `const RunTypeUndeploy RunType = "undeploy"`
  - `type UndeployOptions struct { DeleteConfig bool \`json:"delete_config"\`; DeleteStorage bool \`json:"delete_storage"\` }`
  - `DeploymentRun.UndeployOptions *UndeployOptions`
  - `func NewUndeployRun(wsID kernel.WorkspaceID, projectID kernel.ProjectID, createdBy kernel.MemberID, baseline *DeploymentRun, opts UndeployOptions) *DeploymentRun`
  - `func (r *DeploymentRun) IsUndeploy() bool`
  - `func IsOnlineRun(run *DeploymentRun) bool`
  - `func LatestRunByCluster(runs []*DeploymentRun) map[kernel.ClusterInfoID]*DeploymentRun`（`runs` 必须按 `created_at DESC`）
  - `func OnlineClusters(runs []*DeploymentRun) []kernel.ClusterInfoID`（按 runs 出现顺序）

- [ ] **Step 1: 写失败测试**

`backend/internal/modules/delivery/deployrun/domain/model/undeploy_test.go`：

```go
package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func succeededRun(projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) *model.DeploymentRun {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	run.Status = model.StatusSucceeded
	return run
}

func TestNewUndeployRunCopiesBaselineTarget(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	baseline := succeededRun(kernel.NewProjectID(), kernel.NewClusterInfoID())
	baseline.EnvironmentID = &envID
	baseline.DeployNamespace = "team-a-dev"
	baseline.ImageRepository = "registry.example/team-a/orders"
	actor := kernel.NewMemberID()

	run := model.NewUndeployRun(baseline.WorkspaceID, baseline.ProjectID, actor, baseline, model.UndeployOptions{DeleteConfig: true})

	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, model.StatusQueued, run.Status)
	require.Equal(t, baseline.ClusterID, run.ClusterID)
	require.Equal(t, &envID, run.EnvironmentID)
	require.Equal(t, "team-a-dev", run.DeployNamespace)
	require.Equal(t, "v1", run.ImageTag)
	require.Equal(t, "registry.example/team-a/orders", run.ImageRepository)
	require.Equal(t, actor, run.CreatedBy)
	require.False(t, run.CrossCluster)
	require.NotEqual(t, baseline.ID, run.ID)
	require.Equal(t, &model.UndeployOptions{DeleteConfig: true}, run.UndeployOptions)
	require.True(t, run.IsUndeploy())
	require.True(t, run.SkipsBuild())
}

func TestOnlineClustersUsesLatestRunPerCluster(t *testing.T) {
	projectID := kernel.NewProjectID()
	dev := kernel.NewClusterInfoID()
	uat := kernel.NewClusterInfoID()
	prod := kernel.NewClusterInfoID()

	devUndeploy := model.NewUndeployRun(kernel.NewWorkspaceID(), projectID, kernel.NewMemberID(), succeededRun(projectID, dev), model.UndeployOptions{})
	devUndeploy.Status = model.StatusSucceeded
	uatFailed := succeededRun(projectID, uat)
	uatFailed.Status = model.StatusFailed

	runs := []*model.DeploymentRun{
		devUndeploy,                   // newest: dev offline
		succeededRun(projectID, prod), // prod online
		uatFailed,                     // uat latest failed
		succeededRun(projectID, dev),  // older dev success ignored
		succeededRun(projectID, uat),  // older uat success ignored
	}

	require.Equal(t, []kernel.ClusterInfoID{prod}, model.OnlineClusters(runs))
	require.Len(t, model.LatestRunByCluster(runs), 3)
	require.False(t, model.IsOnlineRun(devUndeploy))
	require.False(t, model.IsOnlineRun(nil))
}
```

`backend/internal/modules/delivery/deployrun/application/command/undeploy_filters_test.go`：

```go
package command

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestResolveRollbackTargetSkipsUndeployRuns(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	clusterID := kernel.NewClusterInfoID()
	older := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	older.Status = model.StatusSucceeded
	newer := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v2")
	newer.Status = model.StatusSucceeded
	undeploy := model.NewUndeployRun(wsID, projectID, kernel.NewMemberID(), newer, model.UndeployOptions{})
	undeploy.Status = model.StatusSucceeded

	current, target, err := ResolveRollbackTarget([]*model.DeploymentRun{undeploy, newer, older}, wsID, projectID, "")

	require.NoError(t, err)
	require.Equal(t, newer.ID, current.ID)
	require.Equal(t, older.ID, target.ID)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/domain/model/ ./internal/modules/delivery/deployrun/application/command/ -run "Undeploy|OnlineClusters|SkipsUndeploy" -count=1`
Expected: 编译失败 `undefined: model.NewUndeployRun`。

- [ ] **Step 3: 实现**

`deployment_run.go` —— 在 `RunTypeSync` 后追加常量，并在 `DeploymentRun` 结构体中 `RollbackOfRunID` 下方加字段：

```go
const (
	RunTypeNormal   RunType = "normal"
	RunTypeRollback RunType = "rollback"
	RunTypeSync     RunType = "sync"
	RunTypeUndeploy RunType = "undeploy"
)

type UndeployOptions struct {
	DeleteConfig  bool `json:"delete_config"`
	DeleteStorage bool `json:"delete_storage"`
}
```

```go
	RollbackOfRunID *kernel.DeploymentRunID
	UndeployOptions *UndeployOptions
```

在 `NewSyncDeploymentRun` 之后追加：

```go
func NewUndeployRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	createdBy kernel.MemberID,
	baseline *DeploymentRun,
	opts UndeployOptions,
) *DeploymentRun {
	options := opts
	return &DeploymentRun{
		ID:              kernel.NewDeploymentRunID(),
		WorkspaceID:     wsID,
		ProjectID:       projectID,
		ClusterID:       baseline.ClusterID,
		EnvironmentID:   baseline.EnvironmentID,
		DeployNamespace: baseline.DeployNamespace,
		CreatedBy:       createdBy,
		GitRefType:      baseline.GitRefType,
		GitRefValue:     baseline.GitRefValue,
		ImageTag:        baseline.ImageTag,
		ImageRepository: baseline.ImageRepository,
		Status:          StatusQueued,
		RunType:         RunTypeUndeploy,
		UndeployOptions: &options,
		CreatedAt:       time.Now().UTC(),
	}
}
```

替换 `SkipsBuild` 并新增 `IsUndeploy`：

```go
func (r *DeploymentRun) IsUndeploy() bool {
	return r.RunType == RunTypeUndeploy
}

func (r *DeploymentRun) SkipsBuild() bool {
	return r.IsRollback() || r.IsSyncOnly() || r.IsUndeploy()
}
```

新建 `online.go`：

```go
package model

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

func IsOnlineRun(run *DeploymentRun) bool {
	return run != nil && run.Status == StatusSucceeded && !run.IsUndeploy()
}

// LatestRunByCluster expects runs ordered by created_at DESC.
func LatestRunByCluster(runs []*DeploymentRun) map[kernel.ClusterInfoID]*DeploymentRun {
	latest := make(map[kernel.ClusterInfoID]*DeploymentRun, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		if _, seen := latest[run.ClusterID]; seen {
			continue
		}
		latest[run.ClusterID] = run
	}
	return latest
}

// OnlineClusters expects runs ordered by created_at DESC.
func OnlineClusters(runs []*DeploymentRun) []kernel.ClusterInfoID {
	seen := make(map[kernel.ClusterInfoID]struct{}, len(runs))
	online := make([]kernel.ClusterInfoID, 0)
	for _, run := range runs {
		if run == nil {
			continue
		}
		if _, ok := seen[run.ClusterID]; ok {
			continue
		}
		seen[run.ClusterID] = struct{}{}
		if IsOnlineRun(run) {
			online = append(online, run.ClusterID)
		}
	}
	return online
}
```

`rollback_deployment.go` `ResolveRollbackTarget` 循环内，在 `run.Status != model.StatusSucceeded` 判断前加：

```go
		if run.IsUndeploy() {
			continue
		}
```

`create_deployment.go` 同步源选择（约 127 行）改为：

```go
				if item.Status == model.StatusSucceeded && !item.IsUndeploy() && strings.TrimSpace(item.ImageTag) != "" {
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/... -count=1`
Expected: PASS（含既有测试）。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/delivery/deployrun/domain/model backend/internal/modules/delivery/deployrun/application/command
git commit -m "feat(delivery): add undeploy run type and online cluster rules"
```

---

### Task 2: 持久化 —— `undeploy_options` 列与视图

**Files:**
- Create: `backend/migrations/000063_deployment_undeploy.up.sql`、`backend/migrations/000063_deployment_undeploy.down.sql`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/query/deployment_queries.go`（`DeploymentView`、`ToDeploymentView`）
- Test: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo_test.go`

**Interfaces:**
- Consumes: Task 1 `UndeployOptions`、`IsUndeploy()`
- Produces: `DeploymentView.UndeployOptions *model.UndeployOptions \`json:"undeploy_options,omitempty"\``

- [ ] **Step 1: 基线**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/infrastructure/persistence/ -count=1`
Expected: PASS。若因 `newRunRepo` 手写表缺 `project_version_id` 列而失败，先在该 `CREATE TABLE` 末尾补 `project_version_id TEXT`，再继续。

- [ ] **Step 2: 写失败测试**

在 `newRunRepo` 的 `CREATE TABLE delivery.deployment_runs (...)` 列表末尾追加 `, undeploy_options TEXT`（与 `project_version_id TEXT` 同级）。文件末尾追加：

```go
func TestRepositoryPersistsUndeployOptionsAndQueuedMessage(t *testing.T) {
	repo, _ := newRunRepo(t)
	baseline := newRun(kernel.NewProjectID())
	baseline.Status = model.StatusSucceeded
	run := model.NewUndeployRun(baseline.WorkspaceID, baseline.ProjectID, kernel.NewMemberID(), baseline,
		model.UndeployOptions{DeleteConfig: true, DeleteStorage: true})
	run.CreatedAt = baseline.CreatedAt

	require.NoError(t, repo.Create(context.Background(), run))
	found, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, found.RunType)
	require.Equal(t, &model.UndeployOptions{DeleteConfig: true, DeleteStorage: true}, found.UndeployOptions)

	events, err := repo.ListEvents(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, "undeploy queued", events[0].Message)
}
```

- [ ] **Step 3: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/infrastructure/persistence/ -run Undeploy -count=1`
Expected: FAIL（`UndeployOptions` 为 nil / message 为 `deployment queued`）。

- [ ] **Step 4: 实现**

迁移 up：

```sql
ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS undeploy_options JSONB NULL;
```

迁移 down：

```sql
ALTER TABLE delivery.deployment_runs
    DROP COLUMN IF EXISTS undeploy_options;
```

`deploymentRunRow` 在 `DeployNamespace` 后加：

```go
	UndeployOptions []byte `gorm:"column:undeploy_options;type:jsonb"`
```

新增辅助函数（文件末尾）：

```go
func runMessagePrefix(run *model.DeploymentRun) string {
	switch {
	case run.IsRollback():
		return "rollback"
	case run.IsUndeploy():
		return "undeploy"
	default:
		return "deployment"
	}
}

func undeployOptionsValue(opts *model.UndeployOptions) []byte {
	if opts == nil {
		return nil
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return nil
	}
	return raw
}
```

`Create` 中替换 message 计算：

```go
	message := runMessagePrefix(run) + " queued"
```

`Update` 中替换：

```go
	message := runMessagePrefix(run) + " " + string(run.Status)
```

`toRunRow` 返回结构体中追加 `UndeployOptions: undeployOptionsValue(run.UndeployOptions),`。

`fromRunRow` 在 `SyncManifests` 解码后追加：

```go
	if len(row.UndeployOptions) > 0 && string(row.UndeployOptions) != "null" {
		var opts model.UndeployOptions
		if err := json.Unmarshal(row.UndeployOptions, &opts); err == nil {
			run.UndeployOptions = &opts
		}
	}
```

`deployment_queries.go`：`DeploymentView` 在 `RollbackOfRunID` 后加 `UndeployOptions *model.UndeployOptions \`json:"undeploy_options,omitempty"\``；`ToDeploymentView` 构造 `view` 时加 `UndeployOptions: run.UndeployOptions,`。

- [ ] **Step 5: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/... -count=1`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/000063_deployment_undeploy.up.sql backend/migrations/000063_deployment_undeploy.down.sql backend/internal/modules/delivery/deployrun
git commit -m "feat(delivery): persist undeploy options on deployment runs"
```

---

### Task 3: runtime `ManifestRemover` 端口与 K8s 实现

**Files:**
- Modify: `backend/internal/modules/runtime/domain/port/ports.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/remover.go`
- Modify: `backend/internal/modules/runtime/infrastructure/k8s/mock_deployer.go`
- Test: `backend/internal/modules/runtime/infrastructure/k8s/remover_test.go`

**Interfaces:**
- Produces:

```go
type RemoveSpec struct {
	Manifests     []Manifest
	AppLabel      string
	DeleteConfig  bool
	DeleteStorage bool
}

type ManifestRemover interface {
	Remove(ctx context.Context, target DeploymentTarget, spec RemoveSpec) error
}
```

  - `(*k8s.ManifestDeployer).Remove` 与 `(*k8s.MockManifestDeployer).Remove` 实现该接口。

- [ ] **Step 1: 写失败测试**

`remover_test.go`：

```go
package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

const removerNS = "team-a"

func removerLabels() map[string]string {
	return map[string]string{"app": "orders", "app.kubernetes.io/managed-by": "sparkops"}
}

func removerFixture() *fake.Clientset {
	return fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: removerNS, Labels: removerLabels()},
			Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "orders-data"},
				}}},
			}}},
		},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: removerNS}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "orders-ing", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "orders-config", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "orders-secret", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "harbor-pull", Namespace: removerNS}, Type: corev1.SecretTypeDockerConfigJson},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "orders-data", Namespace: removerNS}},
	)
}

func removerTarget() port.DeploymentTarget {
	return port.DeploymentTarget{Kubeconfig: "test", Namespace: removerNS}
}

func removerManifests() []port.Manifest {
	return []port.Manifest{
		{YAML: "apiVersion: v1\nkind: Service\nmetadata:\n  name: orders\n"},
		{YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: orders\n"},
	}
}

func TestRemoveDeletesWorkloadsAndKeepsConfigByDefault(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders",
	})

	require.NoError(t, err)
	ctx := context.Background()
	_, err = client.AppsV1().Deployments(removerNS).Get(ctx, "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Services(removerNS).Get(ctx, "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.NetworkingV1().Ingresses(removerNS).Get(ctx, "orders-ing", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().ConfigMaps(removerNS).Get(ctx, "orders-config", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().PersistentVolumeClaims(removerNS).Get(ctx, "orders-data", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().Namespaces().Get(ctx, removerNS, metav1.GetOptions{})
	require.True(t, err == nil || apierrors.IsNotFound(err))
}

func TestRemoveDeletesConfigAndStorageWhenRequested(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders", DeleteConfig: true, DeleteStorage: true,
	})

	require.NoError(t, err)
	ctx := context.Background()
	_, err = client.CoreV1().ConfigMaps(removerNS).Get(ctx, "orders-config", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Secrets(removerNS).Get(ctx, "orders-secret", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Secrets(removerNS).Get(ctx, "harbor-pull", metav1.GetOptions{})
	require.NoError(t, err, "image pull secret must be kept")
	_, err = client.CoreV1().PersistentVolumeClaims(removerNS).Get(ctx, "orders-data", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestRemoveIsIdempotentWhenResourcesMissing(t *testing.T) {
	remover := NewManifestDeployer(nil, fake.NewSimpleClientset(), 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders", DeleteConfig: true, DeleteStorage: true,
	})

	require.NoError(t, err)
}

func TestRemoveFallsBackToAppLabelWhenManifestsEmpty(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	require.NoError(t, remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{AppLabel: "orders"}))
	_, err := client.AppsV1().Deployments(removerNS).Get(context.Background(), "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/runtime/infrastructure/k8s/ -run TestRemove -count=1`
Expected: 编译失败 `remover.Remove undefined`。

- [ ] **Step 3: 实现**

`ports.go` 末尾追加 Interfaces 中的 `RemoveSpec` 与 `ManifestRemover`。

`remover.go`：

```go
package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

const managedBySelector = "app.kubernetes.io/managed-by=sparkops"

type manifestRef struct {
	kind string
	name string
}

func (d *ManifestDeployer) Remove(ctx context.Context, target port.DeploymentTarget, spec port.RemoveSpec) error {
	if len(strings.TrimSpace(target.Kubeconfig)) == 0 {
		return domain.ErrKubeconfigMissing
	}
	namespace := strings.TrimSpace(target.Namespace)
	if namespace == "" {
		return domain.ErrManifestInvalid
	}
	client, _, err := d.clients(target)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("kubernetes clients are not configured")
	}
	refs := manifestRefs(spec.Manifests)
	selector := appSelector(spec.AppLabel)
	deployments := namesOf(refs, "Deployment")
	services := namesOf(refs, "Service")
	if selector != "" {
		deployments = appendUniqueStrings(deployments, listDeploymentNames(ctx, client, namespace, selector)...)
		services = appendUniqueStrings(services, listServiceNames(ctx, client, namespace, selector)...)
	}

	var errs []error
	claims := make([]string, 0)
	if spec.DeleteStorage {
		claims = appendUniqueStrings(claims, deploymentClaimNames(ctx, client, namespace, deployments)...)
	}
	policy := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{PropagationPolicy: &policy}
	for _, name := range deployments {
		errs = append(errs, ignoreNotFound("Deployment", name,
			client.AppsV1().Deployments(namespace).Delete(ctx, name, opts)))
	}
	for _, name := range services {
		errs = append(errs, ignoreNotFound("Service", name,
			client.CoreV1().Services(namespace).Delete(ctx, name, opts)))
	}
	if selector != "" {
		ingresses, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		errs = append(errs, err)
		if err == nil {
			for _, item := range ingresses.Items {
				errs = append(errs, ignoreNotFound("Ingress", item.Name,
					client.NetworkingV1().Ingresses(namespace).Delete(ctx, item.Name, opts)))
			}
		}
	}
	if spec.DeleteConfig {
		errs = append(errs, removeConfig(ctx, client, namespace, selector, refs, opts)...)
	}
	if spec.DeleteStorage {
		if selector != "" {
			pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
			errs = append(errs, err)
			if err == nil {
				for _, item := range pvcs.Items {
					claims = appendUniqueStrings(claims, item.Name)
				}
			}
		}
		for _, name := range claims {
			errs = append(errs, ignoreNotFound("PersistentVolumeClaim", name,
				client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, opts)))
		}
	}
	return errors.Join(errs...)
}

func removeConfig(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, selector string,
	refs []manifestRef,
	opts metav1.DeleteOptions,
) []error {
	var errs []error
	configMaps := namesOf(refs, "ConfigMap")
	secrets := namesOf(refs, "Secret")
	if selector != "" {
		if list, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err != nil {
			errs = append(errs, err)
		} else {
			for _, item := range list.Items {
				configMaps = appendUniqueStrings(configMaps, item.Name)
			}
		}
		if list, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err != nil {
			errs = append(errs, err)
		} else {
			for _, item := range list.Items {
				if item.Type == corev1.SecretTypeDockerConfigJson {
					continue
				}
				secrets = appendUniqueStrings(secrets, item.Name)
			}
		}
	}
	for _, name := range configMaps {
		errs = append(errs, ignoreNotFound("ConfigMap", name,
			client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, opts)))
	}
	for _, name := range secrets {
		existing, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && existing.Type == corev1.SecretTypeDockerConfigJson {
			continue
		}
		errs = append(errs, ignoreNotFound("Secret", name,
			client.CoreV1().Secrets(namespace).Delete(ctx, name, opts)))
	}
	return errs
}

func appSelector(appLabel string) string {
	label := strings.TrimSpace(appLabel)
	if label == "" {
		return ""
	}
	return "app=" + label + "," + managedBySelector
}

func manifestRefs(manifests []port.Manifest) []manifestRef {
	refs := make([]manifestRef, 0, len(manifests))
	for _, manifest := range manifests {
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest.YAML), 4096)
		for {
			var raw map[string]any
			if err := decoder.Decode(&raw); err != nil {
				break
			}
			kind, _ := raw["kind"].(string)
			metadata, _ := raw["metadata"].(map[string]any)
			name, _ := metadata["name"].(string)
			if strings.TrimSpace(kind) != "" && strings.TrimSpace(name) != "" {
				refs = append(refs, manifestRef{kind: kind, name: name})
			}
		}
	}
	return refs
}

func namesOf(refs []manifestRef, kind string) []string {
	names := make([]string, 0)
	for _, ref := range refs {
		if ref.kind == kind {
			names = appendUniqueStrings(names, ref.name)
		}
	}
	return names
}

func listDeploymentNames(ctx context.Context, client kubernetes.Interface, namespace, selector string) []string {
	list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return names
}

func listServiceNames(ctx context.Context, client kubernetes.Interface, namespace, selector string) []string {
	list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return names
}

func deploymentClaimNames(ctx context.Context, client kubernetes.Interface, namespace string, deployments []string) []string {
	claims := make([]string, 0)
	for _, name := range deployments {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claims = appendUniqueStrings(claims, volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	return claims
}

func ignoreNotFound(kind, name string, err error) error {
	if err == nil || apierrors.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("delete %s %q: %w", kind, name, err)
}

var _ port.ManifestRemover = (*ManifestDeployer)(nil)
```

> 说明：`appendUniqueStrings` 已存在于 `deployer.go` 同包（`ensureImagePullSecrets` 使用）；若签名不同（实现前 `rg "func appendUniqueStrings" backend/internal/modules/runtime`），按其实际签名 `func(base []string, values ...string) []string` 调用。

`mock_deployer.go` 在 `RolloutRestart` 之后追加：

```go
func (d *MockManifestDeployer) Remove(context.Context, port.DeploymentTarget, port.RemoveSpec) error {
	return nil
}

var _ port.ManifestRemover = (*MockManifestDeployer)(nil)
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/runtime/... -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/runtime
git commit -m "feat(runtime): add manifest remover for undeploy"
```

---

### Task 4: 编排器 undeploy 分支

**Files:**
- Modify: `backend/internal/modules/delivery/deployrun/application/service/orchestrator.go`
- Modify: `backend/internal/modules/delivery/module.go`（`SetManifestRemover` 接线）
- Test: `backend/internal/modules/delivery/deployrun/application/service/orchestrator_undeploy_test.go`

**Interfaces:**
- Consumes: Task 1 `IsUndeploy()`、`UndeployOptions`；Task 3 `runtimeport.ManifestRemover`、`runtimeport.RemoveSpec`
- Produces: `func (o *Orchestrator) SetManifestRemover(remover runtimeport.ManifestRemover)`

- [ ] **Step 1: 写失败测试**

`orchestrator_undeploy_test.go`（同包 `service`，复用 `orchestrator_test.go` 中的 `newOrchestratorFixture` 等 fake）：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

type recordingRemover struct {
	calls  int
	target runtimeport.DeploymentTarget
	spec   runtimeport.RemoveSpec
	err    error
}

func (r *recordingRemover) Remove(_ context.Context, target runtimeport.DeploymentTarget, spec runtimeport.RemoveSpec) error {
	r.calls++
	r.target = target
	r.spec = spec
	return r.err
}

func undeployFixture(t *testing.T, status model.Status) (*orchestratorFixture, *recordingRemover) {
	t.Helper()
	f := newOrchestratorFixture(status)
	baseline := *f.run
	baseline.DeployNamespace = "team-a-dev"
	undeploy := model.NewUndeployRun(f.wsID, f.run.ProjectID, f.run.CreatedBy, &baseline, model.UndeployOptions{DeleteConfig: true})
	undeploy.Status = status
	f.run = undeploy
	f.repo.run = undeploy
	remover := &recordingRemover{}
	f.o.SetManifestRemover(remover)
	return f, remover
}

func TestUndeployQueuedSkipsBuildAndGoesToDeploying(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusQueued)

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusDeploying, f.run.Status)
	require.Zero(t, remover.calls)
	require.Zero(t, f.prepare.calls)
}

func TestUndeployDeployingRemovesResourcesAndSucceeds(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusDeploying)

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, remover.calls)
	require.Equal(t, "team-a-dev", remover.target.Namespace)
	require.Equal(t, "orders", remover.spec.AppLabel)
	require.True(t, remover.spec.DeleteConfig)
	require.False(t, remover.spec.DeleteStorage)
	require.NotEmpty(t, remover.spec.Manifests)
	require.Zero(t, f.deployer.calls)
	require.Zero(t, f.deployer.restartCalls)
}

func TestUndeployRemoverFailureFailsRun(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusDeploying)
	remover.err = errors.New(`delete Deployment "orders": forbidden`)

	err := f.o.Step(context.Background(), f.run.ID, f.wsID)

	require.Error(t, err)
	require.Equal(t, 1, remover.calls)
	require.NotEqual(t, model.StatusSucceeded, f.run.Status)
}

func TestUndeployWithoutRemoverFailsBusiness(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	baseline := *f.run
	undeploy := model.NewUndeployRun(f.wsID, f.run.ProjectID, f.run.CreatedBy, &baseline, model.UndeployOptions{})
	undeploy.Status = model.StatusDeploying
	f.run = undeploy
	f.repo.run = undeploy

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusFailed, f.run.Status)
}
```

> 若 `orchestratorFixture` 字段名与上面不一致（`f.run` / `f.repo.run` / `f.wsID` / `f.prepare.calls` / `f.deployer.calls` / `f.deployer.restartCalls`），以 `orchestrator_test.go:258-300` 实际定义为准调整。基础设施错误（非 DomainError）走 `recordInfrastructureFailure`，首次返回原错误、状态保持 deploying，所以第三个测试只断言「返回错误且未成功」。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/application/service/ -run Undeploy -count=1`
Expected: 编译失败 `f.o.SetManifestRemover undefined`。

- [ ] **Step 3: 实现**

`domain/errors.go` 追加：

```go
	ErrDeploymentRemoverUnavailable = kernel.NewDomainError("DELIVERY_DEPLOYMENT_REMOVER_UNAVAILABLE", "manifest remover is not configured")
```

`orchestrator.go`：结构体加字段 `remover runtimeport.ManifestRemover`，并新增：

```go
func (o *Orchestrator) SetManifestRemover(remover runtimeport.ManifestRemover) {
	if o != nil {
		o.remover = remover
	}
}
```

`case model.StatusDeploying:` 中，`applyRunImageRepository(&project, run)` 之后、`if !(run.IsSyncOnly() ...` 之前插入：

```go
		if run.IsUndeploy() {
			if err := o.removeKubernetes(operationCtx, run, project); err != nil {
				if operationCtx.Err() != nil {
					return operationCtx.Err()
				}
				return o.handleDependencyError(ctx, run, err)
			}
			if err := run.Transition(model.StatusSucceeded, o.clock()); err != nil {
				return err
			}
			o.clearLease(run)
			if err := o.repo.Update(ctx, run); err != nil {
				return o.resolveConcurrent(ctx, run.ID, err)
			}
			o.notifyTerminal(ctx, run)
			return nil
		}
```

文件中 `finalizeKubernetes` 之后新增：

```go
func (o *Orchestrator) removeKubernetes(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) error {
	if o.remover == nil {
		return domain.ErrDeploymentRemoverUnavailable
	}
	spec := runtimeport.RemoveSpec{AppLabel: project.Name}
	if run.UndeployOptions != nil {
		spec.DeleteConfig = run.UndeployOptions.DeleteConfig
		spec.DeleteStorage = run.UndeployOptions.DeleteStorage
	}
	manifests, deploymentName, serviceName, err := o.renderManifests(ctx, project, run, run.ImageTag)
	if err == nil {
		spec.Manifests = manifests
	} else {
		_ = o.repo.AppendEvent(ctx, run, "render manifests failed, falling back to label selector: "+err.Error())
	}
	target := o.buildDeploymentTarget(project, run, deploymentName, serviceName)
	_ = o.repo.AppendEvent(ctx, run, fmt.Sprintf("removing workloads from %s/%s", project.Cluster.Name, project.Namespace))
	return o.remover.Remove(ctx, target, spec)
}
```

`delivery/module.go` 在 `deployOrchestrator.SetProjectVersionReader(...)` 下方加：

```go
	if remover, ok := c.ManifestDeployer.(runtimeport.ManifestRemover); ok {
		deployOrchestrator.SetManifestRemover(remover)
	}
```

并在 import 中加 `runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/... -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/delivery
git commit -m "feat(delivery): execute undeploy runs through manifest remover"
```

---

### Task 5: `UndeployHandler` + 项目生命周期读取 + HTTP 路由

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/domain/port/project_lifecycle.go`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/acl/workspace_app_acl.go`（`deploymentProjectRow` 加 `Status`、实现 `ProjectLifecycle`）
- Modify: `backend/internal/modules/delivery/deployrun/domain/errors.go`
- Create: `backend/internal/modules/delivery/deployrun/application/command/undeploy.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/command/create_deployment.go`、`rollback_deployment.go`（archiving 拒绝）
- Modify: `backend/internal/modules/delivery/deployrun/interface/http/handler.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `backend/internal/platform/httpx/errors.go`
- Test: `backend/internal/modules/delivery/deployrun/application/command/undeploy_test.go`

**Interfaces:**
- Consumes: Task 1 `NewUndeployRun`、`LatestRunByCluster`、`IsOnlineRun`
- Produces:

```go
// port
type ProjectLifecycle struct {
	Name   string
	Status string // "active" | "archiving" | "archived"
}
type ProjectLifecycleReader interface {
	ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (ProjectLifecycle, error)
}

// command
type UndeployInput struct {
	ClusterID     string `json:"cluster_id"`
	ConfirmName   string `json:"confirm_name"`
	DeleteConfig  bool   `json:"delete_config"`
	DeleteStorage bool   `json:"delete_storage"`
}
func NewUndeployHandler(repo repository.DeploymentRunRepository, lifecycle port.ProjectLifecycleReader, queue messaging.Queue, clock func() time.Time) *UndeployHandler
func (h *UndeployHandler) Handle(ctx, wsID kernel.WorkspaceID, memberID kernel.MemberID, projectID kernel.ProjectID, input UndeployInput) (*model.DeploymentRun, error)
func (h *UndeployHandler) StartForArchive(ctx, wsID kernel.WorkspaceID, memberID kernel.MemberID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) (*model.DeploymentRun, error)
func (h *CreateDeploymentHandler) SetProjectLifecycleReader(port.ProjectLifecycleReader)
func (h *RollbackDeploymentHandler) SetProjectLifecycleReader(port.ProjectLifecycleReader)
```

- 新错误：
  - `ErrConfirmNameMismatch = kernel.NewDomainError("DELIVERY_CONFIRM_NAME_MISMATCH", "confirmation name does not match project name")` → 400
  - `ErrEnvironmentNotDeployed = kernel.NewDomainError("DELIVERY_ENVIRONMENT_NOT_DEPLOYED", "environment has no online deployment")` → 409
  - `ErrProjectArchiving = kernel.NewDomainError("DELIVERY_PROJECT_ARCHIVING", "project is being archived")` → 409
  - `ErrProjectNotActive = kernel.NewDomainError("DELIVERY_DEPLOYMENT_PROJECT_NOT_ACTIVE", "project is not active")` → 409

- [ ] **Step 1: 写失败测试**

`undeploy_test.go`（包 `command`，复用 `fakeRunRepo`、`recordingQueue`）：

```go
package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type fakeLifecycle struct {
	value port.ProjectLifecycle
	err   error
}

func (f fakeLifecycle) ProjectLifecycle(context.Context, kernel.WorkspaceID, kernel.ProjectID) (port.ProjectLifecycle, error) {
	return f.value, f.err
}

func onlineRun(wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) *model.DeploymentRun {
	run := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	run.Status = model.StatusSucceeded
	run.DeployNamespace = "team-a-dev"
	return run
}

func newUndeployFixture(status string) (*UndeployHandler, *fakeRunRepo, *recordingQueue, kernel.WorkspaceID, kernel.ProjectID, kernel.ClusterInfoID) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	clusterID := kernel.NewClusterInfoID()
	repo := &fakeRunRepo{byProject: []*model.DeploymentRun{onlineRun(wsID, projectID, clusterID)}}
	queue := &recordingQueue{}
	handler := NewUndeployHandler(repo, fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: status}}, queue, nil)
	return handler, repo, queue, wsID, projectID, clusterID
}

func TestUndeployCreatesRunFromOnlineBaseline(t *testing.T) {
	handler, repo, queue, wsID, projectID, clusterID := newUndeployFixture("active")

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders", DeleteStorage: true,
	})

	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, clusterID, run.ClusterID)
	require.Equal(t, "team-a-dev", run.DeployNamespace)
	require.True(t, run.UndeployOptions.DeleteStorage)
	require.Len(t, repo.created, 1)
	require.Len(t, queue.payloads, 1)
}

func TestUndeployRejectsConfirmNameMismatch(t *testing.T) {
	handler, _, _, wsID, projectID, clusterID := newUndeployFixture("active")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "order",
	})
	require.ErrorIs(t, err, domain.ErrConfirmNameMismatch)
}

func TestUndeployRejectsWhenRunActive(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")
	repo.active = true
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
}

func TestUndeployRejectsOfflineCluster(t *testing.T) {
	handler, _, _, wsID, projectID, _ := newUndeployFixture("active")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: kernel.NewClusterInfoID().String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrEnvironmentNotDeployed)
}

func TestUndeployRejectsArchivingProjectButStartForArchiveAllows(t *testing.T) {
	handler, _, _, wsID, projectID, clusterID := newUndeployFixture("archiving")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrProjectArchiving)

	run, err := handler.StartForArchive(context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, &model.UndeployOptions{}, run.UndeployOptions)
}

func TestCreateDeploymentRejectsArchivingProject(t *testing.T) {
	repo := &fakeRunRepo{}
	handler := NewCreateDeploymentHandler(repo, fakeDeploymentReader{}, &recordingQueue{}, nil)
	handler.SetProjectLifecycleReader(fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: "archiving"}})

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(), CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.ErrorIs(t, err, domain.ErrProjectArchiving)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/application/command/ -run "Undeploy|Archiving" -count=1`
Expected: 编译失败 `undefined: NewUndeployHandler`。

- [ ] **Step 3: 实现**

`port/project_lifecycle.go`：

```go
package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectLifecycle struct {
	Name   string
	Status string
}

type ProjectLifecycleReader interface {
	ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (ProjectLifecycle, error)
}
```

`workspace_app_acl.go`：`deploymentProjectRow` 加 `Status string`；文件末尾追加：

```go
func (a *WorkspaceAppACL) ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (port.ProjectLifecycle, error) {
	row, err := a.loadProject(ctx, wsID, projectID)
	if err != nil {
		return port.ProjectLifecycle{}, err
	}
	return port.ProjectLifecycle{Name: row.Name, Status: row.Status}, nil
}

var _ port.ProjectLifecycleReader = (*WorkspaceAppACL)(nil)
```

`errors.go` 追加 Interfaces 中列出的 4 个错误。

`command/undeploy.go`：

```go
package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	projectStatusActive    = "active"
	projectStatusArchiving = "archiving"
)

type UndeployInput struct {
	ClusterID     string `json:"cluster_id"`
	ConfirmName   string `json:"confirm_name"`
	DeleteConfig  bool   `json:"delete_config"`
	DeleteStorage bool   `json:"delete_storage"`
}

type UndeployHandler struct {
	repo      repository.DeploymentRunRepository
	lifecycle port.ProjectLifecycleReader
	queue     messaging.Queue
	clock     func() time.Time
}

func NewUndeployHandler(
	repo repository.DeploymentRunRepository,
	lifecycle port.ProjectLifecycleReader,
	queue messaging.Queue,
	clock func() time.Time,
) *UndeployHandler {
	if clock == nil {
		clock = time.Now
	}
	return &UndeployHandler{repo: repo, lifecycle: lifecycle, queue: queue, clock: clock}
}

func (h *UndeployHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input UndeployInput,
) (*model.DeploymentRun, error) {
	lifecycle, err := h.lifecycle.ProjectLifecycle(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ConfirmName) != lifecycle.Name {
		return nil, domain.ErrConfirmNameMismatch
	}
	if err := requireActive(lifecycle); err != nil {
		return nil, err
	}
	clusterID, err := kernel.ParseClusterInfoID(strings.TrimSpace(input.ClusterID))
	if err != nil {
		return nil, domain.ErrDeploymentClusterNotFound
	}
	return h.start(ctx, wsID, memberID, projectID, clusterID, model.UndeployOptions{
		DeleteConfig: input.DeleteConfig, DeleteStorage: input.DeleteStorage,
	})
}

func (h *UndeployHandler) StartForArchive(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
) (*model.DeploymentRun, error) {
	lifecycle, err := h.lifecycle.ProjectLifecycle(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if lifecycle.Status != projectStatusActive && lifecycle.Status != projectStatusArchiving {
		return nil, domain.ErrProjectNotActive
	}
	return h.start(ctx, wsID, memberID, projectID, clusterID, model.UndeployOptions{})
}

func (h *UndeployHandler) start(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	opts model.UndeployOptions,
) (*model.DeploymentRun, error) {
	active, err := h.repo.HasActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}
	runs, err := h.repo.ListByProject(ctx, projectID, 100)
	if err != nil {
		return nil, err
	}
	baseline := model.LatestRunByCluster(workspaceRuns(runs, wsID))[clusterID]
	if !model.IsOnlineRun(baseline) {
		return nil, domain.ErrEnvironmentNotDeployed
	}
	run := model.NewUndeployRun(wsID, projectID, memberID, baseline, opts)
	run.CreatedAt = h.clock().UTC()
	if err := h.repo.Create(ctx, run); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(consumer.Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})
	if err != nil {
		return nil, fmt.Errorf("marshal deployment message: %w", err)
	}
	if err := h.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
		run.Fail("publish deploy message: "+err.Error(), h.clock())
		if updateErr := h.repo.Update(ctx, run); updateErr != nil {
			return nil, errors.Join(err, fmt.Errorf("mark deployment failed: %w", updateErr))
		}
		return nil, err
	}
	return run, nil
}

func workspaceRuns(runs []*model.DeploymentRun, wsID kernel.WorkspaceID) []*model.DeploymentRun {
	out := make([]*model.DeploymentRun, 0, len(runs))
	for _, run := range runs {
		if run != nil && run.WorkspaceID == wsID {
			out = append(out, run)
		}
	}
	return out
}

func requireActive(lifecycle port.ProjectLifecycle) error {
	switch lifecycle.Status {
	case projectStatusActive, "":
		return nil
	case projectStatusArchiving:
		return domain.ErrProjectArchiving
	default:
		return domain.ErrProjectNotActive
	}
}
```

`create_deployment.go`：结构体加 `lifecycle port.ProjectLifecycleReader`，新增：

```go
func (h *CreateDeploymentHandler) SetProjectLifecycleReader(reader port.ProjectLifecycleReader) {
	h.lifecycle = reader
}
```

`Handle` 开头（`var selected` 之前）插入：

```go
	if h.lifecycle != nil {
		lifecycle, err := h.lifecycle.ProjectLifecycle(ctx, wsID, projectID)
		if err != nil {
			return nil, err
		}
		if err := requireActive(lifecycle); err != nil {
			return nil, err
		}
	}
```

`rollback_deployment.go`：同样加 `lifecycle port.ProjectLifecycleReader` 字段、`SetProjectLifecycleReader`，在 `Handle` 开头插入同一段检查（import `port` 包）。

`interface/http/handler.go`：`Handler` 加 `undeploy *command.UndeployHandler` 字段；`NewHandler` 末尾新增参数 `undeploy *command.UndeployHandler` 并赋值；新增方法：

```go
func (h *Handler) Undeploy(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	var input command.UndeployInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	run, err := h.undeploy.Handle(c.Request.Context(), wsID, memberID, projectID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, httpx.Envelope{
		Code: 0, Message: "ok", Data: query.ToDeploymentView(run), TraceID: httpx.GetTraceID(c),
	})
}
```

`delivery/module.go`：

```go
	createDeployment := deploycommand.NewCreateDeploymentHandler(deployRepo, deployReader, c.MessageQueue, nil, envResolver)
	createDeployment.SetProjectLifecycleReader(deployReader)
	rollbackDeployment := deploycommand.NewRollbackDeploymentHandler(deployRepo, c.MessageQueue, nil)
	rollbackDeployment.SetProjectLifecycleReader(deployReader)
	undeploy := deploycommand.NewUndeployHandler(deployRepo, deployReader, c.MessageQueue, nil)
```

`deployhttp.NewHandler(...)` 中 rollback 参数换成 `rollbackDeployment`，末尾追加 `undeploy`。路由区追加：

```go
	c.Engine.POST("/api/v1/projects/:projectId/undeployments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHandler.Undeploy)
```

`httpx/errors.go`：在 `case "DELIVERY_DEPLOYMENT_RUNNING", ...` 那行的 case 列表追加 `"DELIVERY_ENVIRONMENT_NOT_DEPLOYED", "DELIVERY_PROJECT_ARCHIVING", "DELIVERY_DEPLOYMENT_PROJECT_NOT_ACTIVE"`；在 400 分组（`case "DELIVERY_PROJECT_NOT_ACTIVE", ...`）追加 `"DELIVERY_CONFIRM_NAME_MISMATCH"`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/... ./internal/platform/httpx/... -count=1`
Expected: PASS（若 `NewHandler` 有其它调用方/测试，按编译错误补上 `nil` 参数）。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/delivery backend/internal/platform/httpx
git commit -m "feat(delivery): add undeploy command and archiving guards"
```

---

### Task 6: Jenkins Job 禁用

**Files:**
- Modify: `backend/internal/modules/delivery/domain/port/ports.go`
- Modify: `backend/internal/modules/delivery/infrastructure/jenkins/executor.go`
- Modify: `backend/internal/modules/delivery/infrastructure/jenkins/mock_executor.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/service/prepare.go`
- Test: `backend/internal/modules/delivery/deployrun/application/service/prepare_disable_test.go`

**Interfaces:**
- Produces:
  - `type JenkinsJobDisabler interface { DisableJob(ctx context.Context, folder, job string) error }`（`deliveryport`）
  - `func (s *PrepareService) DisableJob(ctx context.Context, project deployport.ProjectDeploymentContext) error`

- [ ] **Step 1: 写失败测试**

```go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
)

func TestPrepareServiceDisableJobUsesProjectFolderAndName(t *testing.T) {
	mock := jenkinsinfra.NewMockExecutor()
	svc := NewPrepareService("-build", nil).UseJenkinsFactory(func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return mock, nil
	})

	err := svc.DisableJob(context.Background(), deployport.ProjectDeploymentContext{Name: "orders", Namespace: "team-a"})

	require.NoError(t, err)
	require.Equal(t, "team-a/orders", mock.LastDisabledJob)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/deployrun/application/service/ -run DisableJob -count=1`
Expected: 编译失败。

- [ ] **Step 3: 实现**

`ports.go` 在 `JenkinsExecutor` 之后追加：

```go
type JenkinsJobDisabler interface {
	DisableJob(ctx context.Context, folder, job string) error
}
```

`executor.go` 在 `CreateOrUpdate` 之后追加：

```go
func (e *Executor) DisableJob(ctx context.Context, folder, job string) error {
	folder = strings.TrimSpace(folder)
	job = strings.TrimSpace(job)
	if folder == "" || job == "" {
		return errors.New("jenkins folder and job are required")
	}
	jobBase := "/job/" + url.PathEscape(folder) + "/job/" + url.PathEscape(job)
	status, err := e.resourceStatus(ctx, jobBase)
	if err != nil {
		return fmt.Errorf("get jenkins job: %w", err)
	}
	if status == http.StatusNotFound {
		return nil
	}
	target, err := e.client.GetJob(ctx, job, folder)
	if err != nil {
		return fmt.Errorf("get jenkins job: %w", err)
	}
	if _, err := target.Disable(ctx); err != nil {
		return fmt.Errorf("disable jenkins job: %w", err)
	}
	return nil
}

var _ port.JenkinsJobDisabler = (*Executor)(nil)
```

> gojenkins `(*Job).Disable(ctx) (bool, error)`；若项目锁定的 gojenkins 版本签名不同，按实际签名调用（`rg "func \(j \*Job\) Disable" $(go env GOMODCACHE)/github.com/bndr`）。

`mock_executor.go`：结构体加 `LastDisabledJob string`，追加：

```go
func (e *MockExecutor) DisableJob(_ context.Context, folder, job string) error {
	e.LastDisabledJob = folder + "/" + job
	return nil
}

var _ port.JenkinsJobDisabler = (*MockExecutor)(nil)
```

`prepare.go` 追加：

```go
func (s *PrepareService) DisableJob(ctx context.Context, project deployport.ProjectDeploymentContext) error {
	executor, err := s.resolveJenkins(project.Jenkins)
	if err != nil {
		return fmt.Errorf("resolve project jenkins: %w", err)
	}
	disabler, ok := executor.(deliveryport.JenkinsJobDisabler)
	if !ok {
		return nil
	}
	return disabler.DisableJob(ctx, strings.TrimSpace(project.Namespace), strings.TrimSpace(project.Name))
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/... -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/delivery
git commit -m "feat(delivery): disable jenkins job for archived projects"
```

---

### Task 7: `appctx.ProjectTeardown` 端口 + delivery 实现 + 事件 `CreatedBy`

**Files:**
- Modify: `backend/internal/platform/appctx/ports.go`、`backend/internal/platform/appctx/container.go`
- Modify: `backend/internal/modules/delivery/domain/event/deploy_completed.go`
- Create: `backend/internal/modules/delivery/teardown_port.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Test: `backend/internal/modules/delivery/teardown_port_test.go`

**Interfaces:**
- Consumes: Task 1 `OnlineClusters`；Task 5 `UndeployHandler.StartForArchive`；Task 6 `PrepareService.DisableJob`
- Produces:

```go
// appctx
type UndeployRequest struct {
	WorkspaceID kernel.WorkspaceID
	ProjectID   kernel.ProjectID
	MemberID    kernel.MemberID
	ClusterID   kernel.ClusterInfoID
}
type ProjectTeardown interface {
	OnlineClusters(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]kernel.ClusterInfoID, error)
	HasActiveRun(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	StartUndeploy(ctx context.Context, req UndeployRequest) (kernel.DeploymentRunID, error)
	DisablePipeline(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error
}
// Container.ProjectTeardown ProjectTeardown

// event
DeployCompleted.CreatedBy kernel.MemberID
```

- [ ] **Step 1: 写失败测试**

`teardown_port_test.go`（包 `delivery`）：

```go
package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type teardownRunLister struct {
	runs   []*model.DeploymentRun
	active bool
}

func (l teardownRunLister) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return l.runs, nil
}
func (l teardownRunLister) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return l.active, nil
}

func TestTeardownPortOnlineClustersFiltersWorkspace(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	own := model.NewDeploymentRun(wsID, projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "v1")
	own.Status = model.StatusSucceeded
	foreign := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "v1")
	foreign.Status = model.StatusSucceeded
	foreign.CreatedAt = own.CreatedAt.Add(time.Second)

	port := &teardownPort{runs: teardownRunLister{runs: []*model.DeploymentRun{foreign, own}}}
	clusters, err := port.OnlineClusters(context.Background(), wsID, projectID)

	require.NoError(t, err)
	require.Equal(t, []kernel.ClusterInfoID{own.ClusterID}, clusters)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/delivery/ -run TeardownPort -count=1`
Expected: 编译失败 `undefined: teardownPort`。

- [ ] **Step 3: 实现**

`appctx/ports.go` 末尾追加 Interfaces 中 `UndeployRequest` 与 `ProjectTeardown`。`container.go` 在 `DeploymentTrigger` 下方加 `ProjectTeardown ProjectTeardown`。

`deploy_completed.go` 结构体末尾加 `CreatedBy kernel.MemberID`（构造函数不变）。

`teardown_port.go`：

```go
package delivery

import (
	"context"

	deploycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	deployservice "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type teardownRunReader interface {
	ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.DeploymentRun, error)
	HasActiveByProject(ctx context.Context, projectID kernel.ProjectID) (bool, error)
}

type teardownPort struct {
	runs     teardownRunReader
	undeploy *deploycommand.UndeployHandler
	reader   deployport.ProjectDeploymentReader
	prepare  *deployservice.PrepareService
}

func (p *teardownPort) OnlineClusters(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	runs, err := p.runs.ListByProject(ctx, projectID, 100)
	if err != nil {
		return nil, err
	}
	scoped := make([]*model.DeploymentRun, 0, len(runs))
	for _, run := range runs {
		if run != nil && run.WorkspaceID == wsID {
			scoped = append(scoped, run)
		}
	}
	return model.OnlineClusters(scoped), nil
}

func (p *teardownPort) HasActiveRun(ctx context.Context, projectID kernel.ProjectID) (bool, error) {
	return p.runs.HasActiveByProject(ctx, projectID)
}

func (p *teardownPort) StartUndeploy(ctx context.Context, req appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	run, err := p.undeploy.StartForArchive(ctx, req.WorkspaceID, req.MemberID, req.ProjectID, req.ClusterID)
	if err != nil {
		return kernel.DeploymentRunID{}, err
	}
	return run.ID, nil
}

func (p *teardownPort) DisablePipeline(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
	project, err := p.reader.Load(ctx, wsID, projectID, kernel.ClusterInfoID{})
	if err != nil {
		return err
	}
	return p.prepare.DisableJob(ctx, project)
}

var _ appctx.ProjectTeardown = (*teardownPort)(nil)
```

`module.go`：
1. 把 `deployservice.NewPrepareService(...)` 提为变量 `prepareService := deployservice.NewPrepareService(c.Config.Jenkins.JobSuffix, deployacl.NewWorkspacePipelineRenderer(c.DB))`，传给 `NewOrchestrator`。
2. terminal hook 改为：

```go
		completed := deliveryevent.NewDeployCompleted(
			run.ID, run.ProjectID, run.WorkspaceID, run.EnvironmentID, "", string(run.Status), string(run.RunType), run.ImageTag,
		)
		completed.CreatedBy = run.CreatedBy
		if err := c.EventBus.Publish(ctx, completed); err != nil && c.Logger != nil {
			c.Logger.Warn("publish deploy completed failed", zap.Error(err))
		}
```

3. 在 `c.DeploymentTrigger = ...` 下方追加：

```go
	c.ProjectTeardown = &teardownPort{runs: deployRepo, undeploy: undeploy, reader: deployReader, prepare: prepareService}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/delivery/... ./internal/platform/... -count=1; go build ./...`
Expected: PASS / 构建成功。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/appctx backend/internal/modules/delivery
git commit -m "feat(delivery): expose project teardown port"
```

---

### Task 8: workspace_app —— `archiving` 状态与仓储/查询

**Files:**
- Modify: `backend/internal/modules/workspace_app/domain/model/project.go`
- Modify: `backend/internal/modules/workspace_app/domain/errors.go`
- Modify: `backend/internal/modules/workspace_app/domain/repository/project_repository.go`
- Modify: `backend/internal/modules/workspace_app/infrastructure/persistence/project_repo.go`
- Modify: `backend/internal/modules/workspace_app/application/query/project_queries.go:161`
- Modify: `backend/internal/platform/httpx/errors.go`
- Test: `backend/internal/modules/workspace_app/domain/model/project_archiving_test.go`

**Interfaces:**
- Produces:
  - `ProjectStatusArchiving ProjectStatus = "archiving"`
  - `func (p *Project) BeginArchiving() error`（仅 active → archiving，否则 `ErrProjectNotActive`）
  - `func (p *Project) Archive() error`（active/archiving → archived；archived 再调 → `ErrProjectAlreadyArchived`）
  - `func (p *Project) AbortArchiving() error`（archiving → active，否则 `ErrProjectNotArchiving`）
  - `func (p *Project) IsArchiving() bool`
  - `ProjectRepository.ListByStatus(ctx context.Context, status model.ProjectStatus) ([]*model.Project, error)`
  - 错误：`ErrProjectNotArchiving`(`WORKSPACE_APP_PROJECT_NOT_ARCHIVING`, 409)、`ErrProjectArchiving`(`WORKSPACE_APP_PROJECT_ARCHIVING`, 409)、`ErrProjectHasOnlineEnvironments`(`WORKSPACE_APP_PROJECT_HAS_ONLINE_ENVIRONMENTS`, 409)、`ErrProjectDeploymentRunning`(`WORKSPACE_APP_PROJECT_DEPLOYMENT_RUNNING`, 409)、`ErrConfirmNameMismatch`(`WORKSPACE_APP_CONFIRM_NAME_MISMATCH`, 400)

- [ ] **Step 1: 写失败测试**

`project_archiving_test.go`（包 `model_test`，复用 `mustName`、`sampleInput`）：

```go
package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProject_ArchivingLifecycle(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput(""))

	require.NoError(t, p.BeginArchiving())
	require.True(t, p.IsArchiving())
	require.ErrorIs(t, p.BeginArchiving(), domain.ErrProjectNotActive)
	require.ErrorIs(t, p.Rename(mustName(t, "other")), domain.ErrProjectNotActive)

	require.NoError(t, p.AbortArchiving())
	require.Equal(t, model.ProjectStatusActive, p.Status)
	require.ErrorIs(t, p.AbortArchiving(), domain.ErrProjectNotArchiving)

	require.NoError(t, p.BeginArchiving())
	require.NoError(t, p.Archive())
	require.Equal(t, model.ProjectStatusArchived, p.Status)
	require.ErrorIs(t, p.Archive(), domain.ErrProjectAlreadyArchived)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/workspace_app/domain/model/ -run Archiving -count=1`
Expected: 编译失败。

- [ ] **Step 3: 实现**

`project.go` 常量：

```go
const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusArchiving ProjectStatus = "archiving"
	ProjectStatusArchived  ProjectStatus = "archived"
)
```

替换 `Archive` 并新增方法：

```go
func (p *Project) BeginArchiving() error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	p.Status = ProjectStatusArchiving
	return nil
}

func (p *Project) Archive() error {
	if p.Status == ProjectStatusArchived {
		return domain.ErrProjectAlreadyArchived
	}
	p.Status = ProjectStatusArchived
	return nil
}

func (p *Project) AbortArchiving() error {
	if p.Status != ProjectStatusArchiving {
		return domain.ErrProjectNotArchiving
	}
	p.Status = ProjectStatusActive
	return nil
}

func (p *Project) IsArchiving() bool { return p.Status == ProjectStatusArchiving }
```

`errors.go` 追加 5 个错误（code / message 见 Interfaces）。

`project_repository.go` 接口追加 `ListByStatus`。

`project_repo.go`：
- `FindByWorkspaceAndName`、`ListByWorkspace`、`CountByWorkspace` 的 `status = ?` 改为 `status IN ?`，参数换成 `visibleProjectStatuses()`：

```go
func visibleProjectStatuses() []string {
	return []string{string(model.ProjectStatusActive), string(model.ProjectStatusArchiving)}
}
```

- 新增：

```go
func (r *ProjectRepository) ListByStatus(ctx context.Context, status model.ProjectStatus) ([]*model.Project, error) {
	var rows []projectModel
	if err := r.db.WithContext(ctx).Where("status = ?", string(status)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Project, 0, len(rows))
	for _, row := range rows {
		p, err := rowToProject(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
```

> 仓储里所有实现 `ProjectRepository` 的测试 fake（`rg "func \(.*\) CountByProduct\(" backend/internal/modules/workspace_app`）都要补一个返回 `nil, nil` 的 `ListByStatus`。

`project_queries.go:161`：

```go
	if project.Status != model.ProjectStatusActive && project.Status != model.ProjectStatusArchiving {
		return nil, domain.ErrProjectNotFound
	}
```

`httpx/errors.go`：409 分组追加 `"WORKSPACE_APP_PROJECT_NOT_ARCHIVING", "WORKSPACE_APP_PROJECT_ARCHIVING", "WORKSPACE_APP_PROJECT_HAS_ONLINE_ENVIRONMENTS", "WORKSPACE_APP_PROJECT_DEPLOYMENT_RUNNING"`；400 分组追加 `"WORKSPACE_APP_CONFIRM_NAME_MISMATCH"`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/workspace_app/... ./internal/platform/httpx/... -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/workspace_app backend/internal/platform/httpx
git commit -m "feat(workspace): add archiving project status"
```

---

### Task 9: workspace_app —— 归档命令与 `ArchiveCoordinator`

**Files:**
- Create: `backend/internal/modules/workspace_app/application/command/archive_coordinator.go`
- Modify: `backend/internal/modules/workspace_app/application/command/archive_project.go`
- Modify: `backend/internal/modules/workspace_app/interface/http/handler.go`
- Modify: `backend/internal/modules/workspace_app/module.go`
- Test: `backend/internal/modules/workspace_app/application/command/archive_coordinator_test.go`

**Interfaces:**
- Consumes: Task 7 `appctx.ProjectTeardown`、`appctx.UndeployRequest`、`deliveryevent.DeployCompleted{RunType, Status, ProjectID, WorkspaceID, CreatedBy}`；Task 8 状态方法与错误
- Produces:

```go
type ArchiveInput struct {
	ConfirmName string `json:"confirm_name"`
	Teardown    bool   `json:"teardown"`
}
type ArchiveResult struct {
	Status          string   `json:"status"`           // "archived" | "archiving"
	PendingClusters []string `json:"pending_clusters"` // 待下线集群 ID
}
func NewArchiveCoordinator(repo repository.ProjectRepository, teardown func() appctx.ProjectTeardown, warn func(string, error)) *ArchiveCoordinator
func (c *ArchiveCoordinator) Advance(ctx context.Context, project *model.Project, memberID kernel.MemberID) error
func (c *ArchiveCoordinator) HandleDeployCompleted(ctx context.Context, e events.Event) error
func (c *ArchiveCoordinator) ResumeAll(ctx context.Context) error
func NewArchiveProjectHandler(repo repository.ProjectRepository, wsChecker port.WorkspaceChecker, coordinator *ArchiveCoordinator) *ArchiveProjectHandler
func (h *ArchiveProjectHandler) Handle(ctx, wsID, memberID, projectID, input ArchiveInput) (ArchiveResult, error)
func (h *ArchiveProjectHandler) HandleLegacy(ctx, wsID, memberID, projectID) error
```

- [ ] **Step 1: 写失败测试**

`archive_coordinator_test.go`（包 `command`）：

```go
package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type memProjectRepo struct {
	projects map[kernel.ProjectID]*model.Project
	updates  int
}

func (r *memProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *memProjectRepo) Update(_ context.Context, p *model.Project) error {
	r.updates++
	r.projects[p.ID] = p
	return nil
}
func (r *memProjectRepo) FindByID(_ context.Context, id kernel.ProjectID) (*model.Project, error) {
	p, ok := r.projects[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return p, nil
}
func (r *memProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *memProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) { return nil, nil }
func (r *memProjectRepo) ListByProduct(context.Context, kernel.ProductID) ([]*model.Project, error)   { return nil, nil }
func (r *memProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error)         { return 0, nil }
func (r *memProjectRepo) CountByProduct(context.Context, kernel.ProductID) (int, error)             { return 0, nil }
func (r *memProjectRepo) ListByStatus(_ context.Context, status model.ProjectStatus) ([]*model.Project, error) {
	out := []*model.Project{}
	for _, p := range r.projects {
		if p.Status == status {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeTeardown struct {
	online     []kernel.ClusterInfoID
	active     bool
	started    []kernel.ClusterInfoID
	disabled   int
	disableErr error
}

func (f *fakeTeardown) OnlineClusters(context.Context, kernel.WorkspaceID, kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	return f.online, nil
}
func (f *fakeTeardown) HasActiveRun(context.Context, kernel.ProjectID) (bool, error) { return f.active, nil }
func (f *fakeTeardown) StartUndeploy(_ context.Context, req appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	f.started = append(f.started, req.ClusterID)
	return kernel.NewDeploymentRunID(), nil
}
func (f *fakeTeardown) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	f.disabled++
	return f.disableErr
}

type allowAllWorkspace struct{}

func (allowAllWorkspace) MemberInWorkspace(context.Context, kernel.MemberID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}

func archiveFixture(t *testing.T, online ...kernel.ClusterInfoID) (*ArchiveProjectHandler, *ArchiveCoordinator, *memProjectRepo, *fakeTeardown, *model.Project) {
	t.Helper()
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	repo := &memProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	teardown := &fakeTeardown{online: online}
	coordinator := NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, func(string, error) {})
	return NewArchiveProjectHandler(repo, allowAllWorkspace{}, coordinator), coordinator, repo, teardown, project
}

func TestArchiveWithoutOnlineEnvironmentsArchivesImmediately(t *testing.T) {
	handler, _, _, teardown, project := archiveFixture(t)

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, "archived", result.Status)
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 1, teardown.disabled)
}

func TestArchiveRequiresTeardownWhenOnline(t *testing.T) {
	handler, _, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders"})

	require.ErrorIs(t, err, domain.ErrProjectHasOnlineEnvironments)
}

func TestArchiveRejectsConfirmMismatch(t *testing.T) {
	handler, _, _, _, project := archiveFixture(t)
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "x"})
	require.ErrorIs(t, err, domain.ErrConfirmNameMismatch)
}

func TestArchiveWithTeardownStartsFirstUndeploy(t *testing.T) {
	dev, uat := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	handler, _, _, teardown, project := archiveFixture(t, dev, uat)

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders", Teardown: true})

	require.NoError(t, err)
	require.Equal(t, "archiving", result.Status)
	require.Equal(t, []string{dev.String(), uat.String()}, result.PendingClusters)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Equal(t, []kernel.ClusterInfoID{dev}, teardown.started)
}

func TestCoordinatorAdvancesOnUndeploySuccessAndFinishes(t *testing.T) {
	dev, uat := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	handler, coordinator, _, teardown, project := archiveFixture(t, dev, uat)
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)

	teardown.online = []kernel.ClusterInfoID{uat}
	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "succeeded")))
	require.Equal(t, []kernel.ClusterInfoID{dev, uat}, teardown.started)

	teardown.online = nil
	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "succeeded")))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 1, teardown.disabled)
}

func TestCoordinatorAbortsArchivingOnUndeployFailure(t *testing.T) {
	handler, coordinator, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)

	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "failed")))

	require.Equal(t, model.ProjectStatusActive, project.Status)
}

func TestCoordinatorArchivesEvenWhenDisableFails(t *testing.T) {
	handler, _, _, teardown, project := archiveFixture(t)
	teardown.disableErr = errors.New("jenkins down")

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, "archived", result.Status)
}

func TestLegacyArchiveRejectsOnlineProject(t *testing.T) {
	handler, _, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())
	err := handler.HandleLegacy(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID)
	require.ErrorIs(t, err, domain.ErrProjectHasOnlineEnvironments)
}

func TestResumeAllRestartsStalledArchiving(t *testing.T) {
	dev := kernel.NewClusterInfoID()
	_, coordinator, _, teardown, project := archiveFixture(t, dev)
	require.NoError(t, project.BeginArchiving())

	require.NoError(t, coordinator.ResumeAll(context.Background()))

	require.Equal(t, []kernel.ClusterInfoID{dev}, teardown.started)
}

func undeployEvent(project *model.Project, status string) deliveryevent.DeployCompleted {
	ev := deliveryevent.NewDeployCompleted(kernel.NewDeploymentRunID(), project.ID, project.WorkspaceID, nil, "", status, "undeploy", "v1")
	ev.CreatedBy = kernel.NewMemberID()
	return ev
}
```

> `port.WorkspaceChecker` 的方法名以 `workspace_app/domain/port` 实际定义为准（`archive_project.go` 现调用 `MemberInWorkspace(ctx, memberID, wsID)`）。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/workspace_app/application/command/ -run "Archive|Coordinator|ResumeAll" -count=1`
Expected: 编译失败。

- [ ] **Step 3: 实现**

`archive_coordinator.go`：

```go
package command

import (
	"context"
	"errors"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var errTeardownUnavailable = errors.New("project teardown is not configured")

type ArchiveCoordinator struct {
	repo     repository.ProjectRepository
	teardown func() appctx.ProjectTeardown
	warn     func(string, error)
}

func NewArchiveCoordinator(
	repo repository.ProjectRepository,
	teardown func() appctx.ProjectTeardown,
	warn func(string, error),
) *ArchiveCoordinator {
	if warn == nil {
		warn = func(string, error) {}
	}
	return &ArchiveCoordinator{repo: repo, teardown: teardown, warn: warn}
}

func (c *ArchiveCoordinator) port() (appctx.ProjectTeardown, error) {
	if c.teardown == nil {
		return nil, errTeardownUnavailable
	}
	teardown := c.teardown()
	if teardown == nil {
		return nil, errTeardownUnavailable
	}
	return teardown, nil
}

func (c *ArchiveCoordinator) Advance(ctx context.Context, project *model.Project, memberID kernel.MemberID) error {
	teardown, err := c.port()
	if err != nil {
		return err
	}
	online, err := teardown.OnlineClusters(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	if len(online) == 0 {
		return c.finish(ctx, teardown, project)
	}
	active, err := teardown.HasActiveRun(ctx, project.ID)
	if err != nil || active {
		return err
	}
	_, err = teardown.StartUndeploy(ctx, appctx.UndeployRequest{
		WorkspaceID: project.WorkspaceID, ProjectID: project.ID, MemberID: memberID, ClusterID: online[0],
	})
	return err
}

func (c *ArchiveCoordinator) finish(ctx context.Context, teardown appctx.ProjectTeardown, project *model.Project) error {
	if err := teardown.DisablePipeline(ctx, project.WorkspaceID, project.ID); err != nil {
		c.warn("disable jenkins job failed during archive", err)
	}
	if err := project.Archive(); err != nil {
		return err
	}
	return c.repo.Update(ctx, project)
}

func (c *ArchiveCoordinator) HandleDeployCompleted(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.DeployCompleted)
	if !ok || ev.RunType != "undeploy" {
		return nil
	}
	project, err := c.repo.FindByID(ctx, ev.ProjectID)
	if err != nil || !project.IsArchiving() {
		return nil
	}
	if ev.Status != "succeeded" {
		if err := project.AbortArchiving(); err != nil {
			return err
		}
		c.warn("archive aborted: undeploy "+ev.RunID.String()+" "+ev.Status, nil)
		return c.repo.Update(ctx, project)
	}
	if err := c.Advance(ctx, project, ev.CreatedBy); err != nil {
		c.warn("advance archive failed", err)
		if abortErr := project.AbortArchiving(); abortErr == nil {
			return c.repo.Update(ctx, project)
		}
		return err
	}
	return nil
}

func (c *ArchiveCoordinator) ResumeAll(ctx context.Context) error {
	projects, err := c.repo.ListByStatus(ctx, model.ProjectStatusArchiving)
	if err != nil {
		return err
	}
	var errs []error
	for _, project := range projects {
		if err := c.Advance(ctx, project, project.CreatedBy); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
```

> `project.CreatedBy` 字段名以 `model.Project` 实际字段为准（`NewProject` 中 `CreatedBy: createdBy`）。

`archive_project.go` 全量替换为：

```go
package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ArchiveInput struct {
	ConfirmName string `json:"confirm_name"`
	Teardown    bool   `json:"teardown"`
}

type ArchiveResult struct {
	Status          string   `json:"status"`
	PendingClusters []string `json:"pending_clusters"`
}

type ArchiveProjectHandler struct {
	repo        repository.ProjectRepository
	wsChecker   port.WorkspaceChecker
	coordinator *ArchiveCoordinator
}

func NewArchiveProjectHandler(
	repo repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
	coordinator *ArchiveCoordinator,
) *ArchiveProjectHandler {
	return &ArchiveProjectHandler{repo: repo, wsChecker: wsChecker, coordinator: coordinator}
}

func (h *ArchiveProjectHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input ArchiveInput,
) (ArchiveResult, error) {
	project, err := h.load(ctx, wsID, memberID, projectID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if strings.TrimSpace(input.ConfirmName) != project.Name.String() {
		return ArchiveResult{}, domain.ErrConfirmNameMismatch
	}
	return h.archive(ctx, project, memberID, input.Teardown)
}

func (h *ArchiveProjectHandler) HandleLegacy(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	project, err := h.load(ctx, wsID, memberID, projectID)
	if err != nil {
		return err
	}
	_, err = h.archive(ctx, project, memberID, false)
	return err
}

func (h *ArchiveProjectHandler) load(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) (*model.Project, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	switch project.Status {
	case model.ProjectStatusArchived:
		return nil, domain.ErrProjectAlreadyArchived
	case model.ProjectStatusArchiving:
		return nil, domain.ErrProjectArchiving
	}
	return project, nil
}

func (h *ArchiveProjectHandler) archive(
	ctx context.Context,
	project *model.Project,
	memberID kernel.MemberID,
	teardown bool,
) (ArchiveResult, error) {
	teardown, err := h.coordinator.port()
	if err != nil {
		return ArchiveResult{}, err
	}
	active, err := teardown.HasActiveRun(ctx, project.ID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if active {
		return ArchiveResult{}, domain.ErrProjectDeploymentRunning
	}
	online, err := teardown.OnlineClusters(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if len(online) == 0 {
		if err := h.coordinator.finish(ctx, teardown, project); err != nil {
			return ArchiveResult{}, err
		}
		return ArchiveResult{Status: string(model.ProjectStatusArchived), PendingClusters: []string{}}, nil
	}
	if !teardown {
		return ArchiveResult{}, domain.ErrProjectHasOnlineEnvironments
	}
	if err := project.BeginArchiving(); err != nil {
		return ArchiveResult{}, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return ArchiveResult{}, err
	}
	if err := h.coordinator.Advance(ctx, project, memberID); err != nil {
		if abortErr := project.AbortArchiving(); abortErr == nil {
			_ = h.repo.Update(ctx, project)
		}
		return ArchiveResult{}, err
	}
	pending := make([]string, 0, len(online))
	for _, id := range online {
		pending = append(pending, id.String())
	}
	return ArchiveResult{Status: string(model.ProjectStatusArchiving), PendingClusters: pending}, nil
}
```

`interface/http/handler.go`：`ArchiveProject` 中 `h.archive.Handle(...)` 改为 `h.archive.HandleLegacy(c.Request.Context(), wsID, memberID, projectID)`；新增：

```go
func (h *Handler) ArchiveProjectWithTeardown(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	var input command.ArchiveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.archive.Handle(c.Request.Context(), wsID, memberID, projectID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}
```

`module.go`：
1. 构造 handler 之前：

```go
	archiveCoordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return c.ProjectTeardown }, func(msg string, err error) {
		if c.Logger != nil {
			c.Logger.Warn(msg, zap.Error(err))
		}
	})
```

2. `command.NewArchiveProjectHandler(repo, wsChecker)` → `command.NewArchiveProjectHandler(repo, wsChecker, archiveCoordinator)`。
3. `if c.EventBus != nil { ... }` 块中追加 `c.EventBus.Subscribe(deliveryevent.DeployCompletedName, archiveCoordinator.HandleDeployCompleted)`。
4. 路由区在 DELETE 之后追加：

```go
	c.Engine.POST("/api/v1/projects/:projectId/archive",
		auth, ws, wsMember, projectAdmin, handler.ArchiveProjectWithTeardown)
```

5. `return nil` 之前启动续推定时器：

```go
	if c.Context != nil && c.Config != nil && c.Config.Deploy.ReconcileInterval > 0 {
		go func() {
			ticker := time.NewTicker(c.Config.Deploy.ReconcileInterval)
			defer ticker.Stop()
			for {
				select {
				case <-c.Context.Done():
					return
				case <-ticker.C:
					if err := archiveCoordinator.ResumeAll(c.Context); err != nil && c.Logger != nil {
						c.Logger.Warn("resume archiving projects failed", zap.Error(err))
					}
				}
			}
		}()
	}
```

import 追加 `time`、`go.uber.org/zap`、`deliveryevent ".../modules/delivery/domain/event"`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/workspace_app/... -count=1; go build ./...`
Expected: PASS / 构建成功。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/workspace_app
git commit -m "feat(workspace): archive projects with environment teardown"
```

---

### Task 10: product 订阅 —— undeploy 清空当前版本

**Files:**
- Modify: `backend/internal/modules/product/application/subscriber/on_deploy_completed.go`
- Test: `backend/internal/modules/product/application/subscriber/on_deploy_completed_test.go`

**Interfaces:**
- Consumes: `deliveryevent.DeployCompleted.RunType`

- [ ] **Step 1: 写失败测试**

```go
package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type envProjectRepo struct {
	repository.EnvironmentRepository
	row   *model.EnvironmentProject
	saved *model.EnvironmentProject
}

func (r *envProjectRepo) FindEnvironmentProject(context.Context, kernel.EnvironmentID, kernel.ProjectID) (*model.EnvironmentProject, error) {
	return r.row, nil
}
func (r *envProjectRepo) SaveEnvironmentProject(_ context.Context, row *model.EnvironmentProject) error {
	r.saved = row
	return nil
}

func TestOnDeployCompletedClearsCurrentVersionOnUndeploy(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	runID := kernel.NewDeploymentRunID()
	deployedAt := time.Now().UTC()
	repo := &envProjectRepo{row: &model.EnvironmentProject{
		EnvironmentID: envID, Enabled: true, CurrentImageTag: "v1", CurrentDeploymentRunID: &runID, LastDeployedAt: &deployedAt,
	}}
	ev := deliveryevent.NewDeployCompleted(kernel.NewDeploymentRunID(), kernel.NewProjectID(), kernel.NewWorkspaceID(), &envID, "", "succeeded", "undeploy", "v1")

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.NotNil(t, repo.saved)
	require.Equal(t, "", repo.saved.CurrentImageTag)
	require.Nil(t, repo.saved.CurrentDeploymentRunID)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend; go test ./internal/modules/product/application/subscriber/ -run Undeploy -count=1`
Expected: FAIL（`CurrentImageTag` 被写成 `v1`）。

- [ ] **Step 3: 实现**

`Handle` 中 `if ev.EnvironmentID == nil || ...` 判断之后插入：

```go
	if ev.RunType == string(deploymodel.RunTypeUndeploy) {
		row, err := s.environments.FindEnvironmentProject(ctx, *ev.EnvironmentID, ev.ProjectID)
		if err != nil {
			if err == domain.ErrEnvironmentProjectNotFound {
				return nil
			}
			return err
		}
		row.CurrentImageTag = ""
		row.CurrentDeploymentRunID = nil
		return s.environments.SaveEnvironmentProject(ctx, row)
	}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend; go test ./internal/modules/product/... -count=1`
Expected: PASS。

- [ ] **Step 5: Commit 并重启后端**

```bash
git add backend/internal/modules/product
git commit -m "feat(product): clear environment version after undeploy"
```

Run: `cd <repo-root>; .\scripts\dev-restart.ps1 -Backend`
Expected: 后端 PID 更新，`http://localhost:8080` 可访问（根路径 404 正常）。

---

### Task 11: 前端契约 / 上游 / BFF / SDK

**Files:**
- Modify: `fronted/packages/contracts/src/delivery/index.ts`
- Modify: `fronted/packages/contracts/src/workspace/index.ts`
- Modify: `fronted/packages/upstream/src/delivery.ts`、`fronted/packages/upstream/src/workspace.ts`
- Modify: `fronted/packages/bff-modules/src/delivery/index.ts`
- Create: `fronted/packages/bff-modules/src/workspace/archiveProjectWithTeardown.ts`
- Modify: `fronted/packages/bff-modules/src/workspace/index.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/undeployments/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/archive/route.ts`
- Modify: `fronted/packages/web-sdk/src/delivery.ts`、`fronted/packages/web-sdk/src/workspace.ts`

**Interfaces:**
- Produces:
  - `DeploymentRunTypeSchema = z.enum(["normal", "rollback", "sync", "undeploy"])`
  - `DeploymentRunVM.deploy_namespace?: string`、`DeploymentRunVM.undeploy_options?: { delete_config: boolean; delete_storage: boolean }`
  - `UndeployCommandSchema` / `UndeployCommand = { cluster_id: string; confirm_name: string; delete_config: boolean; delete_storage: boolean }`
  - `ArchiveProjectCommandSchema` / `ArchiveProjectCommand = { confirm_name: string; teardown: boolean }`
  - `ArchiveProjectResultVM = { status: "archived" | "archiving"; pending_clusters: string[] }`
  - web-sdk：`delivery.undeploy(projectId, workspaceId, input): Promise<DeploymentRunVM>`；`workspace.archiveProjectWithTeardown(workspaceId, projectId, input): Promise<ArchiveProjectResultVM>`

- [ ] **Step 1: contracts**

`delivery/index.ts`：

```ts
export const DeploymentRunTypeSchema = z.enum(["normal", "rollback", "sync", "undeploy"]);
```

`DeploymentRunVMSchema` 中 `rollback_of_run_id` 后追加：

```ts
  deploy_namespace: z.string().optional(),
  undeploy_options: z
    .object({ delete_config: z.boolean(), delete_storage: z.boolean() })
    .optional(),
```

文件末尾追加：

```ts
export const UndeployCommandSchema = z.object({
  cluster_id: z.string().min(1),
  confirm_name: z.string().min(1),
  delete_config: z.boolean().default(false),
  delete_storage: z.boolean().default(false),
});

export type UndeployCommand = z.infer<typeof UndeployCommandSchema>;
```

`workspace/index.ts` 末尾追加，并把 `PROJECT_STATUS_OPTIONS` 改为三项：

```ts
export const PROJECT_STATUS_OPTIONS = [
  { value: "active", label: "活跃" },
  { value: "archiving", label: "归档中" },
  { value: "archived", label: "已归档" },
] as const;

export const ArchiveProjectCommandSchema = z.object({
  confirm_name: z.string().min(1),
  teardown: z.boolean().default(false),
});

export type ArchiveProjectCommand = z.infer<typeof ArchiveProjectCommandSchema>;

export const ArchiveProjectResultVMSchema = z.object({
  status: z.enum(["archived", "archiving"]),
  pending_clusters: z.array(z.string()).default([]),
});

export type ArchiveProjectResultVM = z.infer<typeof ArchiveProjectResultVMSchema>;
```

- [ ] **Step 2: upstream**

`upstream/src/delivery.ts`：import 追加 `UndeployCommand`；在 `rollbackDeployment` 之后追加：

```ts
    async undeploy(workspaceId: string, projectId: string, input: UndeployCommand) {
      const data = await upstreamFetch<unknown>(`/api/v1/projects/${projectId}/undeployments`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
      return normalizeDeploymentRun(data);
    },
```

`upstream/src/workspace.ts` 在 `archiveProject` 之后追加：

```ts
    async archiveProjectWithTeardown(
      workspaceId: string,
      projectId: string,
      input: import("@sparkops/contracts/workspace").ArchiveProjectCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/workspace").ArchiveProjectResultVM>(
        `/api/v1/projects/${projectId}/archive`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },
```

- [ ] **Step 3: bff-modules**

`bff-modules/src/delivery/index.ts` import 追加 `UndeployCommand`，在 `rollbackDeployment` 后追加：

```ts
export async function undeploy(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: UndeployCommand,
): Promise<DeploymentRunVM> {
  return ctx.upstream.delivery.undeploy(workspaceId, projectId, input);
}
```

`bff-modules/src/workspace/archiveProjectWithTeardown.ts`（参照同目录 `archiveProject.ts` 的 import 写法）：

```ts
import type { BffContext } from "@sparkops/bff-core";
import type { ArchiveProjectCommand } from "@sparkops/contracts/workspace";

export async function archiveProjectWithTeardown(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: ArchiveProjectCommand,
) {
  return ctx.upstream.workspace.archiveProjectWithTeardown(workspaceId, projectId, input);
}
```

> 若 `archiveProject.ts` 中 `BffContext` 的 import 路径不是 `@sparkops/bff-core`，与其保持一致。

`bff-modules/src/workspace/index.ts` 追加 `export { archiveProjectWithTeardown } from "./archiveProjectWithTeardown";`。

- [ ] **Step 4: Next BFF 路由**

`.../[projectId]/undeployments/route.ts`：

```ts
import { createBffHandler } from "@sparkops/bff-core";
import { undeploy } from "@sparkops/bff-modules/delivery";
import { UndeployCommandSchema } from "@sparkops/contracts/delivery";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  const input = UndeployCommandSchema.parse(await req.json());
  return undeploy(ctx, wsId, projectId, input);
});
```

`.../[projectId]/archive/route.ts`：

```ts
import { createBffHandler } from "@sparkops/bff-core";
import { archiveProjectWithTeardown } from "@sparkops/bff-modules/workspace";
import { ArchiveProjectCommandSchema } from "@sparkops/contracts/workspace";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  const input = ArchiveProjectCommandSchema.parse(await req.json());
  return archiveProjectWithTeardown(ctx, wsId, projectId, input);
});
```

- [ ] **Step 5: web-sdk**

`web-sdk/src/delivery.ts` import 追加 `UndeployCommand`，在 `rollbackDeployment` 后追加：

```ts
export function undeploy(projectId: string, workspaceId: string, input: UndeployCommand) {
  return bffFetch<DeploymentRunVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/undeployments`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}
```

`web-sdk/src/workspace.ts` 在 `archiveProject` 后追加：

```ts
export function archiveProjectWithTeardown(
  workspaceId: string,
  projectId: string,
  input: import("@sparkops/contracts/workspace").ArchiveProjectCommand,
) {
  return bffFetch<import("@sparkops/contracts/workspace").ArchiveProjectResultVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/archive`,
    { method: "POST", body: JSON.stringify(input), ...withWorkspace(workspaceId) },
  );
}
```

- [ ] **Step 6: 类型检查**

Run: `cd fronted; pnpm typecheck`
Expected: 无新增错误（若已有基线错误，确认未增加）。

- [ ] **Step 7: Commit**

```bash
git add fronted/packages fronted/apps/web/app/api
git commit -m "feat(web): add undeploy and archive teardown client APIs"
```

---

### Task 12: 环境状态逻辑与文案

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/project-environment-status.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/project-environment-status.test.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/deployment-console-utils.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/deployment-console-utils.test.ts`

**Interfaces:**
- Produces:
  - `isRunOnline(run: DeploymentRunVM | null | undefined): boolean`
  - `onlineEnvironmentRows(rows: EnvironmentDeployRow[]): EnvironmentDeployRow[]`
  - `environmentStatusLabel` / `environmentStatusTone` / `environmentVersionLabel` 支持 undeploy
  - `deploymentRunTitle` 对 undeploy 前缀「下线 · 」；`deploymentRunKindLabel` 返回「下线」

- [ ] **Step 1: 写失败测试**

`project-environment-status.test.ts` 末尾追加（import 中补 `environmentStatusTone`、`isRunOnline`、`onlineEnvironmentRows`）：

```ts
describe("undeploy runs", () => {
  const undeployed = {
    id: "u1", cluster_id: "dev", status: "succeeded", run_type: "undeploy", image_tag: "v1", git_ref_value: "main",
  } as DeploymentRunVM;

  it("treats succeeded undeploy as offline", () => {
    expect(isRunOnline(undeployed)).toBe(false);
    expect(environmentStatusLabel(undeployed)).toBe("已下线");
    expect(environmentStatusTone(undeployed)).toBe("neutral");
    expect(environmentVersionLabel(undeployed)).toBe("—");
  });

  it("labels running and failed undeploy", () => {
    expect(environmentStatusLabel({ ...undeployed, status: "deploying" })).toBe("下线中");
    expect(environmentStatusTone({ ...undeployed, status: "deploying" })).toBe("running");
    expect(environmentStatusLabel({ ...undeployed, status: "failed" })).toBe("下线失败");
    expect(environmentStatusTone({ ...undeployed, status: "failed" })).toBe("failed");
  });

  it("keeps normal succeeded run online", () => {
    const run = { ...undeployed, run_type: "normal" } as DeploymentRunVM;
    expect(isRunOnline(run)).toBe(true);
    expect(onlineEnvironmentRows([
      { clusterId: "dev", clusterName: "dev", latest: run },
      { clusterId: "uat", clusterName: "uat", latest: undeployed },
      { clusterId: "prod", clusterName: "prod", latest: null },
    ]).map((row) => row.clusterId)).toEqual(["dev"]);
  });
});
```

`deployment-console-utils.test.ts` 末尾追加（import 补 `deploymentRunKindLabel` 如未导入）：

```ts
describe("undeploy titles", () => {
  it("prefixes undeploy runs", () => {
    const run = { id: "abcdef123", run_type: "undeploy", git_ref_value: "main", image_tag: "v1" } as DeploymentRunVM;
    expect(deploymentRunTitle(run)).toBe("下线 · main · v1");
    expect(deploymentRunKindLabel(run)).toBe("下线");
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/workspace/components/project/project-environment-status.test.ts apps/web/src/modules/workspace/components/project/deployment-console-utils.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现**

`project-environment-status.ts` 替换三个 label/tone/version 函数并新增：

```ts
export function isRunOnline(run: DeploymentRunVM | null | undefined): boolean {
  return Boolean(run && run.status === "succeeded" && run.run_type !== "undeploy");
}

export function onlineEnvironmentRows(rows: EnvironmentDeployRow[]) {
  return rows.filter((row) => isRunOnline(row.latest));
}

function isUndeployRun(run: DeploymentRunVM | null): run is DeploymentRunVM {
  return run?.run_type === "undeploy";
}

export function environmentVersionLabel(run: DeploymentRunVM | null) {
  if (!run) return "—";
  if (isUndeployRun(run) && run.status === "succeeded") return "—";
  const tag = run.image_tag?.trim();
  const ref = run.git_ref_value?.trim();
  if (tag && ref) return tag === ref ? tag : `${ref} · ${tag}`;
  if (tag) return tag;
  if (ref) return ref;
  return run.id.slice(0, 8);
}

export function environmentStatusLabel(run: DeploymentRunVM | null) {
  if (!run) return "未部署";
  if (isUndeployRun(run)) {
    if (run.status === "succeeded") return "已下线";
    if (run.status === "failed") return "下线失败";
    if (run.status === "aborted") return "已中止";
    return "下线中";
  }
  return deploymentRunStatusLabel(run.status);
}

export function environmentStatusTone(run: DeploymentRunVM | null) {
  if (!run) return "neutral";
  if (isUndeployRun(run) && run.status === "succeeded") return "neutral";
  return deploymentStatusTone(run.status);
}
```

`deployment-console-utils.ts`：

```ts
export function deploymentRunTitle(run: DeploymentRunVM) {
  const ref = run.git_ref_value?.trim();
  const tag = run.image_tag?.trim();
  const prefix =
    run.run_type === "rollback"
      ? "回滚 · "
      : run.run_type === "sync"
        ? "同步 · "
        : run.run_type === "undeploy"
          ? "下线 · "
          : "";
  if (ref && tag) return `${prefix}${ref} · ${tag}`;
  if (ref) return `${prefix}${ref}`;
  if (tag) return `${prefix}${tag}`;
  return `${prefix}${run.id.slice(0, 8)}`;
}
```

`deploymentRunKindLabel` 第一行前插入 `if (run.run_type === "undeploy") return "下线";`。

- [ ] **Step 4: 运行确认通过**

Run: 同 Step 2。Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add fronted/apps/web/src/modules/workspace/components/project
git commit -m "feat(web): show offline environment status for undeploy runs"
```

---

### Task 13: 运行摘要 reload、环境行下线按钮与 `UndeployModal`

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/useProjectRunSummary.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectEnvironmentStatus.tsx`
- Create: `fronted/apps/web/src/modules/workspace/components/project/UndeployModal.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectOverviewCard.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectCard.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectDeploymentConsolePanel.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectServiceDetailPage.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes: Task 11 `delivery.undeploy`、`UndeployCommand`；Task 12 `isRunOnline`、`onlineEnvironmentRows`、`buildEnvironmentDeployRows`
- Produces:
  - `ProjectRunSummary.reload: () => void`
  - `ProjectEnvironmentStatus` 新 prop `onUndeploy?: (row: EnvironmentDeployRow) => void`
  - `UndeployModal` props：

```ts
interface UndeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  rows: EnvironmentDeployRow[];   // 所有环境行；组件内只展示在线行
  initialClusterId?: string;
  onClose: () => void;
  onStarted: (run: DeploymentRunVM) => void;
}
```

- [ ] **Step 1: `useProjectRunSummary` 加 reload 与统计排除**

- 新增 `const [reloadKey, setReloadKey] = useState(0);`，把 `reloadKey` 加入主 `useEffect` 依赖数组。
- `listDeployments` 成功分支改为：

```ts
      .then((page) => {
        if (cancelled) return;
        const releaseItems = page.items.filter((item) => item.run_type !== "undeploy");
        setDeployment({
          total: page.total - (page.items.length - releaseItems.length),
          latest: releaseItems[0] ?? null,
          items: page.items,
          loading: false,
          error: null,
        });
      })
```

- `ProjectRunSummary` 接口加 `reload: () => void;`；返回值改为：

```ts
  const reload = useCallback(() => setReloadKey((value) => value + 1), []);
  return useMemo(
    () => ({ deployment, build, environments, reload }),
    [deployment, build, environments, reload],
  );
```

（import 补 `useCallback`。`environments` 仍使用完整 `deployment.items`，保证环境行能看到 undeploy run。）

- [ ] **Step 2: `ProjectEnvironmentStatus` 在线计数与 hover 按钮**

```tsx
"use client";

import {
  environmentStatusLabel,
  environmentStatusTone,
  environmentVersionLabel,
  isRunOnline,
  type EnvironmentDeployRow,
} from "./project-environment-status";

interface ProjectEnvironmentStatusProps {
  items: EnvironmentDeployRow[];
  loading: boolean;
  error: string | null;
  onUndeploy?: (row: EnvironmentDeployRow) => void;
}

function StatusBadge({ label, tone }: { label: string; tone: string }) {
  return (
    <span className={`project-status-badge is-${tone}`}>
      <span className="project-status-badge-dot" aria-hidden />
      {label}
    </span>
  );
}

export function ProjectEnvironmentStatus({
  items,
  loading,
  error,
  onUndeploy,
}: ProjectEnvironmentStatusProps) {
  const deployedCount = items.filter((item) => isRunOnline(item.latest)).length;

  return (
    <section className="project-overview-panel project-env-panel" aria-label="环境部署">
      <header className="project-overview-panel-head">
        <h4 className="project-overview-panel-title">环境部署</h4>
        {!loading && !error && items.length > 0 ? (
          <span className="project-overview-panel-meta">
            {deployedCount}/{items.length} 已上线
          </span>
        ) : null}
      </header>
      {loading ? (
        <p className="project-overview-panel-empty" role="status">加载环境信息…</p>
      ) : error ? (
        <p className="project-overview-panel-empty">{error}</p>
      ) : items.length === 0 ? (
        <p className="project-overview-panel-empty">暂无可用部署环境</p>
      ) : (
        <ul className="project-env-list">
          {items.map((item) => {
            const statusLabel = environmentStatusLabel(item.latest);
            const statusTone = environmentStatusTone(item.latest);
            const version = environmentVersionLabel(item.latest);
            const canUndeploy = Boolean(onUndeploy) && isRunOnline(item.latest);
            return (
              <li key={item.clusterId} className={`project-env-item is-${statusTone}`}>
                <span className="project-env-chip" title={item.clusterName}>
                  {item.clusterName}
                </span>
                <span className="project-env-version" title={version}>
                  {version}
                </span>
                <StatusBadge label={statusLabel} tone={statusTone} />
                {canUndeploy ? (
                  <button
                    type="button"
                    className="project-env-undeploy"
                    aria-label={`下线 ${item.clusterName}`}
                    onClick={() => onUndeploy?.(item)}
                  >
                    下线
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
```

`globals.css` 在 `.project-env-item` 相关规则附近追加：

```css
.project-env-item {
  position: relative;
}

.project-env-undeploy {
  border: 0;
  background: transparent;
  color: var(--text-muted);
  font-size: 12px;
  padding: 0 var(--space-1, 4px);
  opacity: 0;
  transition: opacity 150ms ease, color 150ms ease;
  cursor: pointer;
}

.project-env-item:hover .project-env-undeploy,
.project-env-undeploy:focus-visible {
  opacity: 1;
}

.project-env-undeploy:hover {
  color: var(--danger);
}

.undeploy-resource-list {
  margin: 0;
  padding: var(--space-3, 12px);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-muted, var(--bg));
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  list-style: none;
}

.undeploy-danger-hint {
  color: var(--danger);
  font-size: 12px;
}
```

> 若 `--text-muted` / `--danger` / `--surface-muted` 不存在，替换为 `globals.css` 中已有的同义 token（`rg -- "--danger|--text-muted|--muted" fronted/apps/web/app/globals.css`）。

- [ ] **Step 3: 新建 `UndeployModal.tsx`**

```tsx
"use client";

import { useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { onlineEnvironmentRows, type EnvironmentDeployRow } from "./project-environment-status";

interface UndeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  rows: EnvironmentDeployRow[];
  initialClusterId?: string;
  onClose: () => void;
  onStarted: (run: DeploymentRunVM) => void;
}

export function UndeployModal({
  open,
  workspaceId,
  project,
  rows,
  initialClusterId,
  onClose,
  onStarted,
}: UndeployModalProps) {
  const onlineRows = useMemo(() => onlineEnvironmentRows(rows), [rows]);
  const [clusterId, setClusterId] = useState("");
  const [deleteConfig, setDeleteConfig] = useState(false);
  const [deleteStorage, setDeleteStorage] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setClusterId(initialClusterId ?? onlineRows[0]?.clusterId ?? "");
    setDeleteConfig(false);
    setDeleteStorage(false);
    setConfirmName("");
    setError(null);
  }, [open, initialClusterId, onlineRows]);

  const selected = onlineRows.find((row) => row.clusterId === clusterId) ?? null;
  const namespace = selected?.latest?.deploy_namespace?.trim() || project.namespace || "—";
  const canSubmit = Boolean(selected) && confirmName === project.name && !submitting;

  async function submit() {
    if (!selected || !canSubmit) return;
    setSubmitting(true);
    setError(null);
    try {
      const run = await delivery.undeploy(project.id, workspaceId, {
        cluster_id: selected.clusterId,
        confirm_name: confirmName,
        delete_config: deleteConfig,
        delete_storage: deleteStorage,
      });
      onStarted(run);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "下线失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => (submitting ? undefined : onClose())}
      title={selected ? `下线 ${project.name} · ${selected.clusterName}` : `下线 ${project.name}`}
      description="删除该环境中的工作负载，项目配置与历史记录保留，可随时重新发布。"
      size="md"
      footer={
        <>
          <button type="button" className="btn-secondary" disabled={submitting} onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn-danger-solid" disabled={!canSubmit} onClick={() => void submit()}>
            {submitting ? "下线中…" : "下线"}
          </button>
        </>
      }
    >
      <div className="stack-md">
        {onlineRows.length === 0 ? (
          <InlineAlert tone="info">当前没有在线环境。</InlineAlert>
        ) : (
          <>
            {onlineRows.length > 1 ? (
              <label className="stack-sm">
                <span className="text-sm font-medium">环境</span>
                <select className="input" value={clusterId} onChange={(e) => setClusterId(e.target.value)}>
                  {onlineRows.map((row) => (
                    <option key={row.clusterId} value={row.clusterId}>
                      {row.clusterName}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <div className="stack-sm">
              <span className="text-sm font-medium">将删除以下资源</span>
              <ul className="undeploy-resource-list">
                <li>Deployment/{project.name}</li>
                <li>Service/{project.name}</li>
                <li>Ingress（app={project.name}）</li>
                <li className="text-muted">命名空间：{namespace}</li>
              </ul>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={deleteConfig} onChange={(e) => setDeleteConfig(e.target.checked)} />
              同时删除配置（ConfigMap / Secret）
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={deleteStorage} onChange={(e) => setDeleteStorage(e.target.checked)} />
              同时删除存储卷（PVC）
            </label>
            {deleteStorage ? <p className="undeploy-danger-hint">PVC 删除后数据不可恢复。</p> : null}
            <label className="stack-sm">
              <span className="text-sm">
                输入项目名 <strong>{project.name}</strong> 以确认
              </span>
              <input
                className="input"
                value={confirmName}
                onChange={(e) => setConfirmName(e.target.value)}
                autoComplete="off"
                aria-label="确认项目名"
              />
            </label>
          </>
        )}
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </div>
    </Modal>
  );
}
```

> `btn-danger-solid` 已在 `TemplatePanel.tsx` 使用；`InlineAlert` 若无 `tone="info"`，改用其支持的中性 tone（查看 `src/components/ui/InlineAlert.tsx`）。

- [ ] **Step 4: 卡片接线**

`ProjectOverviewCard.tsx`：props 增加 `onUndeploy?: (row: EnvironmentDeployRow) => void;`（从 `./project-environment-status` import 类型），解构后传给 `<ProjectEnvironmentStatus ... onUndeploy={project.status === "active" ? onUndeploy : undefined} />`。

`ProjectCard.tsx`：

```tsx
"use client";

import { useState } from "react";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";
import type { EnvironmentClusterRef } from "./project-environment-status";
import { useProjectRunSummary } from "./useProjectRunSummary";
import { UndeployModal } from "./UndeployModal";

// ProjectCardProps 不变

export function ProjectCard({ /* 现有解构不变 */ }: ProjectCardProps) {
  const runSummary = useProjectRunSummary(workspaceId, project.id, {
    workspaceClusters,
    workspaceClustersLoading,
    workspaceClustersError,
  });
  const [undeployClusterId, setUndeployClusterId] = useState<string | null>(null);

  return (
    <>
      <ProjectOverviewCard
        /* 现有 props 不变 */
        onUndeploy={(row) => setUndeployClusterId(row.clusterId)}
      />
      <UndeployModal
        open={undeployClusterId !== null}
        workspaceId={workspaceId}
        project={project}
        rows={runSummary.environments.items}
        initialClusterId={undeployClusterId ?? undefined}
        onClose={() => setUndeployClusterId(null)}
        onStarted={() => {
          setUndeployClusterId(null);
          runSummary.reload();
        }}
      />
    </>
  );
}
```

- [ ] **Step 5: 部署历史标签 + 详情页入口**

`ProjectDeploymentConsolePanel.tsx` 约 230 行 `sync` 标签块之后追加：

```tsx
                        {item.run_type === "undeploy" ? (
                          <>
                            <span className="project-tag project-tag-gray">下线</span>
                            {" · "}
                          </>
                        ) : null}
```

并确认该文件中「回滚到此版本」类按钮在 `run.run_type === "undeploy"` 时不渲染（`rg "回滚到" ProjectDeploymentConsolePanel.tsx`；若存在，加 `run.run_type !== "undeploy" &&` 条件）。

`ProjectServiceDetailPage.tsx`：
- import：`import { delivery } from "@sparkops/web-sdk";`（与已有 `cluster` 合并）、`import { UndeployModal } from "./UndeployModal";`、`import { buildEnvironmentDeployRows, onlineEnvironmentRows, type EnvironmentDeployRow } from "./project-environment-status";`
- 状态：

```tsx
  const [undeployOpen, setUndeployOpen] = useState(false);
  const [undeployRows, setUndeployRows] = useState<EnvironmentDeployRow[]>([]);

  const openUndeploy = useCallback(async () => {
    const options = await delivery.getDeploymentOptions(project.id, workspaceId).catch(() => null);
    const clusters = (options?.clusters ?? []).map((item) => ({
      id: item.id,
      cluster_name: item.cluster_name,
      is_build_env: item.is_build_env,
      priority_id: item.priority_id,
    }));
    setUndeployRows(buildEnvironmentDeployRows(clusters, deploymentPage?.items ?? []));
    setUndeployOpen(true);
  }, [deploymentPage?.items, project.id, workspaceId]);
```

- `moreMenuItems` 中 `scale` 之后加入：

```tsx
      {
        key: "undeploy",
        label: "下线环境",
        disabled: !canScale || hasActiveRun,
        title: !canScale ? "当前没有在线环境" : hasActiveRun ? "部署进行中" : undefined,
        onClick: () => void openUndeploy(),
      },
```

并把 `hasActiveRun`、`openUndeploy` 加入其 `useMemo` 依赖。
- 在 `<RuntimeRollbackModal ... />` 之后渲染：

```tsx
      <UndeployModal
        open={undeployOpen}
        workspaceId={workspaceId}
        project={project}
        rows={undeployRows}
        onClose={() => setUndeployOpen(false)}
        onStarted={(run) => {
          setUndeployOpen(false);
          onRunChanged?.(run);
          onViewDeployProgress();
        }}
      />
```

- [ ] **Step 6: 验证**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/workspace/components/project; pnpm typecheck`
Expected: 测试 PASS，类型检查无新增错误。

- [ ] **Step 7: Commit**

```bash
git add fronted/apps/web
git commit -m "feat(web): undeploy environments from project card and detail page"
```

---

### Task 14: `ArchiveProjectModal` 与归档入口替换

**Files:**
- Create: `fronted/apps/web/src/modules/workspace/components/project/archive-progress.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/archive-progress.test.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/ArchiveProjectModal.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/ProjectPanel.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/ProjectSettings.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectOverviewCard.tsx`

**Interfaces:**
- Consumes: Task 11 `workspace.archiveProjectWithTeardown`、`workspace.getProject`、`delivery.listDeployments`、`delivery.getDeploymentOptions`；Task 12 `isRunOnline`、`latestRunByCluster`
- Produces:

```ts
export type ArchiveStepState = "waiting" | "running" | "done" | "failed";
export interface ArchiveStep { clusterId: string; clusterName: string; state: ArchiveStepState; error?: string }
export function archiveSteps(
  pending: { clusterId: string; clusterName: string }[],
  runs: DeploymentRunVM[],
  startedAt: string,
): ArchiveStep[];
```

`ArchiveProjectModal` props：

```ts
interface ArchiveProjectModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM | null;
  onClose: () => void;
  onArchived: () => void;
}
```

- [ ] **Step 1: 写失败测试**

`archive-progress.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import { archiveSteps } from "./archive-progress";

const pending = [
  { clusterId: "dev", clusterName: "DEV" },
  { clusterId: "uat", clusterName: "UAT" },
];
const startedAt = "2026-10-11T02:00:00Z";

function run(partial: Partial<DeploymentRunVM>): DeploymentRunVM {
  return {
    id: "r", cluster_id: "dev", run_type: "undeploy", status: "succeeded",
    created_at: "2026-10-11T02:00:05Z", error_message: "", ...partial,
  } as DeploymentRunVM;
}

describe("archiveSteps", () => {
  it("marks clusters waiting before any undeploy run", () => {
    expect(archiveSteps(pending, [], startedAt).map((s) => s.state)).toEqual(["waiting", "waiting"]);
  });

  it("tracks running, done and failed undeploy runs created after start", () => {
    const runs = [
      run({ id: "u2", cluster_id: "uat", status: "failed", error_message: "forbidden", created_at: "2026-10-11T02:01:00Z" }),
      run({ id: "u1", cluster_id: "dev", status: "succeeded" }),
    ];
    const steps = archiveSteps(pending, runs, startedAt);
    expect(steps.map((s) => s.state)).toEqual(["done", "failed"]);
    expect(steps[1].error).toBe("forbidden");
  });

  it("ignores undeploy runs created before archive started", () => {
    const runs = [run({ created_at: "2026-10-11T01:59:00Z" })];
    expect(archiveSteps(pending, runs, startedAt)[0].state).toBe("waiting");
  });

  it("shows running for active undeploy", () => {
    const runs = [run({ status: "deploying" })];
    expect(archiveSteps(pending, runs, startedAt)[0].state).toBe("running");
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/workspace/components/project/archive-progress.test.ts`
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 实现 `archive-progress.ts`**

```ts
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";

export type ArchiveStepState = "waiting" | "running" | "done" | "failed";

export interface ArchiveStep {
  clusterId: string;
  clusterName: string;
  state: ArchiveStepState;
  error?: string;
}

export function archiveSteps(
  pending: { clusterId: string; clusterName: string }[],
  runs: DeploymentRunVM[],
  startedAt: string,
): ArchiveStep[] {
  const since = Date.parse(startedAt);
  return pending.map((item) => {
    const latest = runs.find(
      (run) =>
        run.cluster_id === item.clusterId &&
        run.run_type === "undeploy" &&
        Date.parse(run.created_at) >= since,
    );
    if (!latest) return { ...item, state: "waiting" };
    if (latest.status === "succeeded") return { ...item, state: "done" };
    if (latest.status === "failed" || latest.status === "aborted") {
      return { ...item, state: "failed", error: latest.error_message || undefined };
    }
    return { ...item, state: "running" };
  });
}
```

> `startedAt` 由前端在提交前取 `new Date(Date.now() - 5000).toISOString()`，容忍客户端与服务端少量时钟偏差。

- [ ] **Step 4: 实现 `ArchiveProjectModal.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery, workspace } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { archiveSteps, type ArchiveStep } from "./archive-progress";
import {
  buildEnvironmentDeployRows,
  onlineEnvironmentRows,
  type EnvironmentDeployRow,
} from "./project-environment-status";
import { projectDisplayName } from "./project-utils";

interface ArchiveProjectModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM | null;
  onClose: () => void;
  onArchived: () => void;
}

type Phase = "loading" | "confirm" | "progress" | "failed" | "done";

const STEP_LABEL: Record<ArchiveStep["state"], string> = {
  waiting: "等待",
  running: "下线中",
  done: "已下线",
  failed: "失败",
};

const STEP_TONE: Record<ArchiveStep["state"], string> = {
  waiting: "neutral",
  running: "running",
  done: "success",
  failed: "failed",
};

export function ArchiveProjectModal({ open, workspaceId, project, onClose, onArchived }: ArchiveProjectModalProps) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [online, setOnline] = useState<EnvironmentDeployRow[]>([]);
  const [confirmName, setConfirmName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [steps, setSteps] = useState<ArchiveStep[]>([]);
  const startedAtRef = useRef("");

  useEffect(() => {
    if (!open || !project) return;
    let cancelled = false;
    setPhase("loading");
    setConfirmName("");
    setError(null);
    setSteps([]);
    void Promise.all([
      delivery.listDeployments(project.id, workspaceId),
      delivery.getDeploymentOptions(project.id, workspaceId).catch(() => ({ clusters: [] })),
    ])
      .then(([page, options]) => {
        if (cancelled) return;
        const clusters = options.clusters.map((item) => ({
          id: item.id,
          cluster_name: item.cluster_name,
          is_build_env: item.is_build_env,
          priority_id: item.priority_id,
        }));
        setOnline(onlineEnvironmentRows(buildEnvironmentDeployRows(clusters, page.items)));
        setPhase("confirm");
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof BffError ? err.message : "加载环境信息失败");
        setOnline([]);
        setPhase("confirm");
      });
    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId]);

  const poll = useCallback(async () => {
    if (!project) return;
    let status: string | null = null;
    try {
      status = (await workspace.getProject(workspaceId, project.id)).status;
    } catch (err) {
      if (err instanceof BffError && err.code === "WORKSPACE_APP_PROJECT_NOT_FOUND") status = "archived";
    }
    const page = await delivery.listDeployments(project.id, workspaceId).catch(() => null);
    const runs: DeploymentRunVM[] = page?.items ?? [];
    const pending = online.map((row) => ({ clusterId: row.clusterId, clusterName: row.clusterName }));
    const next = archiveSteps(pending, runs, startedAtRef.current);
    setSteps(next);
    if (status === "archived") {
      setPhase("done");
      onArchived();
    } else if (status === "active") {
      setPhase("failed");
      setError(next.find((step) => step.state === "failed")?.error ?? "归档已中止，请查看部署历史中的下线记录");
    }
  }, [online, onArchived, project, workspaceId]);

  useEffect(() => {
    if (phase !== "progress") return;
    const timer = window.setInterval(() => void poll(), 3000);
    return () => window.clearInterval(timer);
  }, [phase, poll]);

  if (!project) return null;
  const hasOnline = online.length > 0;
  const canSubmit = phase === "confirm" && confirmName === project.name && !submitting;

  async function submit() {
    if (!project || !canSubmit) return;
    setSubmitting(true);
    setError(null);
    startedAtRef.current = new Date(Date.now() - 5000).toISOString();
    try {
      const result = await workspace.archiveProjectWithTeardown(workspaceId, project.id, {
        confirm_name: confirmName,
        teardown: hasOnline,
      });
      if (result.status === "archived") {
        setPhase("done");
        onArchived();
        return;
      }
      setSteps(archiveSteps(online.map((row) => ({ clusterId: row.clusterId, clusterName: row.clusterName })), [], startedAtRef.current));
      setPhase("progress");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "归档失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  const title = `归档项目「${projectDisplayName(project)}」`;
  const footer =
    phase === "confirm" || phase === "loading" ? (
      <>
        <button type="button" className="btn-secondary" disabled={submitting} onClick={onClose}>
          取消
        </button>
        <button type="button" className="btn-danger-solid" disabled={!canSubmit} onClick={() => void submit()}>
          {submitting ? "提交中…" : hasOnline ? "下线全部环境并归档" : "归档"}
        </button>
      </>
    ) : (
      <button type="button" className="btn-secondary" onClick={onClose}>
        {phase === "progress" ? "后台继续，关闭" : "关闭"}
      </button>
    );

  return (
    <Modal open={open} onClose={() => (submitting ? undefined : onClose())} title={title} size="md" footer={footer}>
      <div className="stack-md">
        {phase === "loading" ? <p className="text-sm text-muted" role="status">检查在线环境…</p> : null}
        {phase === "confirm" ? (
          <>
            {hasOnline ? (
              <div className="stack-sm">
                <p className="text-sm">将依次下线以下环境，完成后禁用 Jenkins 流水线并归档：</p>
                <ul className="undeploy-resource-list">
                  {online.map((row) => (
                    <li key={row.clusterId}>{row.clusterName}</li>
                  ))}
                </ul>
              </div>
            ) : (
              <p className="text-sm">项目没有在线环境。归档将禁用 Jenkins 流水线，项目移出列表。</p>
            )}
            <label className="stack-sm">
              <span className="text-sm">
                输入项目名 <strong>{project.name}</strong> 以确认
              </span>
              <input
                className="input"
                value={confirmName}
                onChange={(e) => setConfirmName(e.target.value)}
                autoComplete="off"
                aria-label="确认项目名"
              />
            </label>
          </>
        ) : null}
        {phase === "progress" || phase === "failed" || phase === "done" ? (
          <ul className="project-env-list" role="status">
            {steps.map((step) => (
              <li key={step.clusterId} className={`project-env-item is-${STEP_TONE[step.state]}`}>
                <span className="project-env-chip">{step.clusterName}</span>
                <span className={`project-status-badge is-${STEP_TONE[step.state]}`}>
                  <span className="project-status-badge-dot" aria-hidden />
                  {STEP_LABEL[step.state]}
                </span>
              </li>
            ))}
          </ul>
        ) : null}
        {phase === "done" ? <InlineAlert tone="success">项目已归档。</InlineAlert> : null}
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </div>
    </Modal>
  );
}
```

> `InlineAlert` 若无 `success` tone，改用其支持的成功态 tone。`workspace.getProject` 返回 `ProjectVM`，已有 `status` 字段。

- [ ] **Step 5: 替换入口**

`ProjectPanel.tsx`：
- import `ArchiveProjectModal`；新增 `const [archiveProject, setArchiveProject] = useState<ProjectVM | null>(null);`
- `onArchive` 函数改为：

```tsx
  function onArchive(item: ProjectVM) {
    setArchiveProject(item);
  }
```

- `onArchive={(p) => void onArchive(p)}` 改为 `onArchive={onArchive}`；表格中 `onClick={() => void onArchive(project)}` 改为 `onClick={() => onArchive(project)}`。
- 在 `<ProjectReviewModal ... />` 之后加：

```tsx
      <ArchiveProjectModal
        open={Boolean(archiveProject)}
        workspaceId={workspaceId}
        project={archiveProject}
        onClose={() => {
          setArchiveProject(null);
          void load();
        }}
        onArchived={() => void load()}
      />
```

- 表格「发布」按钮在 `project.status === "archiving"` 时禁用：`<button type="button" disabled={project.status === "archiving"} ...>`。

`ProjectSettings.tsx` `DeleteProjectButton`：去掉 `useConfirmDialog`、`loading`、`error` 状态与 `onDelete`，改为：

```tsx
export function DeleteProjectButton({ workspaceId, projectId, projectName }: DeleteProjectButtonProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState<ProjectVM | null>(null);

  async function openModal() {
    setProject(await workspace.getProject(workspaceId, projectId));
    setOpen(true);
  }

  return (
    <>
      <div className="flex flex-col items-end gap-2">
        <button
          className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
          type="button"
          onClick={() => void openModal()}
          aria-label={`归档项目 ${projectName}`}
        >
          删除项目
        </button>
      </div>
      <ArchiveProjectModal
        open={open}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setOpen(false)}
        onArchived={() => {
          router.push("/dashboard");
          router.refresh();
        }}
      />
    </>
  );
}
```

（import `ProjectVM`、`ArchiveProjectModal`；若 `useConfirmDialog` 不再被该文件其它组件使用则移除 import。）

`ProjectOverviewCard.tsx`：
- `statusLabelTagClass` 首行加 `if (label === "归档中") return "project-tag project-tag-orange";`
- 计算 `const archiving = project.status === "archiving";`，标题标签改为 `<span className={statusLabelTagClass(archiving ? "归档中" : statusLabel)}>{archiving ? "归档中" : statusLabel}</span>`
- `MoreMenu` 仅在 `onArchive && !archiving` 时渲染；发布按钮 `onDeploy` 分支加 `disabled={archiving}`。

- [ ] **Step 6: 验证**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/workspace; pnpm typecheck`
Expected: PASS / 无新增类型错误。

- [ ] **Step 7: Commit**

```bash
git add fronted/apps/web
git commit -m "feat(web): archive projects with environment teardown modal"
```

---

### Task 15: 端到端手测

**Files:** 无代码变更（发现问题回到对应 Task 修复）。

- [ ] **Step 1:** 重启：`.\scripts\dev-restart.ps1 -Backend`；前端如未运行：`.\scripts\dev-restart.ps1 -Frontend`。
- [ ] **Step 2:** 选一个已发布到 DEV 的项目，项目卡 hover DEV 行 → 点「下线」→ 输入项目名 → 下线。期望：卡片 DEV 行变「下线中」→「已下线」，「N/M 已上线」减 1；集群中 `kubectl -n <ns> get deploy,svc,ing -l app=<name>` 为空，ConfigMap/PVC 仍在。
- [ ] **Step 3:** 详情页 → 更多 → 回滚 / 发布，确认能重新上线；部署历史出现「下线」标签。
- [ ] **Step 4:** 再次下线并勾选「删除存储卷」，确认 PVC 被删、红色警示出现。
- [ ] **Step 5:** 对有在线环境的项目点「归档」→ 弹窗列出在线环境 → 「下线全部环境并归档」。期望：弹窗逐环境进度；完成后项目从列表消失；Jenkins 中 `<namespace>/<name>` Job 为 disabled。
- [ ] **Step 6:** 构造失败（例如临时撤销集群 kubeconfig 权限）再归档。期望：项目回到「活跃」，弹窗显示失败原因。
- [ ] **Step 7:** 调用旧接口 `DELETE /api/v1/projects/:id`（有在线环境）期望 409 `WORKSPACE_APP_PROJECT_HAS_ONLINE_ENVIRONMENTS`。
- [ ] **Step 8:** 全量回归：`cd backend; go test ./... -count=1`；`cd fronted; pnpm vitest run; pnpm typecheck`。
