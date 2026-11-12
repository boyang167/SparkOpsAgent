# Unified File Infrastructure G2 Transfer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付统一 Transfer Session、分片断点续传、NFS/S3 Provider、恶意文件扫描与 Verify/Scan/Commit Protocol。

**Architecture:** 控制面创建 Transfer Session 和配额预占；S3 使用预签名直传，Local/NFS 使用独立 Transfer Gateway；Worker 校验大小、SHA-256 和扫描结果后，在单个 PostgreSQL 事务中提交 Revision、Quota 与 Outbox。

**Tech Stack:** Go 1.26、Gin、PostgreSQL、Redis、RabbitMQ、AWS SDK for Go v2、ClamAV TCP 协议、NFS/POSIX。

## Global Constraints

- 继承 Master Plan 的全部 Global Constraints。
- G1 schema、Provider Contract、权限与文件事件必须已通过验收。
- 上传完成不等于文件可见；只有 `COMPLETED` Session 才能发布 Revision。
- Session 状态迁移必须使用数据库 compare-and-swap 或行锁。
- 预签名 URL 必须绑定对象键、HTTP method 和短有效期。
- Scanner 对 Restricted 文件始终 Fail-Closed。

---

### Task 1: 建立 Transfer schema 与状态机

**Files:**
- Create: `backend/migrations/000078_filesystem_transfer.up.sql`
- Create: `backend/migrations/000078_filesystem_transfer.down.sql`
- Create: `backend/internal/modules/filesystem/domain/model/transfer.go`
- Create: `backend/internal/modules/filesystem/domain/model/transfer_test.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`

**Interfaces:**
- Consumes: G1 Namespace/Node/Quota。
- Produces: `TransferSession`, `TransferPart`, `TransferStatus`, 状态迁移方法。

- [ ] **Step 1: 写状态机失败测试**

```go
func TestTransferStateMachine(t *testing.T) {
	session := model.NewTransferSession(uuid.New(), uuid.New(), 1024, "sha256")
	require.NoError(t, session.StartUpload())
	require.NoError(t, session.StartVerify())
	require.NoError(t, session.StartScan())
	require.NoError(t, session.StartCommit())
	require.NoError(t, session.Complete())
	assert.ErrorIs(t, session.StartUpload(), model.ErrInvalidTransferTransition)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/modules/filesystem/domain/model -run TestTransferStateMachine -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现显式状态转换**

```go
type TransferStatus string

const (
	TransferInitiated  TransferStatus = "INITIATED"
	TransferUploading  TransferStatus = "UPLOADING"
	TransferPaused     TransferStatus = "PAUSED"
	TransferVerifying  TransferStatus = "VERIFYING"
	TransferScanning   TransferStatus = "SCANNING"
	TransferCommitting TransferStatus = "COMMITTING"
	TransferCompleted  TransferStatus = "COMPLETED"
	TransferFailed     TransferStatus = "FAILED"
	TransferRejected   TransferStatus = "REJECTED"
	TransferAborted    TransferStatus = "ABORTED"
)
```

所有转换通过 `transition(fromSet, to)`；终态禁止迁出。

- [ ] **Step 4: 编写 transfer DDL**

创建：

```sql
CREATE TABLE filesystem.transfer_sessions (
  id uuid PRIMARY KEY,
  workspace_id uuid NOT NULL,
  namespace_id uuid NOT NULL REFERENCES filesystem.namespaces(id),
  target_parent_id uuid REFERENCES filesystem.nodes(id),
  target_name text NOT NULL,
  protocol varchar(24) NOT NULL,
  expected_size bigint NOT NULL,
  expected_sha256 char(64),
  actual_size bigint,
  actual_sha256 char(64),
  provider_id uuid,
  temporary_object_key text NOT NULL,
  status varchar(24) NOT NULL,
  idempotency_key text NOT NULL,
  expires_at timestamptz NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  error_code text,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, idempotency_key)
);
```

同时创建 `transfer_parts` 和 `quota_reservations` 的 Session 外键及索引。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/domain/model ./migrations/... -count=1
git add backend/migrations/000078_filesystem_transfer.* backend/internal/modules/filesystem/domain
git commit -m "feat(filesystem): add transfer session state machine"
```

### Task 2: 实现配额预占与幂等创建会话

**Files:**
- Create: `backend/internal/modules/filesystem/application/command/create_transfer.go`
- Create: `backend/internal/modules/filesystem/application/command/create_transfer_test.go`
- Create: `backend/internal/modules/filesystem/infrastructure/persistence/quota_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`

**Interfaces:**
- Consumes: `QuotaAccount`, `TransferSession`, PostgreSQL transaction。
- Produces: `CreateTransferHandler.Handle(ctx, CreateTransfer) (TransferDescriptor, error)`.

- [ ] **Step 1: 写并发配额测试**

```go
func TestConcurrentReservationsDoNotOversell(t *testing.T) {
	h := newCreateTransferHandler(t, quotaBytes(100))
	results := runConcurrently(2, func(i int) error {
		_, err := h.Handle(context.Background(), command.CreateTransfer{
			WorkspaceID: uuid.New(), Size: 60, IdempotencyKey: fmt.Sprintf("upload-%d", i),
		})
		return err
	})
	assert.Equal(t, 1, countSuccess(results))
	assert.Equal(t, 1, countError(results, domain.ErrQuotaExceeded))
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/command -run "TestConcurrentReservations|TestCreateTransferIdempotency" -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现数据库原子预占**

使用以下条件更新，不用 Redis 决定正确性：

```sql
UPDATE filesystem.quota_accounts
SET reserved_bytes = reserved_bytes + $1,
    reserved_files = reserved_files + 1,
    version = version + 1
WHERE scope_type = $2
  AND scope_id = $3
  AND used_bytes + reserved_bytes + $1 <= max_bytes
RETURNING version;
```

同一事务插入 `quota_reservations` 与 `transfer_sessions`。重复 `Idempotency-Key` 返回已有 Session，参数不一致返回 `TRANSFER_IDEMPOTENCY_CONFLICT`。

- [ ] **Step 4: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/command ./internal/modules/filesystem/infrastructure/persistence -count=1
git add backend/internal/modules/filesystem/application/command backend/internal/modules/filesystem/infrastructure/persistence
git commit -m "feat(filesystem): reserve quota when creating transfers"
```

### Task 3: 实现 NFS 与 S3 Provider

**Files:**
- Create: `backend/internal/platform/storage/adapters/nfs/provider.go`
- Create: `backend/internal/platform/storage/adapters/nfs/provider_test.go`
- Create: `backend/internal/platform/storage/adapters/s3/provider.go`
- Create: `backend/internal/platform/storage/adapters/s3/multipart.go`
- Create: `backend/internal/platform/storage/adapters/s3/provider_test.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

**Interfaces:**
- Consumes: G1 `storage.Provider`.
- Produces: `nfs.New(root)`, `s3.New(config)`, `storage.MultipartProvider`, `storage.PresignProvider`.

- [ ] **Step 1: 扩展 contract tests**

```go
type MultipartProvider interface {
	CreateMultipart(ctx context.Context, objectKey string, size int64) (MultipartUpload, error)
	PresignPart(ctx context.Context, uploadID string, partNumber int, expires time.Duration) (string, error)
	CompleteMultipart(ctx context.Context, uploadID string, parts []CompletedPart) (ObjectInfo, error)
	AbortMultipart(ctx context.Context, uploadID string) error
}
```

增加中断后列出已完成 part、重复 complete 幂等、错误 ETag 拒绝测试。

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/platform/storage/... -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 NFS**

复用 Local 的路径逃逸防护和临时文件协议，但 Capability 声明：

```go
Capabilities{
	RangeRead: true, AtomicRename: true, Multipart: false,
	PresignedUpload: false, POSIX: true,
}
```

- [ ] **Step 4: 实现 S3**

使用 AWS SDK v2；配置只接受 endpoint、region、bucket、pathStyle 和凭据 Provider。object key 由控制面生成。Checksum 与 multipart ETag 分开处理，最终 SHA-256 由 Verify Worker 计算或从可信 checksum header 读取后复核。

- [ ] **Step 5: 使用 MinIO 运行 contract suite**

```powershell
$env:REQUIRE_INTEGRATION="1"
go test -tags=integration ./internal/platform/storage/adapters/s3 -count=1
go test ./internal/platform/storage/adapters/nfs -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add backend/internal/platform/storage backend/go.mod backend/go.sum
git commit -m "feat(storage): add NFS and S3 providers"
```

### Task 4: 构建 Transfer Gateway

**Files:**
- Create: `backend/cmd/file-transfer-gateway/main.go`
- Create: `backend/internal/filesystemgateway/server.go`
- Create: `backend/internal/filesystemgateway/auth.go`
- Create: `backend/internal/filesystemgateway/upload.go`
- Create: `backend/internal/filesystemgateway/download.go`
- Create: `backend/internal/filesystemgateway/server_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/config.example.yaml`

**Interfaces:**
- Consumes: Session/Capability 查询 Port、Local/NFS Provider。
- Produces: `/health`, `/ready`, `PUT /v1/transfers/:id/content`, `GET /v1/files/:id/content`.

- [ ] **Step 1: 写 Capability 范围测试**

```go
func TestUploadRejectsCapabilityForAnotherSession(t *testing.T) {
	server := newTestServer(t, capabilityFor("session-a", "transfer:write"))
	res := put(t, server, "/v1/transfers/session-b/content", []byte("payload"))
	assert.Equal(t, http.StatusForbidden, res.Code)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/filesystemgateway -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 Gateway**

Gateway 只做：

1. 校验签名、audience、expiry、nonce、session/resource、action。
2. 从控制面 Port 获取允许的 temporary object key。
3. 流式写 Provider，限制请求大小为 Session 声明值。
4. 计算 SHA-256。
5. 回报实际大小和 checksum。

禁止 Gateway 自行创建 Node、Revision 或修改配额。

下载 Capability 只能在控制面重新授权并成功写入访问审计后签发；审计失败时 Restricted 文件下载 Fail-Closed。Gateway 回报完成字节和结果，控制面补写 transfer completion 审计。

- [ ] **Step 4: 增加配置**

```yaml
filesystem:
  transfer_gateway:
    listen: ":8082"
    public_url: "http://localhost:8082"
    capability_audience: "sparkops-file-transfer"
    max_file_bytes: 21474836480
```

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/filesystemgateway ./cmd/file-transfer-gateway -count=1
git add backend/cmd/file-transfer-gateway backend/internal/filesystemgateway backend/internal/platform/config backend/config.example.yaml
git commit -m "feat(filesystem): add transfer gateway"
```

### Task 5: 实现 Verify、Scanner 与原子 Commit Worker

**Files:**
- Create: `backend/internal/modules/filesystem/domain/port/scanner.go`
- Create: `backend/internal/modules/filesystem/infrastructure/scanner/clamav.go`
- Create: `backend/internal/modules/filesystem/application/command/verify_transfer.go`
- Create: `backend/internal/modules/filesystem/application/command/commit_transfer.go`
- Create: `backend/internal/modules/filesystem/application/command/commit_transfer_atomicity_test.go`
- Create: `backend/cmd/file-worker/main.go`

**Interfaces:**
- Consumes: Provider、Scanner、Repository、Transactor、ReliableBus。
- Produces: `VerifyTransferHandler`, `CommitTransferHandler`, `Scanner.Scan`.

- [ ] **Step 1: 写提交原子性测试**

```go
func TestCommitTransferIsAtomicAndIdempotent(t *testing.T) {
	h := newCommitHandler(t)
	first, err := h.Handle(ctx, command.CommitTransfer{SessionID: sessionID})
	require.NoError(t, err)
	second, err := h.Handle(ctx, command.CommitTransfer{SessionID: sessionID})
	require.NoError(t, err)
	assert.Equal(t, first.RevisionID, second.RevisionID)
	assertQuotaCommittedOnce(t)
	assertOutboxEventCount(t, 1)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/command -run "TestCommitTransfer|TestVerifyTransfer" -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 Scanner Port**

```go
type Scanner interface {
	Scan(ctx context.Context, object storage.ObjectRef) (ScanResult, error)
}

type ScanResult struct {
	Clean      bool
	ThreatName string
	Engine     string
	Signature  string
}
```

ClamAV adapter 使用 `INSTREAM`，设置读取超时、最大字节和明确错误分类。Scanner 不可用时 Session 留在可重试状态；Restricted 分类不得绕过扫描。

- [ ] **Step 4: 实现 Commit 事务**

事务顺序：

```text
SELECT Session FOR UPDATE
→ 确认 VERIFYING/SCANNING 已成功
→ 状态 CAS 到 COMMITTING
→ 创建/复用组织内 Blob
→ 创建 AVAILABLE Replica
→ 创建唯一 Revision
→ 更新 Node.current_revision_id
→ reservation 转 used
→ 写 Outbox
→ Session=COMPLETED
```

- [ ] **Step 5: 实现 Worker**

`file-worker` 订阅 transfer uploaded 事件，使用稳定 subscriber 名：

```text
filesystem.transfer.verify-commit
```

消费失败交给现有 Inbox retry；业务拒绝写 `REJECTED` 后 ack，不进入无限重试。

- [ ] **Step 6: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/command ./internal/modules/filesystem/infrastructure/scanner ./cmd/file-worker -count=1
go test ./internal/platform/messaging/... -count=1
git add backend/internal/modules/filesystem backend/cmd/file-worker
git commit -m "feat(filesystem): verify scan and commit transfers"
```

### Task 6: 暴露 Transfer API 与前端传输 SDK

**Files:**
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Modify: `backend/api/openapi/filesystem.yaml`
- Create: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/contracts/src/index.ts`
- Modify: `fronted/packages/contracts/package.json`
- Create: `fronted/packages/upstream/src/filesystem.ts`
- Modify: `fronted/packages/upstream/src/index.ts`
- Create: `fronted/packages/bff-modules/src/filesystem/index.ts`
- Modify: `fronted/packages/bff-modules/src/index.ts`
- Create: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/packages/web-sdk/src/index.ts`
- Modify: `fronted/packages/bff-core/src/index.ts`
- Create: `fronted/packages/web-sdk/src/filesystem.test.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/transfers/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/transfers/[sessionId]/route.ts`

**Interfaces:**
- Consumes: Transfer handlers。
- Produces: `createTransferSession`, `getTransferSession`, `abortTransfer`, `uploadTransferParts`.

- [ ] **Step 1: 写 SDK 恢复测试**

```ts
it("uploads only missing parts when resuming", async () => {
  const uploaded = await uploadTransfer(fileFixture, {
    session: sessionWithCompletedParts([1, 2]),
    partSize: 5,
  });
  expect(uploaded.requestedParts).toEqual([3, 4]);
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd fronted
pnpm vitest run packages/web-sdk/src/filesystem.test.ts
```

Expected: FAIL。

- [ ] **Step 3: 定义 Zod 契约**

包括 `TransferSessionVMSchema`、`CreateTransferCommandSchema`、`TransferPartVMSchema`。状态枚举必须与 Go 完全一致。

- [ ] **Step 4: 实现 SDK**

控制请求使用 `bffFetch`；S3 part 使用 presigned URL 的 raw `fetch`。进度回调签名固定为：

```ts
export type TransferProgress = {
  uploadedBytes: number;
  totalBytes: number;
  completedParts: number;
  totalParts: number;
};
```

- [ ] **Step 5: 运行测试并提交**

```powershell
pnpm vitest run packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
git add packages apps/web/app/api/bff/v1/workspaces
git commit -m "feat(filesystem): add transfer API and web SDK"
```

### Task 7: G2 验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-g2.md`

**Interfaces:**
- Consumes: G2 实现。
- Produces: G3 可依赖的 Transfer/Provider 契约。

- [ ] **Step 1: 运行测试矩阵**

```powershell
cd backend
go test ./internal/modules/filesystem/... ./internal/filesystemgateway/... ./internal/platform/storage/... -count=1
$env:REQUIRE_INTEGRATION="1"
go test -tags=integration ./internal/platform/storage/adapters/s3 -count=1
cd ..\fronted
pnpm vitest run packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

Expected: PASS。

- [ ] **Step 2: 记录固定验收项**

```markdown
- [x] 相同 Idempotency-Key 不产生重复 Session
- [x] 两个 60-byte 并发上传在 100-byte 配额下仅一个成功
- [x] 已完成分片不会在恢复时重复上传
- [x] Checksum 不匹配不创建 Revision
- [x] Scanner REJECTED 不创建用户可见文件
- [x] Commit 重试只创建一个 Revision 和一个 Outbox 事件
- [x] Local/NFS/S3 Provider Contract Suite 通过
```

- [ ] **Step 3: 重启与提交**

```powershell
cd ..
.\scripts\dev-restart.ps1 -Backend
git add docs/verification/unified-file-infrastructure-g2.md
git commit -m "docs(filesystem): record G2 transfer verification"
```
