# 文件系统回收站实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复软删除后同名文件无法再次上传的问题，并交付可查看、恢复、彻底删除和清空的完整回收站。

**Architecture:** 数据库只对 `ACTIVE` 节点执行同目录名称唯一约束；`TRASHED` 节点保留原位置和内容引用用于恢复。彻底删除在单个事务内将子树标记为 `PURGED`、释放逻辑配额并递减 Blob 引用，物理对象继续由现有 GC 异步清理。前端通过 contracts → upstream → BFF modules → Next BFF route → web SDK 的现有链路接入独立回收站页面。

**Tech Stack:** Go 1.x、Gin、GORM、PostgreSQL、SQLite 测试库、React 19、Next.js、TypeScript、Zod、Vitest。

## Global Constraints

- 普通删除必须移入回收站，并立即允许在原目录创建或上传同名项目。
- 回收站首版仅手动清理，不增加自动过期任务。
- 恢复回原目录；同名活动项目存在时拒绝恢复，不覆盖、不自动改名。
- 移入回收站不释放配额；彻底删除才释放配额和 Blob 引用。
- HTTP 请求不直接删除物理对象，物理清理由现有 GC 完成。
- 所有修改命令必须幂等、可审计，并发布可靠领域事件。
- 不修改或回滚工作区中与本功能无关的未提交改动。
- 未经用户明确要求不创建 Git commit；每个任务结束只保留可审查的工作区变更。
- 后端改动完成后执行 `.\scripts\dev-restart.ps1 -Backend`。
- 不运行全仓 `npm test`；只运行本计划列出的定向测试、`npm run check` 和必要的类型检查。

---

## 文件结构

### 后端新增文件

- `backend/migrations/000089_filesystem_recycle_bin.up.sql`：生产 PostgreSQL 回收站迁移。
- `backend/migrations/000089_filesystem_recycle_bin.down.sql`：回滚索引和删除元数据列。
- `backend/migrations/filesystem_recycle_bin_test.go`：迁移文本约束测试。
- `backend/internal/modules/filesystem/application/query/list_trash.go`：回收站分页查询。
- `backend/internal/modules/filesystem/application/query/list_trash_test.go`：查询行为测试。
- `backend/internal/modules/filesystem/application/command/restore_node.go`：恢复节点命令。
- `backend/internal/modules/filesystem/application/command/restore_node_test.go`：恢复测试。
- `backend/internal/modules/filesystem/application/command/permanent_delete_node.go`：彻底删除命令。
- `backend/internal/modules/filesystem/application/command/permanent_delete_node_test.go`：清理、配额、Blob 测试。
- `backend/internal/modules/filesystem/application/command/empty_trash.go`：清空回收站批次命令。
- `backend/internal/modules/filesystem/application/command/empty_trash_test.go`：部分成功与幂等测试。

### 前端新增文件

- `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/trash/route.ts`：列表与清空 BFF。
- `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/restore/route.ts`：恢复 BFF。
- `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/permanent/route.ts`：彻底删除 BFF。
- `fronted/apps/web/app/(platform)/files/data/trash/page.tsx`：回收站页面。
- `fronted/apps/web/src/modules/filesystem/components/TrashExplorerPanel.tsx`：回收站交互容器。
- `fronted/apps/web/src/modules/filesystem/components/TrashExplorerPanel.test.tsx`：回收站 UI 测试。
- `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.ts`：批量恢复/彻底删除的部分成功收敛。
- `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.test.ts`：批量操作测试。

---

### Task 1: 活动名称唯一索引与直接故障修复

**Files:**
- Create: `backend/migrations/000089_filesystem_recycle_bin.up.sql`
- Create: `backend/migrations/000089_filesystem_recycle_bin.down.sql`
- Create: `backend/migrations/filesystem_recycle_bin_test.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/migrate.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/migrate_test.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo_test.go`

**Interfaces:**
- Produces: `CreateNode` 只把同目录 `ACTIVE` 节点视为名称冲突。
- Produces: PostgreSQL 和 SQLite 的 `ux_filesystem_nodes_root_name`、`ux_filesystem_nodes_child_name` 仅约束 `state = 'ACTIVE'`。

- [ ] **Step 1: 写软删除后同名创建的失败测试**

在 `filesystem_repo_test.go` 增加：

```go
func TestCreateNodeAllowsActiveNameAfterPreviousNodeWasTrashed(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)
	trashed := mustFile(ns.ID, parent.ID, "a.txt")
	require.NoError(t, repo.CreateNode(context.Background(), trashed))
	require.NoError(t, repo.TrashNode(context.Background(), trashed.ID))

	replacement := mustFile(ns.ID, parent.ID, "A.TXT")
	require.NoError(t, repo.CreateNode(context.Background(), replacement))
}
```

- [ ] **Step 2: 运行测试并确认因名称冲突失败**

Run:

```powershell
cd a:\21-Agent\SparkOpsAgent\backend
go test ./internal/modules/filesystem/infrastructure/persistence -run TestCreateNodeAllowsActiveNameAfterPreviousNodeWasTrashed -count=1
```

Expected: FAIL，错误包含 `filesystem node name conflict`。

- [ ] **Step 3: 添加 000089 迁移**

`up.sql` 核心内容：

```sql
DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_root_name;
DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name;

CREATE UNIQUE INDEX ux_filesystem_nodes_root_name
    ON filesystem.nodes(namespace_id, normalized_name)
    WHERE parent_id IS NULL AND state = 'ACTIVE';

CREATE UNIQUE INDEX ux_filesystem_nodes_child_name
    ON filesystem.nodes(namespace_id, parent_id, normalized_name)
    WHERE parent_id IS NOT NULL AND state = 'ACTIVE';

ALTER TABLE filesystem.nodes ADD COLUMN IF NOT EXISTS deleted_by UUID;

CREATE INDEX IF NOT EXISTS ix_filesystem_nodes_trash_entries
    ON filesystem.nodes(namespace_id, deleted_at DESC, id DESC)
    WHERE state = 'TRASHED';
```

`down.sql` 必须先删除新索引，再恢复不含 `state` 的旧索引，并删除 `deleted_by` 与 trash 索引。

- [ ] **Step 4: 对齐 AutoMigrate 与 SQLite**

在 `migrate.go`：

```go
`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_root_name ON filesystem.nodes(namespace_id, normalized_name) WHERE parent_id IS NULL AND state = 'ACTIVE'`,
`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_child_name ON filesystem.nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL AND state = 'ACTIVE'`,
```

SQLite 初始 `nodes` DDL 增加：

```sql
deleted_at DATETIME,
deleted_by BLOB
```

并在 AutoMigrate 升级路径中显式执行 `DROP INDEX` 后按新谓词重建；不能只依赖 `IF NOT EXISTS`。

- [ ] **Step 5: 限定应用层预检查**

`CreateNode` 查询改为：

```go
query := db.Where(
	"namespace_id = ? AND normalized_name = ? AND state = ?",
	node.NamespaceID,
	node.NormalizedName,
	model.NodeStateActive,
)
```

- [ ] **Step 6: 添加迁移与索引定义测试**

迁移文本测试必须断言：

```go
require.Contains(t, up, "DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name")
require.Contains(t, up, "WHERE parent_id IS NOT NULL AND state = 'ACTIVE'")
require.Contains(t, up, "ADD COLUMN IF NOT EXISTS deleted_by UUID")
```

`migrate_test.go` 必须实际创建 `TRASHED` 与同名 `ACTIVE` 两行并断言成功，再创建第二个同名 `ACTIVE` 并断言失败。

- [ ] **Step 7: 验证直接故障已修复**

Run:

```powershell
go test ./migrations -run FilesystemRecycle -count=1
go test ./internal/modules/filesystem/infrastructure/persistence -run "CreateNodeAllowsActiveName|DuplicateNormalizedName|Migrate" -count=1
```

Expected: PASS。

---

### Task 2: 删除元数据、回收站列表与恢复持久化

**Files:**
- Modify: `backend/internal/modules/filesystem/domain/model/node.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/models.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo_test.go`
- Create: `backend/internal/modules/filesystem/application/query/list_trash.go`
- Create: `backend/internal/modules/filesystem/application/query/list_trash_test.go`

**Interfaces:**
- Produces:

```go
type TrashEntry struct {
	Node         model.Node
	OriginalPath string
	DeletedAt    time.Time
	DeletedBy    uuid.UUID
}

type TrashListQuery struct {
	NamespaceID uuid.UUID
	Limit       int
	Cursor      string
}

TrashNode(context.Context, uuid.UUID, time.Time, uuid.UUID) error
ListTrashEntries(context.Context, TrashListQuery) ([]TrashEntry, string, error)
RestoreNode(context.Context, uuid.UUID, uuid.UUID) error
```

- [ ] **Step 1: 写删除元数据和列表失败测试**

覆盖：

```go
func TestTrashNodeStoresDeletionMetadata(t *testing.T)
func TestListTrashEntriesExcludesItemsBelowTrashedAncestor(t *testing.T)
func TestListTrashEntriesUsesStableDeletedAtAndIDCursor(t *testing.T)
```

断言 `deleted_at`、`deleted_by` 非空；父目录和子项都为 `TRASHED` 时只返回父目录入口。

- [ ] **Step 2: 运行测试并确认接口尚不存在**

Run:

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence -run "TrashNodeStores|ListTrashEntries" -count=1
```

Expected: FAIL，编译错误指向新 repository 方法或字段。

- [ ] **Step 3: 增加模型和 repository 类型**

`model.Node` 增加：

```go
DeletedAt *time.Time
DeletedBy *uuid.UUID
```

`nodeRow` 和 `nodeFromRow` 同步映射。`TrashNode` 使用状态条件更新：

```go
Updates(map[string]any{
	"state":      model.NodeStateTrashed,
	"deleted_at": deletedAt.UTC(),
	"deleted_by": deletedBy,
	"updated_at": deletedAt.UTC(),
})
```

`RowsAffected == 0` 时返回 `ErrNodeStateConflict` 或 `ErrNodeNotFound`，不得静默成功。

- [ ] **Step 4: 实现回收站查询**

`ListTrashEntries` 使用递归祖先 CTE：

```sql
WHERE node.namespace_id = ?
  AND node.state = 'TRASHED'
  AND NOT EXISTS (
      SELECT 1 FROM ancestors
      WHERE ancestors.descendant_id = node.id
        AND ancestors.id <> node.id
        AND ancestors.state = 'TRASHED'
  )
```

按 `deleted_at DESC, id DESC` 排序，游标编码这两个字段；`OriginalPath` 通过原父链构造。

- [ ] **Step 5: 实现恢复 CAS**

`RestoreNode` 必须：

1. 读取 `TRASHED` 节点；
2. 验证原父目录为 `ACTIVE DIRECTORY`；
3. 条件更新 `state = TRASHED` 为 `ACTIVE`；
4. 清空 `deleted_at`、`deleted_by`；
5. 依赖活动节点部分唯一索引处理并发同名冲突。

- [ ] **Step 6: 创建查询处理器**

`list_trash.go` 定义：

```go
type ListTrash struct {
	NamespaceID uuid.UUID
	Limit       int
	Cursor      string
}

type ListTrashResult struct {
	Items      []repository.TrashEntry
	NextCursor string
}
```

处理器只负责参数边界和调用 repository；HTTP 层负责 workspace/namespace 与权限过滤。

- [ ] **Step 7: 验证列表与恢复持久化**

Run:

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence -run "Trash|RestoreNode" -count=1
go test ./internal/modules/filesystem/application/query -run ListTrash -count=1
```

Expected: PASS。

---

### Task 3: 彻底删除、配额释放与 Blob 生命周期

**Files:**
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/quota_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo_test.go`
- Modify: `backend/internal/modules/filesystem/application/operation/garbage_collect_test.go`

**Interfaces:**
- Produces:

```go
type PurgeSummary struct {
	NodeIDs  []uuid.UUID
	FileCount int64
	ByteCount int64
	BlobIDs  []uuid.UUID
}

PurgeTrashedSubtree(context.Context, uuid.UUID, uuid.UUID, time.Time) (PurgeSummary, error)
ReleaseNamespaceQuotaUsage(context.Context, uuid.UUID, int64, int64) error
DecrementBlobRef(context.Context, uuid.UUID, time.Time) error
```

- [ ] **Step 1: 写彻底删除失败测试**

测试必须构造包含多版本文件和子目录的树，并断言：

```go
require.Equal(t, model.NodeStatePurged, purged.State)
require.Equal(t, int64(0), quota.UsedFiles)
require.Equal(t, int64(0), quota.UsedBytes)
require.Equal(t, int64(0), blob.RefCount)
require.Equal(t, model.BlobStateTombstoned, blob.State)
require.NotNil(t, blob.TombstonedAt)
```

另加共享 Blob 测试：两个节点引用同一 Blob，purge 一个节点后 `ref_count == 1` 且状态仍为 `AVAILABLE`。

- [ ] **Step 2: 运行测试并确认缺少减引用/减配额**

Run:

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence -run "PurgeTrashedSubtree|ReleaseNamespaceQuota|DecrementBlob" -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现配额释放**

新增 `ReleaseNamespaceQuotaUsage`，只接受非负释放量，并执行下界保护：

```go
Where("namespace_id = ? AND used_bytes >= ? AND used_files >= ?", namespaceID, bytes, files).
Updates(map[string]any{
	"used_bytes": gorm.Expr("used_bytes - ?", bytes),
	"used_files": gorm.Expr("used_files - ?", files),
	"version":    gorm.Expr("version + 1"),
	"updated_at": time.Now().UTC(),
})
```

余额不足必须返回 `ErrPersistence`，不能用 `GREATEST` 掩盖账目错误。

- [ ] **Step 4: 实现 Blob 减引用与 tombstone**

同一事务内锁定 Blob：

```go
if row.RefCount <= 0 {
	return domain.ErrBlobConflict
}
updates := map[string]any{"ref_count": row.RefCount - 1}
if row.RefCount == 1 {
	updates["state"] = model.BlobStateTombstoned
	updates["tombstoned_at"] = now.UTC()
}
```

- [ ] **Step 5: 实现递归 purge**

`PurgeTrashedSubtree`：

1. 要求入口为 `TRASHED`；
2. 用递归 CTE 锁定整棵子树；
3. 若任何节点为 `RETENTION_HOLD` 或关联 Blob `legal_hold = true`，返回 `ErrRetentionHold`；
4. 汇总全部 FILE 节点的全部 revision 大小和 Blob 引用；
5. CAS 标记 `PURGE_PENDING`；
6. 扣配额、逐 revision 减 Blob 引用；
7. 标记全部节点 `PURGED`。

- [ ] **Step 6: 验证 GC 仍保护回收站内容并接受 purge 内容**

扩展 GC 测试：

- `TRASHED` 节点仍计入 live revision，GC 拒绝；
- `PURGED` 节点不计入 live revision；
- ref 为 0、已 tombstone 且过 grace 后可以删除副本。

- [ ] **Step 7: 运行持久层与 GC 测试**

Run:

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence -run "Purge|Quota|Blob" -count=1
go test ./internal/modules/filesystem/application/operation -run GC -count=1
```

Expected: PASS。

---

### Task 4: 删除、恢复、彻底删除和清空命令

**Files:**
- Modify: `backend/internal/modules/filesystem/application/command/node_operations.go`
- Create: `backend/internal/modules/filesystem/application/command/restore_node.go`
- Create: `backend/internal/modules/filesystem/application/command/restore_node_test.go`
- Create: `backend/internal/modules/filesystem/application/command/permanent_delete_node.go`
- Create: `backend/internal/modules/filesystem/application/command/permanent_delete_node_test.go`
- Create: `backend/internal/modules/filesystem/application/command/empty_trash.go`
- Create: `backend/internal/modules/filesystem/application/command/empty_trash_test.go`
- Modify: `backend/internal/modules/filesystem/domain/event/events.go`
- Modify: `backend/internal/modules/filesystem/domain/event/events_test.go`
- Modify: `backend/internal/modules/filesystem/domain/errors.go`

**Interfaces:**
- Produces: `RestoreNodeHandler.Handle`, `PermanentDeleteNodeHandler.Handle`、`EmptyTrashHandler.Handle`。
- Produces: `NodeRestored`、`NodePurged` 领域事件。

- [ ] **Step 1: 写命令层失败测试**

覆盖：

- trash 写入删除操作者和时间；
- restore 对父目录写权限拒绝；
- restore 同名冲突；
- permanent delete 幂等与保留策略；
- empty trash 返回逐项成功/失败且重放结果稳定。

- [ ] **Step 2: 运行定向测试确认失败**

Run:

```powershell
go test ./internal/modules/filesystem/application/command -run "Trash|RestoreNode|PermanentDelete|EmptyTrash" -count=1
```

Expected: FAIL。

- [ ] **Step 3: 更新 TrashNodeHandler**

调用：

```go
h.repo.TrashNode(txCtx, node.ID, time.Now().UTC(), cmd.ActorID)
```

保留 operation `"trash"`，并继续发布 `NodeTrashed`。

- [ ] **Step 4: 实现 RestoreNodeHandler**

命令：

```go
type RestoreNode struct {
	CommandID, ActorID, OrganizationID, WorkspaceID, NamespaceID, NodeID uuid.UUID
	Authorization
}
```

使用 operation `"restore_node"`，不能与版本恢复的 `"restore"` 共用。成功后发布 `NodeRestored`。

- [ ] **Step 5: 实现 PermanentDeleteNodeHandler**

使用 operation `"permanent_delete"`，验证 `TRASHED` 状态和 DELETE 权限，调用 `PurgeTrashedSubtree`，发布 `NodePurged`。

- [ ] **Step 6: 实现 EmptyTrashHandler**

返回：

```go
type EmptyTrashItemResult struct {
	NodeID uuid.UUID
	Error  error
}
```

批次命令以批次 `CommandID` 和入口 `NodeID` 派生稳定子命令 ID：

```go
childCommandID := uuid.NewSHA1(
	cmd.CommandID,
	[]byte("empty-trash:"+entry.Node.ID.String()),
)
```

每个入口独立事务和鉴权，并调用 `PermanentDeleteNodeHandler`，从而保留部分成功。重放批次时，已完成子命令由现有 `command_dedup` 返回成功，不重复扣配额、Blob 引用或发布事件。

- [ ] **Step 7: 注册并验证领域事件**

在 `events.go` 增加：

```go
const NodeRestoredName = "filesystem.node-restored.v1"
const NodePurgedName = "filesystem.node-purged.v1"
```

注册测试必须 round-trip 编解码。

- [ ] **Step 8: 运行命令和事件测试**

Run:

```powershell
go test ./internal/modules/filesystem/application/command -run "Trash|RestoreNode|PermanentDelete|EmptyTrash" -count=1
go test ./internal/modules/filesystem/domain/event -count=1
```

Expected: PASS。

---

### Task 5: 后端 HTTP、模块装配与 OpenAPI

**Files:**
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler_test.go`
- Modify: `backend/internal/modules/filesystem/module.go`
- Modify: `backend/api/openapi/filesystem.yaml`
- Modify: `backend/api/openapi/filesystem_test.go`

**Interfaces:**
- Produces:
  - `GET /api/v1/workspaces/:wsId/files/trash`
  - `POST /api/v1/workspaces/:wsId/files/:nodeId/restore`
  - `DELETE /api/v1/workspaces/:wsId/files/:nodeId/permanent`
  - `DELETE /api/v1/workspaces/:wsId/files/trash`

- [ ] **Step 1: 写四个路由及错误映射失败测试**

断言：

- 列表要求 `PermFileRead`；
- restore 要求原父目录 `WRITE`；
- permanent/empty 要求 `PermFileDelete`；
- 同名恢复返回 `409 FILESYSTEM_RESTORE_NAME_CONFLICT`；
- 父目录不可用返回 `409 FILESYSTEM_RESTORE_PARENT_UNAVAILABLE`；
- permanent 成功返回 204；
- empty 返回逐项结果。

- [ ] **Step 2: 运行 HTTP 测试确认路由不存在**

Run:

```powershell
go test ./internal/modules/filesystem/interface/http -run "Trash|RestoreNode|Permanent|EmptyTrash" -count=1
```

Expected: FAIL，404 或 handler 未定义。

- [ ] **Step 3: 装配 Handler 字段与路由**

在静态 `/files/trash` 路由注册后再注册动态 `/:nodeId` 路由，避免路由歧义：

```go
base.GET("/files/trash", authz.RequirePermission(permissions, authz.PermFileRead), h.ListTrash)
base.DELETE("/files/trash", authz.RequirePermission(permissions, authz.PermFileDelete), requireIdempotencyKey(), h.EmptyTrash)
base.POST("/files/:nodeId/restore", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.RestoreNode)
base.DELETE("/files/:nodeId/permanent", authz.RequirePermission(permissions, authz.PermFileDelete), requireIdempotencyKey(), h.PermanentDeleteNode)
```

- [ ] **Step 4: 实现稳定错误映射**

增加独立领域错误并映射：

```go
case errors.Is(err, domain.ErrRestoreNameConflict):
	httpx.Fail(c, "FILESYSTEM_RESTORE_NAME_CONFLICT", "original directory already contains an item with this name", http.StatusConflict)
case errors.Is(err, domain.ErrRestoreParentUnavailable):
	httpx.Fail(c, "FILESYSTEM_RESTORE_PARENT_UNAVAILABLE", "original parent directory is unavailable", http.StatusConflict)
case errors.Is(err, domain.ErrRetentionHold):
	httpx.Fail(c, "FILESYSTEM_RETENTION_HOLD", "filesystem item is protected by retention policy", http.StatusConflict)
```

- [ ] **Step 5: 在 module.go 注入新处理器**

按现有 command/query 构造方式创建并传入：

```go
listTrash := query.NewListTrashHandler(repo)
restoreNode := command.NewRestoreNodeHandler(repo, tx, bus, auditLogger)
permanentDelete := command.NewPermanentDeleteNodeHandler(repo, tx, bus, auditLogger)
emptyTrash := command.NewEmptyTrashHandler(repo, tx, bus, auditLogger)
```

- [ ] **Step 6: 更新 OpenAPI**

为四个端点定义分页、`TrashItem` 和批次结果 schema；修改类接口必须声明 `Idempotency-Key`。

- [ ] **Step 7: 验证 HTTP 与 OpenAPI**

Run:

```powershell
go test ./internal/modules/filesystem/interface/http -run "Trash|RestoreNode|Permanent|EmptyTrash" -count=1
go test ./api/openapi -count=1
```

Expected: PASS。

---

### Task 6: 前端契约与完整 BFF/SDK 链路

**Files:**
- Modify: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/contracts/src/filesystem/index.test.ts`
- Modify: `fronted/packages/upstream/src/filesystem.ts`
- Modify: `fronted/packages/upstream/src/filesystem.test.ts`
- Modify: `fronted/packages/bff-modules/src/filesystem/index.ts`
- Modify: `fronted/packages/bff-modules/src/filesystem/index.test.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.test.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/trash/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/restore/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/permanent/route.ts`
- Modify: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/filesystem-forwarding.test.ts`

**Interfaces:**
- Produces:

```ts
type TrashItemVM = {
  node: FileNodeVM;
  original_path: string;
  deleted_at: string;
  deleted_by?: string;
  can_restore: boolean;
  can_permanently_delete: boolean;
};

listTrashItems(workspaceId, query): Promise<TrashItemPageVM>
restoreFileNode(workspaceId, nodeId, namespaceId): Promise<void>
purgeFileNode(workspaceId, nodeId, namespaceId): Promise<void>
emptyTrash(workspaceId, namespaceId): Promise<FileMutationBatchResultVM>
```

- [ ] **Step 1: 写 contracts 失败测试**

测试合法 trash page 和非法日期/非法节点状态：

```ts
expect(TrashItemPageVMSchema.parse(validPage).items[0]?.node.state).toBe("TRASHED");
expect(() => TrashItemVMSchema.parse({ ...validItem, deleted_at: "not-a-date" })).toThrow();
```

- [ ] **Step 2: 实现 Zod schema**

增加：

```ts
export const TrashItemVMSchema = z.object({
  node: FileNodeVMSchema,
  original_path: z.string().min(1),
  deleted_at: z.string().datetime({ offset: true }),
  deleted_by: UUIDSchema.optional(),
  can_restore: z.boolean(),
  can_permanently_delete: z.boolean(),
});
```

以及 `TrashItemPageVMSchema`、`FileMutationItemResultSchema`、`FileMutationBatchResultVMSchema`。

- [ ] **Step 3: 写 upstream、BFF module、SDK 和 route 失败测试**

逐层断言精确 URL、HTTP 方法、`namespace_id`、`Idempotency-Key` 与 schema parse。

- [ ] **Step 4: 实现 upstream**

新增 `listTrash`、`restoreFileNode`、`purgeFileNode`、`emptyTrash`，复用现有 `filesQuery` 和 mutation header 模式。

- [ ] **Step 5: 实现 BFF module**

只做参数透传和 schema parse，不在 BFF 重写业务错误。

- [ ] **Step 6: 实现 Next BFF routes**

每个 mutation route 必须校验并转发 `Idempotency-Key`；静态 trash route 同时导出 `GET` 和 `DELETE`。

- [ ] **Step 7: 实现 web SDK**

使用：

```ts
mutationIdempotencyKey("restore")
mutationIdempotencyKey("permanent-delete")
mutationIdempotencyKey("empty-trash")
```

错误继续由 `BffError` 暴露稳定 code。

- [ ] **Step 8: 运行链路测试**

Run:

```powershell
cd a:\21-Agent\SparkOpsAgent\fronted
pnpm exec vitest run packages/contracts/src/filesystem/index.test.ts
pnpm exec vitest run packages/upstream/src/filesystem.test.ts
pnpm exec vitest run packages/bff-modules/src/filesystem/index.test.ts
pnpm exec vitest run packages/web-sdk/src/filesystem.test.ts
pnpm exec vitest run "apps/web/app/api/bff/v1/workspaces/[wsId]/filesystem-forwarding.test.ts"
```

Expected: PASS。

---

### Task 7: 回收站页面、导航与批量交互

**Files:**
- Create: `fronted/apps/web/app/(platform)/files/data/trash/page.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/TrashExplorerPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/TrashExplorerPanel.test.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.test.ts`
- Modify: `fronted/apps/web/src/server/loaders/filesystem.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/DataPlaneModuleNav.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/DataPlaneModuleNav.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileBrowser.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileBrowser.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileListView.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileGridView.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-delete.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/file-explorer-panel-delete.test.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileExplorerPanel.tsx`

**Interfaces:**
- Produces: `/files/data/trash?space=workspace|personal|organization` 页面。
- Produces: 普通删除文案“移入回收站”。
- Produces: 回收站单项/多项恢复、彻底删除和清空操作。

- [ ] **Step 1: 写普通删除文案失败测试**

期望：

```ts
expect(deleteDialogCopy([{ name: "a.txt", type: "FILE" }])).toEqual({
  title: "移入回收站",
  description: "将文件「a.txt」移入回收站。之后可从回收站恢复。",
});
```

- [ ] **Step 2: 更新普通删除文案并运行测试**

Run:

```powershell
pnpm exec vitest run apps/web/src/modules/filesystem/components/file-explorer-panel-delete.test.ts
```

Expected: PASS。

- [ ] **Step 3: 写批量操作失败测试**

`runTrashBatch` 必须顺序调用 SDK，并返回：

```ts
{
  succeeded: ["node-1"],
  failures: [{ nodeId: "node-2", message: "原目录已有同名项目" }],
}
```

成功项从列表移除，失败项保持选择。

- [ ] **Step 4: 实现批量操作 helper**

使用单一函数：

```ts
export async function runTrashBatch(
  nodeIds: string[],
  mutate: (nodeId: string) => Promise<void>,
): Promise<FileMutationBatchResult>
```

不得为 restore 和 purge 复制两份循环。

- [ ] **Step 5: 写 TrashExplorerPanel 失败测试**

覆盖：

- 空态；
- 加载错误与重试；
- 单项恢复；
- 同名恢复错误文案；
- 彻底删除二次确认；
- 清空回收站二次确认；
- 批量部分失败；
- trash 模式禁用重命名、移动和拖拽。

- [ ] **Step 6: 扩展 FileBrowser 的明确模式**

新增：

```ts
type FileBrowserMode = "browse" | "trash";
```

`trash` 模式允许选择 `TRASHED`，但不暴露 rename/move/share/open revision 操作；列表展示原路径和删除时间。

- [ ] **Step 7: 实现 TrashExplorerPanel**

Panel 接收服务端初始页和 SDK mutation。成功后调用 `router.refresh()`；部分失败时只移除成功项并显示失败列表。

- [ ] **Step 8: 增加 loader、页面和导航**

`loadTrashItems` 复用现有 namespace 解析。`DataPlaneModuleNav` 新增：

```ts
{ href: "/files/data/trash", label: "回收站" }
```

页面保留 `FileSpaceTabList`，隐藏目录树。

- [ ] **Step 9: 运行组件测试**

Run:

```powershell
pnpm exec vitest run apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.test.ts
pnpm exec vitest run apps/web/src/modules/filesystem/components/TrashExplorerPanel.test.tsx
pnpm exec vitest run apps/web/src/modules/filesystem/components/FileBrowser.test.tsx
pnpm exec vitest run apps/web/src/modules/filesystem/components/DataPlaneModuleNav.test.tsx
```

Expected: PASS。

---

### Task 8: 集成验证、服务重启与浏览器验收

**Files:**
- Modify only if verification exposes defects in files changed by Tasks 1–7.

**Interfaces:**
- Verifies the full path: 上传 → 移入回收站 → 同名重新上传 → 恢复冲突 → 彻底删除。

- [ ] **Step 1: 格式化**

Run:

```powershell
cd a:\21-Agent\SparkOpsAgent
$files = git diff --name-only -- '*.go'
if ($files) { gofmt -w $files }
```

执行前检查 `$files`，仅保留本功能涉及的 Go 文件；不得格式化其他会话修改的文件。

- [ ] **Step 2: 运行后端文件系统测试**

Run:

```powershell
go test ./internal/modules/filesystem/... -count=1
go test ./migrations ./api/openapi -count=1
```

Expected: PASS。

- [ ] **Step 3: 运行前端定向测试与类型检查**

Run:

```powershell
cd a:\21-Agent\SparkOpsAgent\fronted
pnpm exec vitest run packages/contracts/src/filesystem/index.test.ts packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts
pnpm exec vitest run apps/web/src/modules/filesystem/components/file-explorer-panel-delete.test.ts apps/web/src/modules/filesystem/components/file-explorer-panel-trash-ops.test.ts apps/web/src/modules/filesystem/components/TrashExplorerPanel.test.tsx apps/web/src/modules/filesystem/components/FileBrowser.test.tsx apps/web/src/modules/filesystem/components/DataPlaneModuleNav.test.tsx
pnpm exec tsc -p apps/web/tsconfig.json --noEmit
```

Expected: PASS。

- [ ] **Step 4: 运行仓库检查**

Run:

```powershell
cd a:\21-Agent\SparkOpsAgent
npm run check
```

Expected: 无 error、warning、info。

- [ ] **Step 5: 重启后端**

Run:

```powershell
.\scripts\dev-restart.ps1 -Backend
```

Expected: backend、gateway、worker 均启动并监听。

- [ ] **Step 6: 浏览器验收关键路径**

1. 上传 `a.txt`；
2. 点击“移入回收站”；
3. 在同目录再次上传 `a.txt`，确认成功；
4. 打开“回收站”；
5. 恢复旧 `a.txt`，确认显示同名冲突；
6. 彻底删除旧 `a.txt`；
7. 刷新页面，确认旧项目不再出现且新 `a.txt` 仍可打开。

- [ ] **Step 7: 检查最终差异**

Run:

```powershell
git status --short
git diff --check
```

确认没有回滚或覆盖本会话开始前已存在的未提交改动，并保持所有文件未提交。
