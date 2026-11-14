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
