# Task 7 实施报告

## 改动

- Placement 按 Namespace 类型执行固定优先级：
  - `USER > ORGANIZATION > SYSTEM`
  - `WORKSPACE > ORGANIZATION > SYSTEM`
  - `ORGANIZATION > SYSTEM`
- Placement 忽略跨 Organization 绑定和指向禁用 Pool 的绑定；`module.go` 接入 repository-backed Placement。
- CreateTransfer 按目标 Namespace 解析 Provider/Pool，并将选择结果纳入 session request hash。
- direct-local 上传在 Placement 选中的 LOCAL Provider 与配置网关不一致时返回明确的 `ErrStorageGatewayMismatch`。
- 共享读取测试证明不会调用 Placement，也不会创建 Replica。
- 新增 `000086_filesystem_namespace_quota` PostgreSQL up/down migration：
  - quota account 以 `namespace_id` 为隔离边界；
  - 兼容保留 nullable `workspace_id`；
  - Organization/User Namespace 可在没有 workspace ownership 的情况下拥有独立配额账户；
  - down migration 遇到非 Workspace 配额活动或 Transfer 时拒绝回滚，避免静默数据丢失。
- SQLite/fallback schema 与 repository 同步为 Namespace-aware quota；旧 workspace quota API 保持兼容。

## 测试

- `go test ./internal/modules/filesystem/domain/service -run Placement -count=1`：PASS
- `go test ./internal/modules/filesystem/application/command -run CreateTransfer -count=1`：PASS
- `go test ./internal/modules/filesystem/... -count=1`：PASS
- `go test ./migrations -run NamespaceQuota -count=1`：PASS
- `go test ./internal/modules/filesystem/infrastructure/persistence -count=1`：PASS
- `go vet ./internal/modules/filesystem/...`：PASS
- IDE lint：无错误
- `.\scripts\dev-restart.ps1 -Backend`：PASS，后端已以 PID `25568` 监听 `:8080`

## Commit

- Commit message：`feat(filesystem): place uploads by namespace scope`
- Commit hash：本报告与实现位于同一提交；精确 hash 记录在最终交付回复中（Git commit hash 无法在不改变自身 hash 的情况下写入同一提交内容）。

## 未验证项

- `DATABASE_DSN` 未设置，因此未执行真实 PostgreSQL migration up/down；仅验证了 migration 定义测试以及 SQLite/fallback schema 与持久化测试。
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

---

## Important / Minor 缺口修复

### RED

1. 前端状态、竞态与可访问性定向测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx apps/web/src/components/ui/Drawer.test.tsx apps/web/src/server/loaders/filesystem.test.ts`
   - 修复前 5 个文件失败，共 16 项失败、30 项通过；覆盖 URL/workspace 选中同步、晚响应、重试、分页历史、服务端节点错误和 Drawer 焦点。
2. 后端定向测试：
   `go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/application/command ./internal/modules/filesystem/interface/http ./api/openapi`
   - 递归脱敏、controlled export workspace scope、audit `limit > 100` 和具体 OpenAPI schema 按预期失败；node endpoint 拒绝路径测试已通过现有实现。

### GREEN

1. 前端定向：5 个文件、47 项测试通过。
2. Filesystem 前端回归：
   `pnpm vitest run apps/web/src/modules/filesystem apps/web/src/components/ui/Drawer.test.tsx apps/web/src/server/loaders/filesystem.test.ts`
   - 19 个文件、123 项测试通过。
3. Filesystem Go 与 OpenAPI：
   `go test ./internal/modules/filesystem/... ./api/openapi`
   - 全部通过。
4. Web 类型检查：`pnpm --filter @sparkops/web typecheck` 通过。
5. IDE diagnostics：修改范围无 linter errors；`git diff --check` 通过。

### 修复摘要与真实契约取舍

- Workbench 以 workspace、URL node 和服务端选中结果为权威同步来源；切换 workspace 或浏览器前进/后退时不渲染旧节点。服务端节点 403/404/其他失败映射到 Drawer 的 forbidden/not-found/error，并支持刷新重试，不再中断整页。
- FileDetailDrawer 对 node/workspace 切换立即隔离旧数据；版本与审计请求均使用 AbortController 和 generation 防止晚响应覆盖，错误可重试。详情 tabs 增加 roving tabindex、方向键和 tabpanel 关联。
- AuditWorkspace 同步 URL draft/query，使用 generation + AbortController 防竞态；后端真实 cursor 之外不推测总页数，仅维护已访问 cursor trail，支持上一页以及浏览器前进/后退。
- 服务端对审计 detail 递归脱敏 token、token_hash、secret、credential、capability 等敏感键；UI 不承担安全边界。OpenAPI 明确定义 `FileAuditEvent`、`FileAuditEventPage`、detail 和 next_cursor，并与 SDK 字段一致；`limit > 100` 返回 400。
- controlled export 的 source/target 两条审计分别记录对应 `workspace_id`。复核其他 filesystem 审计写点：mutation、grant、middleware/resource deny、capability issue 均已有 workspace scope；capability 的 token_hash 由查询层统一脱敏。
- Drawer 复用现有 modal focus hook，增加初始聚焦、Tab trap 和关闭恢复；未增加依赖，未扩大 Task 8 视觉范围。

### 提交

- Message: `fix(filesystem-ui): harden task 7 workspaces`
- Commit: 本追加报告所在提交。

### 剩余风险

- 审计后端仍使用既有 offset cursor；前端历史栈只对本次已访问 cursor 提供上一页，深链到未知 cursor 时不会伪造前页。
- 服务端详情重试依赖 Next `router.refresh()`；持续 403/404 会稳定回到对应 Drawer 状态。

---

## 复审剩余 3 Important 与契约 Minor

### RED

1. AuditWorkspace render-frame scope：
   `pnpm vitest run apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx`
   - 退出码 1；URL filter 切换后的首次 render 仍调用旧审计详情 Drawer，证明仅靠 effect 清理会闪现旧 selected/items。
2. 脱敏、ALLOW 写入与 OpenAPI：
   `go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/interface/http ./internal/platform/audit ./api/openapi`
   - 退出码 1；`password` 等规范化敏感键未脱敏、`filesystem.capability.issued` 缺少 `decision: ALLOW`、审计 id/actor_id 缺 UUID format。
   - repository 的 workspace + decision 分页前过滤 characterization 测试通过，确认无需修改存储查询实现。

### GREEN

1. AuditWorkspace 定向：1 个文件、8 项测试通过。
2. 后端定向：query、HTTP、audit repository、OpenAPI 全部通过。
3. Filesystem 前端回归：
   `pnpm vitest run apps/web/src/modules/filesystem apps/web/src/components/ui/Drawer.test.tsx apps/web/src/server/loaders/filesystem.test.ts packages/web-sdk/src/filesystem.test.ts`
   - 20 个文件、133 项测试通过。
4. 相关 Go：
   `go test ./internal/modules/filesystem/... ./internal/platform/audit ./api/openapi`
   - 全部通过。
5. `pnpm --filter @sparkops/web typecheck`、IDE diagnostics、`git diff --check` 全部通过。

### 实现与契约取舍

- AuditWorkspace 将 items、next cursor、load state、selected event 统一绑定到 `workspace + actor/action/resource/result/time/cursor` request scope；scope 不匹配时 render 当帧直接使用空 loading 视图，旧行与旧 Drawer 不等待 effect 才清理。
- 审计 detail 键先统一大小写、连字符和驼峰，再按明确的精确键/命名后缀脱敏；覆盖 token/token_hash/password/secret/api_key/access_key/private_key/authorization/cookie/credential/capability，并递归处理对象和数组。`token_count`、`tokenizer`、`credential_id`、`access_key_id` 等元数据保留。
- `filesystem.capability.issued` 现在明确写入 `decision: ALLOW`；复核 filesystem 其他写入点后，mutation、grant、controlled export 已写 decision，middleware/resource deny 已写 DENY，无其他依赖默认 ALLOW 的写入点。
- OpenAPI 为审计 `id`、`actor_id` 增加 UUID format。服务真实响应始终提供 detail 与 next_cursor，因此 OpenAPI 继续将二者列为 required；SDK 保留 optional+default 的输入容错以兼容旧响应，但解析后的输出类型仍始终包含二者。

### 提交

- Message: `fix(filesystem): close task 7 audit review`
- Commit: 本追加报告所在提交。

---

## 最后 2 个 Important

### RED

`go test ./internal/modules/filesystem/application/query`

- 退出码 1。
- `result=ALLOW` 查询无法返回历史上缺少 decision 的 `filesystem.capability.issued`。
- `PasswordHash`、`API-KEY-HASH`、`secretValue`、`AuthorizationHeader` 等敏感复合键未被脱敏。

### GREEN

1. 定向 query 与 repository：`go test ./internal/modules/filesystem/application/query ./internal/platform/audit` 通过。
2. 相关 Go/OpenAPI：`go test ./internal/modules/filesystem/... ./internal/platform/audit ./api/openapi` 全部通过。
3. 前端必要回归：AuditWorkspace 与 filesystem web SDK 共 2 个文件、17 项测试通过。
4. Web typecheck、IDE diagnostics、`git diff --check` 通过。

### 实现取舍

- audit `ListFilter` 增加显式 `AllowMissingDecisionAction` 兼容开关。仅 filesystem query 在 `result=ALLOW` 时设置为 `filesystem.capability.issued`；repository 在 workspace 条件之后、limit/offset 之前应用：
  `decision=ALLOW OR (action=filesystem.capability.issued AND decision missing)`。
- 因此历史 capability issued 可见；其他缺 decision 的未知、失败或其他 filesystem 事件不会被归入 ALLOW，跨 workspace 记录也不会混入。
- 脱敏改为规范化后的 segment 模式，不使用 substring/Contains。敏感 segment/pattern 可位于键名前部、中部或末尾；仅当其后恰好是允许元数据后缀 `id`、`count`、`algorithm` 时保留。
- 覆盖 password_hash、api_key_hash、secret_value、authorization_header、refresh_token_value 等复合形式；保留 token_count、tokenizer、credential_id、access_key_id、private_key_algorithm 等元数据，并继续递归处理嵌套对象/数组、大小写、连字符和驼峰。

### 提交

- Message: `fix(filesystem): support legacy audit allow`
- Commit: 本追加报告所在提交。
