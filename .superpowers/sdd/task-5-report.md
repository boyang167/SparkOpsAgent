# Task 5 Report

## Status

Implemented unified governed filesystem query APIs:

- `GET /api/v1/filesystem/namespaces`
- `GET /api/v1/filesystem/namespaces/:namespaceId/children`
- `GET /api/v1/filesystem/shared-with-me`
- `GET /api/v1/filesystem/nodes/:nodeId/effective-access`

The legacy `/api/v1/workspaces/:wsId` routes remain unchanged.

## Security and authorization

- Requires canonical `X-Organization-ID`; missing or malformed values return `400`.
- Accepts optional canonical `X-Workspace-ID`; workspace membership and organization ownership are resolved authoritatively, with invalid audience returning `403`.
- Namespace organization/type/owner/workspace audience is checked before query execution.
- Unauthorized namespace or parent discovery is concealed with `404`.
- Effective access reuses the Task 4 authorizer and principal resolver; no parallel authorization path was introduced.
- Organization namespace summaries expose only authorized subtree entry IDs when the root is not visible.

## Query implementation

- Shared-with-me filtering is performed in two bounded repository queries rather than tenant-wide N+1 traversal.
- Repository filtering uses typed `(principal_type, principal_id)` pairs, organization scope, active/non-expired grants, stable node-ID cursors, query filtering, node deduplication, action union, and global deny precedence.
- Effective access returns decision, effective actions, reason code, source grant IDs, policy version, and cache key.

## TDD evidence

RED was observed before implementation for:

- query handlers (missing governance query types/handlers),
- repository shared query (missing `ListSharedNodes`),
- HTTP governance routes (missing handler/registration),
- module route wiring,
- OpenAPI governance headers,
- denied source-grant propagation.

GREEN verification:

- `go test ./internal/modules/filesystem/application/query -run "Namespace|Shared|Effective" -count=1`
- `go test ./internal/modules/filesystem/interface/http -run Governance -count=1`
- `go test ./internal/modules/filesystem/infrastructure/persistence -run "ListSharedNodes" -count=1`
- `go test ./internal/modules/filesystem/... -count=1`
- `go test ./api/openapi -count=1`
- OpenAPI root external-reference load and validation are covered by the OpenAPI test suite.

## Concerns

None known.
# Task 5 报告：多文件上传与统一传输队列

## 状态

完成并随本报告提交；未生成 review package。

## 真实契约核对

- `CreateTransferCommand` 的 `provider_id` 可选；后端在未指定时通过 placement/default local Provider 决定存储位置，因此上传按钮不再依赖前端 Provider 配置。
- DIRECT 会话返回 `upload_url` 和 `capability_token`。客户端必须向 `upload_url` 发送带 `Authorization: Bearer <capability_token>` 的 `PUT`。
- 上传网关写入对象后调用 `RecordUploaded`，发布 `TransferUploaded` 事件；后端订阅者异步执行 verify/commit。前端不能把 PUT 完成等同于文件完成，必须通过 `getTransferSession` 轮询到 `COMPLETED`。
- brief 的 `uploadTransfer` 示例假定 SDK 已覆盖上述 DIRECT 协议，但原实现只遍历 multipart parts；本任务按真实后端契约补齐 DIRECT 上传、capability 鉴权、失败响应和取消 signal。

## TDD 记录

### RED

运行：

`pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts packages/web-sdk/src/filesystem.test.ts`

- 退出码：1。
- 新队列、DropZone、TransferCenter 尚不存在，对应测试按预期失败。
- DIRECT SDK 测试进入原 multipart 路径并以 `missing upload_url for part 1` 失败，确认真实契约缺口。
- 旧 `UploadPanel` 仍自行执行单文件上传并要求 `providerId`，不满足新的回调接口。

### GREEN

1. Task 5 核心与集成测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 6 个测试文件、30 项测试通过。
2. Filesystem 相关回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 15 个测试文件、76 项测试通过。
3. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码：0。
4. IDE diagnostics：
   - Task 5 组件、测试与 SDK 文件无 linter errors。
5. `git diff --check`
   - 退出码：0。

## 行为覆盖

- 多文件进入统一内存队列，固定最大并发数 3。
- 每项保存文件、目标目录、状态、字节进度、会话和错误；状态限定为 `queued | creating | uploading | verifying | completed | failed | canceled`。
- 失败项可单独重试；活动项取消时中止客户端请求，并在已有会话时调用后端 abort。
- 文件选择与桌面拖入共用队列；目录项明确拒绝，不实现递归上传。
- `application/x-sparkops-file-nodes` 站内节点拖拽优先排除，即使 DataTransfer 同时声明 `Files` 也绝不触发上传。
- 工作台持有队列，文件与传输标签只切换视图，不销毁正在运行的任务。
- 文件标签只显示最多三个活动任务摘要；传输标签展示完整进度、状态、错误、取消与重试。
- 上传入口与桌面投放统一受 `file:write` 权限控制。

## 提交

- Message: `feat(filesystem-ui): add multi-file transfer center`
- Commit: 本报告所在提交。

## 风险

- 浏览器 Fetch API 不提供上传字节流事件；DIRECT 单对象上传的进度会从 0 跳到完成，队列和 UI 已支持细粒度进度，后续若 SDK 改为可观测传输可直接接入。
- PUT 完成后的 verify/commit 是异步事件链，当前以前台每 500ms 轮询会话状态；任务仅保证在工作台标签切换期间持续运行，不承诺整页刷新后的持久恢复。
- Vitest 仍输出仓库既有的 Vite/Rolldown deprecated 配置警告，不影响测试结果。

## Important / Minor 审查修复

### 根因与修复

- 原队列在收到取消动作时立即永久写入 `canceled`，后端 abort 的结果不再参与状态决策；创建会话晚于取消时，受终态保护的 `setSession` 又会丢弃 session，形成孤儿风险。
- 队列现在把“请求取消”和终态分离。活动任务先显示“正在取消”，保留 controller 与晚到 session；只有 abort 成功后才进入 `canceled`。abort 失败进入可重试的 `failed`，若查询确认后端已完成则进入 `completed`，避免取消/完成竞争误报。
- 创建请求携带 AbortSignal 与每次尝试独立的幂等键。取消造成创建结果未知时，用同一幂等键无 signal 重取权威 session，再立即 abort；重试会生成新键和新 session，不复用旧会话。
- DIRECT、MULTIPART/presigned PUT 均传递 AbortSignal；multipart 每个 PUT 检查 `response.ok`，非 2xx 不更新进度、不误报上传完成。
- 轮询请求和 500ms 间隔都响应 AbortSignal；取消会清除定时器。非取消触发的 AbortError 进入可重试失败态。
- verify/commit 阶段保持“正在校验”且进度最高显示 99%，只有真实 `COMPLETED` 才显示 100%。
- 所有可见 tab 固定关联其常驻 panel；非活动 panel 使用 `hidden`，`aria-controls` 不再指向不存在的元素。

### RED

1. 竞态与 SDK 测试首次运行：
   `pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx packages/web-sdk/src/filesystem.test.ts`
   - 退出码：1。
   - 9 项按预期失败：创建期取消未 abort 晚到 session、abort 失败仍显示已取消、取消/完成竞争误报、multipart 未传 signal 且忽略 503、verify 显示 100%、tab 关系不稳定。
2. AbortError 分类测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts`
   - 退出码：1。
   - 非 DOMException 的命名 `AbortError` 被直接显示为 `Aborted`，未提供可行动提示。
3. 创建结果未知补偿测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx -t "recovers an unknown create outcome"`
   - 退出码：1。
   - 取消导致网络结果未知时未使用同一幂等键恢复权威 session。

### GREEN

1. Task 5 定向测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 6 个测试文件、42 项测试通过。
2. Filesystem 回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 15 个测试文件、88 项测试通过。
3. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码：0。
4. IDE diagnostics：
   - 本次修改文件无 linter errors。
5. `git diff --check`
   - 退出码：0。

### 提交

- Message: `fix(filesystem-ui): harden transfer cancellation races`
- Commit: 本节所在提交。

### 剩余风险

- 创建结果未知时的同幂等键恢复依赖后端既有幂等重放契约；若 BFF 与后端同时完全不可达，任务会明确进入“取消失败”而不是伪装已取消，需用户在连接恢复后重试。
- DIRECT Fetch 仍无法提供浏览器原生上传字节流事件；该限制与本报告前述一致。

## Important：安全重试旧会话修复

### 根因与恢复语义

- 原 `retry()` 无条件清除 `session`，随后上传回调生成新幂等键并创建新 session。轮询网络失败或 abort 失败时，旧 session 可能仍在上传、校验或提交，重试会造成并行 session 和重复上传风险。
- 队列项现在保留 `sessionId`、创建幂等键、尝试次数和进度。每次重试仍获得新的 controller 和并发槽，但不会丢弃旧会话身份。
- 有旧 session 时，重试首先调用 `getTransferSession`：
  - `COMPLETED`：直接完成，不 create、不 PUT；
  - `INITIATED`：复用原 session 上传；
  - `UPLOADING / PAUSED / VERIFYING / SCANNING / COMMITTING`：只恢复轮询，不 create、不 PUT；
  - `FAILED / REJECTED / ABORTED / EXPIRED`：确认旧 session 已终态后，才清除旧引用、生成新幂等键并创建新 session。
- 只有幂等键但没有 sessionId 时，说明创建结果未知；重试使用原幂等键恢复权威 session。网络未知时继续失败并保留身份，不会换键重复创建。
- abort 失败后的重试同样先查询旧 session；未确认终态前不会创建 session-2。新 session 启动时进度归零，旧 controller 与轮询定时器不会复用。

### RED

运行：

`pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx`

- 退出码：1。
- 5 项按预期失败：
  - 轮询网络失败后重试额外调用 create/upload；
  - abort 失败后重试未查询旧 session；
  - 未等待 session-1 权威终态就进入新会话；
  - 队列重试清除了轮询失败和 abort 失败任务的旧 session。

### GREEN

1. Task 5 定向测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 6 个测试文件、45 项测试通过。
2. Filesystem 回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 15 个测试文件、91 项测试通过。
3. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码：0。
4. IDE diagnostics：
   - 本次修改文件无 linter errors。
5. `git diff --check`
   - 退出码：0。

### 提交

- Message: `fix(filesystem-ui): preserve sessions across transfer retries`
- Commit: 本节所在提交。

### 剩余风险

- 活跃旧 session 会持续按现有 500ms 周期查询，直到后端终态、用户取消或网络再次失败；这是刻意的安全取舍，优先避免重复 PUT 和并行 session。
