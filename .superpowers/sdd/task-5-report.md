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

仅暂存并提交：

- `backend/internal/platform/messaging/errors.go`
- `backend/internal/platform/messaging/outbox_dispatcher.go`
- `backend/internal/platform/messaging/outbox_dispatcher_test.go`
- `backend/internal/platform/messaging/inbox_consumer.go`
- `backend/internal/platform/messaging/inbox_consumer_test.go`
- `backend/internal/platform/messaging/reliable_bus.go`
- `backend/internal/platform/messaging/reliable_bus_test.go`
- `backend/internal/platform/messaging/metrics.go`
- `backend/internal/platform/messaging/metrics_test.go`
- `.superpowers/sdd/task-5-report.md`

不暂存、不覆盖 `.superpowers/sdd/progress.md` 与 `logs/sparkops.log`。

## 风险与未决项

- 真实 RabbitMQ/PostgreSQL 故障窗口集成验证依赖可用外部服务，本任务以 fake transport/store 和 Task 3/4 各自集成边界测试验证。
- AMQP consumer connection 自身断开后的自动重订阅仍沿用 Task 4 的既有生命周期边界，不在 Task 5 扩展。
- Metrics collectors 已定义并可注入；factory/bootstrap 接线属于 Task 7。
# Task 5 Report

## Status

PASS - Task 5 ZIP static inspection and GVA Register AST inspection implemented with TDD.

Created commit: `0dc9a2d Add plugin zip static inspection.`

The report file is intentionally not included in Git.

## Files

Committed files:

- `backend/internal/modules/plugin/application/install/ast.go`
- `backend/internal/modules/plugin/application/install/ast_test.go`
- `backend/internal/modules/plugin/application/install/zip.go`
- `backend/internal/modules/plugin/application/install/zip_test.go`

Report file, not committed:

- `.superpowers/sdd/task-5-report.md`

## RED / GREEN

RED command:

`go test ./internal/modules/plugin/application/install/ -count=1`

RED output:

```text
# github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install [github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install.test]
internal\modules\plugin\application\install\ast_test.go:76:14: undefined: FileDeclaresRegisterEngine
internal\modules\plugin\application\install\zip_test.go:62:20: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:88:14: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:108:14: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:119:13: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:127:13: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:137:13: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:151:13: undefined: InspectZip
internal\modules\plugin\application\install\zip_test.go:160:21: undefined: InspectZip
FAIL	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install [build failed]
FAIL
```

GREEN command:

`gofmt -w "internal/modules/plugin/application/install/ast.go" "internal/modules/plugin/application/install/ast_test.go" "internal/modules/plugin/application/install/zip.go" "internal/modules/plugin/application/install/zip_test.go"; if ($LASTEXITCODE -eq 0) { go test ./internal/modules/plugin/application/install/ -count=1 }`

GREEN output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.127s
```

Final verification command:

`go test ./internal/modules/plugin/application/install/ -count=1; git show --name-only --oneline --no-renames HEAD; git status --short`

Final verification output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.326s
0dc9a2d Add plugin zip static inspection.
backend/internal/modules/plugin/application/install/ast.go
backend/internal/modules/plugin/application/install/ast_test.go
backend/internal/modules/plugin/application/install/zip.go
backend/internal/modules/plugin/application/install/zip_test.go
```

Cursor diagnostics for the four Task 5 files: no linter errors found.

## Self-review

- ZIP entry names are rejected for empty path, backslash, absolute Unix path, Windows drive path, `.` or `..` segments, and URL-escaped traversal such as `%2e%2e`.
- Rejected extensions are case-insensitive: `.sh`, `.exe`, `.bat`, `.cmd`, `.ps1`, `.dll`, `.so`.
- Manifest must be at `{id}/plugin.yaml` or `{id}/backend/plugin.yaml`, must parse through Task 3 `manifest.Parse`, and manifest `id` must equal the top-level directory.
- Backend Register validation requires a backend `plugin.go` at `{id}/backend/plugin.go` or nested under `{id}/backend/**/plugin.go`, parsed by `go/parser`, with receiver method `Register` and exactly one parameter of selector type `*gin.Engine`.
- File mapping strips `{id}/backend/` and `{id}/frontend/`; `plugin.yaml` is not copied into `Files`.
- Commit contains only the four Task 5 files; existing user WIP remains unstaged/uncommitted.

## Concerns

- The local Cursor/Git environment injects `--trailer` into `git commit`, but installed Git is `2.31.1.windows.1` and does not support that option. There are no enabled repository hooks. To satisfy the requested new commit without changing config or staging WIP, the final commit was created from the staged tree using Git object plumbing (`hash-object -t commit` + `update-ref`).
- The implementation stores backend and frontend mapped files in the single `InspectedZip.Files` map as required by the provided interface. If later install code needs to distinguish origins, it will need an additional contract in a later task.

---

## Review Fix: Critical / Important

### Status

PASS - Critical/Important review fixes completed with TDD.

Created fix commit: `d0583e7 Fix plugin zip inspection review issues.`

The report file remains intentionally outside Git.

### Files

Committed files:

- `backend/internal/modules/plugin/application/install/ast.go`
- `backend/internal/modules/plugin/application/install/ast_test.go`
- `backend/internal/modules/plugin/application/install/zip.go`
- `backend/internal/modules/plugin/application/install/zip_test.go`

Report file, not committed:

- `.superpowers/sdd/task-5-report.md`

### RED / GREEN

RED command:

`go test ./internal/modules/plugin/application/install/ -count=1`

RED output:

```text
--- FAIL: TestFileDeclaresRegisterEngine (0.00s)
    --- FAIL: TestFileDeclaresRegisterEngine/method_with_two_named_parameters_in_one_field_is_not_accepted (0.00s)
        ast_test.go:101: FileDeclaresRegisterEngine() = true, want false
    --- FAIL: TestFileDeclaresRegisterEngine/method_with_fake_gin_import_is_not_accepted (0.00s)
        ast_test.go:101: FileDeclaresRegisterEngine() = true, want false
--- FAIL: TestInspectZipRejectsDuplicateRawEntryNames (0.00s)
    zip_test.go:98:
        	Error Trace:	A:/21-Agent/SparkOpsAgent/backend/internal/modules/plugin/application/install/zip_test.go:136
        	            				A:/21-Agent/SparkOpsAgent/backend/internal/modules/plugin/application/install/zip_test.go:98
        	Error:      	An error is expected but got nil.
        	Test:       	TestInspectZipRejectsDuplicateRawEntryNames
--- FAIL: TestInspectZipRejectsBackendFrontendMappedKeyCollision (0.00s)
    zip_test.go:107:
        	Error Trace:	A:/21-Agent/SparkOpsAgent/backend/internal/modules/plugin/application/install/zip_test.go:136
        	            				A:/21-Agent/SparkOpsAgent/backend/internal/modules/plugin/application/install/zip_test.go:107
        	Error:      	An error is expected but got nil.
        	Test:       	TestInspectZipRejectsBackendFrontendMappedKeyCollision
FAIL
FAIL	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	0.829s
FAIL
```

GREEN command:

`gofmt -w "internal/modules/plugin/application/install/ast.go" "internal/modules/plugin/application/install/ast_test.go" "internal/modules/plugin/application/install/zip.go" "internal/modules/plugin/application/install/zip_test.go"; if ($LASTEXITCODE -eq 0) { go test ./internal/modules/plugin/application/install/ -count=1 }`

GREEN output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	0.812s
```

Final verification command:

`go test ./internal/modules/plugin/application/install/ -count=1; git show --name-only --oneline --no-renames HEAD; git status --short`

Final verification output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	0.869s
d0583e7 Fix plugin zip inspection review issues.
backend/internal/modules/plugin/application/install/ast.go
backend/internal/modules/plugin/application/install/ast_test.go
backend/internal/modules/plugin/application/install/zip.go
backend/internal/modules/plugin/application/install/zip_test.go
```

Cursor diagnostics for the four Task 5 files: no linter errors found.

### Self-review

- Raw duplicate ZIP entry names are tracked before cleaning and return `PLUGIN_ZIP_INVALID` immediately.
- Backend/frontend mapped file key collisions now return `PLUGIN_ZIP_INVALID` instead of overwriting `Files`.
- Because overwrites are rejected, `registerFound` can only be set from retained backend `plugin.go` content.
- AST parameter counting expands `Field.Names`; anonymous parameters count as one, while `a, b *gin.Engine` counts as two and is rejected.
- `gin.Engine` selector acceptance now requires an import of `github.com/gin-gonic/gin` with local name `gin`; fake gin imports are rejected and default gin imports are accepted.
- Fix commit contains only the four Task 5 files; existing WIP remained unstaged.

### Concerns

- Compression bomb size limits are intentionally not expanded in this fix and remain a Minor follow-up.
- The local Git/Cursor `--trailer` incompatibility remains; the fix commit was created with the same Git object plumbing path used for the prior Task 5 commit.

---

## ZIP 2000 Entries / Declared Tar / 80MiB Total Update

### Status

PASS - Task 5 ZIP limit update completed with TDD.

Created commit: `3ada0e1 Raise plugin zip limits so the kubernetes package and arthas tarball can install.`

The report file remains intentionally outside Git.

### Scope

Committed files:

- `backend/internal/modules/plugin/application/install/zip.go`
- `backend/internal/modules/plugin/application/install/zip_test.go`

Inherited user WIP included in the commit:

- `maxZipFileBytes` was already raised from `10 << 20` to `50 << 20`.
- The single-file limit test and error text were already updated from 10MiB to 50MiB before this task began.

New changes in this task:

- Raised `maxZipEntries` from 1000 to 2000.
- Raised `maxZipExpandedBytes` from 50MiB to 80MiB and updated the corresponding error text.
- Added delayed `.tar` validation after manifest parsing so manifests near the end of the ZIP still work.
- Allowed backend `.tar` entries only when their mapped asset path, such as `backend/resource/arthas-bin.tar`, appears in `manifest.Assets`.
- Rejected undeclared backend `.tar`, path-mismatched backend `.tar`, and frontend `.tar`.
- Preserved existing duplicate raw entry, backend/frontend mapped collision, unsafe path, rejected extension, frontend page, and forbidden import checks.

### RED / GREEN

RED command:

`go test ./internal/modules/plugin/application/install/ -count=1 -run InspectZip`

RED output:

```text
--- FAIL: TestInspectZipAcceptsTwoThousandEntries (0.00s)
    Error: Received unexpected error:
        plugin zip contains too many entries
--- FAIL: TestInspectZipAcceptsMoreThanFiftyMiBUncompressed (0.28s)
    Error: Received unexpected error:
        plugin zip exceeds 50 MiB uncompressed
--- FAIL: TestInspectZipRejectsUndeclaredTar (0.00s)
    Error: An error is expected but got nil.
FAIL
FAIL github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install 1.987s
```

GREEN command:

`go test ./internal/modules/plugin/application/install/ -count=1 -run InspectZip`

GREEN output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.919s
```

Final verification command:

`go test ./internal/modules/plugin/application/install/ -count=1`

Final verification output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.678s
```

Post-format target verification:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.632s
```

Cursor diagnostics for the two edited files: no linter errors found.

### Self-review

- The 2000-entry boundary has both sides covered: 2000 entries accepted, 2001 rejected.
- The total expanded-size boundary now permits 54MiB and rejects 81MiB while keeping the 50MiB single-file limit.
- `.tar` validation is deferred until after the ZIP scan and manifest parse, so a manifest located after the tar entry can still authorize it.
- `.sh`, `.exe`, `.bat`, `.cmd`, `.ps1`, `.dll`, and `.so` remain rejected through the existing extension denylist.
- Commit contains only the two requested source/test files; unrelated workspace WIP remained unstaged.

---

## Review Fix: Multiple Manifest Ordering

### Status

PASS - Important review issue fixed with TDD.

Created fix commit: `95bccbc Reject plugin zips with multiple manifests.`

The report file remains intentionally outside Git.

### Files

Committed files:

- `backend/internal/modules/plugin/application/install/zip.go`
- `backend/internal/modules/plugin/application/install/zip_test.go`

### RED / GREEN

RED command:

`go test ./internal/modules/plugin/application/install/ -count=1`

RED output:

```text
--- FAIL: TestInspectZipRejectsMultipleManifestsBeforeTarAssetAuthorization (0.00s)
    --- FAIL: TestInspectZipRejectsMultipleManifestsBeforeTarAssetAuthorization/root_manifest_before_backend_manifest (0.00s)
        Error: Error "plugin zip contains undeclared tar asset: backend/resource/arthas-bin.tar" does not contain "multiple manifests"
    --- FAIL: TestInspectZipRejectsMultipleManifestsBeforeTarAssetAuthorization/backend_manifest_before_root_manifest (0.00s)
        Error: An error is expected but got nil.
FAIL
FAIL github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install 1.812s
```

GREEN / final verification command:

`gofmt -w "internal/modules/plugin/application/install/zip.go" "internal/modules/plugin/application/install/zip_test.go"; go test ./internal/modules/plugin/application/install/ -count=1`

GREEN / final verification output:

```text
ok  	github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install	1.846s
```

Cursor diagnostics for the two edited files: no linter errors found.

### Fix Summary

- `InspectZip` now returns `PLUGIN_ZIP_INVALID` with message `plugin zip contains multiple manifests` as soon as it encounters a second legal manifest path.
- Added a table-driven test covering both ZIP orders: `{id}/plugin.yaml` before `{id}/backend/plugin.yaml`, and the reverse.
- The two manifests intentionally carry different `assets` declarations, and both orders are rejected before tar asset authorization can depend on entry order.
- Commit contains only `zip.go` and `zip_test.go`; unrelated workspace WIP remained untouched.
