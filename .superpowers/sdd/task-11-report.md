# Task 11 实施报告

## 范围与提交

- 基线：`185d2d3339537cc8f266527a461117e39a54f0aa`
- 实现提交：`0441bb3ae857c9a719a654a2fc80a5e9d3b4be6a`
- 提交信息：`feat(filesystem-ui): share files with governed audiences`
- 严格限定 Task 11：未实现 Task 12 租户治理页、审批管理页或采纳流程。

## TDD 记录

- RED：聚焦测试首次运行失败，缺少 `share-file-form`、`ShareFileModal` 和“共享与权限”tab。
- RED：后端治理元数据测试首次编译失败，缺少 grants/share-requests 读取 handler。
- GREEN：聚焦测试 3 files / 17 tests 通过。
- 新增 FileActionMenu effective SHARE 与 shared projection reshare 防御测试。

## 实现摘要

- 两步共享弹窗：先选权威受众，再配置权限、目录继承和到期时间；默认 `READ + DOWNLOAD`，目录默认继承。
- 受众仅来自当前 Organization、当前 Workspace 和真实成员目录。组织成员数无法证明时禁用 Organization；成员目录失败时禁用 Workspace，但保留可证明的当前用户选项。
- `RESTRICTED` 禁止 Organization；Organization 动作集合排除 `DELETE / SHARE / ADMIN`。
- 提交复用 Task 8 governed grant/share-request clients；同一次逻辑提交稳定复用 Idempotency-Key，成功后刷新；支持 ACTIVE 与 PENDING_APPROVAL 成功态及 403/409/422 可行动错误。
- 文件操作入口由节点 `effective-access` 的 `SHARE` 决定；shared projection 仅在 `effective_actions` 含 `SHARE` 时允许 reshare，handler 再次防御。
- Node contract/真实 HTTP view 补充持久化 classification，不再由前端硬编码 INTERNAL；缺失 classification 安全按 RESTRICTED 处理。
- Drawer 新增“共享与权限”tab，分离加载、空、错误和受限态，展示 effective actions、直接/继承来源、受众类型、到期、待审批和显式 DENY。
- 最小补齐受管理员授权保护的 GET grants/share-requests 后端、OpenAPI、upstream、BFF、Next route 与 web-sdk 链路；普通只读用户只能读取自己的 effective result，无法枚举 principal IDs。
- Drawer 切换 node/scope 时通过 AbortController 和 scope key 忽略 stale response。

## 验证

- `pnpm vitest run apps/web/src/modules/filesystem`：25 files / 182 tests，通过。
- `pnpm vitest run packages/contracts/src/filesystem/index.test.ts packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts apps/web/src/server/loaders/filesystem.test.ts`：6 files / 57 tests，通过。
- `pnpm typecheck`：6 packages，通过。
- `go test ./internal/modules/filesystem/... ./api/openapi -count=1`：通过。
- IDE diagnostics：无新增 linter errors。
- `git diff --check`：通过。
- `.\scripts\dev-restart.ps1 -Backend`：通过，后端已在 `:8080` 以新 PID 启动。

## 未验证项

- 未执行真实登录态浏览器端到端矩阵。
- 未连接生产身份目录或外部 PostgreSQL；成员选项与计数由现有 Workspace members API 提供。
- Organization 级成员目录当前没有可复用的权威计数 API，因此 Organization 受众在第一版明确禁用，没有估算人数或伪造主体。
# Task 11 Report: Release Blocker Closure

## Scope

- Initial release-blocker closure was committed as `cebbc29269c2123474ddf2eab23d3e44f5d77e69`.
- This follow-up closes the topology-normalization, Helm PodSpec, and DLQ-attempt review findings from that HEAD.
- The commit is limited to Task 11 messaging, DLQ, migration, rollout, CI, tests, and this report.
- Unrelated plugin, frontend, progress, Task 5 report, log, and business-navigation work remains uncommitted and untouched.

## Release blockers

### 1. Same-runtime RabbitMQ recovery

- The production event transport now owns separate publisher and consumer connections.
- Publisher channel/connection failure retires stale state and redials with bounded exponential backoff.
- Consumer connection loss redials, redeclares the exchange plus all main/retry/DLQ topology, and restores every registered subscription and handler.
- Registration/reconnect state is lock-coordinated; stale reconnect signals are ignored; duplicate queue consumers are rejected; `Close` cancels dial/backoff and waits for the reconnect worker.
- Real RabbitMQ coverage closes both live connections on one transport and proves two subscribers resume on that same instance.

### 2. Queue-targeted DLQ replay

- Replay mandatory-publishes to the default exchange with the selected subscriber main queue as the exact routing key; it does not topic-broadcast.
- Original body, `MessageId`, AMQP properties, and headers including `x-sparkops-attempts` are retained.
- Queue parsing is delegated to the exported narrow messaging validator `ValidateEventDeadLetterQueue`; it uses `topologyFor` and `normalizeQueueNamePart` rather than duplicating queue rules in the CLI.
- Queue subscriber/event/version identity, exact `.dlq` suffix, envelope structure, and production codec registration are validated before publish.
- Unit coverage includes valid production codecs whose event names contain underscores: `agent_runtime`, `workspace_app`, and `product_workflow`, plus mismatched event/version rejection.
- A DLQ delivery is ACKed only after publisher confirm and no mandatory return.
- Real two-subscriber coverage proves no cross-delivery; an unroutable target proves return handling leaves the DLQ message available.

### 3. Migration-before-rollout

- Added the packaged `migrate-sql` CLI using the actual `golang-migrate` runner and repository SQL migrations.
- Historical duplicate migration versions are normalized deterministically; the production runner uses the established GORM bootstrap only for an unversioned fresh database and the authoritative SQL 70-75 segment thereafter.
- Docker packages `migrate-schema`, `migrate-sql`, and migration files; Compose gates API startup on migration success.
- Helm supplies `pre-install,pre-upgrade` Secret and Job hooks, `DATABASE_URL`, ordered weights, finite retries/deadline, and the actual packaged runners. Hook failure blocks rollout.
- The migration Job uses the chart service account and inherits `global.imagePullSecrets`, matching the API PodSpec for private registries.
- CI runs `helm lint` and `helm template` and asserts the rendered hook, runner, and secret contract.

### 4. Non-skippable CI integration gates

- Integration helpers Fatal rather than Skip when either `CI=true` or `REQUIRE_INTEGRATION=true` and DSN/AMQP is absent or an isolated PostgreSQL database cannot be created.
- CI probes PostgreSQL, AMQP, and RabbitMQ readiness, then asserts named migration, reconnect, retry, DLQ, and factory-gate tests executed.
- Negative checks were run locally and produced the expected test failures for missing `DATABASE_DSN` and missing `AMQP_URL`.

### 5. Real retry chain

- Temporary failures traverse the actual four retry queues by TTL dead-lettering back to the subscriber main queue.
- Attempt headers increment through `[0,1,2,3,4]`.
- Both permanent and exhausted structured DLQ publications carry the current `x-sparkops-attempts` value.
- Real RabbitMQ coverage proves exhaustion produces DLQ attempts `4`, CLI replay preserves `4` on the exact main queue, and the test does not synthesize that header.

### 6. Actual migration runner and startup gate

- Real PostgreSQL tests run the production runner for fresh bootstrap, 69 to 75, and 74 to 75.
- The application factory rejects versions 69 and 74 before migration and starts after the same runner reaches 75.
- No test manually updates a migration version to claim success.

## Fresh verification matrix

Environment:

- Go `1.26.0`.
- PostgreSQL `12.22`, isolated database per test with cleanup.
- Reachable RabbitMQ is `3.6.14`, isolated vhost per run with cleanup.
- Helm and Docker CLIs are not installed locally; no local Helm version or container-render result is claimed.

Passed:

- Core unit/static scope:
  `go test ./cmd/event-dlq ./internal/platform/messaging/... ./internal/modules ./internal/modules/notification/application/subscriber ./deployments/helm -count=1 -timeout 300s`
- Build:
  `go build ./cmd/server ./cmd/event-dlq ./cmd/migrate ./cmd/migrate-sql`
- Real PostgreSQL:
  - `TestProjectArchiveIntentMigrationUpgradesPhaseSemanticsPostgres`
  - `TestSQLMigrationRunnerFreshReaches75AndPassesStartupGate`
  - `TestSQLMigrationRunnerUpgrades69And74To75WithStartupGate/from-69,from-74`
  - `TestRealMigrationRunnerControlsApplicationFactoryGate/from-69,from-74`
- Real RabbitMQ:
  - `TestRabbitEventTransportSameInstanceReconnectsPublisherAndConsumer` with two restored subscriptions
  - `TestReplayTargetsOnlyRequestedSubscriberAndPreservesHeaders`
  - `TestReplayMandatoryReturnDoesNotAckDLQ`
  - `TestRetryExhaustionDLQAttemptsSurviveCLIReplay`
- Real PostgreSQL plus RabbitMQ:
  - `TestEventPipelineTemporaryFailureUsesEveryRetryTier`
- Full real tagged matrix:
  `go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... ./cmd/event-dlq -count=1 -timeout 300s`
- CI fail-closed:
  - missing `DATABASE_DSN` with `CI=true` failed as required
  - missing `AMQP_URL` with `CI=true` failed as required

Attempted full backend regression:

- `go test ./... -count=1 -timeout 600s` reached all packages.
- Task 11 packages passed. Four protected, unrelated working-tree failures remain:
  - plugin persistence schema assertion from current uncommitted plugin changes
  - Kubernetes bundle descriptor missing `diagnosis/agent.go`
  - Kubernetes stream recording assertion
  - config DSN unit test overridden by the local uncommitted `backend/config.yaml`

## Residual risks and environment limits

- RabbitMQ 3.6 verifies real reconnect, confirms, mandatory returns, TTL dead-lettering, attempts, DLQ outcome, and subscription restoration, but it cannot validate quorum queues. CI uses RabbitMQ 3.13 and is the required quorum-declaration gate; staging must still validate three-node quorum node loss.
- Docker and Helm CLIs are unavailable locally. Image build, Compose startup, `helm lint`, and rendered chart checks are therefore CI-owned. Go static Helm contract tests passed locally, and CI renders with a private `global.imagePullSecrets` value and asserts the Job output.
- Windows race builds are not claimed; CI/Linux should run race coverage if required.
- Delivery remains at-least-once across external-success/mark windows, so stable idempotency keys and receiver reconciliation remain required.

# Task 11 首轮审查修复（2026-11-15）

## 范围与提交

- 修复起点：`6c64714f8300a36760cf8b5ca8b2469885268480`。
- 实现提交：`b435d1da81a2a6c17f074182da9d4aea0447210b`（`feat(filesystem-ui): share files with governed audiences`）。
- 仅修复 Task 11 首轮审查项；未实现 Task 12 租户治理、审批管理页或采纳流程。

## 修复结果

1. Shared reshare：移除 source Workspace 与当前 actor Workspace 必须相同的错误约束；保留组织绑定和 Node/Namespace 权威授权。当前 Workspace 管理员不能越权 source Workspace。Shared source 投影新增有效 `ALLOW` Grant 的 `permission/effect`，前端仅在 source Grant 显式包含 `SHARE` 时展示和执行转分享，`ADMIN` 不视为显式 `SHARE`。
2. 审批预览：将正式分享策略提取为可复用纯策略输入，并新增无副作用 `POST /api/v1/filesystem/nodes/{nodeId}/share-preview`。预览与正式命令共用组织/节点绑定、SHARE 授权、classification、principal scope 和策略判定；前端等待真实 `ACTIVE/PENDING_APPROVAL` 结果后才启用准确按钮。
3. 治理读取上限：grants 与 share-requests 默认最多 50、最大 200，数据库查询使用 `limit + 1` 检测截断并返回 `truncated`；抽屉明确提示“仅显示前 50 项治理记录”。补齐普通读者 403、跨组织 404、默认/最大 limit 和数据库硬上限测试。
4. Organization 受众：非 `RESTRICTED` 时使用当前权威 organization id 提供唯一可选项；影响文案为“将共享给当前租户（所有具备租户成员资格的用户）”。Workspace 仅在成员目录成功时使用真实成员数，目录失败时禁用且不以 0 冒充。
5. 幂等：按 canonical logical payload（受众、actions、effect、inherit、expiry）管理提交 key；同 payload 失败重试复用，任一字段变化刷新，成功/关闭重置；同步 in-flight ref 阻止快速双击重复提交。

## TDD 与验证

- RED 已确认：跨 source Workspace Grant 被 `organization_boundary` 拒绝；Preview API/handler 缺失；Shared source 缺 `permission/effect`；Organization 被永久禁用；逻辑 payload 编辑不刷新 key。
- `go test ./internal/modules/filesystem/... ./api/openapi -count=1`：通过。
- `pnpm vitest run apps/web/src/modules/filesystem/components`：25 files / 186 tests，通过（含 accessibility）。
- `pnpm vitest run apps/web/app/api/bff/v1/filesystem packages/contracts/src/filesystem/index.test.ts packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts`：5 files / 42 tests，通过。
- `pnpm typecheck`：6 packages，通过。
- IDE diagnostics：无新增 linter errors。
- `git diff --check`：通过。
- `.\scripts\dev-restart.ps1 -Backend`：通过，后端已重新监听 `:8080`。

## 未验证项

- 未执行真实登录态浏览器端到端测试。
- 未连接生产成员目录或外部 PostgreSQL；Workspace 真实计数链路以现有成员加载契约及组件测试验证，数据库边界以 repository 测试验证。
