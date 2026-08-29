# 项目部署流水线设计

> 在项目管理页点击「部署」后，经 RabbitMQ 编排：upsert Jenkins Job（CI）→ 构建并推 Harbor → 按生成模板在目标集群发布，并等到 Rollout Ready。参考 `A:\21-Agent\platformsuite` 的业务闭环，落地到 SparkOpsAgent 现有 DDD / Next.js BFF 架构。

## 目标

审核通过的项目可以一键部署：创建 **DeploymentRun**，异步完成 CI 与 CD，并在项目页展示阶段进度、构建日志、部署历史。

## 非目标（本规格不包含）

- 多集群并行发布、跨集群同步 workflow（参考项目 `syncresources`）。
- 自动回滚到上一镜像；Abort 不删除已 Apply 的资源。
- 将 Jenkinsfile 提交进业务 Git 仓库。
- 真实 Jenkins / Harbor / Sonar / 多集群的端到端集成测试（第一期用 mock driver）。
- 把 `READEMD.md` 中的明文凭据迁出仓库（安全债，另开任务）。
- 产品需求发布触发部署（现有 product → Build 路径保持不变）。

## 背景

SparkOpsAgent 已有：`delivery` 构建触发与 SSE、Jenkins mock/real 执行器、`hook.sparkops` 消费、项目生成模板（Deployment / Service / Jenkinsfile）、工作空间密钥、runtime 只读 K8s、项目审核门禁。

缺口：项目页「部署」只打开详情 Modal；Jenkins 不 upsert Job、参数不足；无 Deployment 记录；runtime 不 Apply；`cluster_info` 与 kubeconfig 工厂未打通。

参考项目可迁移：触发 → 写记录 → MQ 异步推进 → 成功后改集群。不可照搬：Nack 轮询 Jenkins、硬编码 `K8s_Server=env-dev`、弱幂等、DVA 前端。

## 决策摘要

1. **独立 DeploymentRun 编排**，不把 CD 状态塞进 `delivery.builds`。
2. **项目绑定默认集群**，部署弹窗只能切换管理员通过 `platform_admin.workspace_clusters` 授权给当前 Workspace 的集群。
3. **Jenkins Job 按项目 upsert**：Folder≈workspace/namespace，Job≈项目名；流水线来自 SparkOps **生成并渲染的 Jenkinsfile**。
4. **Git / Harbor / Sonar / Kubernetes / Jenkins API 一律来自项目关联的工作空间密钥**，缺一则拒绝创建 Run。
5. **CD 成功标准**：Apply 生成模板后，等待 Deployment Rollout Ready，并校验 Service 存在。
6. **RabbitMQ topic `deploy.sparkops`**，消息只带 `run_id` + `workspace_id`；按 DB 状态推进。改为手动 ACK；基础设施错误最多重试 3 次后进入 `.dlq`，业务失败写入 Run 后 ACK，禁止无限 Nack/requeue。
7. **权限**：创建/中止部署需要 `project:operate`；只读进度需要 `project:read`。

---

## 架构

### 限界上下文

| 模块 | 职责 |
|------|------|
| `workspace_app` | 项目、审核、密钥、生成模板；部署门禁查询（审核、密钥、默认集群） |
| `delivery` | Build 记录、Jenkins Job upsert + 触发、构建日志 SSE、webhook 完成回调 |
| `delivery`（新子包 `deployrun`） | DeploymentRun 状态机、MQ 消费、阶段事件 SSE；HTTP 挂在现有 delivery 路由下的 `/deployments` |
| `runtime` | 增加 Apply manifests + 等待 Rollout；kubeconfig 从 Kubernetes 类型密钥加载 |
| `platform_admin` | 集群目录（id / name / server）；部署时只选身份，不直接用库内明文证书 |
| `messaging` | 新增合法 topic `deploy.sparkops` |

不新建独立微服务。Compose 已有 RabbitMQ；本地可用 `MESSAGING_DRIVER=amqp` 或 in-memory。

### 集群与凭据解析

项目新增（或补齐）字段：

- `default_cluster_id`：`platform_admin.cluster_info.id`
- `jenkins_secret_id`：类型 `Jenkins`
- `kubernetes_secret_id`：类型 `Kubernetes`（kubeconfig 或 token）

新增 `platform_admin.workspace_clusters(workspace_id, cluster_id, created_by, created_at)` 授权关系。平台管理员维护授权；项目默认集群和部署时传入的集群都必须存在于当前 Workspace 授权集合中。普通 Workspace 用户不得读取未授权集群元数据。

已有字段继续使用：`secret_id`（Git）、`resources.registry_secret_id`（Harbor）、`resources.sonarqube_secret_id`（Sonar）。静态扫描关闭时仍要求绑定 Sonar 密钥——若产品上 `enable_static_scan=false`，则 **Sonar 密钥可选**，Jenkinsfile 跳过扫描阶段。其余密钥一律必填。

第一期约定 Kubernetes 密钥的 `credentials.extra_config` 保存完整 kubeconfig。Apply 时解密该字段，对选中集群的 `server` 做校验（host 必须匹配或 kubeconfig 当前 context 指向该 server），再 dynamic-client Apply；token/CA 分字段格式不在本期范围内。

### Jenkins Job 约定

- 名称：Jenkins Folder = `project.namespace`，Job = `project.name`（与参考项目一致；不再使用 `{name}{jobSuffix}` 拼单层 Job 名）。
- 每次 preparing：`CreateOrUpdate` Job XML（`FlowDefinition` + 渲染后的 pipeline script）。
- 触发参数至少：`BRANCH`/`CodeVersion`、`IMAGE_TAG`、Harbor 仓库坐标（从 Harbor 密钥与项目名推导，不硬编码 `harbor.local` 作为唯一来源）。
- Git/Harbor/Sonar 凭据：preparing 时用密钥在 Jenkins 创建或更新 credential（ID 稳定：`sparkops-{workspace}-{secretId}`），写入 Job。

---

## DeploymentRun 模型

表 `delivery.deployment_runs`：

| 字段 | 说明 |
|------|------|
| id | UUID |
| workspace_id, project_id | |
| cluster_id | 本次目标集群 |
| git_ref_type, git_ref_value | branch 或 tag |
| image_tag | 默认 DeploymentRun ID 前 12 位，或用户填写 |
| status | 见状态机 |
| build_id | 可空，building 后填 |
| error_message | |
| created_by, created_at | |
| started_at, finished_at | |
| current_stage | preparing / building / deploying |

表 `delivery.deployment_run_events`：`run_id, at, stage, message`（SSE 源）。

**同一 project 同时最多一条** status ∈ `{queued, preparing, building, deploying}`。

### 状态机

```
queued → preparing → building → deploying → succeeded
任意进行中 → failed | aborted
禁止跳跃（例如 queued 直接 succeeded）。
```

- `queued`：HTTP 创建成功并已 Publish MQ。
- `preparing`：门禁再校验、upsert Jenkins Job 与凭据。
- `building`：调用现有 TriggerBuild（扩展参数）；订阅 Build 完成。
- `deploying`：渲染模板（镜像 tag 替换为本次 `image_tag`）→ Apply → wait ready。
- `succeeded`：Deployment availableReplicas 达标且 Service 存在。
- `failed` / `aborted`：终态。重新部署 **新建** Run。

---

## HTTP / BFF / 前端

后端（均在现有 JWT + workspace 范围内）：

- `POST /api/v1/projects/:projectId/deployments`  
  body：`cluster_id?`（默认项目 `default_cluster_id`）、`git_ref_type`、`git_ref_value`、`image_tag?`  
  201：Run。权限 `project:operate`。审核必须 `approved`。
- `GET /api/v1/projects/:projectId/deployments` 列表（分页）
- `GET /api/v1/deployments/:runId`
- `GET /api/v1/deployments/:runId/stream` SSE（阶段事件；building 期间前端可同时订现有 build stream）
- `POST /api/v1/deployments/:runId/abort`

BFF / contracts / web-sdk 按现有 delivery 三层镜像一套 `deployments` 资源。

项目工作台：

- `review_status === approved` 且无进行中 Run：主按钮「部署」打开 **部署弹窗**（不再只打开详情）。
- 弹窗：集群下拉、Git ref、可选镜像版本、缺密钥/无默认集群时禁用并列出缺口。
- 进行中：主区域或抽屉显示阶段 + 构建日志。
- `ProjectDetailModal`「部署次数 / 最近部署」改为列表 API，去掉硬编码 `0` / `N/A`。

---

## 消息与推进

`messaging.TopicDeploy = "deploy.sparkops"`，加入 `ValidateTopic`。

消息 JSON：`{"run_id":"<uuid>","workspace_id":"<uuid>"}`。

消费者：

1. 加载 Run；未知 id → ACK 丢弃。
2. 按 `status` 执行 **一步**（preparing 或从 building 等待完成或 deploying）。
3. 需要等待 Jenkins 时：触发 Build 后把 Run 设为 `building` 并 ACK；由 **BuildCompleted**（进程内 EventBus 或 webhook 更新 Build 后）再 `Publish` 同一 topic，驱动 deploying。
4. 处理函数返回的业务错误：写 `failed` + ACK。仅网络抖动类错误可返回 err 让基础设施重试（次数有上限）。

禁止：参考项目式对 BUILDING 消息 Nack 重入队空转。

AMQP 消费统一使用 manual ACK。handler 返回 nil 时 ACK；返回基础设施错误时把 `x-sparkops-retry` 加一并重新发布，原消息 ACK；达到 3 次后发布到 `deploy.sparkops.dlq` 并 ACK。重试与死信由 messaging 基础设施完成，DeploymentRun 编排器不感知 broker header。

---

## 失败与超时

| 情况 | 行为 |
|------|------|
| 未审核 / 缺密钥 / 无集群 | 400，不建 Run |
| 进行中冲突 | 409 |
| Job upsert 失败 | Run `failed` |
| CI 失败 | Build `failed`，Run `failed`，不 Apply |
| Apply / Rollout 超时 | Run `failed`，默认等待 10 分钟（可配置） |
| Jenkins 超时 | 默认 30 分钟 → `failed` |
| Abort 在 building | abort Jenkins + Run `aborted` |
| Abort 在 deploying | 停止等待，`aborted`，不 rollback |

---

## 测试

后端（`go test`）：

- 门禁与 409 冲突
- 状态机合法/非法迁移
- 重复 `run_id` 消息不重复 Invoke Jenkins
- Build 失败不进入 deploying
- mock runtime：Apply 成功 / Rollout 超时

前端：contracts schema；工作台按钮门禁（能测的组件逻辑）。不做浏览器 E2E 作为第一期必过项。

Driver：单测使用 `delivery.driver=mock`、`runtime.driver=mock`、`messaging` in-memory。

---

## 配置

- `deploy.jenkins_timeout` 默认 30m
- `deploy.rollout_timeout` 默认 10m
- 现有 `DELIVERY_DRIVER` / `RUNTIME_DRIVER` / `MESSAGING_DRIVER` 不变

生产 Helm 已倾向 `amqp` + `jenkins` + `k8s`；本功能在 mock 下必须可演示完整状态机。

---

## 实施顺序（规格级）

1. 迁移 + 领域模型 + 状态机测试
2. Topic + 消费者骨架 + 创建 API 门禁
3. Jenkins upsert + 扩展 TriggerBuild 参数
4. runtime Apply + Rollout
5. 衔接 BuildCompleted → deploying
6. BFF/SDK/工作台 UI

## 验收

1. 已审核项目点部署 → 出现 DeploymentRun，阶段从 queued 走到 succeeded（mock）。
2. `delivery.driver=jenkins` 时 preparing 会 CreateOrUpdate Job 并触发，镜像 tag 进入生成的 Deployment 模板再 Apply。
3. 缺 Harbor 密钥无法创建 Run。
4. CI 失败时集群资源不被本次 Run 修改。
5. 详情页部署次数与最近一次状态来自真实 API。
