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
- 命令驱动使用生成的 operation UUID 作为 trace event ID；幂等键首段仅保留稳定 command 标识；
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
- 命令驱动与事件驱动都严格校验幂等键前三段，第四段保留完整 target；命令使用本次 operation UUID 作为 trace event ID，稳定 command 标识只保留在幂等键中。
- 指标采集错误会由 `DispatchOnce` 返回用于诊断，但不会跳过 claim 或 handler 执行。

## 审查修复：RED/GREEN

基于 commit `d7d6e5078b561e95edeabd00a6e8723e9cf1238b`，先补审查测试并运行：

```text
go test ./internal/platform/messaging/... -run 'Operation|Metrics' -count=1
```

RED 证据：

- `OperationDispatcherError` collector 和 `operationOwnerMaxLength` 尚不存在，messaging 测试编译失败；
- 四组不同意图复用同一幂等键的持久层测试均期望 `ErrOperationIdempotencyConflict`，实际得到 `nil`。

GREEN 修复：

1. runtime 为每个 worker 派生稳定、有界且唯一的 `<instance-owner>.<runtime-uuid>.worker-N`；owner 过长时对完整三元组哈希。claim 与后续状态更新全程使用同一 worker owner。
2. runtime worker 固定单次 claim 1 条；`DispatchOnce` 继续保留显式批处理测试接口。
3. `Insert` 在 `ON CONFLICT DO NOTHING` 后读取既有记录，仅当稳定 event identity、owner、operation type 和 JSON payload 语义相同时返回成功，否则返回 `ErrOperationIdempotencyConflict`。命令随机 trace UUID 不参与稳定 identity 比较。
4. operation key 使用 `SplitN(..., 4)`，前三段非空并校验 owner/type，第四段 target 非空且允许 URI 冒号；事件模式额外校验 metadata event ID。
5. 新增 `sparkops_operation_dispatcher_error_total{stage}`，stage 仅限 `stats/claim/handler/mark/reschedule/lease_lost`。错误在来源处只记录一次，stats 失败不阻断 claim/执行。
6. 增加 parent cancel 与并发、重复 `Close` 覆盖。

关键并发用例连续 50 次通过：

```text
go test ./internal/platform/messaging \
  -run 'TestOperationDispatcher(WorkersUseUniqueBoundedOwnersAndFenceExpiredLease|RuntimeWorkersClaimOnlyOneRecord|ParentCancelAndRepeatedClose|RecordsBoundedErrorStagesOnce|BackgroundLoopObservesDispatchErrors)' \
  -count=50
```

持久层幂等与 lease 用例连续 20 次通过：

```text
go test ./internal/platform/messaging/persistence \
  -run 'TestOperationInsert(TreatsDuplicateIdempotencyKeyAsSuccess|RejectsIdempotencyKeyForDifferentIntent)|TestOperationUpdatesRejectLostLease' \
  -count=20
```

PostgreSQL build-tag 用例已新增；本机未设置 `DATABASE_DSN`，运行时明确 SKIP，未伪报为真实 PostgreSQL PASS。

## 第二轮复审修复

基于 commit `f16dd4b51db919ff9bc75c900cf6fadee8b683be` 补测试后，RED 明确复现：

- command `event_id` 实际为非 UUID 的 key 首段；
- URI target 因全量 `Split` 被拒绝；
- 两个同 `config.Owner` dispatcher 生成相同 worker owner，旧实例可覆盖新 lease；
- parent cancel 直接取消在途 handler；
- `float64` 将 `9007199254740992` 与 `9007199254740993` 误判相等。

GREEN 实现：

- 每个 dispatcher 构造时生成随机 runtime UUID；worker owner 哈希输入包含完整 instance owner、runtime UUID、worker index，最终不超过 160 字符；
- claim 使用 lifecycle context；claim 成功后使用 `context.WithoutCancel` 执行 handler 和 Mark/Reschedule，保留 values 并允许 drain；
- command operation row ID 与 trace event ID 使用同一随机 UUID；稳定 command ID 只在 idempotency key 首段；
- payload decoder 启用 `UseNumber`，递归结构比较通过 `big.Rat` 精确比较 JSON 数值；
- command 重试的随机 trace UUID 不作为稳定冲突字段；事件模式仍严格比较 event ID。

新增关键测试连续 50 次通过：

```text
go test ./internal/platform/messaging \
  -run 'TestOperation(WriterCommandUsesOperationUUIDAsTraceEventID|WriterAllowsColonInTargetID|DispatcherInstancesWithSameConfigUseDifferentWorkerOwners|DispatcherOldInstanceCannotOverwriteRecoveredLease|DispatcherParentCancelAndRepeatedClose)' \
  -count=50

go test ./internal/platform/messaging/persistence \
  -run 'TestOperation(InsertTreatsDuplicateIdempotencyKeyAsSuccess|InsertRejectsIdempotencyKeyForDifferentIntent|PayloadComparisonUsesExactJSONNumberSemantics)' \
  -count=50
```

PostgreSQL integration 编译通过：

```text
go test -tags=integration ./internal/platform/messaging/persistence -run '^$' -count=1
```

## 第三轮复审修复

基于 commit `ebbfb79bd057581b9ea7e0989b9adfb4975b9c9b` 增加确定性阻塞测试：

- RED：store 的 `Claim` 阻塞等待 `ctx.Done()` 时，`Close` 仅关闭 stop channel，100ms 内不能返回；
- RED：store 的 `Stats` 阻塞时，`Close` 不能取消它；强制释放后还会继续进入 Claim，暴露 Close race window。

GREEN：

- `Start` 创建继承 parent values/deadline 的独立 claim lifecycle context；
- parent cancel 或 `Close` 首先取消 claim context，以中断 Stats/Claim，再等待 workers；
- cancel func 只在互斥区内获取和清空，重复/并发 Close 不会 nil 调用或 double-close；
- Stats 返回后在 Claim 前再次检查 claim context，Close 已开始时不再领取；
- Claim 已成功返回的 records 使用 `context.WithoutCancel` 派生 drain context，handler 与 Mark/Reschedule 不受 parent cancel/Close 中断。

目标生命周期测试：

```text
go test ./internal/platform/messaging \
  -run 'TestOperationDispatcher(CloseCancelsBlockedClaim|ParentCancelCancelsBlockedClaim|CloseDuringStatsDoesNotStartClaim|ParentCancelAndRepeatedClose)' \
  -count=50
```

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
- `backend/internal/platform/messaging/metrics.go`
- `backend/internal/platform/messaging/metrics_test.go`
- `backend/internal/platform/messaging/persistence/operation_outbox.go`
- `backend/internal/platform/messaging/persistence/operation_outbox_test.go`
- `backend/internal/platform/messaging/persistence/claim_postgres_test.go`
- `.superpowers/sdd/task-6-report.md`

持久层三个文件属于 Task 6 必需增量：提供原子幂等插入、Operation backlog/oldest stats 及 PostgreSQL 集成覆盖。未修改或暂存 frontend、`.superpowers/sdd/progress.md`、`.superpowers/sdd/task-5-report.md` 和 logs。

## 未决风险

- `-race` 受本机工具链阻塞，需在具备 64-bit C 编译器的 CI/Linux 环境补跑。
- 真实 PostgreSQL 多实例 lease reclaim 与外部系统故障窗口的端到端验证属于 Task 11；本任务通过 store 合约、SQLite 持久层测试和并发 runtime 测试覆盖。
- factory/bootstrap 生命周期接线属于 Task 7；业务 handler 的原生幂等、状态查询和 Reconciler 属于 Task 10。
