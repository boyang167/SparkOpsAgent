# Ai Plugin 
- SuperPowers
- BMAD
    npx bmad-method install


# SparkOps Agent Platform

单体仓库：后端 + 前端 + 规格文档

## 快速开始

### 1. 后端

```bash
cd backend
cp config.example.yaml config.yaml   # 编辑 database / redis
go run ./cmd/seed                     # 初始化 demo 数据
go run ./cmd/server                   # :8080
```

密码含 `@` 等特殊字符时，请用结构化 database 配置（见 `config.example.yaml`），不要用未编码的 DSN URL。

### 2. 前端

```bash
cd fronted
cp apps/web/.env.example apps/web/.env.local
pnpm dev                              # :3000
```

### 3. Docker / Helm

- Compose: `make docker-up`
- Helm: `deployments/helm/sparkops/README.md`
- 发布镜像: 打 tag `v1.0.0` 触发 GHCR 推送

## 项目部署流水线

### 配置与授权

后端默认配置位于 `backend/config.example.yaml`：

```yaml
messaging:
  driver: inmemory
runtime:
  driver: mock
delivery:
  driver: mock
deploy:
  jenkins_timeout: 30m
  rollout_timeout: 10m
  reconcile_interval: 30s
```

真实环境可分别设置 `MESSAGING_DRIVER=amqp`、`DELIVERY_DRIVER=jenkins`、`RUNTIME_DRIVER=k8s`。开始部署前：

1. Super Admin 调用 `POST /api/v1/admin/workspaces/:wsId/clusters`，body 为 `{"cluster_id":"<cluster-id>"}`，授权目标 Workspace 使用该集群。
2. Kubernetes 类型工作空间密钥的 `credentials.extra_config` 必须保存完整 kubeconfig；本期不支持把 token/CA 分字段拼成 kubeconfig。
3. 通过 `PATCH /api/v1/projects/:projectId` 绑定 `default_cluster_id`、Git、Harbor、Jenkins、Kubernetes 密钥；启用静态扫描时还需绑定 SonarQube 密钥。
4. 项目审核状态必须为 `approved`。部署弹窗和 options API 只返回当前 Workspace 已授权的集群。

### API 与状态

- `POST /api/v1/projects/:projectId/deployments`：创建部署；`image_tag` 可省略，后端使用 DeploymentRun ID 去连字符后的前 12 位。
- `GET /api/v1/projects/:projectId/deployments/options`：读取可用授权集群及门禁结果。
- `GET /api/v1/projects/:projectId/deployments`、`GET /api/v1/deployments/:runId`：读取历史与详情。
- `GET /api/v1/deployments/:runId/stream`：SSE 阶段事件。
- `POST /api/v1/deployments/:runId/abort`：中止进行中的 Run。

状态序列为 `queued → preparing → building → deploying → succeeded`；进行中状态也可进入 `failed` 或 `aborted`。同一项目只能有一个进行中的 Run。

### 本地 mock 演示

PowerShell 中设置 mock/in-memory driver 后启动后端：

```powershell
$env:MESSAGING_DRIVER="inmemory"
$env:DELIVERY_DRIVER="mock"
$env:RUNTIME_DRIVER="mock"
go run ./cmd/server
```

完成登录、Workspace 集群授权、项目绑定和审核后，依次调用 options、创建部署、详情或 SSE API 即可观察完整状态推进。可重复验收：

```powershell
go test ./internal/modules/delivery/deployrun/application -run TestDeploymentPipeline -v
```

该本地验收使用真实 in-memory Queue/EventBus 与 mock Jenkins/runtime，不连接真实 Jenkins、Harbor 或 Kubernetes。真实外部系统 E2E 尚未证明，不应把 mock 通过视为生产连通性验证。

## 目录

| 路径 | 说明 |
|------|------|
| [`backend/`](backend/) | Go DDD 后端 · `/api/v1` |
| [`fronted/`](fronted/) | Next.js 前端 |
| [`spec/`](spec/) | 架构与设计文档 |

## 规格文档

- [Greenfield Go DDD（后端）](spec/greenfield-go-ddd.md)
- [前端架构 + BFF](spec/frontend-architecture.md)
- [平台愿景](spec/architecture.md)

## 旧系统（只读参考）

- `A:\21-Agent\platformsuite` — 旧 Go DevOps 后端
- `A:\21-Agent\platformsuite-fronted` — 旧 dva 前端

不做数据迁移、不兼容旧 API。

## 默认 Super Admin

| 项 | 值 |
|---|---|
| 邮箱 | `superadmin@sparkops.local` |
| 密码 | `SuperAdmin1234!` |

账号不存在或登录 401 时，在 `backend` 目录执行（PowerShell 单行）：

```powershell
go run ./cmd/grant-super-admin -email superadmin@sparkops.local -password "SuperAdmin1234!" -display-name "Platform Super Admin" -register
```


Demo / 平台超管（空间 owner）
demo@sparkops.local
Demo1234!
管理员
admin@sparkops.local
Demo1234!
研发
developer@sparkops.local
Demo1234!
审核员
reviewer@sparkops.local
Demo1234!
运维
operator@sparkops.local
Demo1234!
只读
viewer@sparkops.local
Demo1234!

SparkOps = 多租户 Agent 控制面 + 可编排数据面；对外通过 REST/OpenAPI 给系统集成，通过 MCP 给 AI 客户端集成，通过 Webhook 给事件驱动集成。

https://docker.aityp.com/i/search?search=jenkins

gitlab: root asdf!123456
glpat-emaYfTem-JstBup3_U43d286MQp1OjEH.01.0w0hwoe9p

admin 
Harbor12345

test-gitlab  glpat-LOYje5HyX9RQd1Kjs-4Vx286MQp1OjMH.01.0w16qu9hd
ssh://git@192.168.5.24:2224/cicd/test-repo.git
http://192.168.5.24:8929/cicd/test-repo.git


jenkins : root 123456