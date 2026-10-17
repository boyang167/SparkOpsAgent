# Task 7：Configuration、Factory、Bootstrap 与 In-Memory 兼容

日期：2026-10-17
起始 HEAD：`35af97a`
初始实现：`703531da982051463551642acd3df1e649ce685f`

## 范围

本任务只实现 RabbitMQ 事务事件 Task 7 的配置与运行时装配：

- Event Exchange 与 Outbox 配置模型、默认值、YAML/env 覆盖及校验；
- `inmemory` 使用同步 EventBus + NoopQueue，不连接 RabbitMQ、不创建 OperationRuntime；
- `amqp` 构造原任务 Queue、双连接 EventTransport、Outbox/Inbox/Operation stores、Metrics、ReliableBus、Event/Operation dispatchers；
- PostgreSQL AMQP 启动前强制校验官方 SQL migration 000070；
- 模块完成 codecs、subscriptions 和 operation handlers 注册后才启动消息 worker；
- 启动失败逆序回滚；关闭先停止 claim/intake，再 drain 在途工作，最后关闭 AMQP/DB；
- readiness 仍只检查 PostgreSQL/Redis，运行期间 RabbitMQ outage 不直接导致 HTTP not-ready；
- 未迁移 Task 8–10 的具体业务 publisher、subscriber 或 operation handler。

## 配置矩阵

| 模式 | 数据库 | RabbitMQ | EventBus | Task Queue | Operation runtime |
| --- | --- | --- | --- | --- | --- |
| `inmemory` health-only | 无 | 不需要 | 同步内存 | NoopQueue | 不创建 |
| `inmemory` 本地开发 | 有 | 不需要 | 同步内存 | NoopQueue | 不创建 |
| `amqp` SQLite 单测 | 测试 DB | fake/可注入 | ReliableBus | AMQP factory | 创建 |
| `amqp` PostgreSQL | 必需且 migration >= 70 | 必需 | ReliableBus | 原 AMQP Queue | 创建 |
| production + `inmemory` | 任意 | 任意 | 配置校验拒绝 | 配置校验拒绝 | 不创建 |

默认值：

```text
AMQP event exchange: sparkops.events
Outbox poll interval: 1s
Outbox batch size: 100
Outbox lock timeout: 1m
```

环境覆盖：

```text
AMQP_EVENT_EXCHANGE
OUTBOX_POLL_INTERVAL
OUTBOX_BATCH_SIZE
OUTBOX_LOCK_TIMEOUT
```

## 初始 TDD：RED

配置与工厂首次 RED：

```text
go test ./internal/platform/config ./internal/platform/messaging \
  -run 'Messaging|Factory' -count=1
```

失败原因：

- `AMQPConfig.EventExchange`、`MessagingConfig.Outbox` 和 `OutboxConfig` 不存在；
- `NewFromConfig` 仍为旧签名；
- `Factory.Start`、`Factory.Close` 和 `Factory.Operations` 不存在。

生命周期 RED：

```text
go test ./internal/platform/messaging \
  -run TestFactoryConcurrentCloseWaitsAndReturnsSameError -count=1

go test ./internal/platform/messaging \
  -run TestDispatcherCloseCancelsBlockedClaim -count=1

go test ./internal/platform/messaging \
  -run TestFactoryStopsAllIntakeBeforeWaitingAndClosingResources -count=1
```

分别证明：

- 第二个 `Close` 会在第一轮 drain 完成前提前返回；
- Outbox `Close` 不能取消阻塞的 claim；
- Factory 未先停止全部 claim/intake 再统一等待。

## 初始 GREEN

- `inmemory` 使用不可解析 AMQP URL 仍同步发布成功，证明不拨号 RabbitMQ。
- AMQP 事件 transport 使用独立 publisher/consumer connections；原 task Queue 使用自身连接。
- Event Outbox 与 Operation workers 复用同一次 Factory 构造生成的 instance owner/runtime UUID。
- Bootstrap 顺序：DB/migration → Registry/Transactor/Factory/Container → modules/plugins registration → `Factory.Start` → HTTP server。
- Factory 并发/重复 `Close` 只执行一次，后续调用等待同一轮 drain 并返回相同错误。
- Outbox claim context 可取消；已领取记录使用 `context.WithoutCancel` 完成 confirm 与 Mark/Reschedule。
- ReliableBus 启动失败统一进入 stopping/closed，后续 Close 不重复关闭资源。

## 审查修复：RED

审查指出两个 Important 和一个报告污染问题。新增测试后运行：

```text
go test ./internal/platform/messaging \
  ./internal/platform/messaging/persistence \
  ./internal/bootstrap \
  -run 'InMemoryWithDatabase|ValidateTransactionalEventSchema|AutoMigrateDoesNotCreate|AMQPSchemaFailure|MessagingBootstrap|ChecksMessagingSchema' \
  -count=1
```

RED 证据：

1. `TestFactoryInMemoryWithDatabaseDoesNotCreateOperationRuntime`
   - 实际得到非 nil `Factory.Operations`；
   - 本地 `inmemory + DB` 因而会启动 Operation Dispatcher 并访问 `platform.operation_outbox`。
2. `TestFactoryAMQPSchemaFailureDoesNotConstructWorkersOrConnections`
   - migration version 69 时 Factory 仍成功；
   - task Queue/EventTransport 已可能在 schema 校验前建立连接。
3. persistence/bootstrap 测试因 `ValidateTransactionalEventSchema` 与 `validateMessagingBootstrap` 尚不存在而编译失败。
4. 原报告在 RabbitMQ Task 7 风险段落后混入 Kubernetes 插件 Task 7 报告。

## 审查修复：GREEN

### In-memory 隔离

- OperationRuntime 创建条件从 `db != nil` 收紧为 `messaging.driver == amqp`。
- `inmemory + non-nil DB` 下：
  - `Factory.Operations == nil`；
  - `Factory.Start` 不启动 Operation Dispatcher；
  - 不读取三张 transactional event 表；
  - InMemoryBus 保持同步 handler 语义。

### PostgreSQL migration 门禁

新增 `persistence.ValidateTransactionalEventSchema`：

- 非 PostgreSQL（SQLite 单测）显式跳过正式 migration 门禁；
- PostgreSQL 必须可查询 `public.schema_migrations`；
- 必须存在 migration row；
- `dirty` 必须为 `false`；
- `version` 必须 `>= 70`；
- `platform.event_outbox`、`platform.event_inbox`、`platform.operation_outbox` 必须全部存在；
- 缺失、旧版本、dirty、表缺失或查询错误均返回明确错误：

```text
run `make migrate-up` with DATABASE_URL to apply migration 000070
```

门禁执行两次，职责不同：

1. bootstrap 在 Redis、Factory、worker、HTTP server 之前 fail-fast；
2. Factory 在任何 AMQP Queue/EventTransport 构造之前防御性校验。

`messaging/persistence.AutoMigrate` 在 PostgreSQL 上不再创建事务事件表；GORM fallback 仅保留给 SQLite/非 PostgreSQL 测试。正式执行路径保持：

```text
cd backend
make migrate-up DATABASE_URL="<postgres dsn>"
```

### 启动与 readiness

- schema 缺失属于启动配置错误：worker 与 HTTP server 均不启动；
- 启动成功后 RabbitMQ outage 仍允许 Outbox 安全积压；
- readiness 仍不探测 RabbitMQ，PostgreSQL 不可用时继续失败。

### 报告清理

已删除原报告中 RabbitMQ Task 7 风险段落之后的全部 Kubernetes 插件内容；本文件只记录 RabbitMQ transactional events Task 7。

## 验证

审查修复 focused GREEN：

```text
go test ./internal/platform/messaging \
  ./internal/platform/messaging/persistence \
  ./internal/bootstrap \
  -run 'InMemoryWithDatabase|ValidateTransactionalEventSchema|AutoMigrateDoesNotCreate|AMQPSchemaFailure|MessagingBootstrap|ChecksMessagingSchema' \
  -count=1
```

结果：PASS。

最终验证结果将在提交前补充。

提交前完整验证：

```text
go test ./internal/platform/config \
  ./internal/platform/messaging/... \
  ./internal/bootstrap \
  ./migrations \
  -count=1

go build ./cmd/server
```

结果：PASS。

本地 `backend/config.yaml` 保持 `driver: inmemory`，执行：

```powershell
.\scripts\dev-restart.ps1 -Backend
```

结果：后端启动成功，`GET /api/v1/auth/providers` 返回 200；新启动日志不再出现 `platform.operation_outbox` 查询。

使用同一 migration version 69 的开发 PostgreSQL 临时验证 AMQP 门禁：

```powershell
$env:MESSAGING_DRIVER = "amqp"
$env:AMQP_URL = "amqp://invalid:invalid@127.0.0.1:1/"
go run ./cmd/server
```

结果在 RabbitMQ 拨号与 HTTP server 启动前退出：

```text
messaging schema validation: transactional event schema is not ready:
schema migration version 69 is older than required version 70;
run `make migrate-up` with DATABASE_URL to apply migration 000070
```

合理全量回归：

```text
go test ./... -count=1 -timeout 600s
```

Task 7 相关包通过；全量命令仍仅有工作区既存的两个 Kubernetes plugin 失败：

- `TestKubernetesBundledFilesBuildRealInspectableZip`
- `TestExecPersistsRecordingWithoutCommandOrCredentials`

两项均与本提交文件无关，未修改其实现或测试。

## 提交范围

- `backend/internal/platform/messaging/factory.go`
- `backend/internal/platform/messaging/factory_test.go`
- `backend/internal/platform/messaging/factory_lifecycle_test.go`
- `backend/internal/platform/messaging/persistence/migrate.go`
- `backend/internal/platform/messaging/persistence/migrate_test.go`
- `backend/internal/bootstrap/app.go`
- `backend/internal/bootstrap/messaging_schema_test.go`
- `.superpowers/sdd/task-7-report.md`

不修改、不暂存、不提交 frontend、progress、Task 5 report、logs 或 plugin 并发工作。

## 风险与运维动作

- AMQP 模式部署必须先执行 migration 000070，再启动应用。
- 当前 `backend/config.yaml` 为 `inmemory`，即使远程开发库未迁移也应正常启动且不访问 Operation Outbox。
- 真实 RabbitMQ/PostgreSQL 故障窗口端到端集成测试属于 Task 11；本任务覆盖配置、schema gate、Factory 和 bootstrap 启动顺序。

## 第二轮审查修复：schema_migrations 单行约束

### RED

新增顺序无关的表驱动测试：

```text
clean(70) → dirty(70)
dirty(70) → clean(70)
current(70) → old(69)
old(69) → current(70)
```

运行：

```text
go test ./internal/platform/messaging/persistence \
  ./internal/platform/messaging \
  -run 'MultipleMigrationRows' \
  -count=1
```

现有 `SELECT ... LIMIT 1` 的 RED 结果：

- clean/current 先插入时，schema 校验可能继续通过，Factory 会构造 RabbitMQ resources；
- dirty/old 先插入时，错误变为 dirty/old；
- 同一组 migration rows 的结果取决于返回顺序，未检测多行损坏状态。

### GREEN

`ValidateTransactionalEventSchema` 改为：

1. 查询 `public.schema_migrations` 的全部 `version, dirty` rows；
2. 首先要求行数严格等于 1；
3. 0 行明确报 `found 0`；
4. 多行明确报 `must contain exactly one row, found N`；
5. 只有恰好一行时才校验 `dirty=false`、`version>=70`；
6. 再验证三张 `platform` 表。

Factory 继续在 Metrics、task Queue 和 EventTransport 创建之前调用该门禁。四种多行顺序下均断言 Queue/EventTransport 构造计数为零。

Focused GREEN：

```text
go test ./internal/platform/messaging/persistence \
  ./internal/platform/messaging \
  -run 'MultipleMigrationRows' \
  -count=1
```

结果：PASS。

完整验证：

```text
go test ./internal/platform/config \
  ./internal/platform/messaging/... \
  ./internal/bootstrap \
  ./migrations \
  -count=1

go build ./cmd/server
```

结果：PASS。

后端源码修改后执行：

```powershell
.\scripts\dev-restart.ps1 -Backend
```

结果：本地 `inmemory` 后端成功重启并监听 8080。
