# PlatformSuite → SparkOpsAgent DDD 重构总纲

> 目标仓库：`A:\21-Agent\SparkOpsAgent\backend`（Go 后端）+ `A:\21-Agent\SparkOpsAgent\fronted`（Next.js 前端）  
> 策略：**Strangler Fig（绞杀者模式）** — 后端新代码按 DDD 写，旧 API 保持兼容，逐模块替换  
> 语言决策：**后端继续 Go**，前端 **Next.js 15 App Router + TS 全量重写**，不保留 dva 并行

---

## 一、架构愿景：Control Plane + Data Plane

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Presentation（SparkOpsAgent/fronted · Next.js 15）                      │
│  Chat UI · Agent Studio · Workflow Studio · Admin · K8s Console         │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │ REST / WebSocket / SSE
┌───────────────────────────────▼─────────────────────────────────────────┐
│  Interface Layer（BFF 可选）                                             │
│  鉴权 · 限流 · 路由 · OpenAPI · 流式代理                                  │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
        ┌───────────────────────┴───────────────────────┐
        │                                               │
┌───────▼──────────────┐                    ┌───────────▼──────────────┐
│  Control Plane       │                    │  Data Plane              │
│  管理面：配置与治理    │                    │  运行面：执行与编排       │
│                      │                    │                          │
│  identity            │                    │  runtime（K8s 运维）      │
│  organization        │                    │  delivery（CI/CD）       │
│  application         │─── Domain Event ──▶│  agent_runtime（新）     │
│  governance          │                    │  workflow_engine（新）   │
│  notification        │                    │                          │
│  agent（定义）        │                    │                          │
│  workflow（定义）     │                    │                          │
│  knowledge / memory  │                    │                          │
│  platform_admin      │                    │                          │
└──────────────────────┘                    └──────────────────────────┘
                                │
              PostgreSQL · Redis · K8s · Jenkins · LLM Gateway
```

**Control Plane**：「是什么、怎么配、谁有权」  
**Data Plane**：「怎么跑、怎么部署、怎么执行 Agent/Workflow」

---

## 二、后端目标目录（Go DDD）

```
platformsuite/
├── cmd/
│   ├── server/main.go              # 薄入口：bootstrap.Run()
│   └── migrate/main.go
│
├── internal/
│   ├── bootstrap/
│   │   ├── app.go                  # 生命周期：Init → RegisterModules → Start
│   │   ├── container.go            # 依赖容器（替代全局变量 store/db/K8sClient）
│   │   └── legacy.go               # 旧 pkg 路由挂载（过渡期）
│   │
│   ├── platform/                   # 横切能力（从 src/ 迁移）
│   │   ├── config/
│   │   ├── database/
│   │   ├── cache/
│   │   ├── messaging/              # AMQP EventBus
│   │   ├── observability/          # zap + Prometheus + OTel
│   │   ├── security/               # JWT + Casbin 基础设施
│   │   └── http/                   # 全局 middleware、错误码、响应封装
│   │
│   ├── shared/
│   │   ├── kernel/                 # EntityID, TenantID, Result[T], Page[T]
│   │   ├── events/                 # DomainEvent 接口 + EventBus 抽象
│   │   └── errors/                 # 领域错误码体系
│   │
│   └── modules/                    # ★ 限界上下文（BC）
│       ├── identity/
│       ├── organization/
│       ├── application/
│       ├── delivery/
│       ├── runtime/
│       ├── notification/
│       ├── governance/
│       ├── agent/
│       ├── workflow/
│       ├── knowledge/
│       ├── platform_admin/
│       └── legacy/                 # 绞杀期：旧 transport 代理
│
├── api/openapi/                    # 前后端契约（按 BC 分文件）
├── migrations/                     # 按 schema 分目录
├── src/                            # ⚠ 过渡期保留，逐模块删除
└── go.mod
```

### 2.1 单模块标准结构

```
internal/modules/{bc}/
├── module.go                       # ★ 模块自注册入口
├── interface/
│   └── http/
│       ├── routes.go               # 路由 + 旧路径兼容 alias
│       ├── handler.go              # 合并 transport + endpoint
│       └── dto.go                  # Request/Response（与 Domain 隔离）
├── application/
│   ├── command/                    # 写用例：CreateXxxHandler
│   ├── query/                      # 读用例：GetXxxHandler（CQRS 读侧）
│   └── event_handler/              # 跨 BC 事件订阅
├── domain/
│   ├── model/                      # 聚合根、实体、值对象
│   ├── service/                    # 跨聚合领域服务
│   ├── event/                      # 领域事件定义
│   ├── repository/                 # 仓储接口（interface）
│   └── error.go
└── infrastructure/
    ├── persistence/                # GORM 实现 + mapper
    ├── k8s/                        # K8s adapter（如有）
    ├── jenkins/                    # Jenkins adapter（如有）
    └── acl/                        # 跨 BC 防腐层
```

### 2.2 模块自注册（替代 cmd/service.go 600 行 wire）

```go
// internal/modules/application/module.go
type Module struct{}

func (m *Module) Name() string { return "application" }

func (m *Module) Register(c *bootstrap.Container) error {
    repo := persistence.NewProjectRepo(c.DB())
    bus := c.EventBus()
    createHandler := command.NewCreateProjectHandler(repo, bus)

    c.HTTP().Mount("/api/v2/projects", httpx.NewProjectHandler(createHandler, ...))
    // 兼容旧路径（绞杀期）
    c.HTTP().Mount("/projects", legacy.NewProjectProxy(createHandler, ...))

    bus.Subscribe("delivery.BuildCompleted", event_handler.OnBuildCompleted)
    return nil
}
```

```go
// internal/bootstrap/app.go
func Run() error {
    c := NewContainer(cfg)
    for _, mod := range []Module{
        identity.New(), organization.New(), application.New(),
        delivery.New(), runtime.New(), agent.New(),
        legacy.New(), // 最后挂载，兜底未迁移路由
    } {
        if err := mod.Register(c); err != nil { return err }
    }
    return c.Start()
}
```

### 2.3 依赖规则（强制）

```
interface  →  application  →  domain  ←  infrastructure
                                 ↑
                    modules 之间：仅 Event 或 ACL，禁止 domain 互 import
```

| 允许 | 禁止 |
|------|------|
| application 调 domain 聚合 | domain import gorm/k8s/jenkins |
| infrastructure 实现 domain/repository | domain import 其他 BC 的 domain |
| interface 调 application handler | application import infrastructure 具体类型 |
| ACL 封装跨 BC 调用 | 全局 `repository.Repository` 上帝接口 |

**落地工具**：`go-arch-lint` 或 `depguard` 在 CI 中校验。

---

## 三、限界上下文（BC）优化版

在 8 BC 基础上，按 SparkOpsAgent 愿景拆出 **governance** 与 **agent/workflow 定义 vs 运行** 分离。

| BC | 类型 | 聚合根 | 旧 pkg 映射 | 新能力 |
|----|------|--------|-------------|--------|
| **identity** | Control | Member, Session | auth, member, account, wechat | SSO, API Key |
| **organization** | Control | Organization, Workspace, Group | namespace, group, workspace | 四层租户模型 |
| **governance** | Control | Role, Permission, Policy | role, permission, casbin | ABAC, 策略引擎 |
| **application** | Control | Project, Product, Template | project, product, template, market, discovery | Agent 容器 |
| **delivery** | Data | Build, PipelineRun, Hook | build, jenkinsbuild, git, hooks, cronjob, audit | — |
| **runtime** | Data | Cluster, Workload, Config | pod, deployment, ingress, configmap, storage, ... | — |
| **notification** | Control | Notice, Inbox | notice, event, msgs, proclaim | — |
| **agent** | Control+Data | Agent, AgentVersion | agent, azopai, codereview | Agent Studio |
| **workflow** | Control+Data | WorkflowGraph, Run | （新建） | Human-in-the-loop |
| **knowledge** | Control | KnowledgeBase, Document | （新建） | RAG |
| **platform_admin** | Control | SysConfig, SyncJob | syscfg, syncdatabase, statistics, dashboard, tools | — |
| **legacy** | — | — | 未迁移 pkg | 绞杀代理 |

### 3.1 租户模型迁移（关键）

```
旧模型                          新模型
────────────────────────────────────────────────────
namespace (K8s + Cookie)   →   Organization
                               └── Workspace（兼容原 namespace ID）
                                      └── Project（原「服务/Deployment」）
                                             └── Agent / Workflow / Knowledge
```

**过渡方案**：
- 表 `organization.workspaces` 保留 `legacy_namespace` 字段做双向映射
- 中间件 `NamespaceToContext` → 升级为 `TenantContext{OrgID, WorkspaceID}`
- HTTP Header / Cookie 双读：`X-Workspace-ID` 优先，fallback `namespace`

### 3.2 project/service.go 拆分映射

| 原 service 方法域 | 目标位置 |
|-------------------|----------|
| 创建/审核/状态机 | application/command/ |
| 名称校验、配额 | domain/model/project.go |
| K8s Deployment 操作 | infrastructure/k8s/ |
| Jenkins 触发构建 | infrastructure/jenkins/ + delivery BC 事件 |
| Istio VirtualService | runtime/infrastructure/istio/ |
| AMQP 通知 | application 发事件 → notification 订阅 |

---

## 四、领域事件目录（BC 解耦）

| 事件 | 发布者 | 订阅者 |
|------|--------|--------|
| `MemberCreated` | identity | notification, audit |
| `WorkspaceCreated` | organization | runtime（初始化 K8s ns） |
| `ProjectCreated` | application | delivery, audit |
| `ProjectPublished` | application | runtime, agent, notification |
| `BuildCompleted` | delivery | application, notification |
| `BuildFailed` | delivery | notification, agentops |
| `AgentPublished` | agent | knowledge, runtime |
| `WorkflowRunWaitingApproval` | workflow | notification, governance |
| `ApprovalGranted` | workflow | workflow（resume run） |

**实现**：Phase 1 用现有 AMQP；Phase 2 抽象 `shared/events.EventBus`，可换 Kafka。

---

## 五、API 策略：版本 + 兼容

```
/api/v2/{bc}/{resource}     # 新 DDD 模块，OpenAPI 驱动
/projects/ns/{ns}           # 旧路径，legacy 模块代理 → v2 handler
```

| 阶段 | 策略 |
|------|------|
| 绞杀期 | 旧路由 → legacy proxy → 新 handler（或旧 service） |
| 稳定期 | 旧路由标记 `@deprecated`，前端切 v2 |
| 清理期 | 删除 src/pkg 与 legacy 模块 |

**响应格式统一**（消灭前端 request.js / http.js 分裂）：

```json
{
  "code": 0,
  "message": "ok",
  "data": { },
  "trace_id": "abc-123"
}
```

---

## 六、前端目标目录（Next.js + BFF）

> **决策**：不保留 dva 并行。前端代码放在 `SparkOpsAgent/fronted/`，按 BC 逐页重写；旧 `platformsuite-fronted` 仅作 UI/交互参考，Phase 5 归档删除。  
> **BFF**：页面只调 `/api/bff/v1/*` 或 RSC 直调 `bff-modules`；禁止页面直连 Go API。详见 [`frontend-architecture.md`](./frontend-architecture.md)。

### 6.0 技术栈

| 层级 | 选型 | 说明 |
|------|------|------|
| 框架 | **Next.js 15 App Router** | 文件系统路由，替代 560 行 router.js |
| 语言 | TypeScript 5.x | 严格模式 |
| UI | antd 5 + `@ant-design/nextjs-registry` | SSR 兼容 |
| 样式 | Tailwind CSS + CSS Modules | 新页面优先 Tailwind |
| 数据 | TanStack Query v5 | Client Component 内服务端状态 |
| 表单 | react-hook-form + zod | 复杂向导表单 |
| 全局 UI | zustand | 侧边栏、主题、临时 UI 态 |
| API 契约 | `@sparkops/contracts`（BFF 对外）+ `@sparkops/upstream`（Go 对内） | 页面消费 ViewModel，不消费 Go DTO |
| BFF | Next.js Route Handlers + `packages/bff-modules` | 屏幕用例聚合、绞杀期适配 |
| 展示层 SDK | `@sparkops/web-sdk` | Client Component 调 `/api/bff/v1` |
| 流式 | SSE / WebSocket | Chat UI、终端、构建日志 |
| 包管理 | pnpm workspace | monorepo |

### 6.1 Monorepo 结构

```
SparkOpsAgent/
├── backend/                        # ★ Go DDD 后端（见 greenfield-go-ddd.md）
│   ├── cmd/server/
│   ├── internal/modules/
│   ├── api/openapi/
│   └── migrations/
│
├── fronted/                        # ★ 前端 monorepo 根目录
│   ├── apps/
│   │   └── web/                    # Next.js 15 主应用
│   │       ├── app/
│   │       │   ├── (auth)|(platform)|(studio)/   # 展示层路由
│   │       │   └── api/bff/v1/                   # ★ BFF HTTP 入口（薄）
│   │       └── src/
│   │           ├── modules/        # UI 组件 + hooks（按 BC）
│   │           ├── server/         # RSC loaders / Server Actions
│   │           └── shared/
│   │
│   ├── packages/
│   │   ├── contracts/              # BFF 对外契约（ViewModel / Command）
│   │   ├── web-sdk/                # 展示层调 BFF 的 SDK
│   │   ├── bff-core/               # BFF 基础设施（handler、context、errors）
│   │   ├── bff-modules/            # ★ BFF 用例编排（按 BC）
│   │   ├── upstream/               # Go 后端客户端（仅服务端）
│   │   ├── ui/
│   │   └── tenant-context/
│   │
│   ├── pnpm-workspace.yaml
│   └── turbo.json
│
├── spec/
│   ├── refactor.md
│   └── frontend-architecture.md    # BFF 完整设计
└── READEMD.md
```

### 6.2 App Router 路由树（与 BC 对齐）

```
fronted/apps/web/app/
├── (auth)/                           # 无侧边栏布局
│   ├── login/page.tsx
│   └── callback/page.tsx             # OAuth 回调
│
├── (platform)/                       # 主平台布局（AdminLayout）
│   ├── layout.tsx                    # 侧边栏 + Workspace 选择器
│   ├── dashboard/
│   │   ├── workplace/page.tsx        # 原 /dashboard/workplace
│   │   ├── monitor/page.tsx
│   │   └── statistics/page.tsx
│   │
│   ├── projects/                     # application BC
│   │   ├── page.tsx                  # 原 /project/list
│   │   ├── audit/page.tsx
│   │   ├── create/[[...step]]/page.tsx
│   │   └── [namespace]/[name]/
│   │       ├── page.tsx              # 项目详情
│   │       ├── configmap/page.tsx
│   │       ├── dockerfile/page.tsx
│   │       └── webhooks/page.tsx
│   │
│   ├── delivery/                     # delivery BC
│   │   ├── builds/page.tsx
│   │   ├── cronjobs/page.tsx
│   │   └── publish/page.tsx          # 原 PublishCenter
│   │
│   ├── runtime/                      # runtime BC
│   │   ├── pods/[namespace]/[svc]/[name]/page.tsx
│   │   ├── cluster/page.tsx
│   │   ├── nodes/page.tsx
│   │   ├── config/page.tsx           # ConfigMap
│   │   └── security/
│   │       ├── ingress/page.tsx
│   │       ├── egress/page.tsx
│   │       ├── virtual-services/page.tsx
│   │       └── gateway/page.tsx
│   │
│   ├── templates/page.tsx            # 原 /template/list
│   ├── system/                       # governance BC
│   │   ├── members/page.tsx
│   │   ├── roles/page.tsx
│   │   └── permissions/page.tsx
│   │
│   └── notifications/page.tsx
│
├── (studio)/                         # Agent/Workflow 专用布局
│   ├── layout.tsx                    # StudioLayout（全屏编辑器）
│   ├── agents/
│   │   ├── page.tsx                  # Agent 列表
│   │   ├── [id]/page.tsx             # Agent Studio
│   │   └── chat/page.tsx             # Chat UI（SSE 流式）
│   ├── workflows/
│   │   ├── page.tsx
│   │   └── [id]/page.tsx             # Workflow Studio
│   └── knowledge/
│       └── [id]/page.tsx
│
├── api/                              # 可选 Route Handlers（薄 BFF）
│   └── proxy/[...path]/route.ts      # 开发期代理 Go API（生产走 Nginx/Ingress）
│
├── middleware.ts                     # JWT 校验、未登录跳转
└── layout.tsx                        # Root：Providers（QueryClient、antd、Tenant）
```

### 6.3 模块内代码组织

```
fronted/apps/web/src/modules/{bc}/
├── components/                       # BC 专属 UI 组件
├── hooks/                            # useProjects、useBuildLogs…
├── schemas/                          # zod 校验 schema
├── types/                            # BC 类型（复用 api-client 生成类型）
└── index.ts                          # 对外导出
```

**Server vs Client 划分原则**：

| 类型 | 渲染方式 | 示例 |
|------|----------|------|
| 列表页骨架、权限校验 | Server Component | 项目列表初始数据 |
| 表格交互、表单、终端 | Client Component (`'use client'`) | Pod 日志、xterm 终端 |
| 流式 Chat | Client + SSE | Agent Chat UI |
| 敏感 Token | Server Component / httpOnly Cookie | 不在 Client 暴露 JWT |

### 6.4 Next.js 与 Go 后端对接

```typescript
// next.config.ts — 开发期 API 代理
const nextConfig = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.API_URL}/api/:path*`,
      },
    ];
  },
};
```

```typescript
// src/shared/api/client.ts — 统一请求封装
import { createClient } from '@sparkops/api-client';

export const api = createClient({
  baseUrl: '/api',
  headers: () => ({
    'X-Workspace-ID': getWorkspaceId(), // 替代 Cookie namespace
  }),
});
```

```typescript
// middleware.ts — 鉴权守卫
export function middleware(request: NextRequest) {
  const token = request.cookies.get('token');
  if (!token && !request.nextUrl.pathname.startsWith('/login')) {
    return NextResponse.redirect(new URL('/login', request.url));
  }
}
export const config = { matcher: ['/((?!login|callback|_next|favicon).*)'] };
```

**生产部署**：

```
用户 → Nginx/Ingress
         ├── /          → Next.js (fronted/apps/web :3000)
         └── /api/*     → Go API (platformsuite:8081)
```

Next.js 与 Go 同域部署，Cookie `token` / `X-Workspace-ID` 无需 CORS。

### 6.5 前端模块与后端 BC 对齐

| 前端 route group / module | 后端 BC | API 前缀 |
|---------------------------|---------|----------|
| `(auth)` / identity | identity | `/api/v2/auth`, `/api/v2/members` |
| organization（layout 内） | organization | `/api/v2/workspaces` |
| projects | application | `/api/v2/projects` |
| delivery | delivery | `/api/v2/builds`, `/api/v2/pipelines` |
| runtime | runtime | `/api/v2/runtime/...` |
| system | governance | `/api/v2/roles`, `/api/v2/permissions` |
| agents / chat | agent | `/api/v2/agents` |
| workflows | workflow | `/api/v2/workflows` |
| knowledge | knowledge | `/api/v2/knowledge` |

### 6.6 状态管理

| 场景 | 方案 |
|------|------|
| 服务端数据 | TanStack Query（Client Component 内） |
| 租户上下文 | `@sparkops/tenant-context` Provider + middleware |
| 复杂表单 / 向导 | react-hook-form + zod |
| 全局 UI（侧边栏折叠等） | zustand |
| URL 状态（筛选、分页） | `nuqs` 或 searchParams |
| **废弃** | dva models、roadhog、Cookie namespace |

### 6.7 旧 dva 页面迁移策略（无并行）

按 Phase 逐模块重写，每完成一个 BC 即切换 Ingress 路由，**不保留旧前端**：

| Phase | 迁移范围 | 旧路由参考 | 新 Next.js 路由 |
|-------|----------|------------|-----------------|
| 1 | 登录 + Workspace | `/user/login` | `/login` |
| 2 | 项目 CRUD | `/project/*` | `/projects/*` |
| 3 | 发布 + K8s 运维 | `/project/cornjob/*`, `/pods/*`, `/security/*` | `/delivery/*`, `/runtime/*` |
| 4 | Agent/Workflow | （新建） | `/agents/*`, `/workflows/*` |
| 5 | 剩余页面 + 归档 | `/template/*`, `/system/*` 等 | 对应新路由；删除 `platformsuite-fronted` |

**切换方式**：Ingress 将前端流量从旧静态资源切到 Next.js；后端 API 路径不变（绞杀在后端进行）。

---

## 七、数据库：Schema 按 BC 隔离

```sql
-- 每个 BC 独立 schema，便于后期拆库
CREATE SCHEMA identity;
CREATE SCHEMA organization;
CREATE SCHEMA application;
CREATE SCHEMA delivery;
CREATE SCHEMA runtime;
CREATE SCHEMA agent;
CREATE SCHEMA workflow;

-- 跨 schema 禁止 FK，用 ID 引用 + 事件保持一致性
```

**迁移步骤**：
1. 新表写新 schema，旧表只读
2. 双写期：command handler 同时写新旧表
3. 读切换：query handler 读新表
4. 删旧表

---

## 八、分阶段路线图（优化版）

### Phase 0 — 骨架（1~2 周）

- [ ] `internal/bootstrap` + `shared/kernel` + `platform/http`
- [ ] 模块 `module.go` 注册机制
- [ ] OpenAPI 基线导出
- [ ] 初始化 `SparkOpsAgent/fronted/`：Next.js 15 + pnpm monorepo + `@sparkops/api-client`
- [ ] CI：`go-arch-lint` 依赖规则 + Next.js lint/typecheck

### Phase 1 — identity + organization + governance（2~3 周）

- 拆分上帝 Repository → 各 BC 独立 repo 接口
- 租户上下文升级
- 前端：登录 + Workspace 选择器

**验收**：新登录流走 v2 API，旧 Cookie 仍可用。

### Phase 2 — application（3~4 周）

- 拆分 `project/service.go`（最高 ROI）
- 引入 CQRS：List/Detail 走 query，Create/Publish 走 command
- 前端：项目列表/详情切 modules/application

### Phase 3 — delivery + runtime（3~4 周）

- K8s/Jenkins 下沉 infrastructure
- runtime 模块独立，application 通过事件/ACL 调用
- 前端：PublishCenter → delivery，K8s 页 → runtime

### Phase 4 — agent + workflow + knowledge（4~6 周）

- 新 BC 纯 DDD 开发，不迁移旧代码
- Agent Runtime 独立 ports（Planner, Executor, Tool, Memory）
- 前端：Agent Studio + Chat UI（SSE/WebSocket）

### Phase 5 — 清理（2 周）

- [ ] 删除 `platformsuite/src/pkg/`、`legacy/` 模块
- [ ] 归档删除 `platformsuite-fronted`（dva 旧仓库）
- [ ] 文档与 OpenAPI 定版

---

## 九、测试策略

```
modules/{bc}/
├── domain/          → 单元测试（无 DB、无 HTTP）
├── application/     → 单元测试 + mock repository
├── infrastructure/  → 集成测试（testcontainers PostgreSQL）
└── interface/http/  → API 测试（httptest）
```

**每个 BC 合并前必须通过**：
- domain 测试覆盖率 > 80%
- v2 API 与旧 API 响应等价性测试（绞杀期）

---

## 十、go-kit 迁移对照

| 现有 | 目标 | 说明 |
|------|------|------|
| transport.go | interface/http/handler.go | 可保留 kithttp，或逐步换标准 net/http |
| endpoint.go | 合并到 handler | 减少一层间接 |
| service.go | application/ + domain/ | 核心拆分 |
| logging.go | platform/observability 装饰器 | 统一 |
| MakeHandler | module.go Register | 自注册 |

---

## 十一、风险与对策

| 风险 | 对策 |
|------|------|
| 重构周期过长 | 后端绞杀模式；前端按 BC 分批上线，每批切换 Ingress |
| 双写数据不一致 | 事件 + 对账 job |
| 前端全量重写工作量大 | Phase 2 优先项目模块（最高频）；runtime 页复用 `@sparkops/ui` 表格/表单组件 |
| Next.js + antd SSR 水合问题 | 交互组件标记 `'use client'`；antd 用 `@ant-design/nextjs-registry` |
| K8s 逻辑难拆 | runtime BC 先包一层 port interface，再逐步搬 |
| 团队不熟悉 DDD | 以 application BC 为范本，文档 + Code Review checklist |

---

## 十二、Code Review Checklist（每 PR）

- [ ] 新代码只在 `internal/modules/{bc}/` 下
- [ ] domain 层零外部依赖（无 gorm、无 k8s）
- [ ] 跨 BC 无直接 domain import
- [ ] 写操作用 command handler，读操作用 query handler
- [ ] 仓储接口在 domain，实现在 infrastructure
- [ ] 领域事件用于 BC 间通信
- [ ] 旧 API 兼容或有 deprecation 说明
- [ ] 前端 API 调用走 `@sparkops/api-client`，禁止手写 URL
- [ ] 新页面只在 `fronted/apps/web/app/` 对应 BC 路由下，Client Component 需有 `'use client'` 标注

---

## 附录 A：49 pkg → BC 完整映射

<details>
<summary>点击展开</summary>

| pkg | BC |
|-----|-----|
| auth, member, account, wechat | identity |
| role, permission, casbin | governance |
| namespace, group, workspace | organization |
| project, template, product, market, discovery, proclaim | application / notification |
| build, jenkinsbuild, git, hooks, cronjob, audit | delivery |
| pod, deployment, ingress, configmap, storage, pvc, pv, terminal, clusterinfo, syncresources, virtualservice, gateway, consul, secrets, monitor, namespace | runtime |
| notice, event, msgs | notification |
| agent, azopai | agent |
| syscfg, syncdatabase, statistics, dashboard, public, tools | platform_admin |

</details>

## 附录 B：下一步可执行项

1. 在 `SparkOpsAgent/backend` 创建 Phase 0 后端目录骨架
2. 实现 `identity` 模块作为 DDD 范本（含 module.go + 一个 command）
3. 初始化 `A:\21-Agent\SparkOpsAgent\fronted`：Next.js + pnpm monorepo
4. 从 `backend/api/openapi` 生成第一版前端 API 类型
5. 实现 `/login` + AdminLayout + Workspace 选择器（Phase 1 前端 MVP）
