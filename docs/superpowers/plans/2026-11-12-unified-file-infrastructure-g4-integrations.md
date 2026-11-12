# Unified File Infrastructure G4 Integrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 File Explorer、Resource Grant、Agent Capability、Artifact、Knowledge 一次性迁移、MCP、SFTP、Preview 和搜索集成。

**Architecture:** 所有集成只引用 File ID/Artifact ID；控制与元数据经 BFF/Go API，字节流经 G2 数据面。Knowledge 使用 expand/precopy/verify/freeze/switch 流程迁移，旧 content 在观察期只读保留。

**Tech Stack:** Next.js 15、TypeScript、Zod、Go、Gin、PostgreSQL、RabbitMQ、SSH/SFTP、OpenSearch 可选。

## Global Constraints

- 继承 Master Plan 的全部 Global Constraints。
- G1–G3 必须通过并冻结 API。
- Knowledge 迁移校验未达到 100% 时禁止切换读取。
- Agent/Workflow 不得使用用户 JWT 或长期 API Token 访问文件。
- SFTP 只写 Landing Zone，不挂载生产对象目录。
- 搜索结果返回前必须再次执行资源授权。

---

### Task 1: 完成 filesystem 前端契约链与 File Explorer

**Files:**
- Modify: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/contracts/src/index.ts`
- Modify: `fronted/packages/contracts/package.json`
- Modify: `fronted/packages/upstream/src/filesystem.ts`
- Modify: `fronted/packages/upstream/src/index.ts`
- Modify: `fronted/packages/bff-modules/src/filesystem/index.ts`
- Modify: `fronted/packages/bff-modules/src/index.ts`
- Modify: `fronted/packages/bff-core/src/index.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/packages/web-sdk/src/index.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/namespaces/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/files/[nodeId]/revisions/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/directories/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/shares/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/artifacts/route.ts`
- Create: `fronted/apps/web/src/server/loaders/filesystem.ts`
- Create: `fronted/apps/web/app/(platform)/files/page.tsx`
- Create: `fronted/apps/web/app/(platform)/files/[nodeId]/page.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileExplorerPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/UploadPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/TransferProgress.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileExplorerPanel.test.tsx`

**Interfaces:**
- Consumes: G1–G3 REST/OpenAPI、G2 web-sdk transfer。
- Produces: `filesystem` browser SDK、RSC loader 和 File Explorer。

- [ ] **Step 1: 写 UI 失败测试**

```tsx
it("keeps file identity stable after rename", async () => {
  render(<FileExplorerPanel initialNodes={[fileNode({ id: "file-1", name: "a.txt" })]} />);
  await userEvent.click(screen.getByRole("button", { name: "重命名" }));
  await userEvent.type(screen.getByLabelText("新名称"), "b.txt");
  await userEvent.click(screen.getByRole("button", { name: "保存" }));
  expect(mockRename).toHaveBeenCalledWith("file-1", "b.txt");
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/FileExplorerPanel.test.tsx
```

Expected: FAIL。

- [ ] **Step 3: 定义 contracts**

必须导出：

```ts
export const FileNodeVMSchema = z.object({
  id: z.string().uuid(),
  namespaceId: z.string().uuid(),
  parentId: z.string().uuid().nullable(),
  type: z.enum(["FILE", "DIRECTORY"]),
  name: z.string(),
  currentRevisionId: z.string().uuid().nullable(),
  size: z.number().int().nonnegative().nullable(),
  mimeType: z.string().nullable(),
  state: z.string(),
  updatedAt: z.string(),
});
```

同时定义分页、Revision、Transfer、Storage Pool、Artifact 和 Share VM。

- [ ] **Step 4: 实现完整调用链**

`createFilesystemUpstream` → `bff-modules/filesystem` → Route Handler → `web-sdk/filesystem`。BFF context 增加 `upstream.filesystem`，禁止 Client Component import upstream。

- [ ] **Step 5: 实现 File Explorer**

支持：面包屑、分页列表、创建目录、上传、下载、移动、复制、重命名、回收站、版本列表、恢复和搜索。所有 mutation 使用 Node ID；路径只用于展示。

- [ ] **Step 6: 运行测试并提交**

```powershell
pnpm vitest run apps/web/src/modules/filesystem packages/bff-core/src/entitlements.test.ts
pnpm typecheck
git add packages apps/web
git commit -m "feat(filesystem): add file explorer and BFF integration"
```

### Task 2: 增加导航、菜单和 Entitlement UI

**Files:**
- Modify: `fronted/apps/web/src/components/layout/nav-tree.ts`
- Modify: `fronted/apps/web/src/components/layout/nav-tree.test.ts`
- Modify: `fronted/apps/web/src/components/layout/menu-tree-nav.ts`
- Modify: `fronted/apps/web/src/components/layout/menu-tree-nav.test.ts`
- Modify: `fronted/apps/web/src/components/layout/entitlements.ts`
- Modify: `fronted/apps/web/src/components/layout/route-labels.ts`
- Modify: `fronted/apps/web/src/components/layout/route-labels.test.ts`
- Modify: `fronted/apps/web/src/components/command-search-index.ts`

**Interfaces:**
- Consumes: `filesystem` entitlement。
- Produces: `/files` 导航与命令搜索入口。

- [ ] **Step 1: 写隐藏入口测试**

```ts
it("hides files navigation when filesystem entitlement is disabled", () => {
  const items = buildNavTree({ ...entitlements(), filesystem: false });
  expect(items.some((item) => item.href === "/files")).toBe(false);
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
pnpm vitest run apps/web/src/components/layout/nav-tree.test.ts apps/web/src/components/layout/menu-tree-nav.test.ts
```

Expected: FAIL。

- [ ] **Step 3: 实现导航**

新增 `FILES_NAV_ITEM`，模块 key 为 `filesystem`，路由标签为“文件”，但后端仍是安全主闸。

- [ ] **Step 4: 运行测试并提交**

```powershell
pnpm vitest run apps/web/src/components/layout
git add apps/web/src/components
git commit -m "feat(filesystem): add navigation and entitlement UI"
```

### Task 3: 实现 Resource Grant 与 Agent Capability

**Files:**
- Create: `backend/internal/modules/filesystem/domain/service/authorization.go`
- Create: `backend/internal/modules/filesystem/domain/service/authorization_test.go`
- Create: `backend/internal/modules/filesystem/application/command/grant_resource.go`
- Create: `backend/internal/modules/filesystem/application/command/issue_capability.go`
- Create: `backend/internal/modules/filesystem/application/command/revoke_capability.go`
- Create: `backend/internal/modules/filesystem/application/command/start_controlled_export.go`
- Create: `backend/internal/modules/filesystem/application/operation/import_exported_file.go`
- Create: `backend/internal/modules/filesystem/infrastructure/capability/signer.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Modify: `backend/api/openapi/filesystem.yaml`

**Interfaces:**
- Consumes: Workspace RBAC/Casbin/OPA checker、Resource Grant repository。
- Produces: `AuthorizationService.Decide`, `CapabilityIssuer.Issue`.

- [ ] **Step 1: 写显式拒绝和范围交集测试**

```go
func TestExplicitDenyOverridesInheritedAllow(t *testing.T) {
	decision := service.Decide(subject, actionRead, childFile,
		inheritedAllow(parentDirectory), explicitDeny(childFile))
	assert.Equal(t, service.DecisionDeny, decision.Result)
}

func TestCapabilityIsIntersectionNotDelegatorUnion(t *testing.T) {
	token, err := issuer.Issue(ctx, requestForResources(fileA, fileB), runAllows(fileA))
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{fileA}, token.ResourceIDs)
}

func TestCrossOrganizationGrantIsRejected(t *testing.T) {
	err := grantHandler.Handle(ctx, grantFromOrganizationAtoB())
	require.ErrorIs(t, err, domain.ErrCrossOrganizationGrant)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/modules/filesystem/domain/service -run "TestExplicitDeny|TestCapability" -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现决策顺序**

固定顺序：

```text
Authentication
→ Organization/Workspace Boundary
→ Entitlement
→ RBAC/Casbin
→ Resource Relation/Inheritance
→ ABAC Context
→ Explicit Deny
```

Decision 返回 `Result`, `ReasonCode`, `PolicyID`, `PolicyVersion`, `TraceID`。

Resource Grant 默认只允许同 Organization 主体。匿名链接由平台设置显式启用，创建时强制只读、过期时间、撤销状态和最大下载次数；数据库只保存随机 token 的哈希。

- [ ] **Step 4: 实现 Capability**

签名 claims：

```go
type Claims struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	AgentID        *uuid.UUID
	RunID          uuid.UUID
	ResourceIDs    []uuid.UUID
	Actions        []string
	Audience       string
	ExpiresAt      time.Time
	Nonce          string
}
```

数据库只保存 token hash、scope、audience、expiry、revoked_at。

- [ ] **Step 5: 实现受控跨组织 Export/Import**

跨组织请求不得创建 Resource Grant。`StartControlledExport` 创建审计任务；Worker 读取源文件、在目标组织 dedup domain 创建新 Blob/Node/File ID、重新扫描，并记录源/目标双边审计。源文件删除或变更不影响目标文件。

- [ ] **Step 6: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/... ./internal/platform/authz/... -count=1
git add backend/internal/modules/filesystem backend/api/openapi/filesystem.yaml
git commit -m "feat(filesystem): add resource grants and agent capabilities"
```

### Task 4: 实现 File Link 与 Artifact

**Files:**
- Create: `backend/internal/modules/filesystem/domain/model/artifact.go`
- Create: `backend/internal/modules/filesystem/application/command/create_artifact.go`
- Create: `backend/internal/modules/filesystem/application/query/list_artifacts.go`
- Create: `backend/internal/modules/filesystem/application/command/create_artifact_test.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Modify: `backend/api/openapi/filesystem.yaml`

**Interfaces:**
- Consumes: File ID、Producer/Run IDs。
- Produces: `Artifact`, `FileLink`, `filesystem.artifact.created.v1`.

- [ ] **Step 1: 写引用完整性测试**

```go
func TestArtifactCannotReferenceFileFromAnotherWorkspace(t *testing.T) {
	_, err := handler.Handle(ctx, command.CreateArtifact{
		WorkspaceID: workspaceA, FileID: fileInWorkspaceB, RunID: runID,
	})
	require.ErrorIs(t, err, domain.ErrCrossWorkspaceReference)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/command -run TestArtifact -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 Artifact**

Artifact 字段固定为 `artifact_type`, `producer_type`, `producer_id`, `run_id`, `input_refs`, `file_id`, `retention_policy_id`。创建时写 File Link 与版本化 Outbox 事件。

- [ ] **Step 4: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/... -count=1
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): add file links and artifacts"
```

### Task 5: 扩展 Knowledge schema 并构建迁移工具

**Files:**
- Create: `backend/migrations/000080_knowledge_filesystem_expand.up.sql`
- Create: `backend/migrations/000080_knowledge_filesystem_expand.down.sql`
- Modify: `backend/internal/modules/knowledge/domain/model/document.go`
- Modify: `backend/internal/modules/knowledge/domain/repository/knowledge_repository.go`
- Modify: `backend/internal/modules/knowledge/infrastructure/persistence/knowledge_repo.go`
- Create: `backend/internal/modules/knowledge/domain/port/file_store.go`
- Create: `backend/internal/modules/knowledge/infrastructure/acl/filesystem_acl.go`
- Create: `backend/cmd/migrate-knowledge-inventory/main.go`
- Create: `backend/cmd/migrate-knowledge-precopy/main.go`
- Create: `backend/cmd/migrate-knowledge-switch/main.go`
- Create: `backend/cmd/migrate-knowledge-precopy/main_test.go`

**Interfaces:**
- Consumes: Filesystem File Port。
- Produces: Knowledge `Document.FileID`, inventory/precopy/switch/rollback commands。

- [ ] **Step 1: 写迁移幂等测试**

```go
func TestPrecopyIsIdempotentAndRecordsChecksum(t *testing.T) {
	runPrecopy(t, documentID)
	runPrecopy(t, documentID)
	assertFileCount(t, documentID, 1)
	assertMappingHasSHA256(t, documentID, sha256OfLegacyContent)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./cmd/migrate-knowledge-precopy -count=1
```

Expected: FAIL。

- [ ] **Step 3: 扩展 schema**

`knowledge.documents` 墠可空 `file_id`、`migration_status`；新增 `knowledge.document_file_migrations(document_id, file_id, source_sha256, target_sha256, status, error_code, verified_at)`。不删除 `content`。

- [ ] **Step 4: 实现 Inventory**

输出固定 JSON 报告：总行数、空内容、总字节、最大文档、编码异常、重复 SHA-256、Workspace 缺失和预计 Blob 数。

- [ ] **Step 5: 实现 Precopy/Verify**

批次参数：

```text
-batch-size 100
-after-id <uuid>
-dry-run
```

每条文档写 File、File Link、mapping；读取目标内容重新计算 SHA-256 后才标记 `VERIFIED`。

- [ ] **Step 6: 实现 Freeze/Switch/Rollback**

Switch 前必须查询：

```text
legacy document count = verified mapping count
AND failed mapping count = 0
AND every target file readable
```

Rollback 只切换读取策略回 legacy content，不删除 File。

- [ ] **Step 7: 运行测试并提交**

```powershell
go test ./internal/modules/knowledge/... ./cmd/migrate-knowledge-inventory ./cmd/migrate-knowledge-precopy ./cmd/migrate-knowledge-switch -count=1
git add backend/migrations/000080_* backend/internal/modules/knowledge backend/cmd/migrate-knowledge-*
git commit -m "feat(knowledge): migrate documents to filesystem"
```

### Task 6: 接入 Agent、Workflow 与 MCP

**Files:**
- Create: `backend/internal/modules/agent/infrastructure/acl/filesystem.go`
- Create: `backend/internal/modules/workflow/infrastructure/acl/filesystem.go`
- Modify: `backend/internal/modules/mcp/tool_registry.go`
- Create: `backend/internal/modules/mcp/filesystem_tools.go`
- Create: `backend/internal/modules/mcp/filesystem_tools_test.go`
- Modify: `backend/README.md`

**Interfaces:**
- Consumes: File Port、Artifact Port、Capability Issuer。
- Produces: Agent/Workflow file refs 和 MCP file tools。

- [ ] **Step 1: 写 MCP scope 测试**

```go
func TestReadFileToolCannotEscapeCapabilityScope(t *testing.T) {
	tool := newReadFileTool(capabilityFor(fileA))
	_, err := tool.Call(ctx, map[string]any{"file_id": fileB.String()})
	require.ErrorIs(t, err, filesystem.ErrCapabilityScopeDenied)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/mcp -run TestReadFileTool -count=1
```

Expected: FAIL。

- [ ] **Step 3: 注册工具**

固定工具：

```text
list_files
get_file_metadata
read_file
create_upload_session
write_file
create_artifact
```

每个 handler 先校验 MCP token 的 Workspace，再请求 Capability。

- [ ] **Step 4: 接入 Run**

Agent/Workflow Run 保存 File ID/Artifact ID；执行前签发 run-scoped Capability，结束或取消时撤销。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/agent/... ./internal/modules/workflow/... ./internal/modules/mcp/... -count=1
git add backend/internal/modules/agent backend/internal/modules/workflow backend/internal/modules/mcp backend/README.md
git commit -m "feat(filesystem): integrate agent workflow and MCP"
```

### Task 7: 构建 SFTP Gateway

**Files:**
- Create: `backend/cmd/sftp-gateway/main.go`
- Create: `backend/internal/sftpgateway/server.go`
- Create: `backend/internal/sftpgateway/auth.go`
- Create: `backend/internal/sftpgateway/virtualfs.go`
- Create: `backend/internal/sftpgateway/importer.go`
- Create: `backend/internal/sftpgateway/server_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/config.example.yaml`

**Interfaces:**
- Consumes: Identity/API Token validator、Transfer Control Port。
- Produces: SFTP virtual root 与 Landing Zone import。

- [ ] **Step 1: 写目录逃逸测试**

```go
func TestSFTPRejectsEscapeFromVirtualRoot(t *testing.T) {
	fs := newVirtualFS(t, workspaceRoot)
	_, err := fs.Open("../../other-workspace/secret.txt")
	require.ErrorIs(t, err, sftpgateway.ErrPathEscape)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/sftpgateway -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现虚拟根和 Landing Zone**

每个连接绑定 Organization/Workspace/Principal。上传只进入随机 Session 目录；close 成功后调用 Create/Complete Transfer，不允许直接 rename 到 Provider 生产路径。

- [ ] **Step 4: 增加配置**

```yaml
filesystem:
  sftp:
    enabled: true
    listen: ":2022"
    host_key_secret_ref: "secret://sparkops/sftp-host-key"
    landing_zone: "/var/lib/sparkops/sftp"
    idle_timeout: 15m
```

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/sftpgateway ./cmd/sftp-gateway -count=1
git add backend/internal/sftpgateway backend/cmd/sftp-gateway backend/internal/platform/config backend/config.example.yaml
git commit -m "feat(filesystem): add isolated SFTP gateway"
```

### Task 8: 实现 Preview 与授权后搜索

**Files:**
- Create: `backend/internal/modules/filesystem/domain/port/previewer.go`
- Create: `backend/internal/modules/filesystem/application/operation/generate_preview.go`
- Create: `backend/internal/modules/filesystem/application/query/search_files.go`
- Create: `backend/internal/modules/filesystem/application/query/search_files_test.go`
- Modify: `backend/cmd/file-worker/main.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`

**Interfaces:**
- Consumes: File Events、Provider、Authorization Service。
- Produces: 派生 Preview Blob 和授权过滤搜索。

- [ ] **Step 1: 写搜索防旧权限测试**

```go
func TestSearchReauthorizesEveryResult(t *testing.T) {
	index := fakeIndexReturning(fileA, fileB)
	authz := fakeAuthzAllowing(fileA)
	results, err := handler.Search(ctx, query)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{fileA}, resultIDs(results))
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/query -run TestSearchReauthorizesEveryResult -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现分层搜索**

名称、路径、标签、Owner、时间和分类使用 PostgreSQL。OpenSearch 作为可选全文投影；返回前逐项或批量调用 Authorization Service。

- [ ] **Step 4: 实现 Preview**

Preview 作为派生 Blob 和 File Link，继承原文件最高分类。Preview 失败只记录 operation error，不改变原 Revision 可用性。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/... ./cmd/file-worker -count=1
git add backend/internal/modules/filesystem backend/cmd/file-worker
git commit -m "feat(filesystem): add previews and authorized search"
```

### Task 9: 增加 Storage、Lifecycle 与审计控制台

**Files:**
- Create: `fronted/apps/web/app/(platform)/files/admin/page.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/StoragePoolPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/StorageBindingPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/LifecyclePolicyPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileAuditPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx`
- Modify: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`

**Interfaces:**
- Consumes: G3 storage/lifecycle API 和文件审计 API。
- Produces: `storage:admin` / `lifecycle:admin` 管理界面。

- [ ] **Step 1: 写权限与 Secret 脱敏测试**

```tsx
it("never renders provider credentials and hides mutations without storage admin", async () => {
  render(<StoragePoolPanel permissions={["storage:read"]} providers={[providerFixture()]} />);
  expect(screen.queryByText(/secret|access.?key/i)).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "新增存储池" })).not.toBeInTheDocument();
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx
```

Expected: FAIL。

- [ ] **Step 3: 实现管理界面**

Provider 表单只提交 `secretRef`，不读取或回显 secret。Storage Binding 支持 Organization/Workspace/Project/User scope；Lifecycle 编辑器只允许规格中定义的条件和动作。审计面板支持 actor/action/resource/result/time 分页查询。

- [ ] **Step 4: 运行测试并提交**

```powershell
pnpm vitest run apps/web/src/modules/filesystem
pnpm typecheck
git add apps/web packages/contracts/src/filesystem packages/web-sdk/src/filesystem.ts
git commit -m "feat(filesystem): add storage lifecycle and audit consoles"
```

### Task 10: G4 验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-g4.md`

**Interfaces:**
- Consumes: G4 实现。
- Produces: G5 生产化候选。

- [ ] **Step 1: 运行测试矩阵**

```powershell
cd backend
go test ./internal/modules/filesystem/... ./internal/modules/knowledge/... ./internal/modules/mcp/... ./internal/sftpgateway/... -count=1
cd ..\fronted
pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

Expected: PASS。

- [ ] **Step 2: 执行迁移演练**

依次运行 inventory、precopy、switch、rollback、switch；每次输出报告归档到 verification 文档。

- [ ] **Step 3: 记录验收**

```markdown
- [x] File Explorer 全部操作使用 Node ID
- [x] 显式 Deny 覆盖目录继承 Allow
- [x] Agent/MCP 不能访问 Capability Scope 外文件
- [x] Artifact 跨 Workspace 引用被拒绝
- [x] Knowledge 行数、SHA-256、关联关系 100% 匹配
- [x] Knowledge rollback 不复制或丢失内容
- [x] SFTP 不能逃逸虚拟根且不直写生产存储
- [x] 搜索结果在返回前重新授权
```

- [ ] **Step 4: 重启与提交**

```powershell
cd ..
.\scripts\dev-restart.ps1 -Backend
git add docs/verification/unified-file-infrastructure-g4.md
git commit -m "docs(filesystem): record G4 integration verification"
```
