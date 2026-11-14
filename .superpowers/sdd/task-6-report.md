# Task 6 报告：Workspace 传输列表与持久任务

## 状态

完成并随本报告提交；未生成 review package，未实现 Task 7。

## 真实契约核对

- 权限名称沿用后端现有 `authz.PermFileRead`（`file:read`）；GET 列表经过认证、Workspace 成员校验和 Workspace 权限中间件。
- `filesystem.transfer_sessions` 已由 `000078_filesystem_transfer` 创建，真实状态为 `INITIATED | UPLOADING | PAUSED | VERIFYING | SCANNING | COMMITTING | COMPLETED | FAILED | REJECTED | ABORTED | EXPIRED`。
- 列表查询始终以 `workspace_id` 为首要条件；应用查询层再次拒绝任何仓储误返回的跨 Workspace 行。
- 写接口保持原有 `Idempotency-Key` 要求；Task 6 只新增 GET，没有新增写接口。
- 前端真实组件名为 `TransferCenter`。持久任务通过 SDK/BFF/upstream 读取，与 Task 5 内存队列按 `session_id` 去重，本地活动项优先。

## TDD 记录

### RED

1. 查询处理器：
   `go test ./internal/modules/filesystem/application/query -run TestListTransfers -count=1`
   - 退出码 1；`NewListTransfersHandler` 与 `ListTransfers` 尚不存在。
2. 持久化分页：
   `go test ./internal/modules/filesystem/infrastructure/persistence -run TestListTransfersScopesFiltersAndPaginatesWithStableBoundary -count=1`
   - 退出码 1；仓储接口尚无 `ListTransfers`。
3. Migration：
   `go test ./migrations -run TestFilesystemTransferListMigrationAddsStableWorkspaceIndexes -count=1`
   - 退出码 1；`000081_filesystem_transfer_list` 尚不存在。
4. HTTP/OpenAPI：
   - HTTP 定向测试因 Handler 尚无 `listTransfers` 失败。
   - 注册 GET 后，`go test ./api/openapi -run TestFilesystem -count=1` 因 OpenAPI 缺少该路由失败。
5. 前端 SDK：
   `pnpm vitest run packages/web-sdk/src/filesystem.test.ts`
   - 退出码 1；`listTransferSessions` 尚不存在。
6. 持久任务面板：
   `pnpm vitest run apps/web/src/modules/filesystem/components/TransferCenter.test.tsx`
   - 退出码 1；历史任务未加载，无法与本地 session 去重。

### GREEN

1. 后端完整相关测试：
   `go test ./internal/modules/filesystem/... ./api/openapi ./migrations -count=1`
   - 退出码 0。
2. 前端 Filesystem 回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；15 个测试文件、93 项测试通过。
3. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码 0。
4. IDE diagnostics：
   - 修改的后端、TransferCenter、SDK 与 BFF route 无 linter errors。
5. `git diff --check`
   - 退出码 0。

## 实现摘要

- 新增 Workspace scoped 传输列表查询、状态过滤、1–100 limit、基于 `created_at DESC, id DESC` 的不透明 keyset cursor。
- 仓储、查询处理器、HTTP GET、模块装配与 OpenAPI 全链路接通；列表响应不签发上传 capability。
- 新增 `000081` 复合索引 migration，分别覆盖无状态和按状态过滤的稳定排序查询。
- 前端 contracts、upstream、BFF module/route 与 web SDK 接通；`TransferCenter` 区分加载、错误和空态，支持 cursor 加载更多。
- 持久任务与本地队列按后端 session ID 去重，本地任务保留取消、重试和实时进度的权威交互。

## Migration 注意事项

- `000081` 只增加索引，不修改既有数据或状态约束。
- Up 创建 `(workspace_id, created_at DESC, id DESC)` 和 `(workspace_id, status, created_at DESC, id DESC)`；Down 以反序删除。
- PostgreSQL 正式 migration 与项目 `AutoMigrate` 的 PostgreSQL/SQLite fallback 同步，避免测试和生产查询计划漂移。

## 提交

- Message: `feat(filesystem): add workspace transfer listing`
- Commit: 本报告所在提交。

## 风险

- 持久列表没有浏览器内存队列的逐字节上传事件；非完成持久任务只展示后端状态和保守进度，刷新后状态以服务端为准。
- 首屏默认读取最近 100 条，更多历史任务通过 cursor 手动加载；未实现 Task 7 的治理或审计重组。
- 前端测试仍输出仓库既有 Vite/Rolldown deprecated 配置警告，不影响结果。

## Critical / Important 审查修复

### 根因与方案

- 原首屏请求仅用 effect 局部 `active` 标志防止卸载后写入；workspace 切换后的首个 render 仍可短暂读取旧 `persisted`/cursor，分页请求也没有任何失效或取消机制。
- 持久任务状态现在显式绑定 `workspaceId`。render 只消费与当前 workspace 相同的状态，因此 effect 执行前也不会泄露旧 workspace 数据。
- 首屏、刷新和分页统一携带 `AbortSignal`，使用单调 request generation 拒绝所有旧请求结果；workspace 切换、快速 A→B→A、刷新替代分页和组件卸载都会中止当前 controller。
- 采用规格内更小的显式刷新方案，不增加轮询流量。刷新保留同 workspace 的已显示数据并呈现独立刷新态；首次加载、加载更多、错误和空态保持分离。
- 错误态提供“重试”，有数据和空态均提供“刷新”。分页使用同步 in-flight guard 防重复点击，并按 `session_id` 去重已有行及同页重复行。

### RED

1. Workspace、generation、分页与刷新测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/TransferCenter.test.tsx`
   - 退出码 1；5 项按预期失败：
     - SDK 调用未传 AbortSignal；
     - workspace 切换/卸载不能确认请求已取消；
     - 重复分页产生重复 key 和 4 个任务卡；
     - 错误态没有重试按钮；
     - 刷新与请求代际保护尚不存在。
2. 空态刷新测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/TransferCenter.test.tsx -t "offers an explicit refresh"`
   - 退出码 1；空态没有明确刷新入口。

### GREEN

1. TransferCenter 定向测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/TransferCenter.test.tsx`
   - 退出码 0；12 项测试通过。
2. 前端定向集成：
   `pnpm vitest run apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；3 个测试文件、38 项测试通过。
3. Filesystem 前端回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；15 个测试文件、100 项测试通过。
4. 后端目标包：
   `go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/infrastructure/persistence ./internal/modules/filesystem/interface/http ./api/openapi ./migrations -count=1`
   - 退出码 0。
5. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码 0。
6. IDE diagnostics：
   - 本次修改的 TransferCenter、测试和后端测试文件无 linter errors。
7. `git diff --check`
   - 退出码 0。

### 后端测试加强

- 仓储测试以同 workspace 的另一真实状态行证明 `status=FAILED` 会排除 `UPLOADING`。
- 覆盖 malformed cursor、service limit 默认/最小/最大/截断、HTTP limit 1/100 与 0/101 边界。
- 覆盖查询错误透传以及 Handler 对无效 cursor（400）和持久化不可用（503）的映射。
- Migration 测试验证 Up 创建顺序与 Down 反序删除，不引入额外数据库测试基础设施。

### 提交

- Message: `fix(filesystem-ui): harden persisted transfer refresh`
- Commit: 本节所在提交。

### 剩余风险

- 显式刷新不会自动轮询远端活动 session；这是本次选择的最小可靠方案，避免隐藏标签持续产生请求。用户可随时刷新，Task 5 本地活动任务仍由现有队列实时轮询。
