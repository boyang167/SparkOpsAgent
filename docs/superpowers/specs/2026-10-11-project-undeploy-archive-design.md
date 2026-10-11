# 项目环境下线与归档联动设计

日期：2026-10-11
状态：待评审

## 1. 背景与目标

当前项目已具备完整发布链路（prepare → Jenkins → render → K8s apply），但缺少反向操作：

- 「归档」只把 `workspace_app.projects.status` 改为 `archived`，不触碰集群资源、不发事件、不拦截在线环境，导致归档后工作负载继续运行且无人管理。
- 没有按环境（集群）撤下工作负载的能力；项目卡「N/M 已上线」只会因发布而增加。

目标：

1. 提供**环境级下线**：删除某集群上该项目的 Deployment / Service / Ingress，ConfigMap / Secret / PVC 默认保留、可勾选一并删除；环境状态回到「已下线」，并留下可追溯的记录。
2. **归档联动**：存在在线环境时，提供「下线全部环境并归档」，全部下线成功后才归档；归档时禁用 Jenkins Job，Harbor 保留。
3. 下线与归档均需**危险弹窗 + 输入项目名确认**。

非目标：

- 取消归档（unarchive）。
- 删除 Jenkins Job / Harbor 仓库。
- 删除命名空间。
- 停用（缩容到 0）作为独立功能。

## 2. 方案选择

| 方案 | 说明 | 结论 |
|---|---|---|
| A | 下线作为 `DeploymentRun` 的 `undeploy` 类型，复用编排器、队列、租约、项目级互斥、SSE 控制台与 `DeployCompleted` 事件 | **采用** |
| B | runtime 模块新建独立 `undeployments` 表与同步 API | 项目卡需合并两个数据源；互斥需另做；历史分散 |
| C | 复用 runtime `ProjectClusterAccess` 直接同步删除 | runtime 只认项目默认集群/命名空间，按 Product Environment 部署到其它命名空间时可能删错；项目卡状态不变 |

选择 A 的关键理由：项目卡环境状态本就是「每个集群最近一次 run」（`latestRunByCluster`），下线成功的 run 天然成为最新状态；下线与发布天然互斥；审计复用现有历史。

## 3. 领域模型

### 3.1 `DeploymentRun` 扩展

文件：`backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go`

- 新增 `RunTypeUndeploy RunType = "undeploy"`。
- 新增值对象：

```go
type UndeployOptions struct {
	DeleteConfig  bool `json:"delete_config"`  // ConfigMap + Secret
	DeleteStorage bool `json:"delete_storage"` // PVC
}
```

- `DeploymentRun` 新增字段 `UndeployOptions *UndeployOptions`。
- 新增构造函数 `NewUndeployRun(wsID, projectID, createdBy, baseline *DeploymentRun, opts UndeployOptions)`：
  - `baseline` = 该集群最近一次 `succeeded` 且 `RunType != undeploy` 的 run。
  - 从 baseline 复制 `ClusterID`、`EnvironmentID`、`DeployNamespace`、`GitRefType`、`GitRefValue`、`ImageTag`、`ImageRepository`，保证删除位置与部署位置一致。
  - `CrossCluster = false`，`Status = queued`，`RunType = undeploy`。
- `IsUndeploy()`；`SkipsBuild()` 对 undeploy 返回 `true`，状态流转 `queued → deploying → succeeded | failed | aborted`，复用现有 `validTransitions`。

### 3.2 「在线」判定

一个集群对项目「在线」当且仅当：该集群最近一次 run 的 `Status == succeeded` 且 `RunType != undeploy`。后端（`deployrun` 内新增 `OnlineClusters(runs)` 纯函数）与前端（`isRunOnline(run)`）使用同一规则。

### 3.3 项目状态

文件：`backend/internal/modules/workspace_app/domain/model/project.go`

- 新增 `ProjectStatusArchiving ProjectStatus = "archiving"`。
- 新增方法：
  - `BeginArchiving()`：仅 `active` 可进入。
  - `Archive()`：允许从 `active` 或 `archiving` 进入 `archived`。
  - `AbortArchiving()`：`archiving → active`。
- `archiving` 期间 `Rename` 等修改操作沿用「非 active 拒绝」规则。

## 4. 后端流程

### 4.1 环境下线命令

- 路由：`POST /api/v1/projects/:projectId/undeployments`，权限 `PermProjectOperate`。
- 请求体：

```json
{ "cluster_id": "...", "confirm_name": "test-repo", "delete_config": false, "delete_storage": false }
```

- `UndeployHandler.Handle` 步骤：
  1. `confirm_name` 与项目名不一致 → `ErrConfirmNameMismatch`（400）。
  2. 项目非 `active` → `ErrProjectArchiving` / `ErrProjectNotActive`（409）。
  3. `HasActiveByProject` 为 true → `ErrDeploymentAlreadyRunning`（409）。
  4. `ListByProject` 求 baseline；该集群不在线 → `ErrEnvironmentNotDeployed`（409）。
  5. `NewUndeployRun` → `repo.Create` → 发布到 `messaging.TopicDeploy`（与 `RollbackDeploymentHandler` 写法一致，发布失败则标记 run failed）。
- 返回 `DeploymentRunVM`，前端据此进入控制台。

### 4.2 编排器分支

文件：`backend/internal/modules/delivery/deployrun/application/service/orchestrator.go`

- `queued` 且 `SkipsBuild()` → `deploying`（已有逻辑）。
- `deploying` 且 `IsUndeploy()` → 调用新端口 `ManifestRemover.Remove(ctx, target, spec)`，成功 → `succeeded`，失败 → `Fail(msg)`。跳过 `finalizeKubernetes`、`RolloutRestart` 与跨集群同步。
- 终态照常触发 `SetTerminalHook` → 发布 `delivery.DeployCompleted`；事件新增字段 `RunType`。

### 4.3 `ManifestRemover` 端口与 K8s 实现

端口（`runtime/domain/port/ports.go`）：

```go
type RemoveSpec struct {
	Manifests     []Manifest // 用发布同一渲染器渲染所得，用于精确定位名称
	AppLabel      string     // 项目名
	DeleteConfig  bool
	DeleteStorage bool
}

type ManifestRemover interface {
	Remove(ctx context.Context, target DeploymentTarget, spec RemoveSpec) error
}
```

实现（`runtime/infrastructure/k8s/remover.go`）：

1. 从 `Manifests` 中取 `Deployment`、`Service`，按 namespace + name 删除。
2. 在 namespace 内按标签 `app=<AppLabel>,app.kubernetes.io/managed-by=sparkops` 列出并删除 `Ingress`（覆盖 runtime `CreateRuntimeIngress` 创建的 Ingress）。
3. `DeleteConfig`：删除 `Manifests` 中的 `ConfigMap` / `Secret`，以及同标签选择器命中的 ConfigMap / Secret；**不删除** imagePullSecret（`ensureImagePullSecrets` 创建的共享凭据）。
4. `DeleteStorage`：按同标签选择器删除 PVC；若 PVC 由 runtime 存储页按名称绑定而无标签，则额外读取 Deployment（删除前）`spec.template.spec.volumes[].persistentVolumeClaim.claimName` 收集待删 PVC。
5. 删除策略 `PropagationPolicy=Background`；`NotFound` 视为成功（幂等，失败后重试可补删）。
6. 任一资源删除失败：继续尝试其余资源，最后汇总错误返回（`errors.Join`），错误信息包含 kind/name。
7. 不删除 Namespace。

渲染：编排器复用发布时的 renderer（项目模板 + baseline 的镜像信息）得到 `Manifests`；渲染失败时降级为仅按标签选择器删除 Deployment / Service / Ingress。

### 4.4 旁路调整

- `ResolveRollbackTarget`：过滤掉 `RunType == undeploy` 的 run。下线后可通过回滚到历史成功版本或重新发布恢复上线。
- product 模块 `OnDeployCompleted`：undeploy 且 succeeded 且带 `EnvironmentID` 时，清空 `environment_projects.CurrentImageTag` 与 `CurrentDeploymentRunID`。
- product_workflow / notification / outbound webhook 订阅者：按 `RunType` 区分，undeploy 不推进发布工作流阶段。
- 项目运行概况的「部署次数 / 最近部署」统计排除 undeploy（后端查询与前端 `useProjectRunSummary` 两处）。
- `CreateDeployment` / `Rollback` / `Sync`：项目处于 `archiving` 时返回 `ErrProjectArchiving`。

### 4.5 归档联动

- 新路由：`POST /api/v1/projects/:projectId/archive`，请求体 `{ "confirm_name": "...", "teardown": true }`，权限同现有归档（`projectAdmin`）。
- `ArchiveProjectHandler`（扩展）：
  1. 校验 `confirm_name`。
  2. 通过 ACL 端口查询 delivery 的在线集群列表与是否有活跃 run；有活跃 run → 409 `ErrDeploymentAlreadyRunning`。
  3. 无在线集群：`DisableJob` → `project.Archive()` → 保存。
  4. 有在线集群：`project.BeginArchiving()` 保存 → 调用 delivery 端口为第一个在线集群创建 undeploy run（`UndeployOptions` 全 false）→ 返回 `202` 与待下线集群列表。
- 订阅器 `OnUndeployCompletedForArchive`（workspace_app 模块，订阅 `delivery.DeployCompleted`）：
  - 仅处理 `RunType == undeploy` 且项目状态为 `archiving` 的事件。
  - succeeded：重新计算在线集群；仍有 → 为下一个创建 undeploy run；无 → `DisableJob` → `Archive()`。
  - failed / aborted：`AbortArchiving()`，发通知「归档中止：<集群> 下线失败：<原因>」。
  - 多集群因项目级互斥而**串行**执行，每个集群一条 run，项目卡逐行更新。
- 旧路由 `DELETE /api/v1/projects/:projectId`：无在线环境时行为不变；有在线环境返回 409 `ErrProjectHasOnlineEnvironments`。
- 服务重启恢复：`archiving` 状态的项目若无活跃 run 且仍有在线集群，由已有 `Reconciler` 周期中顺带调用 `ResumeArchiving(projectID)` 续推；无在线集群则直接完成归档。

### 4.6 禁用 Jenkins Job

- delivery Jenkins 适配器新增 `DisableJob(ctx, jobName) error`，调用 `POST /job/<name>/disable`；`404` 视为成功。
- Job 名称取自项目 deploy 配置中 prepare 阶段使用的同一命名规则。
- 禁用失败不阻塞归档：记录 warning 日志并发通知，归档照常完成。

## 5. 前端

### 5.1 环境状态

文件：`fronted/apps/web/src/modules/workspace/components/project/project-environment-status.ts`

- 新增 `isRunOnline(run)`（规则见 3.2）。
- `environmentStatusLabel`：undeploy + succeeded →「已下线」；undeploy + 进行中 →「下线中」；undeploy + failed →「下线失败」。
- `environmentStatusTone`：已下线 neutral；下线中 info；下线失败 danger。
- `environmentVersionLabel`：已下线返回「—」。
- `ProjectEnvironmentStatus.tsx` 的「N/M 已上线」改用 `isRunOnline` 计数。

### 5.2 下线入口

- 项目卡环境部署行：hover 在线行时右侧显示文字按钮「下线」（非常驻）。
- 项目详情部署控制台：每个环境卡片操作菜单新增「下线」。
- 二者打开同一个 `UndeployModal`。

### 5.3 `UndeployModal`

- 标题：「下线 {项目名} · {集群名}」。
- 资源清单：将删除的 Deployment / Service / Ingress 名称与命名空间（前端按项目名与 baseline run 的 `deploy_namespace` 展示）。
- 复选框（默认不勾）：「同时删除配置（ConfigMap / Secret）」「同时删除存储卷（PVC，数据不可恢复）」；勾选 PVC 时显示红色警示。
- 输入项目名，一致后危险按钮「下线」可用。
- 提交后切换到 `ProjectDeploymentConsolePanel` 复用 SSE 显示进度。

### 5.4 `ArchiveProjectModal`

替换 `ProjectPanel.onArchive` 与 `ProjectSettings.DeleteProjectButton` 中的 `confirm`。

- 无在线环境：说明「将禁用 Jenkins 流水线，项目移出列表」，输入项目名 → 归档。
- 有在线环境：列出在线环境，说明「将依次下线以下环境，完成后禁用流水线并归档」，主按钮「下线全部环境并归档」。
- 提交后弹窗内展示逐环境进度（下线中 / 等待 / 已下线 / 失败），关闭弹窗不影响后台执行。
- 项目卡在 `archiving` 时显示「归档中」标签并禁用发布按钮；失败恢复 `active` 时 toast 提示原因。

### 5.5 历史与文案

- 部署历史中 undeploy 显示「下线」标签；下线记录不提供「回滚到此版本」。
- `deploymentRunStatusLabel` 结合 `run_type` 输出下线相关文案。

### 5.6 SDK / BFF / contracts

- `@sparkops/contracts/delivery`：`DeploymentRunVM.run_type` 增加 `"undeploy"`，新增 `undeploy_options`。
- `@sparkops/contracts/workspace`：`ProjectVM.status` 增加 `"archiving"`。
- `fronted/packages/web-sdk/src/delivery.ts`：`undeploy(projectId, workspaceId, input)`。
- web-sdk workspace：`archiveProjectWithTeardown(workspaceId, projectId, confirmName)`。
- `fronted/packages/bff-modules`：对应透传路由。

## 6. 错误处理

| 情况 | 行为 |
|---|---|
| 项目名不一致 | 400 `ErrConfirmNameMismatch` |
| 已有发布或下线进行中 | 409 `ErrDeploymentAlreadyRunning`，前端提示稍后重试 |
| 目标集群不在线 | 409 `ErrEnvironmentNotDeployed` |
| 集群不可达或删除出错 | run 置 `failed`，`ErrorMessage` 记录 kind/name 与错误；已删资源不回滚，重试幂等补删 |
| `archiving` 期间发起发布 / 回滚 / 同步 | 409 `ErrProjectArchiving` |
| 归档中某次下线失败 | 项目恢复 `active`，已下线环境保持已下线，发通知 |
| 禁用 Jenkins Job 失败 | 不阻塞归档，记录 warning 并通知 |
| 旧 `DELETE /projects/:id` 且有在线环境 | 409 `ErrProjectHasOnlineEnvironments` |

## 7. 数据迁移

新迁移 `0000NN_deployment_undeploy`：

- `delivery.deployment_runs` 新增 `undeploy_options JSONB NULL`。
- `run_type` 为 `VARCHAR(16)` 且无 CHECK 约束，`undeploy` 无需改约束。
- `workspace_app.projects.status` 为 `VARCHAR(32)`，`archiving` 无需改约束（实现时复核无 CHECK）。
- 无历史数据迁移。

## 8. 测试

- 领域：`NewUndeployRun` 字段复制；`SkipsBuild`；`ResolveRollbackTarget` 排除 undeploy；`OnlineClusters` 规则；`Project.BeginArchiving / Archive / AbortArchiving` 状态迁移。
- 命令：`UndeployHandler` 名称校验、互斥、不在线拒绝、项目 archiving 拒绝；`ArchiveProjectHandler` 无在线直接归档 / 有在线进入 archiving 两路径。
- 编排器：fake `ManifestRemover` 覆盖 undeploy 成功、失败、渲染失败降级。
- K8s Remover：`client-go` fake clientset 验证按名称删除、按标签删除 Ingress、开关控制 ConfigMap / Secret / PVC、NotFound 幂等、错误汇总。
- 归档订阅器：多集群串行推进、中途失败恢复 active、末步禁用 Jenkins、禁用失败仍归档。
- 前端：`project-environment-status` 单测覆盖已下线 / 下线中 / 下线失败与在线计数。
- 端到端手测：发布到 DEV → 下线 → 项目卡「已下线」且集群资源删除 → 回滚重新上线 → 带在线环境执行归档 → Jenkins Job 处于 disabled。
