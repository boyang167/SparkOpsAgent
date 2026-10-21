# SparkOps Greenfield 架构设计

> **前提**：不做数据迁移、不兼容旧 API、旧 PlatformSuite 仅作业务参考。  
> **后端**：Go 1.22+ · **Gin** · DDD · 模块化单体  
> **后端**：`A:\21-Agent\SparkOpsAgent\backend\`  
> **前端**：`A:\21-Agent\SparkOpsAgent\fronted\`，另见 `frontend-architecture.md`（BFF 按需启用）

---

## 一、设计原则

| 原则 | 说明 |
|------|------|
| Greenfield | 新目录 `SparkOpsAgent/backend/`，从零建模，不 import 旧 `platformsuite` 代码 |
| 模块化单体 | 一个进程、按 BC 分 module，BC 间不直接 import domain |
| 实用 DDD | 聚合 + 仓储接口 + 领域事件；不 Day-1 上 CQRS/Event Sourcing |
| API 单一版本 | 仅 `/api/v1/{bc}/...`，无 legacy 代理 |
| 租户内建 | Org → Workspace → Project → Agent 写入领域模型，不是 Cookie 补丁 |
| 演进式复杂化 | module 超 800 行再拆；Casbin/Kafka 在 MVP 后用标准接口替换 |

---

## 二、系统上下文

```
                    ┌─────────────────────────────────────┐
                    │           SparkOps Platform          │
                    │  Control Plane  │  Data Plane       │
                    └────────┬───────────────┬────────────┘
                             │               │
         ┌───────────────────┼───────────────┼───────────────────┐
         ▼                   ▼               ▼                   ▼
   PostgreSQL            Redis          LLM Providers      Kubernetes
   (主数据)              (缓存/会话)     (OpenAI/等)        (P2 Runtime)
```

**Control Plane**：身份、租户、权限、Agent/Workflow/Knowledge 定义与配置  
**Data Plane**：Agent 运行时执行、Workflow 引擎、K8s 操作（后期）

---

## 三、限界上下文（BC）

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  identity   │     │   tenant    │     │ governance  │
│  用户/会话   │     │ 组织/工作区  │     │ 角色/策略    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           ▼
              ┌────────────────────────┐
              │      workspace_app       │
              │  Project · 应用容器       │
              └────────────┬───────────┘
                           │
       ┌───────────────────┼───────────────────┐
       ▼                   ▼                   ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│    agent    │     │  workflow   │     │  knowledge  │
│ Agent 定义   │     │ 流程定义     │     │ 知识库       │
└──────┬──────┘     └──────┬──────┘     └─────────────┘
       │                   │
       └─────────┬─────────┘
                 ▼
        ┌─────────────────┐
        │  agent_runtime  │  ← Data Plane
        │  执行/Chat/SSE   │
        └────────┬────────┘
                 ▼
        ┌─────────────────┐
        │    runtime      │  ← P2：K8s/DevOps 集成
        │  集群/工作负载    │
        └─────────────────┘
```

### 3.1 BC 一览

| BC | 类型 | 核心聚合 | MVP |
|----|------|----------|-----|
| **identity** | Control | `Member`, `Session` | ✅ |
| **tenant** | Control | `Organization`, `Workspace` | ✅ |
| **governance** | Control | `Role`, `Policy` | ✅ 简化 RBAC |
| **workspace_app** | Control | `Project` | ✅ |
| **agent** | Control | `Agent`, `AgentVersion` | ✅ |
| **agent_runtime** | Data | `Run`, `Conversation` | ✅ |
| **workflow** | Control+Data | `Workflow`, `WorkflowRun` | P1 |
| **knowledge** | Control | `KnowledgeBase`, `Document` | P1 |
| **notification** | Control | `Notification` | P1 |
| **runtime** | Data | `Cluster`, `Deployment` | P2 |
| **integration** | 支撑 | LLM/MCP/Tool 注册 | ✅ LLM Gateway |

### 3.2 BC 依赖规则

```
允许：application → domain（本 BC）
允许：application → 其他 BC 的 port 接口（domain/repository 或 domain/service）
允许：infrastructure 实现 domain 的 repository 接口
禁止：domain import 任何 infrastructure / 其他 BC 的 domain 包
禁止：BC 之间 import 对方的 infrastructure
跨 BC：Domain Event 或 ACL（Anti-Corruption Layer）
```

---

## 四、租户领域模型（全新设计）

```
Organization（组织）
  id, name, slug, status, settings, created_at

Workspace（工作区 — 资源与配额边界）
  id, org_id, name, slug, status
  quota: { max_agents, max_projects, token_budget }
  settings: { default_model, allowed_models[] }

Project（项目 — Agent/Workflow 容器）
  id, workspace_id, name, description, status

Agent（智能体）
  id, project_id, name, status: Draft|Published|Archived
  current_version_id

AgentVersion（不可变配置快照）
  id, agent_id, version, system_prompt, model_config
  tools[], knowledge_base_ids[], memory_config
```

**与旧系统差异**：无 `namespace` Cookie；所有 API 显式带 `X-Workspace-Id`（或 JWT claim）。

---

## 五、后端目录结构

```
SparkOpsAgent/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bootstrap/
│   │   ├── platform/
│   │   ├── shared/
│   │   └── modules/              # identity, tenant, agent, ...
│   ├── api/openapi/
│   ├── migrations/
│   ├── deployments/
│   ├── go.mod
│   └── Makefile
│
├── fronted/                      # Next.js 前端
└── spec/
```

---

## 六、单 BC 标准结构（DDD 四层）

```
internal/modules/{bc}/
├── module.go                         # 实现 bootstrap.Module，自注册路由与订阅
│
├── interface/
│   └── http/
│       ├── routes.go
│       ├── handler.go                # 解析请求 → 调 application → 写响应
│       └── dto.go                    # Request/Response（与 domain 隔离）
│
├── application/
│   ├── command/                      # 写用例：CreateXxx, PublishXxx
│   ├── query/                        # 读用例：GetXxx, ListXxx
│   └── subscriber/                   # 订阅其他 BC 事件（可选）
│
├── domain/
│   ├── model/                        # 聚合根、实体、值对象、枚举
│   ├── event/                        # 领域事件：AgentPublished 等
│   ├── repository/                   # 仓储 interface
│   ├── service/                      # 跨聚合领域服务（少量）
│   └── errors.go                     # 领域错误
│
└── infrastructure/
    ├── persistence/                  # GORM 实现 + model mapper
    ├── acl/                          # 调其他 BC 的防腐层（可选）
    └── adapter/                      # 外部系统：LLM、K8s（如有）
```

### 6.1 依赖方向

```
interface/http  →  application  →  domain  ←  infrastructure
                                      ↑
                         其他 BC：仅 domain 层 port 或 event
```

### 6.2 module.go 自注册

```go
// internal/modules/agent/module.go
package agent

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Name() string { return "agent" }

func (m *Module) Register(c *bootstrap.Container) error {
    repo := persistence.NewAgentRepository(c.DB())
    pub := command.NewPublishAgentHandler(repo, c.EventBus())
    list := query.NewListAgentsHandler(repo)

    httpx.Register(c.Router(), "/api/v1/agents", NewHandler(pub, list, ...))

    return nil
}
```

```go
// internal/bootstrap/app.go
func Run(ctx context.Context) error {
    c, err := NewContainer()
    if err != nil { return err }

    modules := []Module{
        identity.New(),
        tenant.New(),
        governance.New(),
        workspace_app.New(),
        agent.New(),
        agent_runtime.New(),
        integration.New(),
        // workflow, knowledge, notification, runtime — 按 Phase 加入
    }
    for _, m := range modules {
        if err := m.Register(c); err != nil { return err }
    }
    return c.StartHTTP(ctx)
}
```

---

## 七、核心聚合设计（MVP）

### 7.1 identity · Member

```go
// domain/model/member.go
type Member struct {
    ID           kernel.MemberID
    Email        string
    PasswordHash string
    DisplayName  string
    Status       MemberStatus  // Active, Disabled
    CreatedAt    time.Time
}

func (m *Member) Authenticate(plainPassword string, hasher PasswordHasher) error
func (m *Member) Disable() error
```

### 7.2 tenant · Workspace

```go
type Organization struct {
    ID     kernel.OrgID
    Name   string
    Slug   string
    Status OrgStatus
}

type Workspace struct {
    ID     kernel.WorkspaceID
    OrgID  kernel.OrgID
    Name   string
    Slug   string
    Quota  WorkspaceQuota
    Status WorkspaceStatus
}

func (w *Workspace) CanCreateAgent(currentCount int) error
```

### 7.3 agent · Agent + AgentVersion

```go
type Agent struct {
    ID               kernel.AgentID
    ProjectID        kernel.ProjectID
    Name             string
    Status           AgentStatus
    CurrentVersionID *kernel.AgentVersionID
}

func (a *Agent) Publish(version *AgentVersion) (event.AgentPublished, error) {
    if a.Status != AgentStatusDraft && a.Status != AgentStatusPublished {
        return event.AgentPublished{}, ErrInvalidStatus
    }
    a.Status = AgentStatusPublished
    a.CurrentVersionID = &version.ID
    return event.AgentPublished{AgentID: a.ID, VersionID: version.ID}, nil
}

type AgentVersion struct {
    ID           kernel.AgentVersionID
    AgentID      kernel.AgentID
    Version      int
    SystemPrompt string
    ModelConfig  ModelConfig
    ToolIDs      []kernel.ToolID
}
```

### 7.4 agent_runtime · Conversation + Run

```go
type Conversation struct {
    ID        kernel.ConversationID
    AgentID   kernel.AgentID
    MemberID  kernel.MemberID
    Messages  []Message
}

type Run struct {
    ID             kernel.RunID
    ConversationID kernel.ConversationID
    Status         RunStatus  // Pending, Running, Completed, Failed
    Input          string
    Output         string
    TokenUsage     TokenUsage
}
```

---

## 八、应用层用例（MVP 清单）

### identity

| 类型 | 用例 | HTTP |
|------|------|------|
| command | RegisterMember | POST `/api/v1/auth/register` |
| command | Login | POST `/api/v1/auth/login` |
| query | GetCurrentMember | GET `/api/v1/auth/me` |

### tenant

| 类型 | 用例 | HTTP |
|------|------|------|
| command | CreateOrganization | POST `/api/v1/orgs` |
| command | CreateWorkspace | POST `/api/v1/orgs/{orgId}/workspaces` |
| query | ListMyWorkspaces | GET `/api/v1/workspaces` |

### workspace_app

| 类型 | 用例 | HTTP |
|------|------|------|
| command | CreateProject | POST `/api/v1/workspaces/{wsId}/projects` |
| query | ListProjects | GET `/api/v1/workspaces/{wsId}/projects` |

### agent

| 类型 | 用例 | HTTP |
|------|------|------|
| command | CreateAgent | POST `/api/v1/projects/{projId}/agents` |
| command | CreateAgentVersion | POST `/api/v1/agents/{id}/versions` |
| command | PublishAgent | POST `/api/v1/agents/{id}/publish` |
| query | GetAgent | GET `/api/v1/agents/{id}` |
| query | ListAgents | GET `/api/v1/projects/{projId}/agents` |

### agent_runtime

| 类型 | 用例 | HTTP |
|------|------|------|
| command | StartChat | POST `/api/v1/agents/{id}/chat` |
| query | StreamChat | GET `/api/v1/runs/{runId}/stream` (SSE) |
| query | GetConversation | GET `/api/v1/conversations/{id}` |

### integration

| 类型 | 用例 | HTTP |
|------|------|------|
| — | LLM 路由 | 内部 port，不直接暴露 |
| query | ListModels | GET `/api/v1/models` |

---

## 九、领域事件

| 事件 | 发布者 | 订阅者（MVP 可选） |
|------|--------|-------------------|
| `MemberRegistered` | identity | notification |
| `WorkspaceCreated` | tenant | governance（初始化默认角色） |
| `ProjectCreated` | workspace_app | — |
| `AgentPublished` | agent | agent_runtime（刷新缓存） |
| `RunCompleted` | agent_runtime | notification |

**MVP 实现**：`platform/eventbus` 进程内 sync/async 分发；接口与 MQ 解耦，后期换 Redis Stream / Kafka 不改 domain。

```go
// shared/events/event.go
type Event interface {
    EventName() string
    OccurredAt() time.Time
}

// platform/eventbus/bus.go
type Bus interface {
    Publish(ctx context.Context, events ...Event) error
    Subscribe(eventName string, handler Handler) error
}
```

---

## 十、数据库设计

### 10.1 Schema 按 BC 隔离

```sql
CREATE SCHEMA identity;
CREATE SCHEMA tenant;
CREATE SCHEMA governance;
CREATE SCHEMA workspace_app;
CREATE SCHEMA agent;
CREATE SCHEMA agent_runtime;
CREATE SCHEMA integration;

-- 跨 schema 禁止 FK；用 UUID 引用 + 应用层校验
```

### 10.2 核心表（MVP）

```sql
-- identity.members
-- tenant.organizations, tenant.workspaces, tenant.workspace_members
-- governance.roles, governance.role_bindings
-- workspace_app.projects
-- agent.agents, agent.agent_versions
-- agent_runtime.conversations, agent_runtime.messages, agent_runtime.runs
-- integration.llm_providers, integration.model_catalog
```

### 10.3 ID 策略

统一 `UUID v7`（或 ULID），kernel 层 typed ID 防混用：

```go
type WorkspaceID struct{ uuid.UUID }
type AgentID struct{ uuid.UUID }
```

---

## 十一、HTTP 与鉴权

### 11.1 统一响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "01J..."
}
```

### 11.2 中间件链

```
Request
  → Recover
  → TraceID
  → Logger
  → Authenticate（JWT / API Key）
  → ResolveTenant（X-Workspace-Id → context）
  → Authorize（governance BC）
  → Handler
```

### 11.3 Context

```go
type RequestContext struct {
    TraceID     string
    MemberID    kernel.MemberID
    WorkspaceID kernel.WorkspaceID  // 可选，部分路由必须
}
```

---

## 十二、integration BC（LLM Gateway）

```
integration/
├── domain/
│   ├── model/provider.go       # LLMProvider 聚合
│   └── port/llm.go             # ChatCompletion port
├── application/
│   └── query/list_models.go
└── infrastructure/
    └── adapter/
        ├── openai/
        ├── azure/
        └── router.go           # 按 model_config 路由
```

`agent_runtime` 通过 **port 接口** 调 integration，不直接 import OpenAI SDK。

---

## 十三、与前端协作

| 项 | 约定 |
|----|------|
| API 版本 | 仅 `/api/v1` |
| 契约 | `api/openapi/` → 生成 TS 类型 |
| 租户 | Header `X-Workspace-Id` |
| 流式 | SSE `/api/v1/runs/{id}/stream` |
| BFF | 前端按需；后端提供细粒度 REST 即可 |

Greenfield 阶段 **后端不必为 BFF 做聚合 API**；Dashboard 聚合交给 Next.js Route Handler。

---

## 十四、MVP Phase 与模块启用顺序

```
Phase 0（1 周）  platform + bootstrap + 空 module 注册机制
Phase 1（2 周）  identity + tenant + governance（简化 RBAC）
Phase 2（2 周）  workspace_app + agent
Phase 3（3 周）  integration（LLM）+ agent_runtime（Chat/SSE）
Phase 4（2 周）  OpenAPI、Helm、可观测性
─────────────────────────────────────────
合计约 10 周 MVP

P1：workflow, knowledge, notification
P2：runtime（K8s，从旧系统抄 adapter 逻辑，不抄架构）
```

---

## 十五、测试策略

```
domain/model/       → 纯单元测试（无 DB）
application/        → mock repository
infrastructure/     → testcontainers PostgreSQL
interface/http/     → httptest 集成测试
```

每个 BC 合并前：domain 覆盖率 > 70%，至少 1 条 HTTP  happy path。

---

## 十六、与旧方案差异

| 项 | 绞杀迁移方案 | Greenfield Go DDD（本文） |
|----|-------------|---------------------------|
| 仓库 | 改 platformsuite | 新 `SparkOpsAgent/backend/` |
| 数据 | ETL 双写 | 空库，新 schema |
| API | v1+v2+legacy | 仅 `/api/v1` |
| legacy module | 需要 | 无 |
| BC 数量 Day-1 | 12+ | 7（MVP） |
| 前端 | 500 页迁移 | 新 UI 对接 v1 |

---

## 十七、下一步

1. 按 [`backend-implementation-plan.md`](./backend-implementation-plan.md) 执行 Phase 0~4  
2. 从 Task 0.1 创建 `backend/go.mod` 与 platform 骨架  
3. 以 **identity BC** 作为 DDD 范本（Task 1.1~1.3）  
4. 前端 `fronted/` 在 Phase 1 完成后对接 `/api/v1/auth/*`
