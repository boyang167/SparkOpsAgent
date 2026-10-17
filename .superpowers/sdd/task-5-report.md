# Task 5：Reliable Bus、Dispatcher、Inbox Consumer 与错误分类

日期：2026-10-17
起始 HEAD：`d54b93bd50c112486b6dde3a5447a95bb7b0a435`

## 范围

本任务只实现计划中的 Task 5：

- `ReliableBus`：事务内编码并写 Event Outbox，不进行网络发布；
- `OutboxDispatcher`：按 owner/lease 领取，等待 publisher confirm，成功后按 owner 标记 published，失败后按 owner reschedule；
- `InboxConsumer`：Inbox 去重、handler 与 Inbox 同事务提交，按永久/临时错误选择 DLQ 或固定退避层级；
- `Permanent` / `IsPermanent` 错误分类；
- Task 5 要求的 Prometheus collectors；
- 声明拓扑、启动 consumer、启动 dispatcher 以及有序幂等关闭。

未实现 Task 6 的 Operation Outbox runtime，也未接入 Task 7 的 factory/bootstrap。

## TDD：RED

先创建 Task 5 测试，再运行：

```text
cd backend
go test ./internal/platform/messaging -run 'Dispatcher|InboxConsumer|ReliableBus' -count=1
```

得到预期 RED（exit 1）：`Permanent`、`IsPermanent`、`InboxConsumer`、`NewMetrics`、`NewOutboxDispatcher`、`OutboxDispatcherConfig` 等生产符号均不存在，证明失败来自 Task 5 行为缺失，而不是测试拼写或环境问题。

自审时另做了两个独立 RED/GREEN 循环：

1. `TestReliableBusCloseRejectsLateHandlersAndWaitsForActiveHandlers`
   - RED：关闭完成后仍可进入新的 handler，期望 `context.Canceled`，实际为 `nil`。
   - GREEN：consumer 增加停止门控；关闭先拒绝新 handler，再等待已有事务完成。
2. `TestDispatcherCloseStopsClaimsAndWaitsForInflightConfirm`
   - RED：`Close` 取消了正在等待的 publisher confirm。
   - GREEN：dispatcher 使用独立 stop 信号停止新 claim，让当前 confirm 完成后再退出。

## GREEN 与行为证据

目标及相关包回归：

```text
go test ./internal/platform/messaging/... -count=1
```

结果：PASS。

覆盖的关键故障窗口：

- confirm 失败：保持稳定 envelope ID，调用 `Reschedule`，不标记 published；
- confirm 成功、标记 published 前数据库失败：记录保持 publishing，租约恢复后可用同一 envelope ID 重发；
- 多 dispatcher：store 的 claim/owner 规则产生互斥批次；
- handler 成功：Inbox 与 handler 副作用同一事务提交后才允许 ACK；
- 重复消息：Inbox 冲突跳过 handler 并 ACK；
- handler 失败：Inbox 和副作用回滚，确认发布 retry 后才 ACK 原消息；
- 永久 decode 错误：确认发布 DLQ 后才 ACK；
- retry/DLQ publish confirm 失败：向 transport 返回错误，由原 delivery NACK/requeue；
- retry 层级：5s、30s、5m、30m，之后持续 30m；
- shutdown：先停 dispatcher claim 并等待 confirm，再取消 consumer、等待活跃事务，最后关闭 transport；重复关闭安全。

## 设计取舍

- 复用 Task 3 的 `OutboxStore` / `InboxStore`，不复制 SQL；owner 与 `RowsAffected` 租约防护继续由 store 强制执行。
- 复用 Task 4 的 `EventTransport`。Inbox handler 仅在 retry/DLQ 发布 confirm 成功时返回 `nil`，因此 Task 4 transport 只会在安全时 ACK 原消息。
- Outbox backoff 与消费 retry 使用同一固定层级函数；不增加动态配置，避免提前实现后续任务。
- `ReliableBus.Publish` 在编码前显式调用 `postgres.RequireTx`，事务外发布立即失败；网络 I/O 只存在于 dispatcher。
- 当前注册的领域事件均为计划规定的 v1，`Subscribe` 因既有 Bus 接口不携带版本而生成 v1 `Subscription`。
- Metrics 使用传入的私有 `prometheus.Registry`/`Registerer`，同一 registerer 只注册一次，不使用全局默认注册表。

## 回归

后端全量命令：

```text
go test ./... -count=1
```

Task 5 相关包通过；全量结果 exit 1，仅命中计划中已知的两项 Kubernetes 基线失败：

1. `TestKubernetesBundledFilesBuildRealInspectableZip`：descriptor 缺少 `backend/internal/plugins/kubernetes/diagnosis/agent.go`。
2. `TestExecPersistsRecordingWithoutCommandOrCredentials`：录制内容将 stdout/stderr 合并在同一 data 字段，和既有精确字符串断言不匹配。

两项均不在 Task 5 范围，未修改。

## 提交范围

提交九个 `backend/internal/platform/messaging` Task 5 源码/测试文件及本报告；不暂存、不覆盖 `.superpowers/sdd/progress.md`、`logs/sparkops.log` 或其他并行工作区修改。

## 风险与未决项

- 真实 RabbitMQ/PostgreSQL 故障窗口集成验证依赖可用外部服务，本任务以 fake transport/store 和 Task 3/4 各自集成边界测试验证。
- AMQP consumer connection 自身断开后的自动重订阅仍沿用 Task 4 的既有生命周期边界，不在 Task 5 扩展。
- Metrics collectors 已定义并可注入；factory/bootstrap 接线属于 Task 7。
