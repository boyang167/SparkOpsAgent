# Task 5 独立审查修复报告

日期：2026-10-17
修复基线：`3750aef66080ae7bec1ce4f008ec5c6d33492573`

## 审查结论验证

四项 Important 均经代码和测试验证有效：

1. Task 4 transport 在 envelope `json.Unmarshal` 失败时直接 `Nack(requeue=true)`，无法进入 Inbox 永久错误/DLQ 流程。
2. `ReliableBus.Close` 取消 intake context；同一 context 原样传入 handler，导致活跃事务收到取消并回滚。
3. `OutboxDispatcher.Start` 在 goroutine 启动前不校验依赖，循环完全丢弃 `DispatchOnce` 错误。
4. Metrics 只在 InboxConsumer 的未注入字段分支中使用；Dispatcher、Outbox backlog 和持久化统计均未接线。

固定 v1 的 `Subscribe` 本轮不改：现有 Bus 接口不携带 version，当前 codec 全部为 v1。新增版本前必须先设计兼容的订阅版本协议。

## TDD RED

### Poison envelope

命令：

```text
go test ./internal/platform/messaging -run 'MalformedRawDelivery' -count=1 -timeout 20s
```

RED：测试等待“DLQ confirm 后 ACK”超时；goroutine 栈显示 `consumeDeliveries` 仍在循环，原消息只被 NACK/requeue，publisher 未被调用。

### Graceful Close

命令：

```text
go test ./internal/platform/messaging -run TestReliableBusClosePreservesActiveHandlerTransactionContext -count=1
```

RED：

```text
Close returned before active transaction completed: <nil>
```

证明 Close 取消了活跃 handler context，事务未等待业务完成。

### Dispatcher 启动校验

命令：

```text
go test ./internal/platform/messaging -run TestDispatcherStartValidatesRequiredDependenciesBeforeLaunching -count=1
```

RED：store、transport、owner 三个子测试均“expected error but got nil”。

### Metrics 与 Outbox stats

命令：

```text
go test ./internal/platform/messaging/persistence -run TestOutboxStats -count=1
```

RED：`OutboxStore` 没有 `Stats`。

命令：

```text
go test ./internal/platform/messaging -run 'TestDispatcherUpdatesPublishAndBacklogMetrics|TestDispatcherTracksPublishFailureAndBackgroundLoopError' -count=1
go test ./internal/platform/messaging -run 'TestInboxConsumerRecords' -count=1
```

RED：Dispatcher 与 InboxConsumer 构造器均不接受 Metrics，证明生产构造路径未接线。

## GREEN 与实现

- Poison 原消息不构造伪 envelope；保留 body、MessageId、headers 和 AMQP 元数据，增加固定诊断 header，经独立 publisher confirm channel 发布到 subscriber DLQ。
- DLQ publish 与 confirm 成功后才 ACK；publish error/NACK 时 NACK 原消息并 requeue。publisher 复用 Task 4 的串行化、attempt 关联、关闭信号及连接退休逻辑。
- ReliableBus 在取消 intake 前关闭 InboxConsumer 接收门；handler 使用保留 values 但与 intake cancellation 解耦的 context。Close 等待活跃事务完成，再关闭 transport。
- Dispatcher `Start` 在创建 goroutine 前验证 store、transport、owner；循环错误写入固定低基数 `result=dispatcher_error`。
- Metrics 成为 InboxConsumer/OutboxDispatcher 构造依赖。覆盖 publish success/failure、consume success/duplicate/permanent/retry、retry/DLQ confirm 成功及发布失败。
- `OutboxStore.Stats` 查询 pending/publishing 总数和最老 `created_at`；Dispatcher 更新 pending 与 oldest gauges。Operation collectors 保留但未接入 Task 6 runtime。

目标 GREEN：

```text
go test ./internal/platform/messaging/... ./internal/platform/postgres -count=1
```

结果：PASS。

后端全量：

```text
go test ./... -count=1
```

Task 5 相关包通过；全量仍仅有两项已知 Kubernetes 基线失败：

- `TestKubernetesBundledFilesBuildRealInspectableZip`：descriptor 缺少 `diagnosis/agent.go`。
- `TestExecPersistsRecordingWithoutCommandOrCredentials`：录制内容精确字符串断言不匹配。

## 提交范围

只提交 Task 5 修复涉及的 messaging、persistence 源码/测试和本报告。不会提交或覆盖：

- `.superpowers/sdd/progress.md`
- `.superpowers/sdd/task-5-report.md` 中并行 plugin 工作追加内容
- `logs/sparkops.log`
- frontend 工作区修改

---

## 第二轮复审修复

复审基线：`edb2ad8aeea984da658a4f880edab63b5dfefb33`

### TDD RED

1. Envelope 结构校验：

```text
go test ./internal/platform/messaging -run TestEventTransportRoutesStructurallyInvalidEnvelopeToSubscriberDLQ -count=1
```

ID 为空/非 UUID、Type 为空/空白、Version 非正数、缺少 occurred_at/payload 的全部子测试均失败：handler 被调用且没有 DLQ publish。

2. Stats 观测旁路：

```text
go test ./internal/platform/messaging -run TestDispatcherStatsFailureDoesNotBlockReliablePublish -count=1
```

RED：`published` 长度为 0，证明 Stats 错误在 Claim 前中断分发。

3. 部分启动失败回滚：

```text
go test ./internal/platform/messaging -run TestReliableBusStartFailureDrainsStartedHandlerAndTerminatesLifecycle -count=1
```

RED：

```text
Start returned before the active handler drained: start second consumer
```

4. 指标语义：

```text
go test ./internal/platform/messaging -run 'TestEventTransportRoutesMalformedRawDeliveryToSubscriberDLQBeforeAck|TestMetricsRegistersRequiredCollectorsOnce|TestDispatcherStatsFailureDoesNotBlockReliablePublish|TestDispatcherTracksPublishFailureAndBackgroundLoopError' -count=1
```

RED：AMQP transport 没有 metrics 注入，Metrics 没有独立 `EventDispatcherError` collector。

### GREEN 实现

- 在 decoded handler 前校验 envelope：ID 必须为 UUID、Type 非空白、Version 为正数、occurred_at 非零、payload 非空且非 null。结构错误与 JSON/time 解析错误共用 raw DLQ confirm 路径，保留原 body/headers。
- Stats 错误记录 `dispatcher_error_total{stage="stats"}` 并加入返回错误，但不阻断 Claim、Publish、MarkPublished。
- 任一 Start 失败都把 bus 置为终止状态：停止 Inbox 接收、取消本批 intake、关闭 dispatcher、等待 detached handler、关闭 transport；后续 Start 明确返回 closed，Close 不重复释放。
- raw poison DLQ confirm 成功后递增有界 subscriber 标签的 `EventDLQ`。
- `EventPublish{result}` 只记录实际 broker publish attempt 的 success/failure。Stats、Claim、Reschedule、MarkPublished 使用独立 `sparkops_event_dispatcher_error_total{stage}`，不再写入 publish total。
- 固定 v1 订阅决策保持不变。

### GREEN 验证

```text
go test ./internal/platform/messaging/... ./internal/platform/postgres -count=1
```

结果：PASS。

关键 poison、Stats side-path、graceful close 和部分启动失败用例以 `-count=20` 重复运行：PASS。

后端全量 `go test ./... -count=1` 中 Task 5 相关包通过，仅命中既有两项 Kubernetes 基线失败：bundled descriptor 缺少 `diagnosis/agent.go`，以及 exec recording 精确字符串断言不匹配。

---

## 最终复审修复

复审基线：`21e60b99107bb227fc6fdba5f93ef2d029fb1eb3`

### TDD RED

```text
go test ./internal/platform/messaging -run TestEventTransportRoutesStructurallyInvalidEnvelopeToSubscriberDLQ -count=1
```

新增的 type 超长、type 含 NUL、type 与 subscription 不匹配、version 与 subscription 不匹配四个子测试全部失败：handler 被调用且没有 DLQ publish。

integration 测试静态核对确认 MessageId 断言仍比较 queue suffix，而生产发布使用随机生成的 envelope UUID；连接 RabbitMQ 时该断言必然失败。

### GREEN 实现

- `validateEventEnvelope` 现在接收当前 `Subscription`，在 handler 前强制 `Type == EventName`、`Version == Version`。
- event type 限制为最多 160 个 ASCII 字节、禁止 NUL，并采用稳定名称最小语法：点分段，段内仅字母、数字、下划线或连字符。
- 所有新增结构错误继续复用 raw body/headers subscriber DLQ confirm 路径；confirm 成功 ACK，失败 NACK/requeue。
- integration 测试保存实际发布的 envelope，并用 `envelope.ID` 断言 `delivery.MessageId`。

### GREEN 验证

```text
go test ./internal/platform/messaging/... -count=1
go test ./internal/platform/messaging -run 'TestEventTransportRoutesStructurallyInvalidEnvelopeToSubscriberDLQ|TestEventTransportRoutesMalformedRawDeliveryToSubscriberDLQBeforeAck|TestEventTransportNacksMalformedRawDeliveryWhenDLQConfirmFails' -count=20
go test -tags=integration ./internal/platform/messaging -run '^$' -count=1
```

结果：全部 PASS。RabbitMQ integration 测试已成功编译；实际运行因本机未设置 `AMQP_URL` 而明确 SKIP。
