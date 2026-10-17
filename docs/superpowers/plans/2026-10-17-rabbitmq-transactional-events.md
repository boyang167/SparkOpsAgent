# RabbitMQ Transactional Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 SparkOps 全部领域/集成事件迁移为 PostgreSQL Transactional Outbox + RabbitMQ quorum queues + Transactional Inbox，并用 Operation Outbox 隔离外部副作用，同时保留 `inmemory` 的本地开发和单元测试语义。

**Architecture:** 业务命令通过上下文事务协调器原子写业务数据和 Event Outbox；多实例 Dispatcher 使用租约领取记录并通过 Publisher Confirm 投递到 RabbitMQ topic exchange。每个具名逻辑订阅者拥有独立 quorum queue，消费时在一个数据库事务内写 Inbox 和本地副作用；Jenkins、Kubernetes、Webhook 及现有任务 Queue 发布改写为 Operation Outbox，由独立执行器重试和对账。

**Tech Stack:** Go 1.26、GORM 1.31、PostgreSQL、RabbitMQ 3.x、`amqp091-go` 1.13、Gin、Testify、GitHub Actions services。

## Global Constraints

- `messaging.driver: inmemory` 继续使用同步 `NewInMemoryBus()`，不要求 RabbitMQ，不经过 Event Outbox/Inbox。
- 生产环境只允许 `messaging.driver: amqp`，并强制启用 Event Outbox/Inbox。
- EventBus 与现有异步任务 `Queue` 保持独立交换机和拓扑。
- 生产事件主队列、重试队列和 DLQ 全部声明为 `x-queue-type=quorum`；生产 RabbitMQ 至少三个节点。
- RabbitMQ 提供至少一次投递；本系统数据库副作用通过 `(event_id, subscriber)` Inbox 唯一键实现效果一次。
- 业务数据与 Event Outbox 必须在同一个 PostgreSQL 事务提交；缺少事务的 AMQP `Publish` 必须返回错误。
- 临时消费错误按 5 秒、30 秒、5 分钟、30 分钟分级退避，之后每 30 分钟持续重试。
- 未知事件类型、未知版本、非法载荷和显式永久业务错误进入订阅者 DLQ。
- 外部调用不得在事件消费事务中执行；事务只写 `platform.operation_outbox`。
- 所有消息携带稳定 `event_id`；Webhook 和支持幂等键的外部 API使用该 ID或派生操作 ID。
- `inmemory` 和 `amqp` 共享事件注册表、事件信封和具名订阅接口。
- 现有 10 条有效生产发布链和 15 个生产订阅者全部迁移；`tenant.WorkspaceCreated` 死代码不注册。
- 每个行为变化先写失败测试并确认失败，再写最小实现。

---

## File Map

### Platform foundation

- Create `backend/internal/platform/postgres/transaction.go`: 上下文事务协调器和事务句柄解析。
- Create `backend/internal/platform/postgres/transaction_test.go`: 提交、回滚、嵌套复用测试。
- Create `backend/migrations/000070_transactional_events.up.sql`: Event Outbox、Inbox、Operation Outbox。
- Create `backend/migrations/000070_transactional_events.down.sql`: 逆向迁移。
- Modify `backend/migrations/project_deploy_pipeline_test.go`: 新迁移静态约束测试。
- Create `backend/internal/platform/messaging/persistence/models.go`: GORM fallback 模型。
- Create `backend/internal/platform/messaging/persistence/migrate.go`: 非 SQL migration 环境的建表入口。
- Modify `backend/internal/bootstrap/migrate.go`: 注册 messaging persistence migration。

### Event contracts and local adapter

- Create `backend/internal/shared/events/envelope.go`: 事件信封、聚合引用、上下文元数据。
- Create `backend/internal/shared/events/registry.go`: 显式事件类型/版本注册表。
- Create `backend/internal/shared/events/registry_test.go`: 编解码、重复注册、未知版本测试。
- Modify `backend/internal/platform/messaging/bus.go`: 具名订阅和生命周期接口。
- Modify `backend/internal/platform/messaging/inmemory.go`: 新接口兼容、重复订阅检测、事件元数据注入。
- Modify `backend/internal/platform/messaging/bus_test.go`: 同步 fan-out 和具名订阅测试。
- Modify `backend/internal/platform/eventbus/bus.go`: 重导出新接口。

### Reliable event transport

- Create `backend/internal/platform/messaging/persistence/outbox.go`: Outbox 插入、租约领取、重试和发布完成。
- Create `backend/internal/platform/messaging/persistence/inbox.go`: Inbox 原子去重。
- Create `backend/internal/platform/messaging/persistence/operation_outbox.go`: 外部操作持久化和租约。
- Create `backend/internal/platform/messaging/outbox_dispatcher.go`: 多实例 Outbox Dispatcher。
- Create `backend/internal/platform/messaging/amqp_event_transport.go`: topic exchange、quorum queues、Publisher Confirm。
- Create `backend/internal/platform/messaging/amqp_event_topology.go`: 主队列、分级重试队列、DLQ。
- Create `backend/internal/platform/messaging/reliable_bus.go`: AMQP Publish/Subscribe/Start/Close。
- Create `backend/internal/platform/messaging/inbox_consumer.go`: Inbox 事务消费包装器。
- Create `backend/internal/platform/messaging/errors.go`: 临时/永久错误分类。
- Create `backend/internal/platform/messaging/metrics.go`: Outbox、Inbox、重试、DLQ 和 Operation 指标。
- Create corresponding `*_test.go` files under `backend/internal/platform/messaging/`.

### Operation Outbox

- Create `backend/internal/platform/messaging/operation.go`: 操作信封、注册表和幂等键。
- Create `backend/internal/platform/messaging/operation_dispatcher.go`: 外部操作执行、租约和持续重试。
- Create `backend/internal/platform/messaging/operation_test.go`.
- Create `backend/internal/modules/delivery/infrastructure/outbound/operation_handlers.go`: Webhook、Jenkins credential、deploy queue 操作执行器。
- Create `backend/internal/modules/product_workflow/infrastructure/outbound/operation_handlers.go`: 工作流 build/deploy/hook 操作执行器。
- Create `backend/internal/modules/product/infrastructure/outbound/operation_handlers.go`: Jira、release-plan hook、工作流触发操作执行器。
- Modify `backend/internal/modules/workspace_app/application/command/archive_coordinator.go`: 归档外部动作写 Operation Outbox。

### Configuration and lifecycle

- Modify `backend/internal/platform/config/config.go`: Event exchange 和 Outbox 配置。
- Modify `backend/internal/platform/config/validate.go`: 生产可靠性校验。
- Modify `backend/internal/platform/config/validate_test.go`: 配置测试。
- Modify `backend/internal/platform/messaging/factory.go`: `inmemory`/`amqp` 双实现工厂。
- Modify `backend/internal/platform/appctx/container.go`: Registry、Transactor、Operation Writer。
- Modify `backend/internal/bootstrap/app.go`: 模块注册后启动消息生命周期并优雅关闭。
- Modify `backend/config.example.yaml`, `backend/config.remote.example.yaml`, `backend/deployments/docker/config.docker.yaml`.
- Modify `backend/deployments/docker/docker-compose.yaml`: 本地 RabbitMQ 集成测试配置，不宣称生产 quorum 容灾。

### Business event migration

- Modify all event definitions under:
  - `backend/internal/modules/identity/domain/event/`
  - `backend/internal/shared/integration/`
  - `backend/internal/modules/workspace_app/domain/event/`
  - `backend/internal/modules/agent/domain/event/`
  - `backend/internal/modules/delivery/domain/event/`
  - `backend/internal/modules/workflow/domain/event/`
  - `backend/internal/modules/product_workflow/domain/event/`
  - `backend/internal/modules/agent_runtime/domain/event/`
- Modify production publishers:
  - `backend/internal/modules/identity/application/command/register_member.go`
  - `backend/internal/modules/tenant/application/command/create_org_workspace.go`
  - `backend/internal/modules/workspace_app/application/command/create_project.go`
  - `backend/internal/modules/workspace_app/application/command/secret_commands.go`
  - `backend/internal/modules/agent/application/command/agent_commands.go`
  - `backend/internal/modules/delivery/application/command/trigger_build.go`
  - `backend/internal/modules/delivery/module.go`
  - `backend/internal/modules/workflow/application/command/workflow_run_commands.go`
  - `backend/internal/modules/product_workflow/domain/engine/scheduler.go`
  - `backend/internal/modules/agent_runtime/application/command/start_chat.go`
- Modify subscription registration in `product`, `product_workflow`, `delivery`, `workspace_app`, `governance`, and `notification` module files.
- Modify participating GORM repositories to call `postgres.DB(ctx, r.db)` instead of `r.db.WithContext(ctx)`.

### Integration and rollout

- Create `backend/internal/platform/messaging/event_pipeline_integration_test.go`.
- Create `backend/cmd/event-dlq/main.go`: 受控 DLQ 检查和原事件 ID 重放命令。
- Create `backend/cmd/event-dlq/main_test.go`: 参数校验和重放行为测试。
- Modify `backend/internal/modules/delivery/deployrun/application/deployment_pipeline_integration_test.go`.
- Modify `.github/workflows/ci.yaml`: Go 1.26 and PostgreSQL/RabbitMQ integration job.
- Modify `deployments/helm/sparkops/values.yaml`: AMQP event defaults.
- Modify `deployments/helm/sparkops/values-production.yaml`: production reliable messaging defaults.
- Modify `deployments/helm/sparkops/templates/api-deployment.yaml`: event exchange environment wiring.
- Modify `deployments/helm/sparkops/README.md`: three-node quorum requirement and rollout order.

---

### Task 1: Transaction Context and Durable Tables

**Files:**
- Create: `backend/internal/platform/postgres/transaction.go`
- Test: `backend/internal/platform/postgres/transaction_test.go`
- Create: `backend/migrations/000070_transactional_events.up.sql`
- Create: `backend/migrations/000070_transactional_events.down.sql`
- Modify: `backend/migrations/project_deploy_pipeline_test.go`
- Create: `backend/internal/platform/messaging/persistence/models.go`
- Create: `backend/internal/platform/messaging/persistence/migrate.go`
- Modify: `backend/internal/bootstrap/migrate.go`

**Interfaces:**

```go
type Transactor interface {
    WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

func NewTransactor(db *gorm.DB) Transactor
func NewDirectTransactor() Transactor
func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB
func RequireTx(ctx context.Context) (*gorm.DB, error)
```

- [ ] **Step 1: Write failing transaction tests**

```go
func TestTransactorRollsBackAllWrites(t *testing.T) {
    db := openTransactionSQLite(t)
    txr := postgres.NewTransactor(db)
    errSentinel := errors.New("stop")

    err := txr.WithinTransaction(context.Background(), func(ctx context.Context) error {
        require.NoError(t, postgres.DB(ctx, db).Create(&txRow{ID: "one"}).Error)
        return errSentinel
    })

    require.ErrorIs(t, err, errSentinel)
    var count int64
    require.NoError(t, db.Model(&txRow{}).Count(&count).Error)
    require.Zero(t, count)
}

func TestRequireTxRejectsContextWithoutTransaction(t *testing.T) {
    _, err := postgres.RequireTx(context.Background())
    require.ErrorContains(t, err, "transaction required")
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/postgres -run 'TestTransactor|TestRequireTx' -count=1`

Expected: FAIL because transaction helpers do not exist.

- [ ] **Step 3: Implement the context transaction coordinator**

```go
type txKey struct{}

type gormTransactor struct{ db *gorm.DB }

func (t *gormTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
    if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
        return fn(ctx)
    }
    return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return fn(context.WithValue(ctx, txKey{}, tx))
    })
}

func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
    if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
        return tx.WithContext(ctx)
    }
    return fallback.WithContext(ctx)
}

func RequireTx(ctx context.Context) (*gorm.DB, error) {
    tx, ok := ctx.Value(txKey{}).(*gorm.DB)
    if !ok {
        return nil, errors.New("postgres transaction required")
    }
    return tx.WithContext(ctx), nil
}
```

`NewDirectTransactor` calls `fn(ctx)` unchanged and exists only for pure unit tests with fake repositories.

- [ ] **Step 4: Add failing migration assertions**

Append tests that require all three `platform` tables, Outbox lease index, Inbox unique key, and Operation idempotency unique key:

```go
func TestTransactionalEventsMigrationDefinesReliabilityConstraints(t *testing.T) {
    up := readMigration(t, "000070_transactional_events.up.sql")
    require.Contains(t, up, "CREATE TABLE IF NOT EXISTS platform.event_outbox")
    require.Contains(t, up, "PRIMARY KEY (event_id, subscriber)")
    require.Contains(t, up, "CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_outbox_idempotency_key")
    require.Contains(t, up, "CREATE INDEX IF NOT EXISTS ix_event_outbox_claim")
}
```

- [ ] **Step 5: Create SQL migration and GORM fallback**

The up migration must use this shape:

```sql
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.event_outbox (
  id UUID PRIMARY KEY,
  event_type VARCHAR(160) NOT NULL,
  event_version INT NOT NULL,
  aggregate_type VARCHAR(120) NOT NULL DEFAULT '',
  aggregate_id VARCHAR(160) NOT NULL DEFAULT '',
  aggregate_version BIGINT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  correlation_id UUID NULL,
  causation_id UUID NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ NULL,
  locked_by VARCHAR(160) NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT event_outbox_status_check CHECK (status IN ('pending','publishing','published'))
);
CREATE INDEX IF NOT EXISTS ix_event_outbox_claim
  ON platform.event_outbox(status, next_attempt_at, created_at);

CREATE TABLE IF NOT EXISTS platform.event_inbox (
  event_id UUID NOT NULL,
  subscriber VARCHAR(200) NOT NULL,
  event_type VARCHAR(160) NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (event_id, subscriber)
);

CREATE TABLE IF NOT EXISTS platform.operation_outbox (
  id UUID PRIMARY KEY,
  event_id UUID NOT NULL,
  subscriber VARCHAR(200) NOT NULL,
  operation_type VARCHAR(160) NOT NULL,
  idempotency_key VARCHAR(240) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ NULL,
  locked_by VARCHAR(160) NOT NULL DEFAULT '',
  completed_at TIMESTAMPTZ NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT operation_outbox_status_check CHECK (status IN ('pending','executing','completed','failed'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_outbox_idempotency_key
  ON platform.operation_outbox(idempotency_key);
CREATE INDEX IF NOT EXISTS ix_operation_outbox_claim
  ON platform.operation_outbox(status, next_attempt_at, created_at);
```

The down migration drops operation, inbox, then outbox tables. GORM fallback creates the same columns and indexes only when `postgres.ShouldRunAutoMigrate(db)` is true.

- [ ] **Step 6: Run focused tests**

Run:

```powershell
cd backend
go test ./internal/platform/postgres ./migrations -count=1
go test ./internal/platform/messaging/persistence -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/platform/postgres backend/internal/platform/messaging/persistence backend/internal/bootstrap/migrate.go backend/migrations
git commit -m "feat(messaging): add transactional event storage"
```

---

### Task 2: Event Envelope, Registry, and Named In-Memory Bus

**Files:**
- Create: `backend/internal/shared/events/envelope.go`
- Create: `backend/internal/shared/events/registry.go`
- Test: `backend/internal/shared/events/registry_test.go`
- Modify: `backend/internal/platform/messaging/bus.go`
- Modify: `backend/internal/platform/messaging/inmemory.go`
- Modify: `backend/internal/platform/messaging/bus_test.go`
- Modify: `backend/internal/platform/eventbus/bus.go`

**Interfaces:**

```go
type Envelope struct {
    ID               string          `json:"id"`
    Type             string          `json:"type"`
    Version          int             `json:"version"`
    AggregateType    string          `json:"aggregate_type,omitempty"`
    AggregateID      string          `json:"aggregate_id,omitempty"`
    AggregateVersion *int64          `json:"aggregate_version,omitempty"`
    OccurredAt       time.Time       `json:"occurred_at"`
    CorrelationID    string          `json:"correlation_id,omitempty"`
    CausationID      string          `json:"causation_id,omitempty"`
    Payload          json.RawMessage `json:"payload"`
}

type AggregateRef struct {
    Type    string
    ID      string
    Version *int64
}

type Bus interface {
    Publish(ctx context.Context, ev ...events.Event) error
    Subscribe(eventName, subscriber string, h Handler) error
}

type Subscription struct {
    EventName  string
    Version    int
    Subscriber string
    Handler    Handler
}

type Lifecycle interface {
    Start(ctx context.Context) error
    Close() error
}
```

- [ ] **Step 1: Write failing registry and local bus tests**

```go
func TestRegistryRoundTripsConcreteValueEvent(t *testing.T) {
    registry := events.NewRegistry()
    require.NoError(t, events.RegisterJSON[event.BuildCompleted](
        registry, event.BuildCompletedName, 1,
        func(ev event.BuildCompleted) events.AggregateRef {
            return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
        },
    ))
    envelope, err := registry.Encode(context.Background(), event.NewBuildCompleted(/* fixed IDs */))
    require.NoError(t, err)
    decoded, err := registry.Decode(envelope)
    require.NoError(t, err)
    require.IsType(t, event.BuildCompleted{}, decoded)
}

func TestInMemoryBusRejectsDuplicateNamedSubscription(t *testing.T) {
    bus := messaging.NewInMemoryBus(events.NewRegistry())
    handler := func(context.Context, events.Event) error { return nil }
    require.NoError(t, bus.Subscribe("test.event", "test.subscriber", handler))
    require.ErrorContains(t, bus.Subscribe("test.event", "test.subscriber", handler), "duplicate")
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/shared/events ./internal/platform/messaging -run 'Registry|InMemoryBus' -count=1`

Expected: FAIL on missing envelope, registry and new subscription signature.

- [ ] **Step 3: Implement registry and metadata propagation**

`RegisterJSON[T]` stores one descriptor per `(name, version)`, rejects duplicates, JSON-marshals value events, and JSON-unmarshals back to value `T` so existing handler assertions remain valid.

```go
type Metadata struct {
    EventID      string
    EventType    string
    EventVersion int
    Subscriber   string
    CorrelationID string
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context
func MetadataFromContext(ctx context.Context) (Metadata, bool)
```

`Registry.Encode` generates a UUID once and reuses parent metadata for correlation/causation. `Registry.Decode` returns a typed permanent error for unknown type/version and malformed payload.

- [ ] **Step 4: Adapt InMemoryBus without adding infrastructure dependencies**

`InMemoryBus` keeps synchronous fan-out, injects one generated event metadata value into each handler context, and rejects duplicate named subscriptions. Its variadic registry argument preserves existing `NewInMemoryBus()` call sites while allowing runtime injection with `NewInMemoryBus(registry)`. It does not write Outbox or Inbox. Lifecycle belongs to the factory/runtime so test buses do not need meaningless `Start`/`Close` methods.

- [ ] **Step 5: Register all concrete event codecs**

Each event owner exposes a small registration function beside its event definitions:

```go
func Register(registry *events.Registry) error {
    return events.RegisterJSON[BuildCompleted](
        registry,
        BuildCompletedName,
        1,
        func(ev BuildCompleted) events.AggregateRef {
            return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
        },
    )
}
```

Add equivalent version-1 registrations for:

- `identity.MemberRegistered`
- `integration.WorkspaceCreated`
- `workspace_app.ProjectCreated`
- `integration.SecretUpdated`
- `agent.AgentPublished`
- `delivery.BuildCompleted`
- `delivery.DeployCompleted`
- `workflow.WorkflowRunCompleted`
- `product_workflow.WorkflowRunCompleted`
- `agent_runtime.RunCompleted`

Do not register unused `tenant.WorkspaceCreated`.

- [ ] **Step 6: Update every subscription call so the repository compiles**

Use the 15 approved stable production names in module registration and stable test-only names in tests:

```go
require.NoError(t, bus.Subscribe("test.event", "test.handler", handler))
```

Keep `NewInMemoryBus` variadic as `func NewInMemoryBus(registries ...*events.Registry) Bus`, so existing no-argument test construction remains valid. Update `backend/cmd/seed/main.go` with `governance.workspace-created.bootstrap.seed` so the seed process cannot collide with the server subscriber. Task 9 adds behavior tests for these names but does not postpone the signature migration.

- [ ] **Step 7: Verify and commit**

Run: `cd backend; go test ./internal/shared/events ./internal/platform/messaging ./internal/platform/eventbus -count=1`

Expected: PASS.

```powershell
git add backend/internal/shared/events backend/internal/platform/messaging backend/internal/platform/eventbus backend/internal/modules/*/domain/event backend/internal/shared/integration
git commit -m "feat(messaging): define versioned named event contracts"
```

---

### Task 3: Outbox, Inbox, and Operation Persistence

**Files:**
- Create: `backend/internal/platform/messaging/persistence/outbox.go`
- Create: `backend/internal/platform/messaging/persistence/outbox_test.go`
- Create: `backend/internal/platform/messaging/persistence/inbox.go`
- Create: `backend/internal/platform/messaging/persistence/inbox_test.go`
- Create: `backend/internal/platform/messaging/persistence/operation_outbox.go`
- Create: `backend/internal/platform/messaging/persistence/operation_outbox_test.go`

**Interfaces:**

```go
type OutboxStore interface {
    Insert(ctx context.Context, envelope events.Envelope) error
    Claim(ctx context.Context, owner string, limit int, now time.Time, lease time.Duration) ([]OutboxRecord, error)
    MarkPublished(ctx context.Context, id string, at time.Time) error
    Reschedule(ctx context.Context, id, reason string, next time.Time) error
}

type InboxStore interface {
    TryInsert(ctx context.Context, eventID, subscriber, eventType string, now time.Time) (bool, error)
}

type OperationStore interface {
    Insert(ctx context.Context, operation OperationRecord) error
    Claim(ctx context.Context, owner string, limit int, now time.Time, lease time.Duration) ([]OperationRecord, error)
    MarkCompleted(ctx context.Context, id string, at time.Time) error
    MarkFailed(ctx context.Context, id, reason string, at time.Time) error
    Reschedule(ctx context.Context, id, reason string, next time.Time) error
}
```

- [ ] **Step 1: Write failing persistence tests**

Use PostgreSQL integration tests for `FOR UPDATE SKIP LOCKED`; keep pure scheduling decisions in table-driven unit tests. Required assertions:

```go
inserted, err := inbox.TryInsert(txCtx, eventID, "notification.build-completed", eventType, now)
require.NoError(t, err)
require.True(t, inserted)
inserted, err = inbox.TryInsert(txCtx, eventID, "notification.build-completed", eventType, now)
require.NoError(t, err)
require.False(t, inserted)
```

Two concurrent `Claim` calls must return disjoint IDs. An expired `publishing`/`executing` lease must be claimable; an active lease must not.

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/messaging/persistence -run 'Outbox|Inbox|Operation' -count=1`

Expected: FAIL because stores do not exist.

- [ ] **Step 3: Implement atomic SQL**

Inbox uses:

```sql
INSERT INTO platform.event_inbox(event_id, subscriber, event_type, processed_at, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (event_id, subscriber) DO NOTHING
```

Claim uses one transaction with `FOR UPDATE SKIP LOCKED`, then updates claimed rows to `publishing`/`executing`, sets lease metadata, and increments attempts. Inserts call `postgres.RequireTx(ctx)` so AMQP publishing and operation scheduling cannot escape the surrounding business transaction.

- [ ] **Step 4: Verify rollback semantics**

Add a test that inserts Inbox and a sample business row, returns an error, and verifies both are absent. Add a test that inserts an Outbox record inside a rolled-back business transaction and verifies it is absent.

- [ ] **Step 5: Run tests and commit**

Run:

```powershell
cd backend
go test ./internal/platform/messaging/persistence -count=1
go test ./internal/platform/postgres -count=1
```

Expected: PASS.

```powershell
git add backend/internal/platform/messaging/persistence
git commit -m "feat(messaging): persist outbox inbox and operations"
```

---

### Task 4: RabbitMQ Event Topology and Publisher Confirms

**Files:**
- Create: `backend/internal/platform/messaging/amqp_event_topology.go`
- Test: `backend/internal/platform/messaging/amqp_event_topology_test.go`
- Create: `backend/internal/platform/messaging/amqp_event_transport.go`
- Test: `backend/internal/platform/messaging/amqp_event_transport_test.go`

**Interfaces:**

```go
type RetryDelay time.Duration
type DeliveryHandler func(ctx context.Context, envelope events.Envelope, attempts int) error

type EventTransport interface {
    Declare(ctx context.Context, subscriptions []Subscription) error
    Publish(ctx context.Context, envelope events.Envelope) error
    Consume(ctx context.Context, subscription Subscription, handler DeliveryHandler) error
    PublishRetry(ctx context.Context, subscription Subscription, envelope events.Envelope, delay RetryDelay, attempts int) error
    PublishDeadLetter(ctx context.Context, subscription Subscription, envelope events.Envelope, reason string) error
    Close() error
}
```

- [ ] **Step 1: Write failing topology tests**

```go
func TestEventTopologyDeclaresQuorumMainRetryAndDLQ(t *testing.T) {
    declarations := topologyFor(Subscription{
        EventName: "delivery.BuildCompleted",
        Version: 1,
        Subscriber: "notification.build-completed",
    })
    require.Equal(t, "sparkops.event.notification.build-completed.delivery-buildcompleted.v1", declarations.Main.Name)
    require.Equal(t, "quorum", declarations.Main.Args["x-queue-type"])
    require.Equal(t, int32(5000), declarations.Retry[0].Args["x-message-ttl"])
    require.Equal(t, "", declarations.Retry[0].Args["x-dead-letter-exchange"])
    require.Equal(t, declarations.Main.Name, declarations.Retry[0].Args["x-dead-letter-routing-key"])
}
```

The retry queue must dead-letter through the default exchange directly to its own main queue. It must not route back through `sparkops.events`, which would duplicate retries into every subscriber queue.

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1`

Expected: FAIL because event transport does not exist.

- [ ] **Step 3: Implement topology**

For each subscription declare:

- one durable quorum main queue bound to `sparkops.events` with `<event-name>.v<version>`;
- four durable quorum retry queues with TTL 5s, 30s, 5m, 30m;
- one durable quorum DLQ;
- retry queues dead-letter to the main queue name using the default exchange.

Normalize queue names to lowercase `[a-z0-9.-]` and reject names longer than 240 characters.

- [ ] **Step 4: Implement confirmed publishing**

Use a dedicated confirm-enabled channel. Publish persistent JSON messages with:

```go
amqp.Publishing{
    MessageId: envelope.ID,
    ContentType: "application/json",
    DeliveryMode: amqp.Persistent,
    Timestamp: envelope.OccurredAt,
    Body: body,
}
```

Wait for the matching deferred confirmation. Treat NACK, returned mandatory publish, closed channel, and context timeout as failure. Serialize channel recreation behind a mutex; never silently downgrade to unconfirmed publish.

- [ ] **Step 5: Add real RabbitMQ integration test**

Under `//go:build integration`, declare two subscribers for one event, publish once, and assert both main queues contain one message. Declare the same topology twice to prove startup idempotency.

- [ ] **Step 6: Verify and commit**

Run:

```powershell
cd backend
go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
$env:AMQP_URL="amqp://sparkops:sparkops@localhost:5672/"
go test -tags=integration ./internal/platform/messaging -run Rabbit -count=1 -timeout 120s
```

Expected: PASS.

```powershell
git add backend/internal/platform/messaging/amqp_event_*
git commit -m "feat(messaging): add quorum event transport"
```

---

### Task 5: Reliable Bus, Dispatcher, Inbox Consumer, and Error Classification

**Files:**
- Create: `backend/internal/platform/messaging/errors.go`
- Create: `backend/internal/platform/messaging/outbox_dispatcher.go`
- Test: `backend/internal/platform/messaging/outbox_dispatcher_test.go`
- Create: `backend/internal/platform/messaging/inbox_consumer.go`
- Test: `backend/internal/platform/messaging/inbox_consumer_test.go`
- Create: `backend/internal/platform/messaging/reliable_bus.go`
- Test: `backend/internal/platform/messaging/reliable_bus_test.go`
- Create: `backend/internal/platform/messaging/metrics.go`
- Test: `backend/internal/platform/messaging/metrics_test.go`

**Interfaces:**

Consumes `Subscription` from Task 2 and `EventTransport` from Task 4.

```go
func Permanent(err error) error
func IsPermanent(err error) bool
```

- [ ] **Step 1: Write failing Outbox crash-window tests**

Required tests:

```go
func TestDispatcherLeavesRecordRetryableWhenPublishConfirmFails(t *testing.T)
func TestDispatcherMayRepublishAfterConfirmBeforeMarkPublished(t *testing.T)
func TestDispatcherClaimsDisjointBatchesAcrossInstances(t *testing.T)
```

Use fake `OutboxStore` and `EventTransport`; assert confirm failure calls `Reschedule`, and confirm success calls `MarkPublished`.

- [ ] **Step 2: Write failing Inbox tests**

```go
func TestInboxConsumerCommitsHandlerAndInboxTogether(t *testing.T)
func TestInboxConsumerSkipsHandlerForDuplicateDelivery(t *testing.T)
func TestInboxConsumerRollsBackInboxWhenHandlerFails(t *testing.T)
func TestInboxConsumerRoutesPermanentDecodeErrorToDLQ(t *testing.T)
func TestInboxConsumerRoutesTemporaryErrorThroughBackoffTiers(t *testing.T)
```

- [ ] **Step 3: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/messaging -run 'Dispatcher|InboxConsumer|ReliableBus' -count=1`

Expected: FAIL on missing components.

- [ ] **Step 4: Implement reliable publishing**

`ReliableBus.Publish`:

1. requires a transaction with `postgres.RequireTx`;
2. encodes each concrete event through Registry;
3. inserts each envelope through OutboxStore;
4. never performs network I/O.

`Subscribe` validates stable names and stores subscriptions until `Start`. `Start` declares all topology before starting any consumer, then starts consumers, then Dispatcher. Duplicate registration is a startup error.

- [ ] **Step 5: Implement transactional consumption**

For each delivery:

```go
err := transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
    inserted, err := inbox.TryInsert(txCtx, env.ID, sub.Subscriber, env.Type, clock.Now())
    if err != nil || !inserted {
        return err
    }
    event, err := registry.Decode(env)
    if err != nil {
        return Permanent(err)
    }
    handlerCtx := events.WithMetadata(txCtx, metadataFor(env, sub.Subscriber))
    return sub.Handler(handlerCtx, event)
})
```

ACK only after transaction commit. On temporary error publish to the selected subscriber-specific retry queue and ACK the original only after retry publish confirm. On permanent error publish to subscriber DLQ and ACK only after DLQ publish confirm.

- [ ] **Step 6: Implement graceful lifecycle**

`Close` stops new Outbox claims, waits for in-flight confirms, cancels consumers, waits for active handler transactions, then closes AMQP resources. Repeated `Close` is safe.

- [ ] **Step 7: Add Prometheus metrics**

Register collectors once for:

```text
sparkops_event_outbox_pending
sparkops_event_outbox_oldest_seconds
sparkops_event_publish_total{result}
sparkops_event_consume_total{subscriber,result}
sparkops_event_retry_total{subscriber,delay}
sparkops_event_dlq_total{subscriber}
sparkops_event_inbox_duplicate_total{subscriber}
sparkops_operation_outbox_pending
sparkops_operation_outbox_oldest_seconds
```

Tests use a private `prometheus.Registry` to avoid duplicate global registration.

- [ ] **Step 8: Verify and commit**

Run: `cd backend; go test ./internal/platform/messaging/... -count=1`

Expected: PASS.

```powershell
git add backend/internal/platform/messaging
git commit -m "feat(messaging): deliver transactional events reliably"
```

---

### Task 6: Operation Outbox Runtime

**Files:**
- Create: `backend/internal/platform/messaging/operation.go`
- Create: `backend/internal/platform/messaging/operation_dispatcher.go`
- Test: `backend/internal/platform/messaging/operation_test.go`

**Interfaces:**

```go
type OperationRequest struct {
    Owner          string
    OperationType  string
    IdempotencyKey string
    Payload        any
}

type OperationWriter interface {
    Schedule(ctx context.Context, request OperationRequest) error
}

type OperationHandler func(ctx context.Context, operation Operation) error

type OperationRegistry interface {
    Register(operationType string, handler OperationHandler) error
    Handler(operationType string) (OperationHandler, bool)
}
```

- [ ] **Step 1: Write failing idempotency and recovery tests**

```go
func TestOperationWriterRequiresTransaction(t *testing.T)
func TestOperationWriterTreatsDuplicateIdempotencyKeyAsSuccess(t *testing.T)
func TestOperationDispatcherRetriesTemporaryFailure(t *testing.T)
func TestOperationDispatcherDoesNotRepeatCompletedOperation(t *testing.T)
func TestOperationDispatcherMarksUnknownOperationPermanent(t *testing.T)
```

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/messaging -run Operation -count=1`

Expected: FAIL because operation runtime does not exist.

- [ ] **Step 3: Implement writer and dispatcher**

`Schedule` serializes the payload and inserts through OperationStore in the current transaction. For event-driven calls it derives `event_id` and subscriber from event metadata and verifies that `Owner` matches the subscriber. For command-driven operations without event metadata, `Owner` is mandatory and the generated operation UUID is also used as the trace `event_id`; the stable business target remains part of `IdempotencyKey`.

Use idempotency key format:

```text
<event-id>:<subscriber>:<operation-type>:<target-id>
```

Dispatcher uses the same lease pattern as Event Outbox. Temporary errors follow 5s/30s/5m/30m/30m retry. Unknown operation type and malformed payload call `MarkFailed`, remain queryable with `status=failed` and `last_error`, and are not marked completed.

- [ ] **Step 4: Verify and commit**

Run: `cd backend; go test ./internal/platform/messaging/... -run Operation -count=1`

Expected: PASS.

```powershell
git add backend/internal/platform/messaging/operation*
git commit -m "feat(messaging): add durable external operation dispatch"
```

---

### Task 7: Configuration, Factory, Bootstrap, and In-Memory Compatibility

**Files:**
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/internal/platform/config/validate.go`
- Modify: `backend/internal/platform/config/validate_test.go`
- Modify: `backend/internal/platform/messaging/factory.go`
- Modify: `backend/internal/platform/appctx/container.go`
- Modify: `backend/internal/bootstrap/app.go`
- Modify: `backend/config.example.yaml`
- Modify: `backend/config.remote.example.yaml`
- Modify: `backend/deployments/docker/config.docker.yaml`

**Configuration types:**

```go
type MessagingConfig struct {
    Driver string `yaml:"driver"`
    AMQP AMQPConfig `yaml:"amqp"`
    Outbox OutboxConfig `yaml:"outbox"`
}

type AMQPConfig struct {
    URL string `yaml:"url"`
    Exchange string `yaml:"exchange"`
    ExchangeType string `yaml:"exchange_type"`
    EventExchange string `yaml:"event_exchange"`
}

type OutboxConfig struct {
    PollInterval time.Duration `yaml:"poll_interval"`
    BatchSize int `yaml:"batch_size"`
    LockTimeout time.Duration `yaml:"lock_timeout"`
}
```

- [ ] **Step 1: Write failing config and factory tests**

Assert defaults:

```go
require.Equal(t, "sparkops.events", cfg.Messaging.AMQP.EventExchange)
require.Equal(t, time.Second, cfg.Messaging.Outbox.PollInterval)
require.Equal(t, 100, cfg.Messaging.Outbox.BatchSize)
require.Equal(t, time.Minute, cfg.Messaging.Outbox.LockTimeout)
```

Assert production rejects `inmemory`, missing AMQP URL, and missing database. Assert `AMQP_EVENT_EXCHANGE`, `OUTBOX_POLL_INTERVAL`, `OUTBOX_BATCH_SIZE`, and `OUTBOX_LOCK_TIMEOUT` override YAML values. Assert `inmemory` factory creates a synchronous bus and does not open RabbitMQ.

- [ ] **Step 2: Run and confirm RED**

Run: `cd backend; go test ./internal/platform/config ./internal/platform/messaging -run 'Messaging|Factory' -count=1`

Expected: FAIL on missing config and factory dependencies.

- [ ] **Step 3: Implement the factory split**

```go
type Factory struct {
    Bus Bus
    Queue Queue
    Operations *OperationRuntime
}

func (f *Factory) Start(ctx context.Context) error
func (f *Factory) Close() error

func NewFromConfig(
    cfg *config.Config,
    db *gorm.DB,
    registry *events.Registry,
    transactor postgres.Transactor,
    logger *zap.Logger,
) (*Factory, error)
```

- `inmemory`: `NewInMemoryBus(registry)` + `NewNoopQueue()`.
- `amqp`: existing task `NewAMQPQueue` + `NewReliableBus` with Outbox/Inbox/EventTransport.
- Operation Dispatcher is created when a database exists and starts independently of EventBus driver.

- [ ] **Step 4: Wire bootstrap lifecycle**

Bootstrap order must be:

1. migrate database;
2. create Registry, Transactor, Factory and Container;
3. register all modules, codecs, subscriptions and operation handlers;
4. call `msgFactory.Start(ctx)`, which starts reliable EventBus and Operation Dispatcher runtimes;
5. start HTTP server.

Shutdown uses `errors.Join(server.Shutdown(...), msgFactory.Close())`; factory close drains the reliable EventBus runtime, Operation Dispatcher and existing task Queue exactly once.

- [ ] **Step 5: Verify current local config**

Keep `backend/config.yaml` unchanged with `driver: inmemory`. Run:

```powershell
cd backend
go test ./internal/platform/config ./internal/platform/messaging ./internal/bootstrap -count=1
go build ./cmd/server
```

Expected: PASS without RabbitMQ.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/platform/config backend/internal/platform/messaging/factory.go backend/internal/platform/appctx backend/internal/bootstrap backend/config.example.yaml backend/config.remote.example.yaml backend/deployments/docker/config.docker.yaml
git commit -m "feat(bootstrap): wire reliable event lifecycle"
```

---

### Task 8: Migrate Event Publishers into Business Transactions

**Files:**
- Modify the 10 production publisher files listed in File Map.
- Modify their module constructors to inject `postgres.Transactor`.
- Modify participating persistence repositories to use `postgres.DB(ctx, r.db)`.
- Add or extend command tests beside each publisher.

**Interfaces:**
- Consumes: `postgres.Transactor`, new `eventbus.Bus`.
- Produces: no new public API; every AMQP publish occurs inside `WithinTransaction`.

- [ ] **Step 1: Add failing atomicity tests for each publisher group**

Use this contract for every publisher:

```go
err := handler.Handle(ctx, command)
require.Error(t, err)
requireBusinessWriteAbsent(t, db)
requireOutboxAbsent(t, db)
```

Then run the success case and assert one business state change plus one Outbox row with the expected event type.

Cover:

- member save + `identity.MemberRegistered`;
- workspace/member/org membership + `integration.WorkspaceCreated`;
- project/member + `workspace_app.ProjectCreated`;
- secret update + `integration.SecretUpdated`;
- agent/version + `agent.AgentPublished`;
- build terminal update + `delivery.BuildCompleted`;
- deployment terminal update + `delivery.DeployCompleted`;
- workflow terminal update + `workflow.WorkflowRunCompleted`;
- product workflow terminal update + `product_workflow.WorkflowRunCompleted`;
- run/message save + `agent_runtime.RunCompleted`.

- [ ] **Step 2: Run and confirm RED**

Run:

```powershell
cd backend
go test ./internal/modules/identity/... ./internal/modules/tenant/... ./internal/modules/workspace_app/... ./internal/modules/agent/... -count=1
go test ./internal/modules/workflow/... ./internal/modules/agent_runtime/... ./internal/modules/product_workflow/... -count=1
go test ./internal/modules/delivery/... -run 'BuildCompleted|DeployCompleted|Outbox' -count=1
```

Expected: new atomicity tests fail because publication is outside repository transactions or errors are ignored.

- [ ] **Step 3: Apply one transaction pattern to synchronous commands**

Example:

```go
var result *model.Workspace
err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
    ws, err := h.createWorkspace(txCtx, orgID, ownerID, name, slug)
    if err != nil {
        return err
    }
    if err := h.bus.Publish(txCtx, integration.NewWorkspaceCreated(ws.ID, orgID, ownerID)); err != nil {
        return err
    }
    result = ws
    return nil
})
return result, err
```

Never retain `_ = h.bus.Publish`. Also stop ignoring `orgRepo.AddMember`.

- [ ] **Step 4: Move asynchronous terminal publication into repository transaction callbacks**

- Build: terminal `BuildRepository.Update` and `BuildCompleted` Outbox insert share one `WithinTransaction`.
- Deployment: replace `SetTerminalHook` post-commit publication with a terminal persistence callback that receives the same transaction context as `DeploymentRunRepository.Update`.
- Workflow and product workflow: terminal `UpdateRun` and completion Outbox insert share one transaction.
- Agent runtime: completed run, assistant message, and completion Outbox share one transaction.

The old three-attempt in-memory `publishBuildCompleted` retry is removed for AMQP because the durable Dispatcher owns retries. The in-memory path still returns synchronous handler errors to the command.

- [ ] **Step 5: Make repositories transaction-aware**

For every repository touched by these commands, replace:

```go
r.db.WithContext(ctx)
```

with:

```go
postgres.DB(ctx, r.db)
```

Do not refactor unrelated repository methods.

- [ ] **Step 6: Register event codecs in owner modules**

Each owner module calls its event package `Register(c.EventRegistry)` before handlers/routes are exposed. Fail module registration on duplicate or invalid registration.

- [ ] **Step 7: Verify and commit by bounded publisher groups**

Create three commits after their respective tests pass:

```powershell
git commit -m "feat(events): publish identity and workspace events transactionally"
git commit -m "feat(events): publish workflow and agent events transactionally"
git commit -m "feat(events): publish delivery events transactionally"
```

After all three:

Run: `cd backend; go test ./internal/modules/... -count=1 -timeout 300s`

Expected: PASS.

---

### Task 9: Migrate Named Subscribers and Local Database Effects

**Files:**
- Modify module registration files:
  - `backend/internal/modules/product/module.go`
  - `backend/internal/modules/product_workflow/module.go`
  - `backend/internal/modules/delivery/module.go`
  - `backend/internal/modules/workspace_app/module.go`
  - `backend/internal/modules/governance/module.go`
  - `backend/internal/modules/notification/module.go`
- Create: `backend/internal/modules/workspace_app/application/subscriber/on_workspace_created.go`
- Add/modify subscriber tests and transaction-aware repositories.

**Stable subscriptions:**

```text
product.build-completed
product.workflow-run-completed
product.deploy-completed
product-workflow.build-completed
product-workflow.deploy-completed
workspace-app.workspace-created.seed-default-templates
workspace-app.deploy-completed.archive-coordinator
delivery.build-completed.deployrun
delivery.build-completed.webhook
delivery.deploy-completed.webhook
delivery.secret-updated.credential-sync
notification.run-completed
notification.workflow-run-completed
notification.build-completed
governance.workspace-created.bootstrap
```

- [ ] **Step 1: Write failing registration tests**

Create a recording bus and assert every module registers the exact stable tuple. Add a global startup test that rejects duplicate `(eventName, subscriber)` pairs.

- [ ] **Step 2: Write failing duplicate delivery tests for pure database consumers**

Cover:

- product build/deploy updates;
- governance workspace bootstrap;
- workspace template seeding;
- three notification creators.

Publish the same envelope twice and assert one final state. Notifications must contain one row, not two; no extra notification schema column is required because Inbox and notification insert commit in one transaction.

- [ ] **Step 3: Run and confirm RED**

Run:

```powershell
cd backend
go test ./internal/modules/product/... ./internal/modules/governance/... ./internal/modules/workspace_app/... ./internal/modules/notification/... -run 'Subscriber|Duplicate|WorkspaceCreated' -count=1
```

Expected: FAIL because duplicate deliveries still repeat local effects or repositories ignore the consumer transaction context.

- [ ] **Step 4: Verify named subscriptions and extract the inline handler**

Use:

```go
if err := c.EventBus.Subscribe(
    deliveryevent.BuildCompletedName,
    "notification.build-completed",
    subscriber.NewOnBuildCompleted(create).Handle,
); err != nil {
    return fmt.Errorf("subscribe notification build completed: %w", err)
}
```

Task 2 already applies all 15 stable names to keep intermediate commits compiling. This step locks them with registration tests and extracts the anonymous workspace template handler into a named subscriber type.

- [ ] **Step 5: Make consumer repositories transaction-aware**

Update only repositories used by the 15 handlers, including notification, product requirement/environment/release-plan/sprint, product-workflow run, governance role/menu/binding, workspace project/template, and delivery deployment-run repositories. Every write and consistency read in a handler must use `postgres.DB(ctx, db)`.

- [ ] **Step 6: Remove assumptions about cross-subscriber ordering**

Handlers must not assume another subscriber has run. Product and archive handlers query current source state and use optimistic/state guards. Product-workflow handlers use database optimistic versions rather than process-local lock as the correctness boundary.

- [ ] **Step 7: Verify and commit**

Run:

```powershell
cd backend
go test ./internal/modules/product/... ./internal/modules/product_workflow/... ./internal/modules/governance/... ./internal/modules/workspace_app/... ./internal/modules/notification/... ./internal/modules/delivery/... -count=1 -timeout 300s
```

Expected: PASS.

```powershell
git add backend/internal/modules
git commit -m "feat(events): consume named events transactionally"
```

---

### Task 10: Move External and Queue Side Effects to Operation Outbox

**Files:**
- Create the three module outbound `operation_handlers.go` files listed in File Map.
- Modify:
  - `backend/internal/modules/delivery/infrastructure/outbound/dispatcher.go`
  - `backend/internal/modules/delivery/deployrun/application/subscriber/on_secret_updated.go`
  - `backend/internal/modules/delivery/deployrun/application/subscriber/on_build_completed.go`
  - `backend/internal/modules/workspace_app/application/command/archive_coordinator.go`
  - `backend/internal/modules/product_workflow/domain/engine/scheduler.go`
  - product release-plan/Jira/hook executor files reached by `on_workflow_run_completed.go`
  - deploy create/rollback/undeploy commands that currently write DB then `Queue.Publish`.

**Operation types:**

```text
delivery.webhook.send.v1
delivery.jenkins.credentials.sync.v1
delivery.queue.deploy.publish.v1
workspace-app.project.undeploy.v1
workspace-app.pipeline.disable.v1
product-workflow.build.trigger.v1
product-workflow.deploy.trigger.v1
product-workflow.webhook.send.v1
product.workflow.trigger.v1
product.jira.transition.v1
product.release-plan.webhook.send.v1
```

- [ ] **Step 1: Write failing “no direct external call” tests**

For each subscriber, inject a fake `OperationWriter` and fail the external adapter if invoked. Assert the handler schedules the expected operation once with an idempotency key containing event ID, subscriber and target.

```go
require.Equal(t,
    eventID+":delivery.build-completed.webhook:delivery.webhook.send.v1:"+hookID,
    scheduled.IdempotencyKey,
)
```

- [ ] **Step 2: Run and confirm RED**

Run:

```powershell
cd backend
go test ./internal/modules/delivery/... -run 'Operation|Webhook|SecretUpdated|BuildCompleted' -count=1
go test ./internal/modules/workspace_app/... -run 'Archive|Operation' -count=1
go test ./internal/modules/product_workflow/... ./internal/modules/product/... -run 'Operation|WorkflowRunCompleted' -count=1
```

Expected: FAIL because handlers still launch goroutines or call external adapters/Queue directly.

- [ ] **Step 3: Replace fire-and-forget goroutines**

- Webhook subscriber reads enabled hooks and schedules one operation per hook.
- SecretUpdated subscriber schedules Jenkins credential sync and returns the database result; remove `go func`.
- Release-plan and product-workflow webhook paths schedule operations; remove `go func`.
- Operation payloads include `event_id`, target ID, immutable request data and timeout.

- [ ] **Step 4: Make deploy Queue publication transactional**

The BuildCompleted subscriber and create/rollback/undeploy commands write `delivery.queue.deploy.publish.v1` in the same transaction as deployment-run state. The operation handler invokes existing `messaging.Queue.Publish(TopicDeploy, payload)`.

For direct commands use owner names `delivery.command.create-deployment`, `delivery.command.rollback-deployment`, and `delivery.command.undeploy`; these calls do not require event metadata. Use deployment run ID as target component of the idempotency key. Existing deploy consumer state guards remain the second line of defense because RabbitMQ task Queue is also at-least-once.

- [ ] **Step 5: Register operation handlers during module registration**

Each module registers owned operation types exactly once. Unknown types remain failed and observable; startup rejects duplicate operation type registration.

- [ ] **Step 6: Add reconciliation tests**

For external APIs without native idempotency:

- Jenkins executor queries job/credential state before mutating.
- Kubernetes/deploy executor queries deployment-run state before publishing or applying.
- Webhook remains at-least-once and always sends `X-SparkOps-Event-ID` and `X-SparkOps-Idempotency-Key`.

- [ ] **Step 7: Verify and commit**

Run: `cd backend; go test ./internal/modules/delivery/... ./internal/modules/workspace_app/... ./internal/modules/product_workflow/... ./internal/modules/product/... -count=1 -timeout 300s`

Expected: PASS with no event subscriber launching untracked goroutines.

```powershell
git add backend/internal/modules
git commit -m "feat(events): persist external side effects"
```

---

### Task 11: Fault-Window Integration Tests, CI, and Production Rollout

**Files:**
- Create: `backend/internal/platform/messaging/event_pipeline_integration_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/deployment_pipeline_integration_test.go`
- Modify: `.github/workflows/ci.yaml`
- Modify: `backend/deployments/docker/docker-compose.yaml`
- Modify: `deployments/helm/sparkops/values.yaml`
- Modify: `deployments/helm/sparkops/values-production.yaml`
- Modify: `deployments/helm/sparkops/templates/api-deployment.yaml`
- Modify: `deployments/helm/sparkops/README.md`
- Create: `backend/cmd/event-dlq/main.go`
- Test: `backend/cmd/event-dlq/main_test.go`

- [ ] **Step 1: Write integration tests for every crash window**

Under `//go:build integration`, cover:

1. business commit while RabbitMQ is stopped, then broker recovery and eventual delivery;
2. confirm succeeds but Outbox mark is skipped, causing duplicate publication and one Inbox effect;
3. consumer transaction commits but ACK is withheld, causing redelivery and one effect;
4. two subscribers each receive one event;
5. two backend consumer instances for one subscriber produce one effect;
6. temporary failure traverses retry tiers;
7. malformed payload reaches the correct subscriber DLQ;
8. DLQ replay with original event ID is safe;
9. expired Outbox and Operation leases are reclaimed;
10. shutdown stops claims and drains in-flight work.

- [ ] **Step 2: Add delivery pipeline acceptance coverage**

Extend the existing integration test to prove:

```text
BuildCompleted Outbox
  → delivery.build-completed.deployrun Inbox
  → delivery.queue.deploy.publish.v1 Operation Outbox
  → TopicDeploy
  → deployment reaches one terminal state
```

Repeat the source event and assert no second deployment transition or notification.

- [ ] **Step 3: Add CI services and commands**

Update backend CI to Go 1.26. Add a separate integration job with PostgreSQL and RabbitMQ services and:

```yaml
- name: Run messaging integration tests
  env:
    DATABASE_DSN: postgres://sparkops:sparkops@localhost:5432/sparkops?sslmode=disable
    AMQP_URL: amqp://sparkops:sparkops@localhost:5672/
  run: go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... -count=1 -timeout 300s
```

The CI RabbitMQ service is a protocol test environment, not a production quorum availability test.

- [ ] **Step 4: Add controlled DLQ inspection and replay**

The command accepts only explicit queue names and operations:

```powershell
go run ./cmd/event-dlq inspect --queue sparkops.event.<subscriber>.<event>.v1.dlq
go run ./cmd/event-dlq replay --queue sparkops.event.<subscriber>.<event>.v1.dlq --limit 100
```

`inspect` uses passive queue inspection. `replay` builds a Registry by explicitly registering the 10 production event codecs, validates the envelope, republishes it with the original `event_id`, waits for Publisher Confirm, and only then ACKs the DLQ copy. It refuses non-DLQ queue names and requires `--limit`.

- [ ] **Step 5: Run complete verification**

```powershell
cd backend
go test ./... -count=1 -timeout 600s
go build ./cmd/server
go run ./scripts/check-deps.go
$env:DATABASE_DSN="postgres://sparkops:sparkops@localhost:5432/sparkops?sslmode=disable"
$env:AMQP_URL="amqp://sparkops:sparkops@localhost:5672/"
go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... -count=1 -timeout 300s
```

Expected: all commands exit 0.

- [ ] **Step 6: Verify local inmemory startup**

With the unchanged local `backend/config.yaml`:

```powershell
cd A:\21-Agent\SparkOpsAgent
.\scripts\dev-restart.ps1 -Backend
```

Expected: backend starts without RabbitMQ, `http://localhost:8080` is reachable, and logs identify the EventBus as `inmemory`.

- [ ] **Step 7: Verify production AMQP startup**

Run migration 000070 first, then start with:

```text
MESSAGING_DRIVER=amqp
AMQP_URL=amqp://...
```

Expected startup order in logs:

1. database migration current;
2. event topology declared;
3. 15 named consumers started;
4. Outbox and Operation dispatchers started;
5. HTTP server started.

Verify metrics/log fields for Outbox oldest age, Inbox duplicates, retries, DLQ, Operation backlog and `event_id/event_type/subscriber/attempt`.

- [ ] **Step 8: Commit rollout support**

```powershell
git add .github/workflows/ci.yaml backend/cmd/event-dlq backend/deployments backend/internal/platform/messaging backend/internal/modules/delivery/deployrun/application/deployment_pipeline_integration_test.go deployments/helm/sparkops
git commit -m "test(events): verify reliable delivery failure windows"
```

---

## Final Review Gate

Before merging:

- [ ] Search production code for ignored event publication: `rg "_ = .*\\.Publish\\(" backend/internal`.
- [ ] Search event subscribers for untracked goroutines: `rg "go func" backend/internal/modules -g "*.go"` and inspect every match reached from an event subscriber.
- [ ] Confirm all production `Subscribe` calls pass stable subscriber names.
- [ ] Confirm all AMQP event queues declare `x-queue-type=quorum`.
- [ ] Confirm retry dead-lettering returns only to the originating subscriber queue.
- [ ] Confirm every AMQP `Bus.Publish` test runs inside a transaction.
- [ ] Confirm all consumer repositories use the transaction from context.
- [ ] Confirm `backend/config.yaml` still works with `driver: inmemory`.
- [ ] Confirm production validation rejects `inmemory`.
- [ ] Confirm SQL migration and GORM fallback schemas agree.
- [ ] Run `go test ./...`, `go build ./cmd/server`, dependency checks, and integration tests with fresh PostgreSQL/RabbitMQ.
