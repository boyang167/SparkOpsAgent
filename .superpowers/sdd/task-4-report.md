# Task 4：RabbitMQ Event Topology and Publisher Confirms

日期：2026-10-17
起始 HEAD：`e1179b6d081703ad65e3e9126f3f38219e316882`

## 中断恢复审查

发现四个未跟踪的部分实现文件同时存在：拓扑实现与测试、传输实现与测试。为验证测试不是在既有实现下“事后通过”，先临时将两份生产 `.go` 文件移出 Go 构建，再运行：

```text
cd backend
go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
```

得到真实 RED：测试因 `eventConfirmation`、`publishEventConfirmed`、`eventExchangeName`、`topologyFor` 和 `validateEventTopologies` 未定义而构建失败。恢复原实现后，同一命令通过（GREEN）。

审查后保留了正确的既有实现，没有重写 `amqp.go`，也没有实现 Inbox。新增了 build tag 为 `integration` 的真实 RabbitMQ 测试：

- 对同一个事件声明两个具名订阅者；
- 重复声明拓扑，以验证启动幂等性；
- 发布一次，并从两个独立主队列各读取一条消息；
- 清理该测试创建的主、retry 与 DLQ 队列。

## 实现核对

- 独立且 durable 的 topic exchange：`sparkops.events`。
- 每个订阅者独立的主队列、四级 retry 队列（5s、30s、5m、30m）和 DLQ 都是 durable quorum 队列。
- retry 队列通过默认 exchange（空 `x-dead-letter-exchange`）及自己的主队列名作为 routing key 回流，不会经 topic exchange 扇出给其他订阅者。
- 队列名规范化为小写 `[a-z0-9.-]`，超过 240 字符及规范化碰撞在声明前拒绝。
- 发布使用独立 confirm-enabled channel、`mandatory=true`、持久化 JSON、`MessageId`、时间戳和 deferred publisher confirm。
- NACK、mandatory return、channel close、上下文超时和缺少确认均被视为失败；失败时丢弃 publisher channel，下一次发布在状态锁内重建；发布串行化与资源状态锁分离，保证关闭可唤醒待确认发布。

## 验证

| 检查 | 结果 |
| --- | --- |
| 临时移开实现后的 Task 4 目标测试 | RED：缺少生产符号，符合预期 |
| 恢复后的 `go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1` | PASS |
| `go test -tags=integration ./internal/platform/messaging -run Rabbit -count=1 -timeout 120s` | 未通过：`localhost:5672` 明确拒绝连接，RabbitMQ 不可用 |
| `go test ./...` | Task 4 messaging 包通过；两个既有 Kubernetes 失败未处理：`TestKubernetesBundledFilesBuildRealInspectableZip` 缺 `diagnosis/agent.go`，`TestExecPersistsRecordingWithoutCommandOrCredentials` 的录制内容断言不匹配 |
| 后端重启 | PASS：PID 已更新为 6336，监听 `http://localhost:8080` |

## 自审

未发现 Task 4 范围内需修复的问题。剩余验证关注点是连接可用 RabbitMQ 后重新运行 integration 测试；该失败由本机 5672 端口无服务引起，而不是测试断言失败。

## 提交范围

只暂存并提交以下 Task 4 文件及本报告：

- `backend/internal/platform/messaging/amqp_event_topology.go`
- `backend/internal/platform/messaging/amqp_event_topology_test.go`
- `backend/internal/platform/messaging/amqp_event_transport.go`
- `backend/internal/platform/messaging/amqp_event_transport_test.go`
- `backend/internal/platform/messaging/amqp_event_transport_integration_test.go`
- `.superpowers/sdd/task-4-report.md`

未暂存 `.superpowers/sdd/progress.md`、日志或其他未跟踪文件。

## P1 修复：发布确认与关闭并发

审查发现：原来的 `publish` 在 `t.mu` 持锁状态下等待 deferred confirm。无 deadline 且 broker 不确认时，`Close` 需要同一把锁才能关闭 publisher channel，无法唤醒发布，形成循环等待。

### TDD 记录

先新增 `TestEventTransportCloseUnblocksPendingPublish`，以 fake channel 和未完成的 fake confirmation 让发布卡在确认等待；调用 `Close` 后要求其在 100ms 内返回，且发布因 channel close 返回错误。修复前运行该测试得到真实 RED：

```text
Close blocked while publish waited for a confirmation
```

随后拆分锁职责：

- `mu` 只保护连接、publisher、监听通道和关闭状态；
- `publishMu` 串行化同一 publisher channel 上的声明与发布；
- 发布在 `mu` 下取得 channel 及监听通道快照后立即释放状态锁，再等待 confirm；
- `Close` 在状态锁下先标记关闭并摘除资源，再在锁外关闭 channel/connection；关闭通知可唤醒待确认发布；
- 发布失败仅当 publisher 身份仍匹配时才失效并关闭该 channel，避免关闭后的清理影响其他状态。

GREEN 后补充并通过：

- pending publish 与 `Close` 并发时，`Close` 有界返回且发布被关闭通知唤醒；
- 已确认的发布后重复 `Close` 安全，之后的发布被拒绝；
- 两个并发发布中第一个 NACK 后，第二个在串行锁下使用重建的 channel 成功发布。

### P1 验证

```text
go test ./internal/platform/messaging -run 'EventTransport|Close|Publish' -count=1
PASS

go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
PASS
```

## 终审修订：attempt 关联与超时确认收敛

终审反馈经代码与 amqp091-go 语义验证后确认：

- 稳定的 Outbox `envelope.ID` 会在重试中复用为 `MessageId`，仅以该值关联 `basic.return` 会将旧 attempt 的迟到 return 错误归属给新 attempt；
- `PublishWithDeferredConfirmWithContext` 在已经交付 deferred confirmation 后，调用方的 context 超时只停止本层等待，不能取消库内 confirmation。因此继续复用同一 connection 会累积未决 confirmation。

### TDD RED/GREEN

新增 `TestEventTransportIgnoresLateReturnFromEarlierAttemptWithSameMessageID`：同一 `MessageId` 的 attempt-1 超时后，向共享 return stream 注入其迟到 return；attempt-2 已 ACK。修复前得到真实 RED：

```text
publish event returned: code=312 text=NO_ROUTE
```

修复后，每次 transport 发布都保留稳定 `MessageId=envelope.ID`，同时添加唯一 `x-sparkops-publish-attempt` header。return 必须同时匹配 `MessageId` 与该 attempt header；不匹配的迟到 return 在等待及 ACK 后 drain 时都会被忽略。

对于确认超时/取消，`publishEventConfirmed` 标记 confirmation 已被放弃；transport 从状态中摘除 publisher 和当前 connection，并同步调用 `Connection.CloseDeadline(now + 1s)`。这有界地关闭该 connection 的全部 channels、Notify listeners 与 amqp091-go 内部 deferred confirmations，且不调用可能在网络分区下阻塞的 `Channel.Close`。生产 constructor 保存 dial factory，下一次发布透明地建立新的 connection 和确认 channel。

### 设计权衡

一次确认超时会退休整个 transport connection，而不是尝试取消单个 deferred confirmation（amqp091-go 未提供该能力）。因此后续发布会重连并可继续；该 connection 上已注册的消费者会随 connection 关闭，需要其上层生命周期负责重新订阅。该选择避免了无法证明有界的 deferred confirmation 与 listener 累积，也不引入无限后台关闭 goroutine。

新增并通过：

- `TestEventTransportRetiresConfirmationSessionsAfterTimeout`：连续三次超时均恰好对旧 session 调用一次 `CloseDeadline`，随后新 session 发布成功；
- `TestEventTransportKeepsEventIDAndGeneratesUniqueAttemptHeaders`：相同事件 UUID 的两次发布保持相同 `MessageId`、但 attempt header 不同；
- `TestEventTransportIgnoresLateReturnFromEarlierAttemptWithSameMessageID`：旧 attempt return 不污染新 attempt。

```text
go test ./internal/platform/messaging -run 'EventTransport|Close|Publish' -count=1
PASS

go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
PASS
```

## P1 最终修订：消息级失败的资源上界

复审发现上一版把每个发布错误都传给 `invalidatePublisher`。由于为避免同步 `Channel.Close` 阻塞而只脱离本地引用，连续 NACK、mandatory return 或 context timeout 会留下健康旧 channel 及其 Notify listeners，并为下一次发布创建新 channel，形成无界累积。

### TDD 记录

先新增 `TestEventTransportReusesHealthyPublisherAfterMessageFailures`：同一 transport 连续三个 NACK，只提供一个健康 fake channel，断言 `connection.Channel()` 与两类 Notify listener 各只调用一次。修复前得到真实 RED：

```text
open event publisher channel: no event channels
```

这证明每次 NACK 都错误地使 publisher 脱离状态。随后调整失效规则：

- NACK、匹配的 mandatory return、context timeout/cancel，以及 Publish 返回但 channel 仍健康时，只返回消息级错误并复用 publisher；
- `amqp.Return.MessageId` 必须等于当前 envelope ID 才能使本次发布失败；不匹配的迟到 return 在等待及 ACK 后 drain 时均被忽略；
- 仅 `NotifyClose`、`IsClosed()`，或 Publish 返回错误且 `IsClosed()` 为真时，才从状态中失效；
- `eventChannel` 增加最小 `IsClosed()`；真实适配调用 `amqp.Channel.IsClosed()`；
- 未恢复同步 `Channel.Close`，全局 `closing` 信号与 `Connection.CloseDeadline(1s)` 方案保持不变。

### 资源上界验证

新增并通过：

- `TestEventTransportReusesHealthyPublisherAfterMessageFailures`：重复 NACK 不增加 channel/listener；
- `TestEventTransportReusesHealthyPublisherAfterReturnAndTimeout`：重复匹配 return 与 timeout 不增加 channel/listener；
- `TestEventTransportReusesHealthyPublisherAfterPublishError`：健康 channel 上的 Publish 错误不重建；
- `TestEventTransportIgnoresLateReturnForDifferentMessageID`：前一条消息的迟到 return 不污染当前确认；
- `TestEventTransportRecreatesPublisherAfterNotifyClose`：收到关闭通知后恰好重建一次。

```text
go test ./internal/platform/messaging -run 'EventTransport|Close|Publish' -count=1
PASS

go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
PASS
```

额外尝试 `go test -race` 未能启动：本机 C 编译器报 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。这属于本地工具链限制；上述非 race 单元测试均实际通过。

## P1 修订：真实 AMQP 同步关闭语义

后续复审指出上一版仍在全局关闭路径调用 `amqp.Channel.Close()`。amqp091-go 的该方法同步等待 broker `channel.close-ok`；网络分区时它可能一直阻塞，且 `NotifyClose` 不保证会及时唤醒 pending confirm。

### TDD 记录

新增 `TestEventTransportCloseDoesNotWaitForChannelClose`：fake publisher channel 的 `Close` 永不返回且不发送 `NotifyClose`，pending publish 无 confirm。修复前得到真实 RED：

```text
Close waited for synchronous channel close
```

修复后，transport 建立进程内 `closing` 信号。`Close` 在状态锁内原子标记关闭并关闭该信号；`publishEventConfirmed` 同时等待该信号、confirm、mandatory return、broker NotifyClose 与 context。因而 pending publish 无需等待 broker 即返回 `transport closed`。

全局 `Close` 不再逐个调用 `Channel.Close`。它仅摘除本地状态后调用 `amqp.Connection.CloseDeadline(now + 1s)`；amqp091-go 明确保证该 API 将 deadline 传给底层 socket，并在返回后关闭 connection、channels、Notify listeners 和消费者，即使 I/O timeout 也把 connection 视为关闭。抽象 `eventConnection` 同步改为 `CloseDeadline(time.Time)`，fake 记录该调用。

`invalidatePublisher` 也不再同步调用 `Channel.Close`：仅在 publisher 身份匹配时脱离本地状态，旧 channel 由下一次 transport `CloseDeadline` 收敛。这样 NACK、return 或 channel failure 不会在网络失联时阻塞发布返回，也没有额外的无限后台关闭 goroutine。

新增 `TestEventTransportFailedPublishDoesNotWaitForChannelClose`，验证 NACK 发布在 fake `Channel.Close` 永不返回时仍有界返回，且未调用同步 channel close。

### P1 修订验证

```text
go test ./internal/platform/messaging -run 'EventTransport|Close|Publish' -count=1
PASS

go test ./internal/platform/messaging -run 'EventTopology|EventTransport' -count=1
PASS
```
