# SparkOps Delivery Center 完整设计

> 版本：v1.0 · 2026-09-02  
> 状态：设计稿（待评审）  
> 范围：企业级多集群、可编排、Human-in-the-loop 持续交付控制面

---

## 1. 文档目的

本文定义 SparkOps **Delivery Center（持续交付中心）** 的完整产品设计与技术方案，用于替代/演进当前「单集群 + 硬编码 Orchestrator」的部署模型，支持：

- **多集群**：一次发布覆盖多个 Kubernetes 集群（Fleet）
- **可编排**：声明式 Pipeline DAG（构建 → 验证 → 审批 → 分波部署 → Promote）
- **配置差异**：Environment Profile + Cluster Patch 分层配置
- **Human-in-the-loop**：发布前审批、Canary 后 Promote、多级 Wave 卡点
- **企业治理**：Policy Engine、变更窗口、审计、RBAC

---

## 2. 背景与现状

### 2.1 现有能力（SparkOps）

| 模块 | 路径 | 能力 |
|------|------|------|
| 部署 Run | `delivery/deployrun` | `DeploymentRun` 状态机：queued → preparing → building → deploying |
| 编排 | `deployrun/application/service/orchestrator.go` | **Go 硬编码**阶段顺序 |
| 项目配置 | `workspace_app` | 模板、`project_config`、单 `kubernetes_secret_id`、`default_cluster_id` |
| Jenkins | `delivery/infrastructure/jenkins` | 构建；`PipelineRenderer` 渲染 **Jenkinsfile**（非 CD DAG） |
| K8s 部署 | `runtime/infrastructure/k8s/deployer.go` | YAML apply + rollout wait |
| 集群 | `platform_admin.cluster_info` + `workspace_clusters` | 多集群授权 |
| 工作流 | `workflow` | JSON steps + `approval` 能力可复用 |
| 前端 | `ProjectDeployModal` | **单集群**选择 + SSE 日志 |

### 2.2 核心缺口

1. 无 **Release / PipelineRun** 概念，一次只能发一个集群  
2. 无 **Fleet 拓扑** 与 **分波/并行** 编排  
3. 配置/凭证 **项目级全局**，无法 per-cluster 差异化  
4. **Human-in-the-loop** 仅在项目审核，无「每次 prod 发布审批」  
5. 无 **可视化 Pipeline** 配置与 **Policy** 门禁  

### 2.3 设计原则

| 原则 | 说明 |
|------|------|
| 构建一次，部署多次 | 同一 Release 锁定 `image digest` |
| 编排声明式 | Pipeline DAG 存库、可版本化、可视化 + YAML 双模 |
| 执行插件化 | Step Executor 注册表，Run Engine 只负责调度 |
| 决策与执行分离 | Release 向导决策 → Fleet Board 观测执行 |
| 渐进演进 | 保留 `DeploymentRun` 兼容期，映射为 `TaskRun` |
| 凭证统一 | 集群连通以 `cluster_info` 为主，废弃「一项目一 kubeconfig 打天下」 |

---

## 3. 目标架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Presentation · Next.js + BFF                                            │
│  Project Workbench │ Release Wizard │ Fleet Board │ Pipeline Studio     │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────────────┐
│ Delivery Center（新 BC：cd）                                               │
│  Pipeline Registry │ Run Engine │ Target Resolver │ Render Engine         │
│  Policy Engine │ Approval Hub │ Artifact Registry │ Step Executors      │
└───────┬─────────────────┬─────────────────┬─────────────────┬─────────────┘
        │                 │                 │                 │
        ▼                 ▼                 ▼                 ▼
   workspace_app    delivery(jenkins)   runtime(k8s)    workflow(approval)
   platform_admin   platform_admin      plugins/k8s     notification
   (projects)       (cluster_info)      (metrics)
```

### 3.1 与平台四层模型对齐

```
Organization → Workspace → Project → Pipeline / Fleet / Release
```

Delivery Center 为 **Workspace 级** 能力；Project 绑定默认 Pipeline 与允许的 Fleet。

---

## 4. 核心概念

| 概念 | 说明 |
|------|------|
| **Pipeline** | 编排定义（DAG），版本化，Workspace 级模板 |
| **PipelineRun** | 一次「发布」执行实例（用户点「发布」创建） |
| **StageRun** | Pipeline 中一个 stage 的运行实例 |
| **StepRun** | Stage 内原子步骤实例 |
| **TaskRun** | 可选，单集群任务（如 `fleet.deploy` 展开后每集群一条） |
| **Fleet** | 逻辑集群组 + Wave 拓扑 |
| **Profile** | 环境/区域配置档（staging、prod-cn） |
| **Cluster Patch** | 单集群 L4 配置覆盖 |
| **ArtifactSet** | 锁定的制品（image digest、manifest bundle） |
| **ConfigSnapshot** | 某次 Run 的不可变渲染快照（diff/回滚/审计） |

### 4.1 与现有 DeploymentRun 映射

```
PipelineRun (1)
  └── TaskRun (N)  ≈  DeploymentRun (legacy, 每集群一条)
```

兼容期：`deployment_runs.release_id` 外键关联；旧 API `POST .../deployments` 等价于 `targets.length === 1` 的 PipelineRun。

---

## 5. 领域模型与数据表

Schema：`cd`（PostgreSQL）

### 5.1 pipelines

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid PK | |
| workspace_id | uuid | |
| name | varchar | 唯一 (workspace_id, name) |
| description | text | |
| status | enum | draft, active, archived |
| version | int | 当前活跃版本号 |
| created_at / updated_at | timestamptz | |

### 5.2 pipeline_versions

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid PK | |
| pipeline_id | uuid FK | |
| version | int | |
| definition | jsonb | Pipeline DSL 文档 |
| layout | jsonb | Studio 画布坐标（可选） |
| created_by | uuid | |
| created_at | timestamptz | |

### 5.3 pipeline_runs

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid PK | |
| workspace_id | uuid | |
| project_id | uuid | |
| pipeline_id | uuid | |
| pipeline_version | int | 创建时锁定 |
| status | enum | pending, running, paused, succeeded, partial_failed, failed, aborted |
| parameters | jsonb | git_ref, image_tag 等 |
| artifact_set_id | uuid | 构建完成后写入 |
| created_by | uuid | |
| created_at / started_at / finished_at | timestamptz | |

**约束**：同一 project 仅允许一个 `status in (pending, running, paused)` 的 Run（可配置是否允许多 Run）。

### 5.4 stage_runs / step_runs / task_runs

```
stage_runs:  id, pipeline_run_id, stage_id, status, started_at, finished_at, error
step_runs:   id, stage_run_id, step_index, uses, status, input, output, error
task_runs:   id, step_run_id, cluster_id, status, ...  (fleet.deploy 展开)
```

### 5.5 fleets / fleet_members

```
fleets: id, workspace_id, name, selector (jsonb), topology (jsonb waves)
fleet_members: fleet_id, cluster_id, wave_name, sort_order
```

**selector 示例**：`matchLabels: { env: prod, region: cn }` 或静态 cluster_id 列表。

### 5.6 config_profiles / cluster_patches

```
config_profiles: id, workspace_id, name, values (jsonb), patch (jsonb)
cluster_patches: id, project_id, cluster_id, patch (jsonb)
```

### 5.7 config_snapshots

```
id, pipeline_run_id, cluster_id, manifest_hash, manifest_yaml (bytea), created_at
```

### 5.8 artifact_sets

```
id, pipeline_run_id, image_repository, image_digest, image_tag, locked_at
```

### 5.9 project 扩展字段

```sql
ALTER TABLE workspace_app.projects ADD COLUMN default_pipeline_id uuid;
ALTER TABLE workspace_app.projects ADD COLUMN allowed_fleet_ids jsonb; -- uuid[]
```

### 5.10 approvals（或复用 workflow_runs）

```
approval_requests: id, pipeline_run_id, stage_run_id, workflow_run_id, status, expires_at
approval_decisions: id, request_id, member_id, decision, comment, decided_at
```

---

## 6. Pipeline DSL 规范

### 6.1 顶层结构

```yaml
apiVersion: cd.sparkops.io/v1
kind: Pipeline
metadata:
  name: string
spec:
  parameters: ParameterDef[]
  stages: Stage[]
```

### 6.2 Stage

```yaml
- id: string              # DAG 节点 ID，唯一
  needs: string[]         # 依赖的 stage id 列表（AND）
  if: string              # 可选 CEL 表达式
  steps: Step[]
```

### 6.3 Step

```yaml
- uses: string            # Step Catalog 中的 step id
  with: object            # 参数 map，支持 {{ params.* }} 模板
  timeout: duration
  retry: { max: int, backoff: duration }
```

### 6.4 参数定义

```yaml
parameters:
  - name: git_ref_value
    type: string
    required: true
  - name: image_tag
    type: string
    required: false
```

### 6.5 模板上下文

| 前缀 | 来源 |
|------|------|
| `params.*` | PipelineRun.parameters |
| `project.*` | 项目元数据 |
| `artifact.*` | ArtifactSet |
| `render.<cluster>.*` | Render Engine 输出 |
| `run.*` | PipelineRun id、时间等 |

### 6.6 校验规则

- `stages[].id` 唯一  
- `needs` 引用存在的 id，**无环**  
- 每个 `uses` 必须在 Step Catalog 注册  
- `with` 符合 step 的 JSON Schema  
- `approval.manual` 的 prod Pipeline 必须通过 Policy「含审批步骤」  

---

## 7. Step Catalog（内置步骤）

| uses | 说明 | pauses | 复用模块 |
|------|------|--------|----------|
| `jenkins.build` | 触发 Jenkins 构建 | no | delivery/jenkins |
| `artifact.lock` | 锁定 digest | no | cd/artifact |
| `fleet.deploy` | 多集群部署 | no | runtime/k8s + cd/fleet |
| `k8s.wait_rollout` | 等待 Deployment | no | runtime/k8s |
| `http.smoke` | HTTP 冒烟 | no | cd/verify |
| `prometheus.query` | 指标 Gate | no | plugins/k8s metrics |
| `approval.manual` | 人工审批 | **yes** | workflow |
| `policy.evaluate` | 策略评估 | no | cd/policy |
| `change_window.check` | 变更窗口 | no | cd/policy |
| `harbor.promote` | 镜像复制 | no | delivery/harbor |
| `gitops.sync` | ArgoCD 同步 | no | 集成（Phase D） |
| `notify.webhook` | 通知 | no | notification |
| `rollback.release` | 回滚 | no | cd/rollback |

### 7.1 fleet.deploy 参数

```yaml
with:
  fleet: string              # fleet name 或 id
  strategy: parallel | sequential | wave
  progressive: none | canary | bluegreen
  profile: string
  onFailure: pause | abort | continue
  clusters: string[]         # 可选，覆盖 fleet 成员
```

---

## 8. Run Engine（调度器）

### 8.1 状态机

**PipelineRun**：`pending → running ↔ paused → succeeded | partial_failed | failed | aborted`

**StageRun**：`pending → running → succeeded | failed | skipped`  
**paused** 仅当 stage 含 `approval.manual` 或 `onFailure: pause`

### 8.2 调度算法

```
1. 加载 pipeline_version.definition
2. 拓扑排序 stages（Kahn）
3. 循环：
   - 找出所有 needs 已 succeeded 且自身 pending 的 stage → runnable
   - 并行执行 runnable stages（可配置 max_parallel_stages）
   - 每 stage 内 steps 顺序执行
   - fleet.deploy → 展开为 N 个 task_run（按 strategy/wave）
4. approval.manual → stage = paused，Publish 事件，等待 Workflow 回调
5. 全部 terminal → 聚合 PipelineRun 状态
```

### 8.3 失败策略

| onFailure | 行为 |
|-----------|------|
| abort | 终止 Run，未执行 stage 跳过 |
| pause | Run = paused，等人处理（继续/回滚/中止） |
| continue | 记录 partial_failed，继续无依赖的后继 stage |

### 8.4 事件

经 EventBus / SSE 推送：

- `pipeline_run.updated`
- `stage_run.updated`
- `step_run.log`
- `task_run.updated`
- `approval.requested` / `approval.decided`

---

## 9. Fleet 与 Target Resolver

### 9.1 Fleet 定义示例

```yaml
name: prod-cn
selector:
  static:
    - cluster_id: prod-beijing-1
    - cluster_id: prod-beijing-2
topology:
  waves:
    - name: canary
      clusters: [prod-beijing-1]
    - name: primary
      clusters: [prod-beijing-2, prod-shanghai-1]
    - name: edge
      clusters: [prod-guangzhou-1]
```

### 9.2 解析流程

```
fleet.deploy(fleet=prod-cn)
  → TargetResolver.Resolve(fleet, project.allowed_fleets)
  → 过滤：connector ready、Policy 允许、变更窗口
  → 按 strategy 生成执行计划
  → 每 cluster 调用 RenderEngine → ManifestDeployer
```

### 9.3 Unified Fleet Connector

优先级：

1. `cluster_patches.kubernetes_secret_id`（显式）  
2. workspace cluster 默认 secret  
3. `platform_admin.cluster_info` 凭证（与 K8s 插件一致）  

部署前校验：`kubeconfig.server == cluster_info.server`

---

## 10. Configuration Plane（五层）

```
L0 Application Contract   项目模板 + project_config
L1 Run Parameters         git_ref, image_tag, 临时 overrides
L2 Profile                staging / prod-cn / prod-ap
L3 Fleet Overlay          区域级 patch
L4 Cluster Patch          单集群 replicas/ingress/env
```

**渲染**：

```
Manifest(c) = Render(L0, L2(profile), L3(fleet), L4(c), L1, ArtifactSet)
```

每次 Run 对每集群生成 **ConfigSnapshot**（hash + yaml），供 diff、审计、回滚。

---

## 11. Human-in-the-loop

### 11.1 三类卡点

| 类型 | 时机 | UI |
|------|------|-----|
| A. 项目准入 | 首次/变更后 | `ProjectReviewModal`（已有） |
| B. 发布审批 | staging 验证后、prod 前 | Fleet Board 审批抽屉 + 待办 |
| C. Canary Promote | 指标 Gate 后 | Fleet Board `[Promote]` |

### 11.2 approval.manual 配置

```yaml
- uses: approval.manual
  with:
    approvers: [role:ops-lead, role:release-manager]
    min_approvals: 1
    timeout: 24h
    on_timeout: abort
```

### 11.3 执行表现

- PipelineRun → `paused`  
- DAG 节点「人工审批」高亮  
- prod 集群显示「等待审批」  
- 顶栏待办 + Workflow 回调 `POST .../runs/:id/approve`  

---

## 12. Policy Engine

### 12.1 评估时机

- **编译期**：保存 Pipeline 时（Studio Validate）  
- **Preflight**：Release 向导 Step 3  
- **Run 期**：`policy.evaluate` step、`change_window.check`  

### 12.2 规则示例（CEL/Rego）

```yaml
policies:
  - name: prod-requires-approval
    when: params.target_env == "prod"
    assert: pipeline.contains_step("approval.manual")

  - name: forbid-latest-tag
    assert: artifact.tag != "latest"

  - name: cn-data-residency
    when: fleet.name == "prod-cn"
    assert: fleet.clusters.all(c, c.labels.region == "cn")
```

---

## 13. API 设计

Base：`/api/v1/workspaces/:wsId` 或项目 scoped

### 13.1 Pipeline CRUD

```
GET    /delivery/pipelines
POST   /delivery/pipelines
GET    /delivery/pipelines/:id
PUT    /delivery/pipelines/:id
POST   /delivery/pipelines/:id/versions        # 新版本
POST   /delivery/pipelines/:id/validate        # 校验 DAG
POST   /delivery/pipelines/:id/dry-run         # 模拟
```

### 13.2 Fleet / Profile

```
GET/POST/PUT  /delivery/fleets
GET/POST/PUT  /delivery/profiles
GET/PUT       /projects/:projectId/targets     # fleet 绑定 + cluster patch
```

### 13.3 Release

```
GET    /projects/:projectId/release/preflight
POST   /projects/:projectId/releases
GET    /delivery/runs
GET    /delivery/runs/:runId
GET    /delivery/runs/:runId/stream            # SSE 聚合
POST   /delivery/runs/:runId/abort
POST   /delivery/runs/:runId/approve
POST   /delivery/runs/:runId/reject
POST   /delivery/runs/:runId/promote           # Canary 晋升
POST   /delivery/runs/:runId/rollback
POST   /projects/:projectId/render-preview     # diff 预览
```

### 13.4 兼容

```
POST   /projects/:projectId/deployments
  → 内部创建 PipelineRun(pipeline=legacy-single-cluster, targets=[cluster_id])
```

---

## 14. BFF 与前端

### 14.1 路由

```
/delivery/runs
/delivery/runs/[runId]           Fleet Board
/delivery/pipelines
/delivery/pipelines/[id]/studio  Pipeline Studio
/delivery/fleets
/delivery/fleets/[id]
/delivery/profiles
/delivery/policies

/projects/[id]                   Tab: 概览 | 发布记录 | 目标与配置
```

### 14.2 核心页面

| 页面 | 职责 |
|------|------|
| **ReleaseWizardModal** | 4 步：参数 → Fleet → Diff+Policy → 确认 |
| **FleetBoardPage** | DAG + Fleet 热力图 + 上下文抽屉 |
| **PipelineStudioPage** | 可视化 DAG + Step 表单 + YAML 同步 |
| **FleetEditorPage** | Wave 拓扑可视化 |
| **ProfileEditorPage** | Profile KV / YAML |
| **ProjectTargetsTab** | 项目 Fleet 绑定 + L4 Patch |

### 14.3 Release 向导 4 步

1. Pipeline + git/tag  
2. Fleet 多选 + Wave 展开 + Ready/Blocked  
3. Config diff 矩阵 + Policy + YAML diff  
4. 摘要 → 确认 → 跳转 Fleet Board  

### 14.4 Pipeline Studio

- **Phase B**：Stage 列表 + needs 多选 + Step 表单  
- **Phase C**：React Flow 画布 + Step 库拖拽 + YAML 双向同步  
- 保存 → `pipeline_versions`；发布 → status=active  

### 14.5 ViewModel 契约（@sparkops/contracts/delivery）

新增：`PipelineVM`, `PipelineRunVM`, `StageRunVM`, `FleetVM`, `ReleasePreflightVM`, `ConfigDiffVM`, `ApprovalRequestVM`

### 14.6 原型参考

交互原型 Canvas：`canvases/delivery-ui-prototype.canvas.tsx`

---

## 15. 渐进式交付

| 层级 | 配置 | 实现 |
|------|------|------|
| Fleet Wave | `strategy: wave` | Run Engine 按 wave 顺序 |
| 集群内 Canary | `progressive: canary` | Kruise Rollout / Argo Rollouts |
| Metric Gate | `prometheus.query` step | 自动/人工 Promote |
| GitOps | `gitops.sync` step | 更新 Git + ArgoCD（Phase D） |

---

## 16. 可观测与审计

| 能力 | 实现 |
|------|------|
| Run Timeline | stage/step/task 事件流 |
| Fleet 热力图 | 每集群状态聚合 |
| 审计 | Run + Snapshot + Policy 决策 + 审批记录 |
| 告警 | failed / paused timeout / partial_failed |
| 关联 K8s 控制台 | 深链 cluster/ns/workload |

---

## 17. 安全与多租户

- Workspace 隔离：所有 cd 表带 `workspace_id`  
- RBAC：`delivery:pipeline:edit`, `delivery:release:create`, `delivery:release:approve`, `delivery:run:abort`  
- 项目 `review_status=approved` 才能发 prod Pipeline  
- Secret 不入 Snapshot 明文；manifest 脱敏展示  
- 审批权限与 Workflow 角色绑定  

---

## 18. 迁移与兼容

### 18.1 阶段 0（修复基础）

- `ProjectDeploymentReader.Load(clusterID)` 按集群解析凭证  
- `deployments/options`  per-cluster readiness  

### 18.2 阶段 1（MVP）

- 新表 + Run Engine 最小 DAG  
- 内置模板 `legacy-single-cluster` = 现有 Orchestrator  
- 内置 `staging-then-prod` 含 `approval.manual`  
- Release 向导 + Fleet Board（只读 DAG）  

### 18.3 数据迁移

- 历史 `deployment_runs` 保留，不强制填 `release_id`  
- 新 Run 双写可选（过渡期）  

---

## 19. 实施路线图

| 阶段 | 周期 | 交付 |
|------|------|------|
| **0** | 1–2 周 | 凭证统一、options 按集群、文档 |
| **A** | 6–8 周 | cd 模块、Run Engine、Fleet、Release 向导、Fleet Board、TaskRun→K8s |
| **B** | 8–10 周 | Profile/Patch、Policy、approval、Studio 表单版、待办 |
| **C** | 8–10 周 | Studio 画布、Canary/Promote、partial_failed、回滚 |
| **D** | 持续 | GitOps、Harbor promote、SBOM、合规策略包 |

---

## 20. 附录 A：标准 Pipeline 模板

### A.1 standard-java-prod

见第 6 节完整 YAML（build → lock → staging → smoke → approve → prod-cn wave → promote ap）。

### A.2 staging-only

```yaml
stages:
  - id: build
    steps: [{ uses: jenkins.build }]
  - id: deploy
    needs: [build]
    steps:
      - uses: fleet.deploy
        with: { fleet: staging-all, strategy: parallel, profile: staging }
  - id: verify
    needs: [deploy]
    steps: [{ uses: k8s.wait_rollout }, { uses: http.smoke }]
```

### A.3 hotfix

```yaml
stages:
  - id: build
    steps: [{ uses: jenkins.build }]
  - id: approve
    needs: [build]
    steps:
      - uses: approval.manual
        with: { approvers: [role:ops-lead, role:release-manager], min_approvals: 2 }
  - id: deploy
    needs: [approve]
    steps:
      - uses: fleet.deploy
        with: { fleet: prod-cn, strategy: parallel, profile: prod-cn }
```

### A.4 legacy-single-cluster（兼容）

```yaml
stages:
  - id: prepare
    steps: [{ uses: jenkins.prepare }]  # 映射现有 PrepareService
  - id: build
    needs: [prepare]
    steps: [{ uses: jenkins.build }]
  - id: deploy
    needs: [build]
    steps:
      - uses: fleet.deploy
        with:
          clusters: ["{{ params.cluster_id }}"]
          strategy: parallel
          profile: default
```

---

## 21. 附录 B：Release Preflight 响应示例

```json
{
  "pipelines": [{ "id": "...", "name": "标准生产 v3", "version": 3 }],
  "fleets": [
    {
      "id": "prod-cn",
      "name": "prod-cn",
      "clusters": [
        { "id": "...", "name": "prod-beijing-1", "ready": true, "wave": "canary" },
        { "id": "...", "name": "prod-guangzhou-1", "ready": false, "reason": "缺少 cluster patch" }
      ]
    }
  ],
  "policies": [
    { "name": "prod-change-window", "passed": true },
    { "name": "prod-gz-patch", "passed": false, "message": "prod-guangzhou-1 缺少 L4 patch" }
  ],
  "blocked": true
}
```

---

## 22. 附录 C：目录结构（后端）

```
backend/internal/modules/cd/
  domain/
    model/          pipeline, pipeline_run, fleet, profile, ...
    port/           StepExecutor, TargetResolver, Renderer, PolicyEvaluator
    repository/
  application/
    command/        CreateRelease, ApproveRelease, ...
    query/          GetRun, Preflight, ...
    service/        RunEngine, Scheduler, RenderEngine
  infrastructure/
    executor/       jenkins, fleet_deploy, approval, ...
    persistence/
    acl/            workspace_app, cluster_info, workflow
  interface/
    http/
```

---

## 23. 附录 D：目录结构（前端）

```
fronted/apps/web/src/modules/delivery/
  components/
    ReleaseWizard/
    FleetBoard/
    PipelineStudio/
    FleetEditor/
  hooks/
  views/
fronted/packages/contracts/src/delivery/   # 扩展 VM
fronted/packages/bff-core/src/delivery/    # BFF 编排
```

---

## 24. 评审检查清单

- [ ] 单集群路径是否平滑迁移（legacy 模板）  
- [ ] prod 发布是否强制 approval.manual（Policy）  
- [ ] 凭证是否与 K8s 插件统一  
- [ ] ConfigSnapshot 是否满足审计与回滚  
- [ ] SSE 是否支持 Run 级 multiplex  
- [ ] Studio 校验是否阻止 DAG 成环  
- [ ] 多 Run 并发策略是否明确  

---

## 25. 参考文档

- `spec/architecture.md` — 平台 Workflow / ArgoCD 愿景  
- `spec/frontend-architecture.md` — BFF 三层  
- `READEMD.md` — 现有部署流水线 API  
- `canvases/delivery-ui-prototype.canvas.tsx` — UI 原型 v2  

---

*End of document*
