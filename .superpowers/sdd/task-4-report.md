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
- NACK、mandatory return、channel close、上下文超时和缺少确认均被视为失败；失败时丢弃 publisher channel，下一次发布在互斥锁内重建；发布、声明及关闭均受同一互斥锁保护。

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
