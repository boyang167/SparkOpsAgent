# SparkOps Backend 完整重写实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `A:\21-Agent\SparkOpsAgent\backend` 从零实现 Greenfield Go DDD 后端 MVP（identity → tenant → governance → workspace_app → agent → integration → agent_runtime），提供 `/api/v1` REST + SSE Chat。

**Architecture:** 模块化单体；每 BC 四层（interface/application/domain/infrastructure）；`bootstrap.Module` 自注册；BC 间通过 port 接口与领域事件解耦；PostgreSQL schema 按 BC 隔离。

**Tech Stack:** Go 1.22+ · **Gin** · GORM · PostgreSQL 15 · Redis 7 · golang-jwt/jwt v5 · google/uuid · zap · testify · testcontainers-go · golang-migrate

## Global Constraints

- 代码根目录：`A:\21-Agent\SparkOpsAgent\backend`
- Go module：`github.com/sparkops/sparkops-agent/backend`
- 仅 `/api/v1/*`；不兼容旧 PlatformSuite API
- 不做数据迁移；空库 + migrations
- domain 层禁止 import GORM/HTTP/Redis
- BC 之间禁止 import 对方 infrastructure/domain
- 统一响应 envelope：`{ code, message, data, trace_id }`
- 租户 Header：`X-Workspace-Id`（UUID）
- JWT claim：`member_id`（UUID）
- ID 类型：UUID v4（kernel typed ID wrapper）
- 密码：bcrypt cost=12
- 每个 Task 完成后：`go test ./...` 必须通过（该 Task 涉及包）

---

## 里程碑总览

| Phase | 周期 | 交付 | 验收 |
|-------|------|------|------|
| **0** | 3~4 天 | 骨架、platform、health | `GET /health` 200 |
| **1** | 8~10 天 | identity + tenant + governance | 注册/登录/建 Org/Workspace/RBAC |
| **2** | 8~10 天 | workspace_app + agent | CRUD Project/Agent/Publish |
| **3** | 10~12 天 | integration + agent_runtime | Chat + SSE 流式 |
| **4** | 5~7 天 | OpenAPI、Docker、CI | 容器一键启动 |
| **P1** | 后续 | workflow、knowledge、notification | 按需 |
| **P2** | 后续 | runtime（K8s） | 按需 |

**MVP 合计：约 5~7 周（1 人全职）**

---

## 文件结构总览（MVP 结束时）

```
backend/
├── cmd/server/main.go
├── internal/
│   ├── bootstrap/
│   │   ├── app.go
│   │   ├── container.go
│   │   └── module.go              # Module interface
│   ├── platform/
│   │   ├── config/config.go
│   │   ├── postgres/postgres.go
│   │   ├── redis/redis.go
│   │   ├── httpx/                  # Gin Engine、middleware、统一响应
│   │   ├── authn/{jwt,password}.go
│   │   ├── authz/checker.go
│   │   ├── eventbus/bus.go
│   │   └── observability/{logger,metrics}.go
│   ├── shared/
│   │   ├── kernel/{ids,page,errors}.go
│   │   └── events/event.go
│   └── modules/
│       ├── identity/
│       ├── tenant/
│       ├── governance/
│       ├── workspace_app/
│       ├── agent/
│       ├── integration/
│       └── agent_runtime/
├── migrations/
├── api/openapi/
├── deployments/docker/
├── Makefile
└── go.mod
```

---

# Phase 0：平台骨架（3~4 天）

### Task 0.1: 初始化 Go module 与目录

**Files:**
- Create: `backend/go.mod`
- Create: `backend/Makefile`
- Create: `backend/.gitignore`
- Create: `backend/cmd/server/main.go`

**Interfaces:**
- Produces: `bootstrap.Run(ctx context.Context) error` — 进程入口

- [ ] **Step 1: 创建 go.mod**

```bash
cd A:\21-Agent\SparkOpsAgent\backend
go mod init github.com/sparkops/sparkops-agent/backend
```

- [ ] **Step 2: 创建 main.go**

```go
// cmd/server/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: 创建 Makefile**

```makefile
.PHONY: run test migrate-up
run:
	go run ./cmd/server
test:
	go test ./...
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up
```

- [ ] **Step 4: 验证编译**

Run: `go build ./cmd/server`  
Expected: 成功（bootstrap 包下一步创建）

---

### Task 0.2: shared/kernel 与 events

**Files:**
- Create: `backend/internal/shared/kernel/ids.go`
- Create: `backend/internal/shared/kernel/page.go`
- Create: `backend/internal/shared/kernel/errors.go`
- Create: `backend/internal/shared/events/event.go`
- Test: `backend/internal/shared/kernel/errors_test.go`

**Interfaces:**
- Produces: `kernel.MemberID`, `kernel.WorkspaceID`, `kernel.AgentID` 等 typed ID
- Produces: `kernel.DomainError` — `Code() string`, `Error() string`
- Produces: `events.Event` — `EventName() string`, `OccurredAt() time.Time`

- [ ] **Step 1: 写 failing test**

```go
// internal/shared/kernel/errors_test.go
package kernel_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDomainError(t *testing.T) {
	err := kernel.NewDomainError("IDENTITY_INVALID_CREDENTIALS", "invalid credentials")
	require.Equal(t, "IDENTITY_INVALID_CREDENTIALS", err.Code())
	require.Contains(t, err.Error(), "invalid credentials")
}
```

- [ ] **Step 2: 运行确认 FAIL**

Run: `go test ./internal/shared/kernel/... -v`  
Expected: FAIL — `NewDomainError` undefined

- [ ] **Step 3: 实现 kernel**

```go
// internal/shared/kernel/errors.go
package kernel

type DomainError struct {
	code    string
	message string
}

func NewDomainError(code, message string) *DomainError {
	return &DomainError{code: code, message: message}
}

func (e *DomainError) Code() string    { return e.code }
func (e *DomainError) Error() string   { return e.message }
```

```go
// internal/shared/kernel/ids.go
package kernel

import "github.com/google/uuid"

type MemberID struct{ uuid.UUID }
type OrgID struct{ uuid.UUID }
type WorkspaceID struct{ uuid.UUID }
type ProjectID struct{ uuid.UUID }
type AgentID struct{ uuid.UUID }
type AgentVersionID struct{ uuid.UUID }
type ConversationID struct{ uuid.UUID }
type RunID struct{ uuid.UUID }

func NewMemberID() MemberID     { return MemberID{UUID: uuid.New()} }
func ParseMemberID(s string) (MemberID, error) {
	id, err := uuid.Parse(s)
	return MemberID{UUID: id}, err
}
// 其余 ID 类型同理
```

```go
// internal/shared/kernel/page.go
package kernel

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
```

```go
// internal/shared/events/event.go
package events

import "time"

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type BaseEvent struct {
	Name string
	At   time.Time
}

func (e BaseEvent) EventName() string    { return e.Name }
func (e BaseEvent) OccurredAt() time.Time { return e.At }
```

- [ ] **Step 4: 运行测试 PASS**

Run: `go test ./internal/shared/kernel/... -v`

- [ ] **Step 5: 添加依赖**

Run: `go get github.com/google/uuid github.com/stretchr/testify`

---

### Task 0.3: platform/config + postgres + logger

**Files:**
- Create: `backend/internal/platform/config/config.go`
- Create: `backend/internal/platform/postgres/postgres.go`
- Create: `backend/internal/platform/observability/logger.go`
- Create: `backend/config.example.yaml`

**Interfaces:**
- Produces: `config.Load() (*Config, error)`
- Produces: `postgres.Open(cfg) (*gorm.DB, error)`
- Produces: `observability.NewLogger(cfg) *zap.Logger`

- [ ] **Step 1: config 结构**

```go
// internal/platform/config/config.go
package config

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"` // default :8080
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireHour int    `yaml:"expire_hour"`
}
```

```yaml
# config.example.yaml
server:
  addr: ":8080"
database:
  dsn: "postgres://sparkops:sparkops@localhost:5432/sparkops?sslmode=disable"
redis:
  addr: "localhost:6379"
jwt:
  secret: "change-me-in-production"
  expire_hour: 24
```

- [ ] **Step 2: postgres 连接**

```go
// internal/platform/postgres/postgres.go
package postgres

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}
```

- [ ] **Step 3: 添加依赖**

Run: `go get gorm.io/gorm gorm.io/driver/postgres go.uber.org/zap gopkg.in/yaml.v3`

---

### Task 0.4: platform/httpx 统一响应与路由

**Files:**
- Create: `backend/internal/platform/httpx/response.go`
- Create: `backend/internal/platform/httpx/router.go`
- Create: `backend/internal/platform/httpx/middleware.go`
- Test: `backend/internal/platform/httpx/response_test.go`

**Interfaces:**
- Produces: `httpx.OK(w, traceID, data)` — 写 envelope JSON
- Produces: `httpx.Fail(w, traceID, code, message, httpStatus)`
- Produces: `httpx.NewRouter() chi.Router`
- Produces: `httpx.TraceID(next)` middleware

- [ ] **Step 1: 实现 response envelope**

```go
// internal/platform/httpx/response.go
package httpx

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id"`
}

func WriteJSON(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

func OK(w http.ResponseWriter, traceID string, data interface{}) {
	WriteJSON(w, http.StatusOK, Envelope{Code: 0, Message: "ok", Data: data, TraceID: traceID})
}

func Fail(w http.ResponseWriter, traceID, code, message string, status int) {
	WriteJSON(w, status, Envelope{Code: 1, Message: message, Data: map[string]string{"error_code": code}, TraceID: traceID})
}
```

- [ ] **Step 2: middleware TraceID + Recover**

```go
// internal/platform/httpx/middleware.go
package httpx

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey string

const TraceIDKey ctxKey = "trace_id"

func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get("X-Trace-Id")
		if tid == "" {
			tid = uuid.NewString()
		}
		w.Header().Set("X-Trace-Id", tid)
		ctx := context.WithValue(r.Context(), TraceIDKey, tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}
```

- [ ] **Step 3: router + health**

```go
// internal/platform/httpx/router.go
package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(TraceID)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		OK(w, GetTraceID(r.Context()), map[string]string{"status": "up"})
	})
	return r
}
```

Run: `go get github.com/go-chi/chi/v5`

---

### Task 0.5: bootstrap 容器与 Module 注册

**Files:**
- Create: `backend/internal/bootstrap/module.go`
- Create: `backend/internal/bootstrap/container.go`
- Create: `backend/internal/bootstrap/app.go`

**Interfaces:**
- Produces: `bootstrap.Module` — `Name() string`, `Register(*Container) error`
- Produces: `bootstrap.Run(ctx) error`

- [ ] **Step 1: Module interface**

```go
// internal/bootstrap/module.go
package bootstrap

type Module interface {
	Name() string
	Register(c *Container) error
}
```

- [ ] **Step 2: Container**

```go
// internal/bootstrap/container.go
package bootstrap

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
)

type Container struct {
	Config   *config.Config
	DB       *gorm.DB
	Logger   *zap.Logger
	Router   chi.Router
	EventBus eventbus.Bus
	Server   *http.Server
}
```

- [ ] **Step 3: app.go Run**

```go
// internal/bootstrap/app.go
package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func Run(ctx context.Context) error {
	cfg, err := config.LoadFromEnv() // CONFIG_PATH or default config.yaml
	if err != nil {
		return err
	}
	logger := observability.NewLogger(cfg)
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}

	c := &Container{
		Config:   cfg,
		DB:       db,
		Logger:   logger,
		Router:   httpx.NewRouter(),
		EventBus: eventbus.NewInMemory(),
	}

	modules := []Module{
		// Phase 1+ modules appended here
	}
	for _, m := range modules {
		if err := m.Register(c); err != nil {
			return fmt.Errorf("register module %s: %w", m.Name(), err)
		}
	}

	c.Server = &http.Server{Addr: cfg.Server.Addr, Handler: c.Router}
	go func() {
		<-ctx.Done()
		_ = c.Server.Shutdown(context.Background())
	}()
	logger.Info("server starting", zap.String("addr", cfg.Server.Addr))
	return c.Server.ListenAndServe()
}
```

- [ ] **Step 4: eventbus 内存实现**

```go
// internal/platform/eventbus/bus.go
package eventbus

import (
	"context"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type Handler func(ctx context.Context, event events.Event) error

type Bus interface {
	Publish(ctx context.Context, ev ...events.Event) error
	Subscribe(eventName string, h Handler)
}

type inMemory struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewInMemory() Bus {
	return &inMemory{handlers: make(map[string][]Handler)}
}

func (b *inMemory) Subscribe(eventName string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], h)
}

func (b *inMemory) Publish(ctx context.Context, ev ...events.Event) error {
	for _, e := range ev {
		b.mu.RLock()
		hs := append([]Handler(nil), b.handlers[e.EventName()]...)
		b.mu.RUnlock()
		for _, h := range hs {
			if err := h(ctx, e); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 5: 验收 Phase 0**

Run: `go run ./cmd/server` + `curl http://localhost:8080/health`  
Expected: `{"code":0,"message":"ok","data":{"status":"up"},"trace_id":"..."}`

---

# Phase 1：身份 · 租户 · 权限（8~10 天）

### Task 1.1: identity BC — domain 层

**Files:**
- Create: `backend/internal/modules/identity/domain/model/member.go`
- Create: `backend/internal/modules/identity/domain/model/session.go`
- Create: `backend/internal/modules/identity/domain/repository/member_repo.go`
- Create: `backend/internal/modules/identity/domain/errors.go`
- Create: `backend/internal/modules/identity/domain/event/member_registered.go`
- Test: `backend/internal/modules/identity/domain/model/member_test.go`

**Interfaces:**
- Produces: `model.Member.Authenticate(password, hasher) error`
- Produces: `repository.MemberRepository` — `Save`, `FindByEmail`, `FindByID`

- [ ] **Step 1: Member 聚合测试**

```go
func TestMember_Authenticate_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	m := model.NewMember("a@b.com", "Alice", string(hash))
	err := m.Authenticate("secret", model.BcryptHasher{})
	require.NoError(t, err)
}

func TestMember_Authenticate_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	m := model.NewMember("a@b.com", "Alice", string(hash))
	err := m.Authenticate("wrong", model.BcryptHasher{})
	require.ErrorIs(t, err, model.ErrInvalidCredentials)
}
```

- [ ] **Step 2: 实现 Member + 领域错误**

```go
// domain/errors.go
var (
	ErrInvalidCredentials = kernel.NewDomainError("IDENTITY_INVALID_CREDENTIALS", "invalid email or password")
	ErrMemberDisabled     = kernel.NewDomainError("IDENTITY_MEMBER_DISABLED", "member is disabled")
)
```

- [ ] **Step 3: 运行 domain 测试 PASS**

Run: `go test ./internal/modules/identity/domain/... -v`

---

### Task 1.2: identity BC — infrastructure + migration

**Files:**
- Create: `backend/migrations/000001_identity.up.sql`
- Create: `backend/migrations/000001_identity.down.sql`
- Create: `backend/internal/modules/identity/infrastructure/persistence/member_model.go`
- Create: `backend/internal/modules/identity/infrastructure/persistence/member_repo.go`

**SQL:**

```sql
-- migrations/000001_identity.up.sql
CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.members (
    id          UUID PRIMARY KEY,
    email       VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    status      VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_members_email ON identity.members(email);
```

- [ ] **Step 1: GORM repo 实现 MemberRepository**
- [ ] **Step 2: integration test（testcontainers 可选，或 dev DB）**

Run: `go test ./internal/modules/identity/infrastructure/... -v -tags=integration`

---

### Task 1.3: identity BC — application + HTTP

**Files:**
- Create: `backend/internal/modules/identity/application/command/register_member.go`
- Create: `backend/internal/modules/identity/application/command/login.go`
- Create: `backend/internal/modules/identity/application/query/get_current_member.go`
- Create: `backend/internal/modules/identity/interface/http/handler.go`
- Create: `backend/internal/modules/identity/interface/http/dto.go`
- Create: `backend/internal/modules/identity/module.go`
- Create: `backend/internal/platform/authn/jwt.go`
- Create: `backend/internal/platform/authn/password.go`
- Test: `backend/internal/modules/identity/interface/http/handler_test.go`

**HTTP 路由:**

| Method | Path | Handler |
|--------|------|---------|
| POST | `/api/v1/auth/register` | RegisterMember |
| POST | `/api/v1/auth/login` | Login |
| GET | `/api/v1/auth/me` | GetCurrentMember |

**Interfaces:**
- Produces: `command.LoginHandler.Handle(ctx, email, password) (token string, memberID, error)`
- Produces: JWT middleware `authn.RequireAuth(next)`

- [ ] **Step 1: login handler 集成测试**

```go
func TestLoginHandler_Success(t *testing.T) {
	// setup httptest + in-memory or test DB
	// POST /api/v1/auth/login {"email":"...","password":"..."}
	// assert code=0, data.token non-empty
}
```

- [ ] **Step 2: 注册 identity.Module 到 bootstrap**
- [ ] **Step 3: 验收**

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@sparkops.local","password":"Admin123!","display_name":"Admin"}'
```

---

### Task 1.4: tenant BC — Organization + Workspace

**Files:**
- Create: `backend/migrations/000002_tenant.up.sql`
- Create: `backend/internal/modules/tenant/domain/model/{organization,workspace}.go`
- Create: `backend/internal/modules/tenant/domain/repository/*.go`
- Create: `backend/internal/modules/tenant/application/command/create_org.go`
- Create: `backend/internal/modules/tenant/application/command/create_workspace.go`
- Create: `backend/internal/modules/tenant/application/query/list_workspaces.go`
- Create: `backend/internal/modules/tenant/infrastructure/persistence/*.go`
- Create: `backend/internal/modules/tenant/interface/http/handler.go`
- Create: `backend/internal/modules/tenant/module.go`
- Test: domain + handler tests

**SQL:**

```sql
CREATE SCHEMA IF NOT EXISTS tenant;

CREATE TABLE tenant.organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tenant.workspaces (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    max_agents INT NOT NULL DEFAULT 10,
    max_projects INT NOT NULL DEFAULT 20,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE TABLE tenant.workspace_members (
    workspace_id UUID NOT NULL,
    member_id UUID NOT NULL,
    role VARCHAR(64) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, member_id)
);
```

**HTTP:**

| Method | Path |
|--------|------|
| POST | `/api/v1/orgs` |
| POST | `/api/v1/orgs/{orgId}/workspaces` |
| GET | `/api/v1/workspaces` |

- [ ] **Step 1: Workspace.CanCreateAgent 领域测试**
- [ ] **Step 2: 实现 CRUD + workspace_members 关联**
- [ ] **Step 3: middleware ResolveWorkspace — 读 `X-Workspace-Id`**

```go
// platform/httpx/workspace.go
func RequireWorkspace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsID := r.Header.Get("X-Workspace-Id")
		if wsID == "" {
			Fail(w, GetTraceID(r.Context()), "TENANT_WORKSPACE_REQUIRED", "workspace required", 400)
			return
		}
		// parse UUID, inject context
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: 验收 — 创建 Org → Workspace → List**

---

### Task 1.5: governance BC — 简化 RBAC

**Files:**
- Create: `backend/migrations/000003_governance.up.sql`
- Create: `backend/internal/modules/governance/domain/model/{role,binding}.go`
- Create: `backend/internal/modules/governance/application/command/assign_role.go`
- Create: `backend/internal/modules/governance/application/query/check_permission.go`
- Create: `backend/internal/modules/governance/infrastructure/persistence/*.go`
- Create: `backend/internal/modules/governance/interface/http/handler.go`
- Create: `backend/internal/modules/governance/module.go`
- Create: `backend/internal/platform/authz/checker.go`
- Create: `backend/internal/modules/governance/application/subscriber/on_workspace_created.go`

**SQL:**

```sql
CREATE SCHEMA IF NOT EXISTS governance;

CREATE TABLE governance.roles (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name VARCHAR(64) NOT NULL,
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

CREATE TABLE governance.role_bindings (
    id UUID PRIMARY KEY,
    role_id UUID NOT NULL,
    member_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(role_id, member_id)
);
```

**默认权限字符串（MVP）：**

```
project:read, project:write, agent:read, agent:write, agent:publish, chat:use
```

**HTTP:**

| Method | Path |
|--------|------|
| GET | `/api/v1/workspaces/{wsId}/roles` |
| POST | `/api/v1/workspaces/{wsId}/roles` |
| POST | `/api/v1/workspaces/{wsId}/members/{memberId}/roles` |

- [ ] **Step 1: 订阅 `WorkspaceCreated` 事件自动创建 admin 角色**
- [ ] **Step 2: authz.Checker — `HasPermission(ctx, memberID, wsID, perm) bool`**
- [ ] **Step 3: 在 agent/project 路由前挂 authz middleware**

**Phase 1 验收清单:**
- [ ] 注册 + 登录拿 JWT
- [ ] 创建 Org + Workspace
- [ ] Workspace 创建后自动有 admin 角色
- [ ] `GET /api/v1/auth/me` 返回当前用户
- [ ] 无 Token 访问受保护路由 → 401

---

# Phase 2：Project + Agent（8~10 天）

### Task 2.1: workspace_app BC — Project

**Files:**
- Create: `backend/migrations/000004_workspace_app.up.sql`
- Create: `backend/internal/modules/workspace_app/domain/model/project.go`
- Create: `backend/internal/modules/workspace_app/application/command/create_project.go`
- Create: `backend/internal/modules/workspace_app/application/query/list_projects.go`
- Create: `backend/internal/modules/workspace_app/infrastructure/persistence/project_repo.go`
- Create: `backend/internal/modules/workspace_app/infrastructure/acl/tenant_acl.go`
- Create: `backend/internal/modules/workspace_app/interface/http/handler.go`
- Create: `backend/internal/modules/workspace_app/module.go`

**SQL:**

```sql
CREATE SCHEMA IF NOT EXISTS workspace_app;

CREATE TABLE workspace_app.projects (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);
```

**HTTP:**

| Method | Path |
|--------|------|
| POST | `/api/v1/workspaces/{wsId}/projects` |
| GET | `/api/v1/workspaces/{wsId}/projects` |
| GET | `/api/v1/projects/{projectId}` |

- [ ] **Step 1: tenant_acl 校验 workspace 归属**
- [ ] **Step 2: authz 检查 `project:write` / `project:read`**
- [ ] **Step 3: 发布 `ProjectCreated` 事件**

---

### Task 2.2: agent BC — Agent + AgentVersion + Publish

**Files:**
- Create: `backend/migrations/000005_agent.up.sql`
- Create: `backend/internal/modules/agent/domain/model/{agent,agent_version}.go`
- Create: `backend/internal/modules/agent/domain/event/agent_published.go`
- Create: `backend/internal/modules/agent/application/command/{create_agent,create_version,publish_agent}.go`
- Create: `backend/internal/modules/agent/application/query/{get_agent,list_agents}.go`
- Create: `backend/internal/modules/agent/infrastructure/persistence/*.go`
- Create: `backend/internal/modules/agent/interface/http/handler.go`
- Create: `backend/internal/modules/agent/module.go`
- Test: `domain/model/agent_test.go` — Publish 状态机

**SQL:**

```sql
CREATE SCHEMA IF NOT EXISTS agent;

CREATE TABLE agent.agents (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    current_version_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE TABLE agent.agent_versions (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL,
    version INT NOT NULL,
    system_prompt TEXT NOT NULL,
    model_name VARCHAR(128) NOT NULL,
    model_config JSONB NOT NULL DEFAULT '{}',
    tool_ids JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, version)
);
```

**HTTP:**

| Method | Path |
|--------|------|
| POST | `/api/v1/projects/{projectId}/agents` |
| GET | `/api/v1/projects/{projectId}/agents` |
| GET | `/api/v1/agents/{agentId}` |
| POST | `/api/v1/agents/{agentId}/versions` |
| POST | `/api/v1/agents/{agentId}/publish` |

- [ ] **Step 1: Agent.Publish 领域测试 — draft→published，非法状态报错**
- [ ] **Step 2: 检查 Workspace quota `max_agents`**
- [ ] **Step 3: 发布 `AgentPublished` 事件**

**Phase 2 验收:**
- [ ] Project CRUD
- [ ] Agent 创建 → 添加 Version → Publish
- [ ] 超 quota 创建 Agent → 403

---

# Phase 3：LLM Integration + Agent Runtime（10~12 天）

### Task 3.1: integration BC — LLM Gateway

**Files:**
- Create: `backend/migrations/000006_integration.up.sql`
- Create: `backend/internal/modules/integration/domain/port/llm.go`
- Create: `backend/internal/modules/integration/infrastructure/adapter/openai/client.go`
- Create: `backend/internal/modules/integration/infrastructure/adapter/router.go`
- Create: `backend/internal/modules/integration/application/query/list_models.go`
- Create: `backend/internal/modules/integration/interface/http/handler.go`
- Create: `backend/internal/modules/integration/module.go`

**Interfaces:**
- Produces: `port.LLMClient.ChatCompletion(ctx, req) (stream iter.Seq[string], err error)` 或 callback 模式

```go
// domain/port/llm.go
package port

type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
}

type Message struct {
	Role    string
	Content string
}

type LLMClient interface {
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(string) error) error
	ListModels(ctx context.Context) ([]ModelInfo, error)
}
```

**HTTP:**

| Method | Path |
|--------|------|
| GET | `/api/v1/models` |

**Config 扩展:**

```yaml
llm:
  default_provider: openai
  providers:
    openai:
      api_key: ${OPENAI_API_KEY}
      base_url: https://api.openai.com/v1
```

- [ ] **Step 1: OpenAI adapter 单元测试（mock HTTP）**
- [ ] **Step 2: router 按 model_name 选 provider**

---

### Task 3.2: agent_runtime BC — Conversation + Run + Chat

**Files:**
- Create: `backend/migrations/000007_agent_runtime.up.sql`
- Create: `backend/internal/modules/agent_runtime/domain/model/{conversation,run,message}.go`
- Create: `backend/internal/modules/agent_runtime/application/command/start_chat.go`
- Create: `backend/internal/modules/agent_runtime/application/query/stream_run.go`
- Create: `backend/internal/modules/agent_runtime/infrastructure/persistence/*.go`
- Create: `backend/internal/modules/agent_runtime/infrastructure/acl/{agent_acl,llm_acl}.go`
- Create: `backend/internal/modules/agent_runtime/interface/http/handler.go`
- Create: `backend/internal/modules/agent_runtime/interface/http/sse.go`
- Create: `backend/internal/modules/agent_runtime/module.go`

**SQL:**

```sql
CREATE SCHEMA IF NOT EXISTS agent_runtime;

CREATE TABLE agent_runtime.conversations (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL,
    member_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_runtime.messages (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL,
    role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_runtime.runs (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL,
    agent_version_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    input TEXT NOT NULL,
    output TEXT,
    prompt_tokens INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
```

**HTTP:**

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/agents/{agentId}/chat` | body: `{ "message": "...", "conversation_id": "optional" }` → 返回 `{ run_id, conversation_id }` |
| GET | `/api/v1/runs/{runId}/stream` | SSE 流式输出 |
| GET | `/api/v1/conversations/{id}` | 历史消息 |

- [ ] **Step 1: start_chat — 加载已发布 AgentVersion，创建 Run(status=running)**
- [ ] **Step 2: 异步 goroutine 调 LLM stream，写 output + messages**
- [ ] **Step 3: SSE handler**

```go
// interface/http/sse.go
func StreamRun(w http.ResponseWriter, r *http.Request, runID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	// poll run output or subscribe in-memory channel
	fmt.Fprintf(w, "data: %s\n\n", chunk)
	flusher.Flush()
}
```

- [ ] **Step 4: Run 完成发布 `RunCompleted` 事件**

**Phase 3 验收:**
- [ ] 对已发布 Agent 发起 Chat
- [ ] SSE 收到流式 token
- [ ] 对话历史可查询
- [ ] 未发布 Agent Chat → 400

---

# Phase 4：工程化与交付（5~7 天）

### Task 4.1: OpenAPI 文档

**Files:**
- Create: `backend/api/openapi/identity.yaml`
- Create: `backend/api/openapi/tenant.yaml`
- Create: `backend/api/openapi/agent.yaml`
- Create: `backend/api/openapi/agent_runtime.yaml`
- Create: `backend/api/openapi/openapi.yaml`（聚合）

- [ ] **Step 1: 每个 BC 导出 paths + schemas**
- [ ] **Step 2: 前端 `fronted/packages/api-types` 用 openapi-typescript 生成**

Run: `npx openapi-typescript backend/api/openapi/openapi.yaml -o fronted/packages/api-types/src/schema.ts`

---

### Task 4.2: Docker Compose 本地环境

**Files:**
- Create: `backend/deployments/docker/docker-compose.yaml`
- Create: `backend/deployments/docker/Dockerfile`

```yaml
# docker-compose.yaml
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: sparkops
      POSTGRES_PASSWORD: sparkops
      POSTGRES_DB: sparkops
    ports: ["5432:5432"]
  redis:
    image: redis:7
    ports: ["6379:6379"]
  api:
    build: .
    ports: ["8080:8080"]
    depends_on: [postgres, redis]
    environment:
      DATABASE_DSN: postgres://sparkops:sparkops@postgres:5432/sparkops?sslmode=disable
```

- [ ] **Step 1: Dockerfile 多阶段构建**
- [ ] **Step 2: `make docker-up` 一键启动**

---

### Task 4.3: CI 与架构约束

**Files:**
- Create: `backend/.github/workflows/ci.yaml`
- Create: `backend/scripts/check-deps.go` 或 `go-arch-lint.yml`

- [ ] **Step 1: CI — test + lint + build**
- [ ] **Step 2: 禁止 domain import gorm 的静态检查**

```yaml
# .github/workflows/ci.yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go test ./...
      - run: go build ./cmd/server
```

---

### Task 4.4: 种子数据 CLI

**Files:**
- Create: `backend/cmd/seed/main.go`

- [ ] **Step 1: seed 命令创建 demo org/workspace/admin/project/agent**

Run: `go run ./cmd/seed`  
Expected: 输出 demo 账号与 API 调用示例

**Phase 4 验收:**
- [ ] docker-compose up 后全流程可用
- [ ] OpenAPI 可导入 Apifox/Postman
- [ ] CI green

---

# Phase P1（后续迭代）

| BC | 核心能力 | 预估 |
|----|----------|------|
| workflow | WorkflowGraph CRUD + Run + 人工审批 | 3 周 |
| knowledge | Document 上传 + 分块 + 向量检索 RAG | 3 周 |
| notification | 站内信 + 订阅领域事件 | 1 周 |

---

# Phase P2（后续迭代）

| BC | 核心能力 | 预估 |
|----|----------|------|
| runtime | K8s client — Pod 列表/日志/终端 | 4 周 |
| runtime | 作为 Agent Tool 暴露，非完整 DevOps UI | — |

参考旧系统 `platformsuite/src/pkg/pod`、`terminal`，只抄 adapter 逻辑。

---

## 跨 BC Port 接口清单（MVP）

| 消费方 | Port | 提供方 | 方法 |
|--------|------|--------|------|
| workspace_app | WorkspaceChecker | tenant | `MemberInWorkspace(memberID, wsID) bool` |
| agent | ProjectChecker | workspace_app | `ProjectInWorkspace(projectID, wsID) bool` |
| agent | QuotaChecker | tenant | `CanCreateAgent(wsID) error` |
| agent_runtime | AgentReader | agent | `GetPublishedVersion(agentID) (*AgentVersion, error)` |
| agent_runtime | LLMClient | integration | `ChatStream(...)` |
| governance subscriber | — | tenant event | `WorkspaceCreated` |

实现方式：infrastructure/acl 或 application 层 interface，bootstrap  wiring。

---

## 测试矩阵

| 层 | 工具 | 覆盖率目标 |
|----|------|-----------|
| domain/model | testify | > 80% |
| application | testify + mock | > 70% |
| infrastructure | testcontainers PG | 关键 repo |
| interface/http | httptest | 每 BC ≥ 1 happy path |

Run: `go test ./... -cover`

---

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| SSE 生产环境缓冲 | Nginx `proxy_buffering off`；文档注明 |
| LLM 成本高 | Workspace token_budget 字段预留；P1 做限流 |
| BC 耦合 | Code Review 检查 cross-import；acl 强制 |
| 周期超预期 | Phase 3 可先用非流式 Chat 降级交付 |

---

## 执行顺序建议

```
Week 1     Task 0.1 ~ 0.5
Week 2     Task 1.1 ~ 1.3
Week 3     Task 1.4 ~ 1.5
Week 4     Task 2.1 ~ 2.2
Week 5-6   Task 3.1 ~ 3.2
Week 7     Task 4.1 ~ 4.4
```

---

## Spec 覆盖自检

| spec 要求 | 对应 Task |
|-----------|-----------|
| Greenfield 新 backend | 全部 |
| 7 BC MVP | Phase 1~3 |
| Schema 隔离 | 各 migration Task |
| 领域事件 | Task 0.5, 1.5, 2.1, 2.2, 3.2 |
| JWT + X-Workspace-Id | Task 1.3, 1.4 |
| SSE Chat | Task 3.2 |
| OpenAPI | Task 4.1 |
| 无 legacy | Global Constraints |

---

**Plan complete.** 保存路径：`A:\21-Agent\SparkOpsAgent\spec\backend-implementation-plan.md`

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每个 Task 派发独立 subagent，Task 间 review  
2. **Inline Execution** — 当前会话按 Task 顺序直接实现，Phase 边界做 checkpoint  

你希望用哪种方式开始？若确认，可从 **Task 0.1** 在 `backend/` 创建 Go module 骨架。
