# Unified File Infrastructure G1 Kernel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可独立使用的文件内核：Namespace、目录、不可变 Revision、Blob/Replica、Local Provider、配额、资源授权、审计和基础 REST API。

**Architecture:** 新增 `filesystem` BC，使用 PostgreSQL `filesystem` schema 保存唯一元数据事实；Local Adapter 实现统一 Storage Provider Contract。写操作在事务中修改聚合、配额并发布 Outbox 事件。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL 15、现有 authz/entitlements/audit/messaging、Local/PVC。

## Global Constraints

- 继承 Master Plan 的全部 Global Constraints。
- 下一 SQL migration 从 `000077` 开始。
- G1 不实现分片上传、S3、NFS、扫描、生命周期和跨存储迁移。
- G1 文件内容通过 Local Provider 保存，但元数据模型必须与 G2/G3 兼容。
- Node 路径不是跨模块身份；Revision 创建后不可变。

---

### Task 1: 建立 filesystem schema 与领域不变量

**Files:**
- Create: `backend/migrations/000077_filesystem_kernel.up.sql`
- Create: `backend/migrations/000077_filesystem_kernel.down.sql`
- Create: `backend/internal/modules/filesystem/domain/model/types.go`
- Create: `backend/internal/modules/filesystem/domain/model/node.go`
- Create: `backend/internal/modules/filesystem/domain/model/revision.go`
- Create: `backend/internal/modules/filesystem/domain/model/node_test.go`

**Interfaces:**
- Consumes: `uuid.UUID`、现有 Workspace/Member UUID。
- Produces: `model.Node`, `model.FileRevision`, `model.Blob`, `model.StorageReplica` 及状态常量。

- [ ] **Step 1: 写失败的领域测试**

```go
func TestNodeRenamePreservesIdentity(t *testing.T) {
	node := model.NewFile(uuid.New(), uuid.New(), "report.csv", uuid.New())
	id := node.ID

	require.NoError(t, node.Rename("final.csv"))
	assert.Equal(t, id, node.ID)
	assert.Equal(t, "final.csv", node.Name)
	assert.Equal(t, "final.csv", node.NormalizedName)
}

func TestRevisionContentIsImmutable(t *testing.T) {
	revision := model.NewRevision(uuid.New(), 1, uuid.New(), 42, "text/plain", uuid.New())
	assert.Equal(t, int64(42), revision.Size)
	assert.Equal(t, 1, revision.RevisionNo)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```powershell
cd backend
go test ./internal/modules/filesystem/domain/model -run "TestNode|TestRevision" -count=1
```

Expected: FAIL，package 或构造函数未定义。

- [ ] **Step 3: 实现最小领域类型**

```go
type NodeType string

const (
	NodeTypeFile      NodeType = "FILE"
	NodeTypeDirectory NodeType = "DIRECTORY"
)

type Node struct {
	ID                uuid.UUID
	NamespaceID       uuid.UUID
	ParentID          *uuid.UUID
	Type              NodeType
	Name              string
	NormalizedName    string
	CurrentRevisionID *uuid.UUID
	State             string
	OwnerID           uuid.UUID
}

func (n *Node) Rename(name string) error {
	normalized := strings.TrimSpace(norm.NFC.String(name))
	if normalized == "" || strings.ContainsAny(normalized, `/\`) {
		return ErrInvalidNodeName
	}
	n.Name = name
	n.NormalizedName = strings.ToLower(normalized)
	return nil
}
```

Revision 只暴露构造函数和只读字段，不提供修改 Blob、Size、MIME 的方法。

- [ ] **Step 4: 编写 kernel DDL**

DDL 必须创建：

```sql
CREATE SCHEMA IF NOT EXISTS filesystem;

CREATE TABLE filesystem.namespaces (
  id uuid PRIMARY KEY,
  organization_id uuid NOT NULL,
  workspace_id uuid NOT NULL UNIQUE,
  root_node_id uuid,
  status varchar(32) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE filesystem.nodes (
  id uuid PRIMARY KEY,
  namespace_id uuid NOT NULL REFERENCES filesystem.namespaces(id),
  parent_id uuid REFERENCES filesystem.nodes(id),
  node_type varchar(16) NOT NULL CHECK (node_type IN ('FILE','DIRECTORY')),
  name text NOT NULL,
  normalized_name text NOT NULL,
  current_revision_id uuid,
  state varchar(32) NOT NULL,
  owner_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE(namespace_id, parent_id, normalized_name)
);
```

同一 migration 继续创建 `file_revisions`、`blobs`、`storage_replicas`、`quota_accounts`、`quota_reservations`、`resource_grants`、`file_links`。外键添加顺序必须避免 Node 与 Revision 的循环依赖：先建表，再 `ALTER TABLE nodes ADD CONSTRAINT ... current_revision_id`。

- [ ] **Step 5: 运行领域和 migration 检查**

```powershell
go test ./internal/modules/filesystem/domain/model ./migrations/... -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add backend/migrations/000077_filesystem_kernel.* backend/internal/modules/filesystem/domain/model
git commit -m "feat(filesystem): add kernel schema and domain models"
```

### Task 2: 实现仓储与 AutoMigrate

**Files:**
- Create: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Create: `backend/internal/modules/filesystem/infrastructure/persistence/models.go`
- Create: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Create: `backend/internal/modules/filesystem/infrastructure/persistence/migrate.go`
- Create: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo_test.go`
- Modify: `backend/internal/bootstrap/migrate.go`

**Interfaces:**
- Consumes: Task 1 的领域类型。
- Produces: `repository.FilesystemRepository` 与 `persistence.AutoMigrate`.

- [ ] **Step 1: 写仓储契约测试**

```go
func TestCreateChildRejectsDuplicateNormalizedName(t *testing.T) {
	repo, db := newTestRepository(t)
	ctx := context.Background()
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)

	require.NoError(t, repo.CreateNode(ctx, model.NewFile(ns.ID, parent.ID, "Report.csv", uuid.New())))
	err := repo.CreateNode(ctx, model.NewFile(ns.ID, parent.ID, "report.CSV", uuid.New()))
	require.ErrorIs(t, err, domain.ErrNodeNameConflict)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence -run TestCreateChildRejectsDuplicateNormalizedName -count=1
```

Expected: FAIL。

- [ ] **Step 3: 定义仓储接口**

```go
type FilesystemRepository interface {
	CreateNamespace(ctx context.Context, namespace *model.Namespace) error
	GetNamespaceByWorkspace(ctx context.Context, workspaceID uuid.UUID) (*model.Namespace, error)
	CreateNode(ctx context.Context, node *model.Node) error
	GetNode(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error)
	ListChildren(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, limit int, cursor string) ([]model.Node, string, error)
	SaveRevision(ctx context.Context, revision *model.FileRevision, blob *model.Blob, replica *model.StorageReplica) error
	SetCurrentRevision(ctx context.Context, nodeID, revisionID uuid.UUID) error
	MoveNode(ctx context.Context, nodeID uuid.UUID, parentID *uuid.UUID, normalizedName string) error
	TrashNode(ctx context.Context, nodeID uuid.UUID) error
}
```

- [ ] **Step 4: 实现 GORM 映射和错误转换**

`filesystem_repo.go` 必须把 PostgreSQL unique violation 转成 `domain.ErrNodeNameConflict`，not found 转成 `domain.ErrNodeNotFound`，不得向 application 泄漏 GORM 错误类型。

- [ ] **Step 5: 注册 AutoMigrate**

在 `bootstrap/migrate.go` 的 steps 中加入：

```go
{"filesystem", filesystempersistence.AutoMigrate},
```

`AutoMigrate` 必须先执行 `CREATE SCHEMA IF NOT EXISTS filesystem`，再迁移 GORM model。

- [ ] **Step 6: 运行测试**

```powershell
go test ./internal/modules/filesystem/infrastructure/persistence ./internal/bootstrap/... -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add backend/internal/modules/filesystem backend/internal/bootstrap/migrate.go
git commit -m "feat(filesystem): add kernel persistence"
```

### Task 3: 建立 Storage Provider Contract 与 Local Adapter

**Files:**
- Create: `backend/internal/platform/storage/provider.go`
- Create: `backend/internal/platform/storage/capability.go`
- Create: `backend/internal/platform/storage/contracttest/suite.go`
- Create: `backend/internal/platform/storage/adapters/local/provider.go`
- Create: `backend/internal/platform/storage/adapters/local/provider_test.go`

**Interfaces:**
- Consumes: `context.Context`, `io.Reader`, `io.ReadCloser`。
- Produces: `storage.Provider`、`storage.Capabilities`、`local.New(root string)`.

- [ ] **Step 1: 写 Local Provider contract 测试**

```go
func TestLocalProviderContract(t *testing.T) {
	root := t.TempDir()
	provider, err := local.New(root)
	require.NoError(t, err)
	contracttest.Run(t, provider)
}
```

Contract suite 必须覆盖 Put/Get/Range/Stat/Delete、重复 Put、取消 Context、Checksum 和路径逃逸。

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/platform/storage/adapters/local -count=1
```

Expected: FAIL。

- [ ] **Step 3: 定义 Provider**

```go
type Provider interface {
	ID() string
	Capabilities() Capabilities
	Put(ctx context.Context, req PutRequest) (ObjectInfo, error)
	Get(ctx context.Context, req GetRequest) (io.ReadCloser, ObjectInfo, error)
	Stat(ctx context.Context, objectKey string) (ObjectInfo, error)
	Delete(ctx context.Context, objectKey string) error
	Copy(ctx context.Context, sourceKey, destinationKey string) (ObjectInfo, error)
}

type PutRequest struct {
	ObjectKey string
	Body      io.Reader
	Size      int64
	SHA256    string
}
```

- [ ] **Step 4: 实现 Local Adapter**

物理路径必须使用系统生成的 object key，并在 `filepath.Clean` 后验证仍位于 root 下。写入流程使用同目录临时文件、`fsync`、Checksum 校验和原子 rename。

- [ ] **Step 5: 运行 contract suite**

```powershell
go test ./internal/platform/storage/... -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add backend/internal/platform/storage
git commit -m "feat(storage): add provider contract and local adapter"
```

### Task 4: 接入权限码与 Entitlement

**Files:**
- Modify: `backend/internal/platform/authz/permissions.go`
- Modify: `backend/internal/platform/entitlements/entitlements.go`
- Modify: `backend/internal/platform/entitlements/api_paths.go`
- Modify: `backend/internal/platform/entitlements/api_paths_test.go`
- Modify: `fronted/packages/contracts/src/platform/index.ts`
- Modify: `fronted/packages/bff-core/src/entitlements.ts`
- Modify: `fronted/packages/bff-core/src/entitlements.test.ts`

**Interfaces:**
- Consumes: 现有 Workspace Role、APIPathEntitlement。
- Produces: `PermFileRead` 等权限码和 `filesystem` 模块开关。

- [ ] **Step 1: 写失败测试**

```go
func TestFilesystemAPIPathEntitlement(t *testing.T) {
	assert.Equal(t, "filesystem", APIPathEntitlement("/api/v1/workspaces/ws-1/files"))
	assert.Equal(t, "filesystem", APIPathEntitlement("/api/v1/workspaces/ws-1/namespaces"))
	assert.Equal(t, "filesystem", APIPathEntitlement("/api/v1/workspaces/ws-1/transfers"))
}
```

```ts
expect(apiPathEntitlement("/api/v1/workspaces/ws/files")).toBe("filesystem");
expect(isModuleEnabled({ ...DEFAULT_ENTITLEMENTS, filesystem: false }, "filesystem")).toBe(false);
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/platform/entitlements -count=1
cd ..\fronted
pnpm vitest run packages/bff-core/src/entitlements.test.ts
```

Expected: FAIL。

- [ ] **Step 3: 增加权限码**

```go
const (
	PermFileRead       = "file:read"
	PermFileWrite      = "file:write"
	PermFileDelete     = "file:delete"
	PermFileShare      = "file:share"
	PermFileAdmin      = "file:admin"
	PermStorageRead    = "storage:read"
	PermStorageAdmin   = "storage:admin"
	PermLifecycleAdmin = "lifecycle:admin"
)
```

将文件读写权限加入默认 admin/developer，storage/lifecycle admin 仅加入 admin。

- [ ] **Step 4: 增加 Entitlement 字段**

Go `WorkspaceEntitlements` 和 TS `WorkspaceEntitlementsVMSchema` 都增加：

```text
filesystem: boolean
```

默认启用，以保持现有私有化部署的开箱行为；明确禁用时返回 `ENTITLEMENT_MODULE_DISABLED`。

- [ ] **Step 5: 运行测试**

```powershell
cd backend
go test ./internal/platform/authz ./internal/platform/entitlements -count=1
cd ..\fronted
pnpm vitest run packages/bff-core/src/entitlements.test.ts
pnpm typecheck
```

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add backend/internal/platform/authz backend/internal/platform/entitlements fronted/packages/contracts/src/platform fronted/packages/bff-core/src
git commit -m "feat(filesystem): add permissions and entitlement"
```

### Task 5: 实现 Namespace、目录和本地文件写入用例

**Files:**
- Create: `backend/internal/modules/filesystem/domain/event/events.go`
- Create: `backend/internal/modules/filesystem/application/command/create_namespace.go`
- Create: `backend/internal/modules/filesystem/application/command/create_directory.go`
- Create: `backend/internal/modules/filesystem/application/command/put_local_file.go`
- Create: `backend/internal/modules/filesystem/application/command/rename_node.go`
- Create: `backend/internal/modules/filesystem/application/command/move_node.go`
- Create: `backend/internal/modules/filesystem/application/command/copy_node.go`
- Create: `backend/internal/modules/filesystem/application/command/trash_node.go`
- Create: `backend/internal/modules/filesystem/application/command/restore_revision.go`
- Create: `backend/internal/modules/filesystem/application/query/list_children.go`
- Create: `backend/internal/modules/filesystem/application/query/list_revisions.go`
- Create: `backend/internal/modules/filesystem/application/command/put_local_file_test.go`
- Create: `backend/internal/modules/filesystem/application/command/node_operations_test.go`

**Interfaces:**
- Consumes: `repository.FilesystemRepository`, `storage.Provider`, `postgres.Transactor`, `events.Bus`.
- Produces: `CreateDirectoryHandler`, `PutLocalFileHandler`, `ListChildrenHandler`.

- [ ] **Step 1: 写原子性失败测试**

测试必须模拟 `bus.Publish` 失败，并断言 Node、Revision、Quota 均未提交。

```go
func TestPutLocalFileRollsBackWhenEventPublishFails(t *testing.T) {
	h := newPutHandler(t, failingBus{})
	_, err := h.Handle(context.Background(), command.PutLocalFile{ /* fixed fixture */ })
	require.Error(t, err)
	assertDatabaseUnchanged(t)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/command -run TestPutLocalFileRollsBackWhenEventPublishFails -count=1
```

Expected: FAIL。

- [ ] **Step 3: 注册版本化事件**

```go
const (
	FileCreatedName         = "filesystem.file.created.v1"
	FileRevisionCreatedName = "filesystem.file.revision-created.v1"
)

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[FileCreated](
		registry,
		FileCreatedName,
		1,
		func(ev FileCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	); err != nil {
		return err
	}
	return events.RegisterJSON[FileRevisionCreated](
		registry,
		FileRevisionCreatedName,
		1,
		func(ev FileRevisionCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	)
}
```

- [ ] **Step 4: 实现文件操作与事务用例**

`PutLocalFileHandler.Handle` 顺序固定为：授权前置结果 → Local 临时写与校验 → `WithinTransaction` 创建 Blob/Replica/Revision → 更新 Node current revision → 更新配额 → `bus.Publish(txCtx, event)`。事务失败时删除临时对象或留下带 Session 前缀的可回收对象，不暴露 Revision。

同时实现：

- Rename：只修改 `name/normalized_name`，File ID 不变。
- Move：仅允许同 Namespace 原子修改 `parent_id`；防止目录移动到自身后代。
- Copy：创建新 Node 和 Revision 引用；同组织可复用 Blob。
- Trash：Node 进入 `TRASHED`，不立即删除 Revision/Blob。
- Restore Revision：原子切换 `current_revision_id`，不修改历史 Revision。
- List Revisions：按 `revision_no DESC` 游标分页。

每个成功或拒绝的 mutation 调用现有 Audit Service，记录 actor、workspace、action、resource、decision、reason 和 trace ID；不得记录正文或物理 object key。

- [ ] **Step 5: 写并运行操作与审计测试**

```go
func TestMoveRejectsDirectoryCycle(t *testing.T) {
	h := newMoveHandler(t)
	err := h.Handle(ctx, command.MoveNode{NodeID: parentID, NewParentID: childID})
	require.ErrorIs(t, err, domain.ErrDirectoryCycle)
}

func TestRejectedMutationIsAuditedWithoutContent(t *testing.T) {
	h, audit := newPutHandlerWithAudit(t)
	_, err := h.Handle(ctx, command.PutLocalFile{ /* quota exceeds fixture */ })
	require.Error(t, err)
	assert.Equal(t, "DENY", audit.Last().Decision)
	assert.NotContains(t, audit.Last().Detail, "fixture-content")
}
```

Run:

```powershell
go test ./internal/modules/filesystem/application/... -count=1
go test ./internal/platform/messaging/... -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add backend/internal/modules/filesystem/application backend/internal/modules/filesystem/domain/event
git commit -m "feat(filesystem): add namespace and local file use cases"
```

### Task 6: 暴露 REST、OpenAPI 与模块注册

**Files:**
- Create: `backend/internal/modules/filesystem/interface/http/handler.go`
- Create: `backend/internal/modules/filesystem/interface/http/handler_test.go`
- Create: `backend/internal/modules/filesystem/module.go`
- Modify: `backend/internal/bootstrap/app.go`
- Create: `backend/api/openapi/filesystem.yaml`
- Modify: `backend/api/openapi/openapi.yaml`

**Interfaces:**
- Consumes: Task 5 handlers、现有 `httpx`、`authz.RequirePermission`.
- Produces: `/api/v1/workspaces/:wsId/namespaces`, `/files`, `/directories`.

- [ ] **Step 1: 写 HTTP 鉴权测试**

```go
func TestCreateDirectoryRequiresFileWrite(t *testing.T) {
	router := newRouterWithPermissions(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws/directories", strings.NewReader(`{"name":"docs"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assert.Equal(t, http.StatusForbidden, res.Code)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/interface/http -count=1
```

Expected: FAIL。

- [ ] **Step 3: 注册路由**

```go
workspace := api.Group("/workspaces/:wsId")
workspace.Use(httpx.RequireWorkspace())
workspace.Use(httpx.RequireWorkspaceMember(c.WorkspaceMemberChecker))
workspace.GET("/namespaces", authz.RequirePermission(c.PermissionChecker, authz.PermFileRead), h.GetNamespace)
workspace.GET("/files", authz.RequirePermission(c.PermissionChecker, authz.PermFileRead), h.ListChildren)
workspace.POST("/directories", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.CreateDirectory)
workspace.POST("/files", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.PutLocalFile)
workspace.PATCH("/files/:nodeId/name", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.RenameNode)
workspace.POST("/files/:nodeId/move", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.MoveNode)
workspace.POST("/files/:nodeId/copy", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.CopyNode)
workspace.DELETE("/files/:nodeId", authz.RequirePermission(c.PermissionChecker, authz.PermFileDelete), h.TrashNode)
workspace.GET("/files/:nodeId/revisions", authz.RequirePermission(c.PermissionChecker, authz.PermFileRead), h.ListRevisions)
workspace.POST("/files/:nodeId/revisions/:revisionId/restore", authz.RequirePermission(c.PermissionChecker, authz.PermFileWrite), h.RestoreRevision)
```

- [ ] **Step 4: 注册 Module**

`module.Register` 必须：

1. 验证 `DB`、`Transactor`、`EventBus`、`PermissionChecker` 非空。
2. 注册 filesystem event codecs。
3. 构造 repository、Local Provider、command/query handlers。
4. 挂载 HTTP routes。

在 `bootstrap/app.go` 中放到 `governance.New(...)` 之后。

- [ ] **Step 5: 更新 OpenAPI**

为 Node、Revision、分页 cursor、稳定 error code 和 Idempotency-Key 建模。二进制内容端点不在 G1 暴露。

- [ ] **Step 6: 运行测试**

```powershell
go test ./internal/modules/filesystem/... ./internal/bootstrap/... -count=1
go test ./...
```

Expected: PASS。

- [ ] **Step 7: 重启后端并验证**

```powershell
cd ..
.\scripts\dev-restart.ps1 -Backend
Invoke-WebRequest http://localhost:8080/ready
```

Expected: readiness 成功。

- [ ] **Step 8: 提交**

```powershell
git add backend/internal/modules/filesystem backend/internal/bootstrap/app.go backend/api/openapi
git commit -m "feat(filesystem): expose kernel API"
```

### Task 7: G1 验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-g1.md`

**Interfaces:**
- Consumes: G1 全部实现。
- Produces: G2 可依赖的冻结 schema 和 API。

- [ ] **Step 1: 运行 G1 测试矩阵**

```powershell
cd backend
go test ./internal/modules/filesystem/... -count=1
go test ./internal/platform/storage/... -count=1
go test ./internal/platform/authz ./internal/platform/entitlements -count=1
go test ./internal/platform/messaging/... -count=1
go test ./...
```

Expected: PASS。

- [ ] **Step 2: 记录验收结果**

文档必须包含：

```markdown
- [x] Workspace namespace 自动创建且唯一
- [x] 同目录规范化名称冲突被拒绝
- [x] Revision 不可变
- [x] Local Provider 防路径逃逸并校验 SHA-256
- [x] 配额事务与 Outbox 失败时完整回滚
- [x] filesystem Entitlement 和 file:* 权限强制生效
```

- [ ] **Step 3: 提交 G1 证据**

```powershell
git add docs/verification/unified-file-infrastructure-g1.md
git commit -m "docs(filesystem): record G1 kernel verification"
```
