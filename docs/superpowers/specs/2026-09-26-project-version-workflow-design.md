# 项目版本管理（工作流驱动）设计

> Date: 2026-09-26  
> Reference: [Zadig 版本管理](https://docs.koderover.com/zadig/cn/Zadig%20v4.3/project/version/) · [KodeRover yaml-poc/version](https://course.koderover.com/v1/projects/detail/yaml-poc/version)  
> Scope: **C — 版本创建/发布作为 Product Workflow Job；项目页只读版本列表**

## 目标

在 SparkOps 中实现对标 KodeRover/Zadig 的「交付版本」能力，但**入口放在 Product Workflow**，而非项目页向导。

典型流水线：

```
构建 (sparkops-build)
  → 创建版本 (sparkops-create-version)
  → [可选] 审批 (sparkops-approval)
  → 部署版本 (sparkops-deploy-version)
```

项目详情页仅提供**只读版本列表**（版本号、状态、镜像、来源工作流 Run、创建时间），不提供创建/编辑入口。

## 非目标（本期）

- 项目页「创建版本」向导
- 服务级 YAML diff / 一键回滚 UI
- 环境「回溯」创建（新建环境选历史版本）
- Helm Chart 交付、外部 Webhook Hook
- 版本与 Git 仓库双向同步（GitOps）

---

## 架构概览

```
ProductWorkflowRun
  ├─ sparkops-build          → BuildExecutor (已有)
  ├─ sparkops-create-version → CreateVersionExecutor (新)
  └─ sparkops-deploy-version → DeployVersionExecutor (新)
                                    ↓
                          workspace_app.ProjectVersion (新)
                                    ↓
                          delivery.DeploymentRun (扩展 project_version_id)
                                    ↓
                          runtime Apply 快照 YAML
```

| 模块 | 职责 |
|------|------|
| `workspace_app` | `ProjectVersion` 聚合、快照持久化、项目页只读查询 API |
| `product_workflow` | 两个新 Job 类型 + Executor + Scheduler dispatch |
| `delivery/deployrun` | `DeploymentRun` 支持 `project_version_id`，渲染时用版本快照 |
| 前端 `product` | 工作流编辑器/Run Form/Run 详情扩展 |
| 前端 `workspace` | 项目详情新增只读「版本」Tab |

---

## 领域模型

### ProjectVersion（workspace_app）

表 `workspace_app.project_versions`：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | PK |
| workspace_id | UUID | |
| project_id | UUID | |
| product_id | UUID | 可空；来自工作流 Run 的 product |
| version_tag | VARCHAR(64) | 用户可见版本号，如 `v1.2.0` |
| status | VARCHAR(16) | `creating` / `ready` / `failed` |
| source | VARCHAR(16) | 固定 `workflow`（本期） |
| workflow_run_id | UUID | 创建该版本的工作流 Run |
| workflow_job_name | VARCHAR(128) | 创建该版本的 Job 名 |
| manifests | JSONB | `[{kind, content, hash}]` — Service/Deployment 快照 |
| images | JSONB | `[{service_name, image, tag, build_id?}]` |
| error_message | TEXT | 失败原因 |
| created_by | UUID | |
| created_at | TIMESTAMPTZ | |

约束：`UNIQUE (project_id, version_tag)`。

**快照来源：** 创建版本 Job 执行时，对每个关联 project 调用现有 `GenerateProjectTemplatesHandler`，合并 `manifest_overrides`，写入 `manifests`；镜像坐标来自上游 `sparkops-build` 成功输出的 `ImageTag` / `ImageRepository`。

### JobRun 运行时输出

在 `JobRun` 上新增字段（JSON 持久化于 `stage_runs`）：

```go
type ProjectVersionRun struct {
    ProjectID       string `json:"project_id"`
    VersionID       string `json:"version_id"`
    VersionTag      string `json:"version_tag"`
    Status          string `json:"status"`
    ErrorMessage    string `json:"error_message,omitempty"`
}

type JobRun struct {
    // ...existing
    Versions []ProjectVersionRun `json:"versions,omitempty"`
}
```

---

## 新 Job 类型

### sparkops-create-version

**分类：** `build`（与构建/打包同一阶段语义）

**Spec：**

```json
{
  "version_tag": "{{version}}",
  "source": "fromjob",
  "job_name": "build-services",
  "project_ids": []
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `version_tag` | 是 | 版本号；支持工作流变量插值 |
| `source` | 是 | 固定 `fromjob` |
| `job_name` | 是 | 上游 `sparkops-build` Job 名 |
| `project_ids` | 否 | 空则取上游 build 全部成功 project；非空则过滤 |

**执行（同步）：**

1. `resolveFromJobBuilds(run, job_name)` — 复用 deploy 同名 helper
2. 对每个 target project：
   - 渲染 generated-templates + overrides → 快照
   - 取 build 输出的 image tag/repo
   - 创建 `ProjectVersion(status=ready)`
   - 写入 `jobRun.Versions[]`
3. 任一 project 失败 → Job failed；全部成功 → Job success

**Run Form 输入：** `version_tag` 可在启动运行时覆盖（写入 `input.version_tag_overrides[jobName]`，与 `environment_overrides` 模式一致）。

### sparkops-deploy-version

**分类：** `deploy`

**Spec：**

```json
{
  "environment_id": "{{prod_env_id}}",
  "source": "fromjob",
  "job_name": "create-release"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `environment_id` | 是 | 目标 Product Environment |
| `source` | 是 | 固定 `fromjob` |
| `job_name` | 是 | 上游 `sparkops-create-version` Job 名 |

**执行（异步，与 sparkops-deploy 一致）：**

1. `resolveFromJobVersions(run, job_name)` — 收集上游 Job 的 `Versions[]` 且 status=success
2. 对每个 version 调用 `DeploymentTrigger.TriggerForEnvironment`，携带 `ProjectVersionID`
3. 保存 `WorkflowDeployRef` 用于 DeployCompleted 回调
4. Scheduler `refreshJobStatuses` 聚合 deploy 状态

**Run Form 输入：** `environment_overrides[jobName]` 可覆盖 `environment_id`（复用现有 deploy 逻辑）。

---

## 端口与下游改动

### appctx.WorkflowDeploymentRequest（扩展）

```go
type WorkflowDeploymentRequest struct {
    // ...existing
    ProjectVersionID *kernel.ProjectVersionID // 非空时用版本快照
}
```

### deployrun 渲染

`deployrun/application/service/renderer.go`：

- 若 `run.ProjectVersionID != nil` → 从 `project_versions.manifests` 取 YAML，替换镜像字段后 Apply
- 否则 → 现有 generated-templates 路径（不变）

### 新 Port（可选，推荐）

```go
type VersionCreator interface {
    CreateFromBuild(ctx context.Context, req CreateVersionRequest) (kernel.ProjectVersionID, error)
}

type CreateVersionRequest struct {
    WorkspaceID, ProjectID, ProductID kernel.*
    VersionTag, WorkflowRunID, WorkflowJobName string
    Manifests []VersionManifest
    Images    []VersionImage
}
```

由 `workspace_app` 实现，`CreateVersionExecutor` 注入调用。

---

## API

### 工作流（无新路由）

现有 `POST .../workflows/:id/runs` + Job dispatch 即可；Run 详情 `JobRun.versions` / `JobRun.deploys` 随 stage_runs 返回。

### 项目只读版本列表（新）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/workspaces/:wsId/projects/:projectId/versions` | 分页列表 |
| GET | `/api/v1/workspaces/:wsId/projects/:projectId/versions/:versionId` | 详情（manifests + images + workflow_run 链接） |

权限：`project:read`。

BFF：`/api/bff/v1/workspaces/[wsId]/projects/[projectId]/versions`

**列表 VM：**

```typescript
{
  id, version_tag, status,
  images: [{ service_name, image, tag }],
  workflow_run_id?, product_id?,
  created_at, created_by_name?
}
```

不提供 POST/PUT/DELETE — 版本仅由工作流 Job 创建。

---

## 前端

### Product Workflow 编辑器

| 文件 | 改动 |
|------|------|
| `product-workflow.ts` | 注册两个 job type |
| `workflow-editor-utils.ts` | `defaultJob()` cases |
| `create-version-job-spec.ts` | parse ↔ record |
| `deploy-version-job-spec.ts` | parse ↔ record |
| `CreateVersionJobConfigDrawer.tsx` | 版本号、上游 build 选择、project 过滤 |
| `DeployVersionJobConfigDrawer.tsx` | 环境、上游 create-version 选择 |
| `ProductWorkflowEditor.tsx` | 接线 drawer |

### Run Form

| 文件 | 改动 |
|------|------|
| `workflow-run-form.ts` | 新分组：`versionJobs`（version_tag）、`deployVersionJobs`（environment） |
| `StartWorkflowRunModal.tsx` | 提交 `version_tag_overrides` |

### Run 详情

| 文件 | 改动 |
|------|------|
| `workflow-run-canvas-utils.ts` | 新 task kind：`version` |
| Run 详情面板 | 展示创建的版本号、镜像列表、部署 Run 链接 |

### 项目页（只读）

| 文件 | 改动 |
|------|------|
| `ProjectDetailModal.tsx` 或项目详情页 | 新 Tab「版本」 |
| `ProjectVersionsTab.tsx`（新） | 表格：版本号 / 状态 / 镜像摘要 / 来源 Run / 时间；点击行 → 版本详情 Drawer（只读 YAML + 镜像） |

**不提供：** 创建版本按钮、编辑、回滚、diff。

---

## 标准工作流模板

内置模板「标准发布（版本化）」示例：

```json
{
  "stages": [
    {
      "name": "构建",
      "jobs": [{
        "name": "build-all",
        "type": "sparkops-build",
        "spec": { "project_builds": [] }
      }]
    },
    {
      "name": "打包",
      "jobs": [{
        "name": "create-release",
        "type": "sparkops-create-version",
        "spec": {
          "version_tag": "{{version}}",
          "source": "fromjob",
          "job_name": "build-all"
        }
      }]
    },
    {
      "name": "发布",
      "jobs": [{
        "name": "deploy-prod",
        "type": "sparkops-deploy-version",
        "spec": {
          "environment_id": "",
          "source": "fromjob",
          "job_name": "create-release"
        }
      }]
    }
  ],
  "variables": [{ "key": "version", "value": "" }]
}
```

同步更新 `sync_builtin_templates.go`。

---

## 后端实现清单

| # | 位置 | 内容 |
|---|------|------|
| 1 | `migrations/000059_project_versions.up.sql` | 建表 |
| 2 | `workspace_app/domain/model/project_version.go` | 模型 |
| 3 | `workspace_app/domain/repository/` + persistence | CRUD |
| 4 | `workspace_app/application/command/create_version.go` | 创建命令 |
| 5 | `workspace_app/application/query/list_versions.go` | 列表/详情 |
| 6 | `workspace_app/interface/http/` | GET handlers |
| 7 | `product_workflow/domain/model/workflow_definition.go` | Job 常量 |
| 8 | `product_workflow/domain/model/job_catalog.go` | Catalog 注册 |
| 9 | `product_workflow/domain/engine/create_version_executor.go` | 同步执行 |
| 10 | `product_workflow/domain/engine/deploy_version_executor.go` | 异步部署 |
| 11 | `product_workflow/domain/engine/scheduler.go` | switch + refresh + ApplyRunInput |
| 12 | `product_workflow/module.go` | 装配 |
| 13 | `appctx/ports.go` | WorkflowDeploymentRequest 扩展 |
| 14 | `delivery/deployrun/` | project_version_id 字段 + renderer |
| 15 | contracts + BFF + web-sdk | 版本列表 API |

---

## 测试策略

| 层 | 覆盖 |
|----|------|
| 单元 | CreateVersionExecutor：build 输出 → 快照；DeployVersionExecutor：version → deploy trigger |
| 单元 | deployrun renderer：有/无 project_version_id |
| 集成 | 完整 Run：build → create-version → deploy-version |
| 前端 | workflow-run-form 分组；editor defaultJob |

---

## 分期

| 阶段 | 交付 |
|------|------|
| **M1** | ProjectVersion 表 + create-version Job + 项目只读列表 |
| **M2** | deploy-version Job + DeploymentRun 绑定版本 |
| **M3** | 内置模板 + Run Form version_tag 覆盖 + Run 详情可视化 |

M1+M2 即可闭环对标 KodeRover「创建版本 → 按版本部署」主路径。

---

## 与现有 sparkops-deploy 的关系

| | sparkops-deploy | sparkops-deploy-version |
|--|-----------------|-------------------------|
| 输入 | 上游 build 镜像 | 上游 create-version 快照 + 镜像 |
| 配置 | deploy_contents, config_resources | 仅整包部署（快照 YAML） |
| 审计 | DeploymentRun | DeploymentRun + ProjectVersion |
| 场景 | 日常 dev 快速部署 | 正式发布、可回溯交付 |

两者并存，不互相替代。
