# Unified File Infrastructure G3 Storage Fabric Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 Storage Pool、Scope Binding、Placement Policy、复制/迁移、Lifecycle、延迟 GC 和可恢复 Reconciler。

**Architecture:** 业务绑定 Pool，不绑定 Provider。Placement 依据 scope、合规、能力和健康度做确定性决策；迁移先创建并校验新 Replica，再原子提升，旧 Replica 宽限期后删除。

**Tech Stack:** Go 1.26、PostgreSQL、RabbitMQ Operation Outbox、Redis 租约辅助、Local/NFS/S3 Provider。

## Global Constraints

- 继承 Master Plan 的全部 Global Constraints。
- G1 与 G2 必须已通过。
- Placement Policy 与 IAM Policy 分离并独立版本化。
- 只有校验通过的 `AVAILABLE` Replica 能参与读取和故障切换。
- 生命周期和 GC 通过可审计 Job 执行，禁止 HTTP 请求同步批量删除。

---

### Task 1: 建立 Storage Fabric schema 与聚合

**Files:**
- Create: `backend/migrations/000079_filesystem_storage_fabric.up.sql`
- Create: `backend/migrations/000079_filesystem_storage_fabric.down.sql`
- Create: `backend/internal/modules/filesystem/domain/model/storage_pool.go`
- Create: `backend/internal/modules/filesystem/domain/model/lifecycle.go`
- Create: `backend/internal/modules/filesystem/domain/model/storage_pool_test.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`

**Interfaces:**
- Consumes: G2 Provider/Replica。
- Produces: `StorageProvider`, `StoragePool`, `StorageBinding`, `LifecyclePolicy`, `LifecycleJob`.

- [ ] **Step 1: 写失败测试**

```go
func TestPoolRejectsTwoPrimaryProvidersWithSamePriority(t *testing.T) {
	pool := model.NewStoragePool(uuid.New(), "default")
	require.NoError(t, pool.AddProvider(uuid.New(), model.ProviderModePrimary, 10))
	err := pool.AddProvider(uuid.New(), model.ProviderModePrimary, 10)
	require.ErrorIs(t, err, model.ErrAmbiguousPrimaryProvider)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/modules/filesystem/domain/model -run TestPool -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现聚合与 DDL**

创建 `storage_providers`、`storage_pools`、`storage_pool_members`、`storage_bindings`、`placement_policies`、`mounts`、`lifecycle_policies`、`lifecycle_jobs`。`secret_ref` 非空，禁止明文 credential 字段。

```go
type ProviderMode string

const (
	ProviderPrimary  ProviderMode = "PRIMARY"
	ProviderReplica  ProviderMode = "REPLICA"
	ProviderArchive  ProviderMode = "ARCHIVE"
	ProviderDraining ProviderMode = "DRAINING"
)
```

- [ ] **Step 4: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/domain/model ./migrations/... -count=1
git add backend/migrations/000079_filesystem_storage_fabric.* backend/internal/modules/filesystem/domain
git commit -m "feat(filesystem): add storage fabric schema"
```

### Task 2: 实现 Provider Registry 与健康状态

**Files:**
- Create: `backend/internal/platform/storage/registry.go`
- Create: `backend/internal/platform/storage/registry_test.go`
- Create: `backend/internal/modules/filesystem/application/command/register_provider.go`
- Create: `backend/internal/modules/filesystem/application/query/provider_health.go`

**Interfaces:**
- Consumes: Provider factories 与 Secret Resolver。
- Produces: `storage.Registry.Resolve(ctx, providerID)`, `ProviderHealth`.

- [ ] **Step 1: 写失败测试**

```go
func TestRegistryNeverReturnsProviderWithMissingSecret(t *testing.T) {
	registry := storage.NewRegistry(fakeSecrets{missing: true})
	_, err := registry.Resolve(context.Background(), providerRecord())
	require.ErrorIs(t, err, storage.ErrProviderSecretUnavailable)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/platform/storage -run TestRegistry -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 Registry**

Registry 根据 `provider_type` 构造 Local/NFS/S3 Adapter；只接受 `secret_ref`，通过现有 Vault/Kubernetes Secret resolver 获取凭据。缓存键包含 provider 配置版本，轮换时主动失效。

Restricted 数据的候选 Provider 必须声明 Encryption capability；S3 使用 SSE-KMS key ref，Local/NFS 使用平台 envelope encryption key ref。Placement 在缺少加密能力时返回 `STORAGE_COMPLIANCE_UNSATISFIED`，不得降级为明文写入。

- [ ] **Step 4: 实现健康采样**

健康状态包含：

```go
type ProviderHealth struct {
	Status       string
	Latency      time.Duration
	ErrorRate    float64
	CapacityFree int64
	CheckedAt    time.Time
}
```

健康采样不修改 Placement 历史；只影响新决策。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/platform/storage/... ./internal/modules/filesystem/application/... -count=1
git add backend/internal/platform/storage backend/internal/modules/filesystem/application
git commit -m "feat(storage): add provider registry and health"
```

### Task 3: 实现确定性 Placement Engine

**Files:**
- Create: `backend/internal/modules/filesystem/domain/service/placement.go`
- Create: `backend/internal/modules/filesystem/domain/service/placement_test.go`
- Create: `backend/internal/modules/filesystem/application/query/place_object.go`

**Interfaces:**
- Consumes: `PlacementRequest`, Scope Bindings、Provider Capability/Health。
- Produces: `PlacementDecision`.

- [ ] **Step 1: 写决策优先级测试**

```go
func TestPlacementOrdersScopeComplianceCapabilityHealthFallback(t *testing.T) {
	decision, err := engine.Place(ctx, PlacementRequest{
		WorkspaceID: workspaceID,
		Classification: "RESTRICTED",
		Size: 1024,
		RequiresPresignedUpload: true,
	})
	require.NoError(t, err)
	assert.Equal(t, restrictedS3PoolID, decision.PoolID)
	assert.Equal(t, policyVersion, decision.PolicyVersion)
	assert.NotEmpty(t, decision.MatchedRuleIDs)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/domain/service -run TestPlacement -count=1
```

Expected: FAIL。

- [ ] **Step 3: 定义稳定接口**

```go
type PlacementDecision struct {
	PoolID          uuid.UUID
	ProviderID      uuid.UUID
	PolicyVersion   int64
	MatchedRuleIDs  []string
	DecisionReason  string
}
```

排序固定为 Scope Binding → Compliance → Capability → Health → Explicit Fallback。相同输入和相同快照必须得到相同结果。

- [ ] **Step 4: 集成 CreateTransfer**

G2 `CreateTransferHandler` 改为调用 Placement Engine，并把 `pool_id`、`provider_id`、`policy_version`、`matched_rule_ids` 固化到 Session。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/domain/service ./internal/modules/filesystem/application/... -count=1
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): add deterministic placement engine"
```

### Task 4: 实现副本复制与跨存储迁移

**Files:**
- Create: `backend/internal/modules/filesystem/application/operation/replicate_blob.go`
- Create: `backend/internal/modules/filesystem/application/operation/replicate_blob_test.go`
- Create: `backend/internal/modules/filesystem/application/command/promote_replica.go`
- Create: `backend/internal/modules/filesystem/operation_factory.go`

**Interfaces:**
- Consumes: Operation Outbox、Provider Registry、Replica Repository。
- Produces: `filesystem.replicate-blob.v1` operation handler。

- [ ] **Step 1: 写中断恢复测试**

```go
func TestReplicationResumesWithoutSwitchingBeforeVerification(t *testing.T) {
	op := newReplicationOperation(t, failAfterBytes(512))
	err := op.Execute(ctx, payload(blobID, sourceID, targetID))
	require.Error(t, err)
	assertSourceStillPreferred(t)
	assertCheckpoint(t, int64(512))

	require.NoError(t, op.Execute(ctx, payload(blobID, sourceID, targetID)))
	assertTargetAvailableAndPreferred(t)
	assertSourceDraining(t)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/operation -run TestReplication -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现复制协议**

固定顺序：

```text
CREATE target replica(CREATING)
→ copy/range resume from checkpoint
→ independent SHA-256 verification
→ target AVAILABLE
→ transactionally promote read preference
→ source DRAINING with delete_after
```

- [ ] **Step 4: 注册 Operation Descriptor**

`OperationFactory.Descriptors` 使用稳定名称：

```text
filesystem.replicate-blob.v1
```

Payload 只包含 ID 和策略版本，不包含 credentials。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/operation ./internal/platform/messaging/... -count=1
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): add resumable replica migration"
```

### Task 5: 实现 Lifecycle 与安全 GC

**Files:**
- Create: `backend/internal/modules/filesystem/domain/service/lifecycle.go`
- Create: `backend/internal/modules/filesystem/application/operation/run_lifecycle.go`
- Create: `backend/internal/modules/filesystem/application/operation/garbage_collect.go`
- Create: `backend/internal/modules/filesystem/application/operation/garbage_collect_test.go`

**Interfaces:**
- Consumes: Lifecycle Policy、Revision/Blob/Replica repository。
- Produces: 归档、复制、回收站、Purge、GC operations。

- [ ] **Step 1: 写 GC 安全测试**

```go
func TestGCRefusesBlobWithLiveRevisionOrLegalHold(t *testing.T) {
	gc := newGC(t)
	assert.ErrorIs(t, gc.Delete(ctx, blobWithLiveRevision()), domain.ErrBlobStillReferenced)
	assert.ErrorIs(t, gc.Delete(ctx, blobUnderLegalHold()), domain.ErrRetentionHold)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/operation -run TestGC -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 Lifecycle 选择器**

选择条件固定支持 scope、classification、mime、size、created_at、last_accessed_at、retention、legal_hold；动作固定支持 replicate、archive、trash、purge、rescan。

- [ ] **Step 4: 实现 GC 双重确认**

即使 `ref_count=0`，GC 也必须在同一事务中查询：

```text
0 live revisions
AND 0 active file links
AND no legal hold
AND tombstoned_at + grace_period <= now
```

删除物理对象成功后才标记 Replica/Blob `DELETED`；失败保留 `GC_PENDING` 并重试。

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/domain/service ./internal/modules/filesystem/application/operation -count=1
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): add lifecycle and safe garbage collection"
```

### Task 6: 构建独立 Reconciler

**Files:**
- Create: `backend/cmd/file-reconciler/main.go`
- Create: `backend/internal/modules/filesystem/application/service/reconciler.go`
- Create: `backend/internal/modules/filesystem/application/service/reconciler_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/config.example.yaml`

**Interfaces:**
- Consumes: Repository、Provider Registry、DB Lease。
- Produces: `Reconciler.RunOnce`, `Reconciler.Start`.

- [ ] **Step 1: 写租约单活测试**

```go
func TestOnlyLeaseOwnerReconciles(t *testing.T) {
	first, second := twoReconcilers(t)
	require.NoError(t, first.RunOnce(ctx))
	require.ErrorIs(t, second.RunOnce(ctx), service.ErrLeaseNotOwned)
	assert.Equal(t, 1, reconciliationPasses(t))
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/application/service -run TestOnlyLeaseOwner -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现 DB Lease 与对账**

RunOnce 顺序：

1. 获取/续租 `filesystem-reconciler` lease。
2. 释放过期 Quota Reservation。
3. 处理超时 Transfer 与临时对象。
4. 对账 Blob/Replica。
5. 认领 Lifecycle/GC backlog。
6. 记录指标和审计摘要。

- [ ] **Step 4: 增加配置**

```yaml
filesystem:
  reconciler:
    interval: 30s
    lease_ttl: 90s
    batch_size: 100
    gc_grace_period: 168h
```

- [ ] **Step 5: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/application/service ./cmd/file-reconciler -count=1
git add backend/cmd/file-reconciler backend/internal/modules/filesystem/application/service backend/internal/platform/config backend/config.example.yaml
git commit -m "feat(filesystem): add reconciliation process"
```

### Task 7: 暴露 Storage 管理 API

**Files:**
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Create: `backend/internal/modules/filesystem/interface/http/storage_handler_test.go`
- Modify: `backend/api/openapi/filesystem.yaml`

**Interfaces:**
- Consumes: Provider/Pool/Binding/Lifecycle handlers。
- Produces: storage admin REST endpoints。

- [ ] **Step 1: 写权限测试**

```go
func TestCreateStoragePoolRequiresStorageAdmin(t *testing.T) {
	res := postAs(t, ordinaryMember, "/api/v1/workspaces/ws/storage-pools", `{"name":"archive"}`)
	assert.Equal(t, http.StatusForbidden, res.Code)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
go test ./internal/modules/filesystem/interface/http -run TestCreateStoragePoolRequiresStorageAdmin -count=1
```

Expected: FAIL。

- [ ] **Step 3: 注册管理路由**

`GET` 使用 `storage:read`；provider/pool/binding mutation 使用 `storage:admin`；Lifecycle mutation 使用 `lifecycle:admin`。

- [ ] **Step 4: 运行测试并提交**

```powershell
go test ./internal/modules/filesystem/interface/http ./internal/modules/filesystem/... -count=1
git add backend/internal/modules/filesystem/interface/http backend/api/openapi/filesystem.yaml
git commit -m "feat(filesystem): expose storage fabric administration"
```

### Task 8: G3 验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-g3.md`

**Interfaces:**
- Consumes: G3 实现。
- Produces: G4 可依赖的冻结 Storage Fabric。

- [ ] **Step 1: 运行测试矩阵**

```powershell
cd backend
go test ./internal/modules/filesystem/... ./internal/platform/storage/... ./cmd/file-reconciler -count=1
go test ./internal/platform/messaging/... -count=1
go test ./...
```

Expected: PASS。

- [ ] **Step 2: 记录验收**

```markdown
- [x] 相同 Placement 输入得到相同决策与 policy_version
- [x] 不健康 Provider 不接收新写入
- [x] 新 Replica 校验前不会切换读取
- [x] 迁移中断后从 checkpoint 恢复
- [x] Legal Hold 和有效引用阻止 GC
- [x] 两个 Reconciler 仅一个持有租约
- [x] 过期配额预占和临时对象最终收敛
```

- [ ] **Step 3: 重启与提交**

```powershell
cd ..
.\scripts\dev-restart.ps1 -Backend
git add docs/verification/unified-file-infrastructure-g3.md
git commit -m "docs(filesystem): record G3 storage fabric verification"
```
