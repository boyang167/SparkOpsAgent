# Task 7 报告：文件详情、存储治理与审计工作区

## 状态

完成并随本报告提交；未生成 review package，未实现 Task 8 的全局视觉重构。

## 真实 API 与权限取舍

- 文件详情只展示 `FileNodeVM` 和真实 revisions；后端 preview 目前只返回 `preview_blob_id`，没有可展示内容或下载 capability URL，因此详情明确显示预览不可用占位，且不展示下载入口。
- 为支持 `/files?node=<nodeId>` 刷新恢复，最小新增 workspace-scoped `GET /api/v1/workspaces/{wsId}/files/{nodeId}`，沿用 `file:read`、Workspace 成员校验和资源级读授权；同步补齐 OpenAPI、upstream、BFF、SDK 与 server loader。
- `/files/[nodeId]?mode=revisions` 继续服务端渲染真实版本表；普通 `/files/[nodeId]` 重定向到 `/files?node=<nodeId>`。
- 存储治理一级标签仍由 `storage:read` 控制；二级面板写操作继续分别由 `storage:admin`、`lifecycle:admin` 控制。UI gating 不替代后端授权。
- 审计接口真实权限为 `file:admin`。原接口只按 Organization 查询，且 `result` 在数据库分页后过滤，可能跨 Workspace 泄漏并产生空页；现将 `workspace_id` 与 `decision` 作为审计存储过滤条件，在 limit/offset 前执行。actor/action/resource/time/cursor/limit 保持真实契约。
- Task 7 新增接口均为 GET；现有 Provider、Pool、Binding、Lifecycle 写 SDK 继续自动携带 `Idempotency-Key`，后端写路由继续强制该请求头。
- 未实现实时容量曲线或任何伪造指标。

## TDD 记录

### RED

1. 新组件与 URL 衔接：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx apps/web/src/modules/filesystem/components/GovernanceWorkspace.test.tsx apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/workbench-state.test.ts packages/web-sdk/src/filesystem.test.ts`
   - 退出码 1；三个组件、`getFileNode` SDK、`node` URL 状态和工作台 Drawer 衔接均尚不存在。
2. 审计 scope 与分页前过滤：
   `go test ./internal/modules/filesystem/application/query ./internal/platform/audit -run 'TestListFileAuditEventsScopesWorkspaceAndDecisionBeforePagination|TestGormStoreListFiltersWorkspaceAndDecisionBeforeLimit' -count=1`
   - 退出码 1；查询和存储过滤器尚无 `WorkspaceID` / `Decision`。
3. 单节点真实读取：
   HTTP 测试首先要求 `GET /files/{nodeId}`；实现前路由不存在，SDK 定向测试报 `getFileNode is not a function`。

### GREEN

1. Task 7 前端定向：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx apps/web/src/modules/filesystem/components/GovernanceWorkspace.test.tsx apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/workbench-state.test.ts packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；6 个测试文件、44 项测试通过。
2. Filesystem 前端回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；18 个测试文件、116 项测试通过。
3. Filesystem 相关 Go 包：
   `go test ./internal/modules/filesystem/... ./internal/platform/audit ./api/openapi -count=1`
   - 退出码 0。
4. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码 0。
5. IDE diagnostics：
   - Task 7 修改范围无 linter errors。
6. `git diff --check`
   - 退出码 0。

## 实现摘要

- 文件详情 Drawer 提供详情、版本、审计标签；版本打开时加载，审计按标签加载，并区分 loading/error/empty/forbidden。
- 文件点击写入 `node` URL；刷新由服务端加载真实节点恢复 Drawer，关闭时只移除 `node` 参数。
- 存储治理改为 Provider/存储池、作用域绑定、生命周期策略三个二级标签，每次只挂载一个现有 Panel。
- 审计工作区默认折叠高级筛选，筛选与 `audit_cursor` 写入 URL；行点击展示 actor、resource、decision、trace 与 detail JSON。
- Drawer 与表格使用现有响应式类，窄屏保持全宽、可换行和横向滚动。

## 提交

- Message: `feat(filesystem-ui): add details governance and audit workspaces`
- Commit: 本报告所在提交。

## 风险

- 预览接口只有 blob 标识，没有面向浏览器的读取 capability；本次明确占位，不猜测 URL。后续若增加 capability 契约，应另行 TDD 接入预览和下载。
- 审计分页仍沿用后端既有 offset cursor；已增加稳定 `created_at DESC, id DESC` 排序并保证 Workspace/decision 在分页前过滤，但高并发插入下 offset 分页仍可能发生位移，若要快照一致性需后续改为 keyset cursor。
- `FileAuditPanel` 保留为 `AuditWorkspace` 的兼容薄封装；新工作台直接使用 `AuditWorkspace`。
