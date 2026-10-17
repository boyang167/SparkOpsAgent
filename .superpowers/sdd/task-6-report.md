# Task 6：Operation Outbox Runtime

日期：2026-10-17
起始 HEAD：`ecc2bb96d4944b74e8e6f5610b7eff55e9edcadd`

## 范围

本任务只实现 Operation Outbox runtime：

- 事务内 `OperationWriter.Schedule`，事件元数据 owner 校验和命令追踪 ID；
- 稳定幂等键去重，重复插入使用 `ON CONFLICT DO NOTHING`，不破坏 PostgreSQL 事务；
- 线程安全的 operation handler 注册表，拒绝重复类型；
- owner/lease/RowsAffected 防护下的领取、完成、失败和重排；
- 未知 operation type、非法 JSON 和永久 handler 错误进入 `failed`；
- 临时错误按 5s、30s、5m、30m、之后持续 30m 退避；
- 多 worker 并发执行以及停止新 claim、等待在途操作的优雅关闭；
- 复用 Task 5 的 Operation backlog/oldest Prometheus collectors；
- 外部执行成功、数据库 `MarkCompleted` 失败后的租约恢复与稳定幂等键重执行窗口。

未实现 Task 7 factory/bootstrap 接线，也未实现 Task 10 业务 operation handlers。

## TDD：RED

首次创建 `operation_test.go` 后运行：

```text
cd backend
go test ./internal/platform/messaging -run Operation -count=1
```

结果为预期 RED（exit 1）：缺少 `OperationStats`、`NewOperationWriter`、`OperationRequest`、`OperationRegistry`、`OperationDispatcher` 等 Task 6 符号。

审查中另完成一个独立 RED/GREEN：

```text
go test ./internal/platform/messaging/persistence \
  -run TestOperationInsertTreatsDuplicateIdempotencyKeyAsSuccess -count=1
```

RED：重复幂等键返回错误。该实现即使在 writer 层吞掉错误，也会使 PostgreSQL 当前事务进入 aborted 状态。
GREEN：持久层改为针对 `idempotency_key` 的 `ON CONFLICT DO NOTHING`，重复调度成功返回，且同一事务可继续插入下一条 operation。

## GREEN 与行为证据

```text
go test ./internal/platform/messaging/... -run Operation -count=1
```

结果：PASS。

覆盖：

- writer 拒绝事务外 schedule；
- 重复幂等键视为成功；
- 事件驱动 owner 必须与 subscriber 一致，event ID 来自事件元数据；
- 命令驱动 operation ID 同时作为 trace event ID；
- registry 重复注册失败，未知类型永久失败；
- JSON 损坏和显式永久错误标记 `failed`，保留 `last_error`；
- 临时错误按固定层级 reschedule；
- 成功后仅由匹配 owner 的 lease 标记 completed；
- 外部成功、完成状态写库失败时保留 executing，租约到期后用同一幂等键重执行；
- 两个 worker 可并发执行，`Close` 等待在途 handler 完成；
- stats 查询失败不会阻止已领取 operation 执行；
- backlog 和 oldest gauges 从 pending/executing 状态刷新。

## 设计取舍

- Task 3 的 `OperationStore` 已经强制所有状态更新匹配 `id + owner + executing` 并检查 `RowsAffected == 1`；dispatcher 复用该边界，不复制租约逻辑。
- handler 接收原始 JSON 和稳定 `IdempotencyKey`。支持原生幂等的外部 API 应直接透传该键；不支持幂等的 API 仍需由 Task 10 handler 在调用前查询状态并对账。
- 事件驱动幂等键严格校验为 `<event-id>:<subscriber>:<operation-type>:<target-id>`。
- 命令驱动 operation 的 trace event ID 是新 operation UUID，但调用方幂等键必须保持业务稳定，因此不强制幂等键首段等于该新 trace ID；否则同一命令重试会生成不同键而失去去重能力。
- 指标采集错误会由 `DispatchOnce` 返回用于诊断，但不会跳过 claim 或 handler 执行。

## 验证

```text
go test ./internal/platform/messaging/... -count=1
go test ./internal/platform/postgres ./internal/platform/messaging/... -count=1
```

结果：PASS。

```text
go test -race ./internal/platform/messaging -run Operation -count=1
```

结果：未执行到测试，Windows 本机 C 编译器不支持 Go race 所需 64-bit 模式：
`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。

IDE lint：Task 6 修改文件无诊断。

## 提交范围

- `backend/internal/platform/messaging/operation.go`
- `backend/internal/platform/messaging/operation_dispatcher.go`
- `backend/internal/platform/messaging/operation_test.go`
- `backend/internal/platform/messaging/persistence/operation_outbox.go`
- `backend/internal/platform/messaging/persistence/operation_outbox_test.go`
- `.superpowers/sdd/task-6-report.md`

持久层两个文件属于 Task 6 必需增量：提供原子幂等插入以及 Operation backlog/oldest stats。未修改或暂存 frontend、`.superpowers/sdd/progress.md`、`.superpowers/sdd/task-5-report.md` 和 logs。

## 未决风险

- `-race` 受本机工具链阻塞，需在具备 64-bit C 编译器的 CI/Linux 环境补跑。
- 真实 PostgreSQL 多实例 lease reclaim 与外部系统故障窗口的端到端验证属于 Task 11；本任务通过 store 合约、SQLite 持久层测试和并发 runtime 测试覆盖。
- factory/bootstrap 生命周期接线属于 Task 7；业务 handler 的原生幂等、状态查询和 Reconciler 属于 Task 10。
