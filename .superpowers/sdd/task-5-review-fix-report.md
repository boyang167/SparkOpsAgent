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
