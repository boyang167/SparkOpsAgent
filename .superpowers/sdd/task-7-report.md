# Task 7：Configuration、Factory、Bootstrap 与 In-Memory 兼容

日期：2026-10-17
起始 HEAD：`35af97a`

## 范围

本任务只实现 Task 7 的配置与运行时装配：

- 增加 Event Exchange 与 Outbox 配置模型、默认值、YAML/env 覆盖及校验；
- 保持 `messaging.driver` 同时控制现有异步任务 Queue 与事件 Bus 的兼容语义，但为任务交换机和事件交换机保留独立配置；
- `inmemory` 使用同步 EventBus + NoopQueue，不连接 RabbitMQ；
- `amqp` 构造原任务 Queue、双连接 EventTransport、Outbox/Inbox/Operation stores、Metrics、ReliableBus、Event/Operation dispatchers；
- 生成一次 instance owner/runtime ID，并在同一 Factory 内稳定复用于 Event Outbox 与 Operation workers；
- 模块完成 codecs、subscriptions 和 operation handlers 注册后才启动消息 worker；
- 启动失败逆序回滚；关闭先停止所有 claim/intake，再 drain 在途工作，最后关闭 EventTransport、任务 Queue 和数据库；
- readiness 继续只检查 PostgreSQL/Redis，不把 RabbitMQ 暂时不可用直接映射为 HTTP not-ready；
- 更新三个示例配置，不修改本地 `backend/config.yaml`；
- 未迁移 Task 8–10 的任何具体业务 publisher、subscriber 或 operation handler。

## TDD：RED

首次配置与工厂测试：

```text
go test ./internal/platform/config ./internal/platform/messaging \
  -run 'Messaging|Factory' -count=1
```

预期 RED（exit 1）：

- `AMQPConfig.EventExchange`、`MessagingConfig.Outbox` 和 `OutboxConfig` 不存在；
- `NewFromConfig` 仍为旧签名；
- `Factory.Start`、`Factory.Close` 和 `Factory.Operations` 不存在。

关闭并发 RED：

```text
go test ./internal/platform/messaging \
  -run TestFactoryConcurrentCloseWaitsAndReturnsSameError -count=1
```

第二个 `Close` 在第一轮 drain 尚未完成时提前返回 `nil`。

Outbox goroutine RED：

```text
go test ./internal/platform/messaging \
  -run TestDispatcherCloseCancelsBlockedClaim -count=1
```

`Close` 不能取消阻塞中的 `Stats/Claim`，100ms 后仍未返回。

分阶段关闭 RED：

```text
go test ./internal/platform/messaging \
  -run TestFactoryStopsAllIntakeBeforeWaitingAndClosingResources -count=1
```

实际只有 `operation-close/event-close/queue-close`，未先停止全部 claim/intake 再统一等待。

AMQP 工厂装配 RED：

```text
go test ./internal/platform/messaging \
  -run 'TestFactory(AMQPAssembly|ConstructionFailure)' -count=1
```

缺少可测试的 AMQP 构造边界和完整构造失败回滚。

## GREEN

### 配置矩阵

| 模式 | 数据库 | RabbitMQ | EventBus | Task Queue | Operation runtime |
| --- | --- | --- | --- | --- | --- |
| `inmemory` health-only | 无 | 不需要 | 同步内存 | NoopQueue | 不启动 |
| `inmemory` 本地开发 | 有 | 不需要 | 同步内存 | NoopQueue | 独立启动 |
| `amqp` | 必需 | 必需 | ReliableBus | 原 AMQP Queue | 独立启动 |
| production + `inmemory` | 任意 | 任意 | 校验拒绝 | 校验拒绝 | 不进入启动 |

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

非法 duration/int、非正 Outbox 值、AMQP 缺 URL/Event Exchange、production 使用 `inmemory` 或缺数据库均被拒绝。

### Factory 与 bootstrap

- `inmemory` 测试使用不可解析 AMQP URL，仍同步发布成功，证明没有 RabbitMQ 拨号。
- AMQP 事件 transport 使用独立 publisher/consumer connections；原 task Queue 使用自身连接和 `exchange/exchange_type`。
- 可配置 `event_exchange` 同时用于 topology declaration 与事件 publish。
- Factory 同一次构造生成一个 instance owner/runtime UUID：
  - Outbox owner：`<instance>.<runtime>.event-outbox`；
  - Operation worker：`<instance>.<runtime>.worker-N`。
- Bootstrap 顺序为 DB connection/migration → Registry/Transactor/Factory/Container → modules/plugins registration → `Factory.Start` → HTTP server。
- readiness dependencies 仍只有 DB/Redis；RabbitMQ 故障由 Outbox 指标和日志表达 degraded，不直接打失败 readiness。

### 回滚与关闭

启动失败按以下逆序释放：

```text
operation runtime → reliable event runtime → task queue
```

正常关闭分阶段执行：

```text
stop operation claims
stop event outbox claims and consumer intake
wait operation in-flight
wait event confirms and handler transactions
close operation runtime
close event AMQP publisher/consumer connections
close task queue connection
close Redis / PostgreSQL
```

- Factory 并发/重复 `Close` 只执行一次，后续调用等待同一轮 drain 并返回相同 error。
- Outbox Dispatcher 的 claim context 可取消；已领取记录改用 `context.WithoutCancel` 完成 confirm 与 Mark/Reschedule。
- Operation Dispatcher 接受 Factory 提供的 runtime ID，仍保留无配置时自行生成 UUID 的兼容行为。
- ReliableBus 启动失败设置统一 stopping/closed 状态，后续 Close 不重复关闭 dispatcher/transport。

## 验证

通过：

```text
go test ./internal/platform/config ./internal/platform/messaging ./internal/bootstrap -count=1
go test ./internal/platform/messaging/... -count=1
go test ./internal/platform/config ./internal/bootstrap ./internal/platform/appctx ./internal/platform/httpx -count=1
go test -tags=integration ./internal/platform/messaging/... -run '^$' -count=1
go build ./cmd/server
```

关键生命周期用例重复通过：

```text
go test ./internal/platform/messaging \
  -run 'TestFactory|TestDispatcherClose|TestOperationDispatcher(ParentCancelAndRepeatedClose|CloseCancelsBlockedClaim)' \
  -count=20

go test ./internal/platform/messaging \
  -run TestReliableBusStartFailureDrainsStartedHandlerAndTerminatesLifecycle \
  -count=50
```

全量回归：

```text
go test ./... -count=1 -timeout 600s
```

Task 7 相关包全部通过；全量命令仅有两个与本任务无关的 Kubernetes 插件失败：

- `TestKubernetesBundledFilesBuildRealInspectableZip`：当前并发修改的 plugin descriptor 缺 `diagnosis/agent.go`；
- `TestExecPersistsRecordingWithoutCommandOrCredentials`：当前 stream 录制把 stdout/stderr 合并为一条记录，与测试期望不符。

两项均已独立复现，未修改其文件。

## 本地重启

按规则执行：

```powershell
.\scripts\dev-restart.ps1 -Backend
```

脚本首次在 90 秒窗口超时；捕获同一 `go run ./cmd/server` 命令后约 62 秒完成模块注册并启动：

```text
server starting {"addr": ":8080"}
GET /api/v1/auth/providers → 200
messaging.driver: inmemory
未连接 RabbitMQ
```

当前开发 PostgreSQL 尚未应用 `000070_transactional_events.up.sql`，独立 Operation Dispatcher 因而记录 `platform.operation_outbox` 不存在。该库属于共享远程配置，未擅自执行 DDL；应用 000070 后该告警才会消失。

## 提交范围

- `backend/internal/platform/config/config.go`
- `backend/internal/platform/config/validate.go`
- `backend/internal/platform/config/validate_test.go`
- `backend/internal/platform/messaging/factory.go`
- `backend/internal/platform/messaging/factory_test.go`
- `backend/internal/platform/messaging/factory_lifecycle_test.go`
- `backend/internal/platform/messaging/amqp_event_transport.go`
- `backend/internal/platform/messaging/reliable_bus.go`
- `backend/internal/platform/messaging/outbox_dispatcher.go`
- `backend/internal/platform/messaging/outbox_dispatcher_test.go`
- `backend/internal/platform/messaging/operation_dispatcher.go`
- `backend/internal/platform/appctx/container.go`
- `backend/internal/bootstrap/app.go`
- `backend/config.example.yaml`
- `backend/config.remote.example.yaml`
- `backend/deployments/docker/config.docker.yaml`
- `.superpowers/sdd/task-7-report.md`

未修改、暂存或提交 frontend、progress、Task 5 report、logs，以及并发出现的 backend plugin 文件。

## 风险

- 当前共享开发数据库必须先应用 migration 000070，Operation Dispatcher 才能正常工作；生产 rollout 同样必须严格先迁移再启动。
- 真实 RabbitMQ/PostgreSQL 故障窗口集成测试属于 Task 11；本任务只完成 integration-tag 编译和工厂级连接/生命周期测试。
- 全量回归受两项现有 Kubernetes plugin 失败阻塞，Task 7 相关回归均通过。
# Task 7 Report

## Status

PASS

已实现内置 `kubernetes` 插件骨架、完整 manifest、`deployed` persistence seed、最小标准 tar 资源和前端根页面占位，并创建 Git commit。

## Files

- `backend/internal/plugins/kubernetes/plugin.go`
- `backend/internal/plugins/kubernetes/plugin.yaml`
- `backend/internal/plugins/kubernetes/http/router.go`
- `backend/internal/plugins/kubernetes/plugin_test.go`
- `backend/internal/plugins/kubernetes/resource/arthas-bin.tar`
- `backend/internal/plugins/register.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_test.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_seed_test.go`
- `fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx`

## RED

- `go test ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ -count=1` 按预期失败：
  - `plugin` 未定义，证明 ready 路由和 `RegisterWithContext` 骨架尚未实现。
  - `seedBundledKubernetesPlugin` 未定义，证明 persistence seed 尚未实现。

## GREEN

- `go test ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ -count=1`
- `go test ./internal/platform/plugin/ ./internal/plugins/... ./internal/modules/governance/application/command/ -count=1`
- `python -c "import tarfile; ...; print(tarfile.is_tarfile(p))"` 输出 `True`。
- `git diff --check` 仅报告 Windows CRLF 转换提示，无 whitespace error。
- ReadLints：无诊断。

## Self-review

- `init` 只执行 `sparkplugin.Register(&plugin{})`；`Register` 和 `RegisterWithContext` 都委托同一个无状态 `register(engine)`，正常框架路径只调用 `RegisterWithContext`。
- `plugin.yaml` 使用 `id=kubernetes`、`entitlement=plugin:kubernetes`、`route_prefix=/api/v1/plugins/kubernetes`；菜单覆盖 brief/GVA 列表，包含 Velero backup、cloudtty、velaux，不包含 Karmada。
- manifest 中 `accessControls` 保持规格/GVA 大小写；未额外创建 `access-controls` 变体，前端任务若需要别名应在后续显式处理。
- `metrics` POST 显式 `read_only: true`；其它写、Exec、Chat API 未标只读；`ANY` 仅用于 route prefix 下并以 `/*` 结尾的 proxy/diagnosis/velaux 前缀。
- `frontend.pages` 只列已创建的根 `app/(platform)/plugins/kubernetes/page.tsx`。
- persistence seed 为 `deployed`，使用 `FirstOrCreate` 语义，不覆盖已存在的 `kubernetes` 记录；测试同时断言 seed manifest 可 parse、关键菜单/API/asset 存在，且 seed manifest 来自插件包 `go:embed plugin.yaml`。

## Commit

`Bundle the kubernetes plugin skeleton as a deployed workspace plugin.`

## Review Remediation

### Status

PASS

### Files

- `backend/internal/plugins/kubernetes/plugin.go`
- `backend/internal/plugins/kubernetes/plugin_test.go`
- `backend/internal/plugins/kubernetes/http/router.go`
- `backend/internal/plugins/kubernetes/http/router_test.go`
- `backend/internal/plugins/kubernetes/resource/arthas-bin.tar`
- `backend/internal/modules/plugin/infrastructure/persistence/store.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_test.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_seed_test.go`
- `.superpowers/sdd/task-7-report.md`

### RED

- `go test ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ ./internal/platform/plugin/ -count=1` 按预期失败：
  - `bundledkubernetes.ManifestYAML` 未定义，证明 manifest 仍是 store 双源。
  - `arthas-bin.tar` 读取首个成员返回 `EOF`，证明最小 tar 还没有占位成员。
- `go test ./internal/plugins/kubernetes/http -count=1` 按预期失败：重复 `RegisterRoutes(engine)` 未 panic，证明 router 仍在静默跳过重复路由。

### GREEN

- `go test ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ ./internal/platform/plugin/ -count=1`
- `go test ./internal/plugins/... ./internal/modules/governance/application/command/ -count=1`
- tar 成员检查输出 `[('README.placeholder', 0, 420)]`。
- `git diff --check` 仅报告 Windows CRLF 转换提示，无 whitespace error。
- ReadLints：无诊断。

### Self-review

- `backend/internal/plugins/kubernetes` 通过 `//go:embed plugin.yaml` 暴露只读 `ManifestYAML`；persistence seed 直接 import bundled kubernetes 包使用该内容，未形成 import cycle。
- `store.go` 已删除整份 kubernetes manifest 常量；store 字面一致性测试已删除，改为断言 seed 持久化的是 embed manifest。
- `seedBundledKubernetesPlugin` 在创建前查询 `uninstallReceiptModel{plugin_id: kubernetes}`；存在卸载 receipt 且无 plugin record 时不会重建 kubernetes，不影响 example seed。
- router 删除 `engine.Routes()` 检查；`Register` 与 `RegisterWithContext` 共享无状态 `register(engine)`，兼容性测试分别使用不同 gin.Engine，同一 engine 重复挂载由 Gin panic 暴露。
- manifest API 测试已逐项覆盖 clusters/settings/proxy/metrics、files、logs、exec、records、cloudtty、nodetty、diagnosis、velaux、chat，并确认 metrics `read_only: true`。
- 最小 tar 仍是标准 tar，包含零字节 `README.placeholder`，没有脚本或可执行内容。

### Commit

`9ba711a Honor kubernetes plugin review constraints.`

## Critical Remediation

### Status

PASS

### Files

- `backend/internal/platform/plugin/registry.go`
- `backend/internal/platform/plugin/registry_test.go`
- `backend/internal/plugins/kubernetes/plugin.go`
- `backend/internal/plugins/kubernetes/plugin_test.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_test.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_seed_test.go`
- `.superpowers/sdd/task-7-report.md`

### RED

- `go test ./internal/platform/plugin/ ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ ./internal/bootstrap/ -count=1` 按预期失败：
  - `sparkplugin.RegisterBundled`、`sparkplugin.Bundled`、`sparkplugin.BundledDescriptor` 未定义。
  - `registerBundledDescriptor` 未定义。
  - persistence seed 测试无法通过稳定平台注册 descriptor 建立前提。

### GREEN

- `go test ./internal/platform/plugin/ ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ ./internal/bootstrap/ -count=1`
- `go test ./internal/plugins/... -count=1`
- `rg "internal/plugins/kubernetes" backend/internal/modules/plugin/infrastructure/persistence` 仅命中 installed file 字符串和测试 descriptor 字符串，无 persistence 生产 import。
- `git diff --check` 仅报告 Windows CRLF 转换提示，无 whitespace error。
- ReadLints：无诊断。

### Self-review

- `platform/plugin` 新增稳定 bundled descriptor 注册表：空 id/空 manifest 忽略，`Bundled` 对未知 id 返回 false，注册和读取均复制 `InstalledFiles`，`ResetRegistryForTest` 同时清理测试注册表以避免顺序污染。
- `kubernetes` 插件包保留 `//go:embed plugin.yaml` 单一 manifest 内容源；同一个 `init` 先 `Register(&plugin{})`，再 `RegisterBundled("kubernetes", descriptor)`，descriptor 列完整 Task 7 骨架 installed files。
- `persistence` 只 import 稳定 `platform/plugin`，通过 `Bundled("kubernetes")` 获取 seed descriptor；descriptor 缺失时跳过 seed，不报错；卸载 receipt 检查仍在 descriptor 查询前保留。
- 该结构允许卸载 PR 删除 kubernetes 插件目录和 register.go blank import 后，稳定 persistence 继续编译，且不会重种 kubernetes。

### Commit

`4ef35df Decouple bundled kubernetes seed from plugin package.`
