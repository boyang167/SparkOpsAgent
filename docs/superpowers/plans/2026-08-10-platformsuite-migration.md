# PlatformSuite 全量功能迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `A:\21-Agent\platformsuite` 的全部业务能力迁移至 SparkOpsAgent，按 Phase A→B→C→D→E→F 顺序推进，实现功能清单完整覆盖。

**Architecture:** Greenfield 能力迁移——只迁移 domain 规则与 infrastructure adapter，不搬运 go-kit transport；后端按 DDD BC 组织，前端 Next.js BFF 全量重写；旧系统 `platformsuite-fronted` 仅作 UI 参考。

**Tech Stack:** Go 1.25 · Gin · GORM · PostgreSQL 15 · Redis 7 · RabbitMQ · client-go v0.23 · gojenkins · Next.js 15 · pnpm workspace

## Global Constraints

- 代码根目录：`A:\21-Agent\SparkOpsAgent`
- 旧系统只读参考：`A:\21-Agent\platformsuite`、`A:\21-Agent\platformsuite-fronted`
- 仅 `/api/v1/*`；**不兼容**旧 PlatformSuite API 路径
- **不做**旧库 ETL（Greenfield）；如需历史数据另开 `cmd/migrate-legacy`
- 统一响应 envelope：`{ code, message, data, trace_id }`
- 租户 Header：`X-Workspace-Id`（UUID）
- domain 层禁止 import GORM/HTTP/K8s/Jenkins
- BC 之间禁止 import 对方 infrastructure/domain；跨 BC 用 port/ACL 或 EventBus
- 每个 Task 完成后：`go test ./...` 必须通过（该 Task 涉及包）
- 密钥禁止硬编码；从 env/Secret 注入

## 进度基线（已完成，本计划不重复）

| 已完成 BC | 对应旧模块 |
|-----------|-----------|
| identity, tenant, governance | auth, member, role, permission, namespace, group |
| workspace_app, agent, agent_runtime | project（基础）, agent |
| workflow, knowledge, notification | （新能力 + notice/event） |
| delivery, runtime | build/pod（**Mock 状态，Phase B 替换**） |

---

## 文件结构总览（迁移结束时新增）

```
backend/
├── internal/
│   ├── platform/
│   │   ├── redis/              # Phase A
│   │   ├── messaging/          # Phase A（AMQP + EventBus 抽象）
│   │   ├── k8s/                # Phase A（多集群 client 工厂）
│   │   └── observability/metrics.go
│   └── modules/
│       ├── runtime/            # Phase B 扩展
│       │   └── infrastructure/k8s/{deployment,ingress,configmap,storage,terminal}.go
│       ├── delivery/           # Phase B 扩展
│       │   └── infrastructure/{jenkins,gitlab,webhook,cronjob}/
│       ├── workspace_app/      # Phase C 扩展（template, market, discovery）
│       ├── product/            # Phase C 新建 BC（发布中心）
│       └── platform_admin/     # Phase D 新建 BC
│           ├── sysconfig/
│           ├── cluster/
│           ├── sync/
│           └── statistics/
├── migrations/
│   ├── 000009_platform_infra.up.sql
│   ├── 000010_delivery_runtime.up.sql
│   ├── 000011_workspace_extensions.up.sql
│   ├── 000012_product.up.sql
│   └── 000013_platform_admin.up.sql
fronted/
├── apps/web/app/(platform)/
│   ├── config/                 # Phase E
│   ├── security/
│   ├── cluster/
│   ├── publish/
│   └── dashboard/{monitor,statistics}/
└── packages/
    ├── contracts/src/{runtime,delivery,product,platform_admin}/
    └── bff-modules/src/{runtime,delivery,product,platform_admin}/
```

---

# Phase A：平台基础设施（2–3 周）

> **交付标准：** Compose 启动 PG+Redis+RabbitMQ；`/metrics` 可 scrape；K8s client 可连测试集群；workflow/knowledge 等表有正式 SQL migration。

---

### Task A1: Redis 客户端接入

**Files:**
- Create: `backend/internal/platform/redis/client.go`
- Create: `backend/internal/platform/redis/client_test.go`
- Modify: `backend/internal/platform/appctx/container.go`
- Modify: `backend/internal/bootstrap/app.go`
- Modify: `backend/config.example.yaml`

**Interfaces:**
- Produces: `redis.Client` with `Get(ctx, key) (string, error)`, `Set(ctx, key, val, ttl) error`, `Ping(ctx) error`
- Consumes: `config.RedisConfig`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/platform/redis/client_test.go
package redis_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
	"github.com/stretchr/testify/require"
)

func TestClient_Ping_skips_when_addr_empty(t *testing.T) {
	c, err := redis.NewClient(redis.Config{Addr: ""})
	require.NoError(t, err)
	require.NoError(t, c.Ping(context.Background()))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/platform/redis/... -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement minimal client**

```go
// backend/internal/platform/redis/client.go
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Addr string
}

type Client struct {
	raw *goredis.Client
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return &Client{}, nil
	}
	raw := goredis.NewClient(&goredis.Options{Addr: cfg.Addr})
	return &Client{raw: raw}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c.raw == nil {
		return nil
	}
	return c.raw.Ping(ctx).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c.raw == nil {
		return "", fmt.Errorf("redis not configured")
	}
	return c.raw.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if c.raw == nil {
		return fmt.Errorf("redis not configured")
	}
	return c.raw.Set(ctx, key, val, ttl).Err()
}
```

- [ ] **Step 4: Wire into bootstrap**

在 `appctx.Container` 增加 `Redis *redis.Client`；`bootstrap/app.go` 在 `openDatabase` 后初始化 Redis 并 Ping。

- [ ] **Step 5: Run tests**

Run: `cd backend && go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/platform/redis/ backend/internal/platform/appctx/ backend/internal/bootstrap/ backend/config.example.yaml
git commit -m "feat(platform): add Redis client and bootstrap wiring"
```

---

### Task A2: EventBus 抽象 + AMQP 实现

**Files:**
- Create: `backend/internal/platform/messaging/bus.go`（接口）
- Create: `backend/internal/platform/messaging/inmemory.go`（现有逻辑迁移）
- Create: `backend/internal/platform/messaging/amqp.go`
- Modify: `backend/internal/platform/eventbus/bus.go` → 迁移或 re-export
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/deployments/docker/docker-compose.yaml`

**Interfaces:**
- Produces: `messaging.EventBus` with `Publish(ctx, topic, payload)`, `Subscribe(topic, handler)`
- 队列命名沿用旧系统：`build.sparkops`, `notice.sparkops`, `hook.sparkops` 等

**参考旧代码:** `A:\21-Agent\platformsuite\src\amqp\client.go`

- [ ] **Step 1: 定义 EventBus 接口与 InMemory 实现（带测试）**
- [ ] **Step 2: 扩展 config 增加 `messaging.driver: inmemory|amqp` 与 AMQP URL**
- [ ] **Step 3: docker-compose 增加 RabbitMQ 3 服务（5672/15672）**
- [ ] **Step 4: 实现 AMQP driver；delivery/notification 模块改用新接口**
- [ ] **Step 5: `go test ./...` PASS**
- [ ] **Step 6: Commit** `feat(platform): abstract EventBus with AMQP driver`

---

### Task A3: Prometheus Metrics

**Files:**
- Create: `backend/internal/platform/observability/metrics.go`
- Modify: `backend/internal/platform/httpx/router.go`（注册 `GET /metrics`）
- Modify: `backend/go.mod`（`prometheus/client_golang`）

**参考旧代码:** `A:\21-Agent\platformsuite\cmd\service.go` 中 `/metrics` 挂载

- [ ] **Step 1: 注册 `http_requests_total` counter 与 middleware**
- [ ] **Step 2: handler_test 验证 `/metrics` 返回 200 且含 metric 名**
- [ ] **Step 3: Commit** `feat(platform): expose Prometheus metrics`

---

### Task A4: 多集群 K8s Client 工厂

**Files:**
- Create: `backend/internal/platform/k8s/factory.go`
- Create: `backend/internal/platform/k8s/factory_test.go`
- Modify: `backend/internal/platform/config/config.go`（`k8s.kubeconfig_path`, `k8s.clusters[]`）
- Modify: `backend/config.example.yaml`

**参考旧代码:**
- `A:\21-Agent\platformsuite\src\kubernetes\`
- `A:\21-Agent\platformsuite\config.yml`

**Interfaces:**
- Produces: `k8s.ClusterClientFactory.Get(clusterName string) (kubernetes.Interface, error)`
- Produces: `k8s.ClusterClientFactory.Default() kubernetes.Interface`

- [ ] **Step 1: config 结构体与加载**
- [ ] **Step 2: factory 单集群/多集群单元测试（可用 fake clientset）**
- [ ] **Step 3: 注入 `appctx.Container.K8sFactory`**
- [ ] **Step 4: Commit** `feat(platform): multi-cluster K8s client factory`

---

### Task A5: 补全 SQL Migrations

**Files:**
- Create: `backend/migrations/000009_platform_infra.up.sql` / `.down.sql`
- Create: `backend/migrations/000010_delivery_runtime.up.sql` / `.down.sql`

**内容:**
- `000009`: 确认 workflow/knowledge/notification/delivery schema 正式建表（从各 repo AutoMigrate 提取 DDL）
- `000010`: runtime 元数据表（config_map 元数据、ingress 记录等，如需要）

- [ ] **Step 1: 从 `*_repo.go` 的 GORM model 导出 DDL**
- [ ] **Step 2: `go run ./cmd/migrate` 空库验证**
- [ ] **Step 3: Commit** `chore(migrations): add workflow/knowledge/delivery/runtime DDL`

---

### Phase A 验收清单

- [ ] `docker compose up` 启动 PG + Redis + RabbitMQ + API
- [ ] `curl localhost:8080/metrics` 返回 Prometheus 格式
- [ ] 配置 kubeconfig 后 factory 可 `ListNamespaces`
- [ ] 全部测试 PASS

---

# Phase B：Data Plane 真实化（4–5 周）

> **交付标准：** 真实 K8s 集群上完成「触发 Jenkins 构建 → SSE 日志 → 部署 → Pod 日志/终端」端到端。

---

### Task B1: 真实 Pod Lister / Log Reader

**Files:**
- Create: `backend/internal/modules/runtime/infrastructure/k8s/pod_lister.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/pod_logs.go`
- Modify: `backend/internal/modules/runtime/module.go`（按 config 切换 mock/real）
- Test: `backend/internal/modules/runtime/infrastructure/k8s/pod_lister_test.go`

**参考旧代码:** `A:\21-Agent\platformsuite\src\pkg\pod\service.go`

**Interfaces:**
- Implements: `port.PodLister`, `port.PodLogReader`
- Consumes: `k8s.ClusterClientFactory`, project→namespace 映射（workspace 表 `legacy_namespace` 字段）

- [ ] **Step 1: 扩展 `tenant.workspaces` 增加 `legacy_namespace VARCHAR(128)`（migration 000010）**
- [ ] **Step 2: 实现 PodLister 调 client-go List Pods by label/namespace**
- [ ] **Step 3: 实现 PodLogReader 调 `Pods().GetLogs()`**
- [ ] **Step 4: module.go 根据 `config.Runtime.Driver` 选择 mock|k8s**
- [ ] **Step 5: 集成测试（fake clientset）PASS**
- [ ] **Step 6: Commit** `feat(runtime): real K8s pod list and logs`

---

### Task B2: Web 终端（Exec Stream）

**Files:**
- Create: `backend/internal/modules/runtime/infrastructure/k8s/pod_terminal.go`
- Modify: `backend/internal/modules/runtime/domain/port/ports.go`（`PodTerminal` 改为 `io.Reader` 流式）
- Modify: `backend/internal/modules/runtime/interface/http/handler.go`

**参考旧代码:** `A:\21-Agent\platformsuite\src\pkg\terminal\`

- [ ] **Step 1: 实现 SPDY exec stream adapter**
- [ ] **Step 2: handler SSE 推送终端输出（复用 `httpx/sse.go`）**
- [ ] **Step 3: 前端 `RuntimePodPanel` 验证流式终端**
- [ ] **Step 4: Commit** `feat(runtime): K8s pod exec terminal stream`

---

### Task B3: Deployment / Ingress / ConfigMap / Storage

**Files:**
- Create: `backend/internal/modules/runtime/infrastructure/k8s/deployment.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/ingress.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/configmap.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/storage.go`
- Create: `backend/internal/modules/runtime/application/command/`（scale, apply_yaml 等）
- Create: `backend/internal/modules/runtime/interface/http/` 新 routes
- Modify: `backend/api/openapi/runtime.yaml`

**旧模块映射:**

| 旧 pkg | 新 API 前缀 |
|--------|------------|
| deployment | `PUT /api/v1/workspaces/:wsId/deployments/:name/scale` |
| ingress | `/api/v1/workspaces/:wsId/ingresses` |
| configmap | `/api/v1/workspaces/:wsId/configmaps` |
| storage/pvc/pv | `/api/v1/workspaces/:wsId/storage/...` |
| virtualservice | `/api/v1/workspaces/:wsId/virtualservices` |

**参考旧代码:** `src/pkg/deployment/`, `ingress/`, `configmap/`, `storage/`, `virtualservice/`, `src/istio/`

- [ ] **Task B3a:** Deployment 扩缩容 + YAML 读写
- [ ] **Task B3b:** Ingress CRUD
- [ ] **Task B3c:** ConfigMap 元数据 PG + K8s 内容同步
- [ ] **Task B3d:** StorageClass / PVC / PV 列表
- [ ] **Task B3e:** Istio VirtualService（可选子任务，依赖集群 Istio）
- [ ] **每子任务:** domain test + fake clientset test + OpenAPI 更新 + commit

---

### Task B4: 真实 Jenkins Executor

**Files:**
- Create: `backend/internal/modules/delivery/infrastructure/jenkins/executor.go`
- Create: `backend/internal/modules/delivery/infrastructure/jenkins/executor_test.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `backend/internal/platform/config/config.go`（jenkins.url, user, token）

**参考旧代码:**
- `A:\21-Agent\platformsuite\src\jenkins\`
- `A:\21-Agent\platformsuite\src\pkg\build\`
- `A:\21-Agent\platformsuite\src\pkg\jenkinsbuild\`

- [ ] **Step 1: 添加 `github.com/bndr/gojenkins` 依赖**
- [ ] **Step 2: 实现 `port.JenkinsExecutor` — TriggerJob + PollConsoleOutput → hub.Publish**
- [ ] **Step 3: config `delivery.driver: mock|jenkins`**
- [ ] **Step 4: 端到端：TriggerBuild → SSE stream → BuildCompleted 事件**
- [ ] **Step 5: Commit** `feat(delivery): real Jenkins build executor`

---

### Task B5: GitLab + Webhook + CronJob + Audit

**Files:**
- Create: `backend/internal/modules/delivery/infrastructure/gitlab/client.go`
- Create: `backend/internal/modules/delivery/infrastructure/webhook/handler.go`
- Create: `backend/internal/modules/delivery/application/command/cron_build.go`
- Create: `backend/internal/modules/delivery/interface/http/public_webhook.go`
- Extend: `backend/internal/modules/workflow/`（审批流 `audit` 对接）

**旧模块映射:** `git/`, `hooks/`, `cronjob/`, `audit/`, `public/`

- [ ] **Task B5a:** Git 分支/Tag/Dockerfile 列表（GitLab API）
- [ ] **Task B5b:** Webhook 入站 + AMQP `hook.sparkops` 消费
- [ ] **Task B5c:** CronJob 定时构建（K8s CronJob + DB 记录）
- [ ] **Task B5d:** 发布审批 — Workflow 增加 `approval` 节点类型或 delivery 状态机
- [ ] **验收:** 外部 Webhook 触发构建 → 审批 → 部署闭环

---

### Phase B 验收清单

- [ ] Mock adapter 仍可通过 config 切换（本地无 K8s/Jenkins 时可开发）
- [ ] 真实环境端到端演示录屏 / 集成测试文档
- [ ] OpenAPI `runtime.yaml` + `delivery.yaml` 更新
- [ ] `go test ./...` PASS

---

# Phase C：Control Plane 扩展（4–6 周）

> **交付标准:** 项目模板、Dockerfile 市场、服务发现、发布中心（产品/冲刺/需求）全流程可用。

---

### Task C1: workspace_app 扩展 — Template + Market

**Files:**
- Create: `backend/internal/modules/workspace_app/domain/model/template.go`
- Create: `backend/internal/modules/workspace_app/infrastructure/persistence/template_repo.go`
- Create: `backend/migrations/000011_workspace_extensions.up.sql`
- Modify: `backend/internal/modules/workspace_app/interface/http/handler.go`

**旧模块:** `template/`, `market/`（dockerfile）

**表结构参考:** `platformsuite/src/repository/types/template.go`, `dockerfile.go`

- [ ] **Step 1: Template CRUD API** `/api/v1/workspaces/:wsId/templates`
- [ ] **Step 2: Dockerfile Market CRUD** `/api/v1/workspaces/:wsId/market/dockerfiles`
- [ ] **Step 3: 创建 Project 时可选 Template 初始化 Jenkins/K8s 资源**
- [ ] **Step 4: Commit**

---

### Task C2: 服务发现（Discovery）

**Files:**
- Create: `backend/internal/modules/runtime/application/query/services.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/service_discovery.go`

**旧模块:** `discovery/`

- [ ] **Step 1: List K8s Services + Endpoints by namespace**
- [ ] **Step 2: API** `GET /api/v1/workspaces/:wsId/services`
- [ ] **Step 3: Commit**

---

### Task C3: 项目高级生命周期

**Files:**
- Modify: `backend/internal/modules/workspace_app/domain/model/project.go`（状态机：draft/review/published/archived）
- Create: `backend/internal/modules/workspace_app/application/command/publish_project.go`
- Create: `backend/internal/modules/workspace_app/application/command/review_project.go`

**参考旧代码:** `platformsuite/src/pkg/project/service.go`（审核、发布、Istio 路由联动）

- [ ] **Step 1: 领域状态机单元测试**
- [ ] **Step 2: Publish 触发 `ProjectPublished` 事件 → runtime/delivery 订阅**
- [ ] **Step 3: Commit**

---

### Task C4: 新建 product BC（发布中心）

**Files:**
- Create: `backend/internal/modules/product/`（完整四层结构）
- Create: `backend/migrations/000012_product.up.sql`
- Modify: `backend/internal/bootstrap/app.go`
- Create: `backend/api/openapi/product.yaml`

**旧表:** `product`, `product_plan`, `product_requirements`, `product_members`

**API:**

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/workspaces/:wsId/products` | 产品列表/创建 |
| GET/PATCH | `/api/v1/products/:productId` | 产品详情 |
| GET/POST | `/api/v1/products/:productId/plans` | 冲刺/迭代 |
| GET/POST | `/api/v1/products/:productId/requirements` | 需求 |
| POST | `/api/v1/products/:productId/releases` | 关联 project 发布 |

**参考旧代码:** `src/pkg/product/`, `platformsuite-fronted/src/services/publishCenter.js`

- [ ] **Task C4a:** domain models + migrations
- [ ] **Task C4b:** CRUD handlers + tests
- [ ] **Task C4c:** 与 delivery BC 联动（AppRelease → TriggerBuild）
- [ ] **Task C4d:** bootstrap 注册 + OpenAPI

---

### Phase C 验收清单

- [ ] 模板创建项目成功
- [ ] 发布中心：产品→冲刺→需求→应用发布 链路通
- [ ] `go test ./...` PASS

---

# Phase D：platform_admin BC（4–5 周）

> **交付标准:** 系统配置、多集群管理、跨集群同步、统计大盘、工具箱、公告管理可用。

---

### Task D1:  scaffold platform_admin 模块

**Files:**
- Create: `backend/internal/modules/platform_admin/module.go`
- Create: `backend/migrations/000013_platform_admin.up.sql`
- Modify: `backend/internal/bootstrap/app.go`

**表（参考旧 types）:**
- `platform_admin.sys_cfg` ← `syscfg`
- `platform_admin.cluster_info` ← `clusterinfo`
- `platform_admin.sync_jobs` ← `sync_info`, `sync_mutil_cluster`
- `platform_admin.sync_database_jobs` ← `syncdatabase`

---

### Task D2: SysConfig（外部集成密钥）

**旧模块:** `syscfg/`

- [ ] **API:** `GET/PATCH /api/v1/admin/sysconfig`（仅 platform_admin 权限）
- [ ] 存储 Git/Jenkins/Harbor/Sonar/AI 等配置；密钥字段加密 at rest
- [ ] delivery/runtime 通过 ACL 读取配置，不再硬编码

---

### Task D3: ClusterInfo + 多集群证书

**旧模块:** `clusterinfo/`

- [ ] **API:** `/api/v1/admin/clusters` CRUD
- [ ] 上传 kubeconfig / 证书；与 `platform/k8s/factory` 动态注册

---

### Task D4: 跨集群资源同步

**旧模块:** `syncresources/`, `syncdatabase/`

- [ ] **API:** `/api/v1/admin/sync/resources`, `/api/v1/admin/sync/database`
- [ ] 异步 Job + AMQP；状态可查
- [ ] 参考 `src/sshvm/` 做 DB 同步

---

### Task D5: Statistics + Dashboard + Tools + Proclaim

**旧模块:** `statistics/`, `dashboard/`, `tools/`, `proclaim/`

| 子模块 | API | 说明 |
|--------|-----|------|
| statistics | `GET /api/v1/workspaces/:wsId/statistics/deployments` | 上线统计 |
| dashboard | `GET /api/v1/workspaces/:wsId/dashboard` | 聚合 workplace 数据 |
| tools | `POST /api/v1/admin/tools/clone` 等 | 工具箱 |
| proclaim | `/api/v1/admin/proclaims` | 公告（或并入 notification BC） |

---

### Task D6: Monitor 集成

**旧模块:** `monitor/`

- [ ] 代理 Grafana/Prometheus 链接或 embed URL
- [ ] `GET /api/v1/workspaces/:wsId/monitor/overview`
- [ ] 公开 Webhook: `POST /api/v1/public/prometheus/alerts`（参考 `public/`）

---

### Task D7: 可选 — identity 扩展

**旧模块:** `wechat/`, LDAP, GitHub OAuth（`auth/`）

- [ ] 评估后实现：`POST /api/v1/auth/oauth/github/callback`
- [ ] 若不需要则标记 **WONTFIX**

---

### Phase D 验收清单

- [ ] 系统配置页可管理 Jenkins/Git 凭证
- [ ] 跨集群同步任务可触发并查状态
- [ ] 统计/监控页有数据（真实或 stub 外部 URL）
- [ ] `go test ./...` PASS

---

# Phase E：前端迁移（6–8 周，与 B/C/D 并行）

> **原则:** 页面只调 `/api/bff/v1/*`；每页按 contracts → bff-modules → route handler → page 四步交付。

---

### Task E0: 共享 UI 基建

**Files:**
- Create: `fronted/packages/ui/`（DataTable, FormWizard, YamlEditor, LogViewer 复用组件）
- Modify: `fronted/apps/web/src/components/layout/PlatformNav.tsx`（补全菜单）

**旧菜单参考:** `A:\21-Agent\platformsuite\menu.json`

---

### Task E1: Runtime 控制台页面（与 Phase B 并行）

| 页面 | 路径 | 旧路由 |
|------|------|--------|
| ConfigMap 列表 | `(platform)/config/configmaps/page.tsx` | `/conf/configMap` |
| PVC/Storage | `(platform)/config/storage/page.tsx` | `/conf/storage/list` |
| Ingress 列表 | `(platform)/security/ingress/page.tsx` | `/security/ingress/list` |
| 命名空间 | `(platform)/security/namespaces/page.tsx` | `/security/namespace` |
| 服务发现 | `(platform)/discovery/services/page.tsx` | `/discovery/services/list` |
| CronJob | `(platform)/cronjobs/page.tsx` | `/cornjob/list` |

**每页交付物:**
- `packages/contracts/src/runtime/*.ts`
- `packages/bff-modules/src/runtime/*.ts`
- `apps/web/app/api/bff/v1/.../route.ts`
- `apps/web/src/modules/runtime/*.tsx`

---

### Task E2: Delivery 页面增强（与 Phase B 并行）

| 页面 | 路径 |
|------|------|
| 构建历史（已有） | `/delivery/builds` ✅ 对齐真实 API |
| Webhook 管理 | `(platform)/config/webhooks/page.tsx` |
| 发布审批 | `(platform)/projects/audit/page.tsx` |
| Git 分支/Tag | 嵌入 project detail tab |

---

### Task E3: 发布中心（与 Phase C 并行）

| 页面 | 路径 | 旧路由 |
|------|------|--------|
| 产品列表 | `(platform)/publish/products/page.tsx` | `/PublishCenter/ProductList` |
| 迭代计划 | `(platform)/publish/plans/page.tsx` | `/IterationPlan` |
| 应用发布 | `(platform)/publish/releases/page.tsx` | `/AppRelease` |

**参考:** `platformsuite-fronted/src/routes/PublishCenter/`

---

### Task E4: Platform Admin 页面（与 Phase D 并行）

| 页面 | 路径 |
|------|------|
| 集群列表 | `(platform)/cluster/list/page.tsx` |
| 资源同步 | `(platform)/cluster/sync/page.tsx` |
| 密钥配置 | `(platform)/cluster/config/page.tsx` |
| 系统管理 | `(platform)/system/{members,roles,groups,permissions}/page.tsx` |
| 模板管理 | `(platform)/templates/page.tsx` |
| 工具箱 | `(platform)/tools/page.tsx` |
| 公告 | `(platform)/proclaims/page.tsx` |
| Dockerfile 市场 | `(platform)/market/dockerfiles/page.tsx` |

---

### Task E5: Dashboard 扩展

| 页面 | 路径 | 旧路由 |
|------|------|--------|
| 工作台 | `(platform)/dashboard/workplace/page.tsx` | `/workplace` |
| 监控 | `(platform)/dashboard/monitor/page.tsx` | `/monitor` |
| 统计 | `(platform)/dashboard/statistics/page.tsx` | `/statistics` |

---

### Task E6: 前端 CI

**Files:**
- Create: `.github/workflows/frontend-ci.yaml`

- [ ] pnpm lint + typecheck + build
- [ ] 可选：Playwright smoke（login → projects list）

---

### Phase E 验收清单

- [ ] `menu.json` 所有路由在新前端有对应页或明确 WONTFIX
- [ ] 浏览器只访问 `/api/bff/v1/*`
- [ ] frontend CI green

---

# Phase F：收尾与归档（2 周）

### Task F1: 测试补全

- [ ] 每 BC 至少 1 个 HTTP handler 集成测试
- [ ] runtime/delivery 使用 fake clientset / httptest mock Jenkins
- [ ] testcontainers PostgreSQL 集成测试（governance, product）

### Task F2: 部署

- [ ] Helm Chart: `backend/deployments/helm/sparkops/`
- [ ] 生产 K8s 清单（参考 `platformsuite/install/k8s/`）
- [ ] 前端 Docker 镜像 + Nginx SSE 配置

### Task F3: 文档

- [ ] 更新 `READEMD.md` 迁移完成状态
- [ ] 更新 `spec/refactor.md` 对齐 Greenfield 实际（移除 legacy/v2 未实现描述）
- [ ] 功能对照表：PlatformSuite 48 pkg → SparkOps BC 完成状态

### Task F4: 归档

- [ ] 旧仓库标记 archived（只读）
- [ ] 删除或忽略 Mock adapter 默认启用（生产 config 仅 real driver）

---

## 功能覆盖对照表（完成时逐项打勾）

| # | PlatformSuite 模块 | 目标 BC | Phase |
|---|-------------------|---------|-------|
| 1 | auth | identity | ✅ |
| 2 | member, account | identity | ✅ |
| 3 | role, permission | governance | ✅ |
| 4 | namespace, group, workspace | tenant | ✅ |
| 5 | project | workspace_app | C3 |
| 6 | template | workspace_app | C1 |
| 7 | market | workspace_app | C1 |
| 8 | discovery | runtime | C2 |
| 9 | product | product | C4 |
| 10 | build, jenkinsbuild | delivery | B4 |
| 11 | git | delivery | B5 |
| 12 | hooks, public | delivery | B5 |
| 13 | cronjob | delivery | B5 |
| 14 | audit | delivery/workflow | B5 |
| 15 | pod | runtime | B1 |
| 16 | deployment | runtime | B3 |
| 17 | terminal | runtime | B2 |
| 18 | ingress | runtime | B3 |
| 19 | configmap | runtime | B3 |
| 20 | storage, pvc, pv | runtime | B3 |
| 21 | virtualservice | runtime | B3 |
| 22 | secrets | runtime | B3 |
| 23 | consul | runtime | D6 |
| 24 | clusterinfo | platform_admin | D3 |
| 25 | syncresources | platform_admin | D4 |
| 26 | syncdatabase | platform_admin | D4 |
| 27 | syscfg | platform_admin | D2 |
| 28 | statistics, dashboard | platform_admin | D5 |
| 29 | tools | platform_admin | D5 |
| 30 | proclaim | notification/platform_admin | D5 |
| 31 | notice, event, msgs | notification | ✅ |
| 32 | monitor | platform_admin | D6 |
| 33 | agent, codereview | agent/agent_runtime | ✅ |
| 34 | wechat | identity | D7 可选 |
| 35 | gateway, service(pkg) | — | 评估 WONTFIX |

---

## 依赖关系图

```
Phase A (infra)
    ├── Phase B (runtime + delivery) ── Phase E1/E2 (前端)
    ├── Phase C (workspace + product) ── Phase E3 (前端)
    └── Phase D (platform_admin) ──────── Phase E4/E5 (前端)
                                              └── Phase F (收尾)
```

**并行建议:**
- A 完成后，B/C/D 可分给不同开发者并行
- E1/E2 紧跟 B；E3 紧跟 C；E4/E5 紧跟 D

---

## 风险与回滚

| 风险 | 对策 |
|------|------|
| 无 K8s/Jenkins 测试环境 | 保留 mock driver；CI 用 fake clientset |
| 旧 project/service.go 逻辑复杂 | 按 command 逐个迁移，每命令独立 PR |
| 前端页面多 | 优先高频路径：projects → delivery → runtime → publish |
| Casbin 策略未迁移 | 继续自研 RBAC；admin 权限用 `PermPlatformAdmin` 新常量 |

---

## 执行入口

**推荐从 Task A1 开始。** Phase A 全部完成后进入 Phase B1。

旧代码阅读顺序（Phase B 前预习）:
1. `platformsuite/src/kubernetes/`
2. `platformsuite/src/pkg/pod/`
3. `platformsuite/src/jenkins/`
4. `platformsuite/src/pkg/build/`
