# Unified File Infrastructure G5 Production Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 G1–G4 候选系统生产化，完成镜像、Compose、Helm、网络隔离、监控告警、安全、容量、恢复和发布证据。

**Architecture:** 同一后端镜像包含 API、Transfer Gateway、Worker、SFTP、Reconciler 和迁移命令；Kubernetes 以独立 Deployment/HPA 运行。PostgreSQL、Redis、RabbitMQ、对象存储、KMS/Vault 和 Scanner 使用企业外部基础设施。

**Tech Stack:** Docker、Docker Compose、Kubernetes、Helm、Prometheus、Grafana、PostgreSQL PITR、MinIO/NFS 测试环境。

## Global Constraints

- 继承 Master Plan 的全部 Global Constraints。
- G1–G4 必须通过。
- 正式环境 API/Web/Transfer Gateway/Worker/SFTP 均至少 2 副本；Reconciler 多副本部署、DB 租约单活。
- Provider credential 只能通过 `secret_ref` 注入。
- 发布证据必须来自目标环境或等价容量环境，不以单元测试替代。

---

### Task 1: 扩展后端镜像与 Compose

**Files:**
- Modify: `backend/deployments/docker/Dockerfile`
- Modify: `backend/deployments/docker/docker-compose.yaml`
- Modify: `backend/deployments/docker/config.docker.yaml`
- Modify: `backend/deployments/docker/.env.example`
- Modify: `backend/deployments/helm/migration_job_test.go`
- Modify: `Makefile`
- Modify: `backend/Makefile`

**Interfaces:**
- Consumes: G1–G4 的所有 cmd。
- Produces: 一个包含全部二进制的后端镜像和本地全栈。

- [ ] **Step 1: 写镜像二进制失败测试**

在 `migration_job_test.go` 增加断言：

```go
for _, binary := range []string{
	"server", "migrate-schema", "migrate-sql",
	"file-transfer-gateway", "file-worker", "sftp-gateway", "file-reconciler",
	"migrate-knowledge-inventory", "migrate-knowledge-precopy", "migrate-knowledge-switch",
} {
	require.Contains(t, dockerfile, `-o /out/`+binary)
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./deployments/helm -count=1
```

Expected: FAIL。

- [ ] **Step 3: 构建全部二进制**

Dockerfile builder 阶段逐个 `go build`，runtime 阶段复制到 `/app/bin/`。镜像以非 root 用户运行；SFTP 监听 2022，不申请特权端口。

- [ ] **Step 4: 扩展 Compose**

增加 MinIO、ClamAV、gateway、worker、sftp、reconciler；所有应用服务依赖 migration 完成和基础设施 healthy。为 Local/NFS 测试使用命名 volume，不绑定任意宿主机目录。

- [ ] **Step 5: 增加 Make targets**

```make
filesystem-up:
	docker compose -f backend/deployments/docker/docker-compose.yaml up -d --build

filesystem-test:
	cd backend && go test -tags=integration ./tests/filesystem/... -count=1
```

- [ ] **Step 6: 验证并提交**

```powershell
go test ./deployments/helm -count=1
docker compose -f deployments/docker/docker-compose.yaml config
cd ..
make filesystem-up
git add backend/deployments Makefile backend/Makefile
git commit -m "build(filesystem): package data plane workloads"
```

### Task 2: 增加 Helm Values Schema 与 Workload

**Files:**
- Create: `deployments/helm/sparkops/values.schema.json`
- Modify: `deployments/helm/sparkops/values.yaml`
- Modify: `deployments/helm/sparkops/values-production.yaml`
- Modify: `deployments/helm/sparkops/templates/_helpers.tpl`
- Create: `deployments/helm/sparkops/templates/file-transfer-gateway-deployment.yaml`
- Create: `deployments/helm/sparkops/templates/file-transfer-gateway-service.yaml`
- Create: `deployments/helm/sparkops/templates/file-transfer-gateway-hpa.yaml`
- Create: `deployments/helm/sparkops/templates/file-worker-deployment.yaml`
- Create: `deployments/helm/sparkops/templates/file-worker-hpa.yaml`
- Create: `deployments/helm/sparkops/templates/sftp-gateway-deployment.yaml`
- Create: `deployments/helm/sparkops/templates/sftp-gateway-service.yaml`
- Create: `deployments/helm/sparkops/templates/file-reconciler-deployment.yaml`
- Create: `deployments/helm/sparkops/templates/file-pdb.yaml`
- Create: `deployments/helm/sparkops/templates/file-networkpolicy.yaml`
- Modify: `deployments/helm/sparkops/templates/secret.yaml`
- Create: `backend/deployments/helm/filesystem_templates_test.go`

**Interfaces:**
- Consumes: 单一后端镜像、配置和 Secret refs。
- Produces: 独立可扩缩容 Workloads。

- [ ] **Step 1: 写 Helm 模板测试**

```go
func TestFilesystemProductionValuesRequireTwoReplicas(t *testing.T) {
	rendered := helmTemplate(t, "values-production.yaml")
	for _, workload := range []string{"file-transfer-gateway", "file-worker", "sftp-gateway"} {
		requireDeploymentReplicasAtLeast(t, rendered, workload, 2)
	}
	requireContainsNetworkPolicy(t, rendered, "file-transfer-gateway")
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./deployments/helm -run TestFilesystem -count=1
```

Expected: FAIL。

- [ ] **Step 3: 定义 values**

```yaml
filesystem:
  enabled: true
  maxFileBytes: 21474836480
  providerSecretRefs: []
fileTransferGateway:
  replicaCount: 2
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
fileWorker:
  replicaCount: 2
sftpGateway:
  enabled: true
  replicaCount: 2
fileReconciler:
  replicaCount: 2
```

- [ ] **Step 4: 实现安全模板**

所有 Pod：

- non-root、read-only root filesystem。
- drop ALL capabilities。
- resource requests/limits。
- topology spread 和 PDB。
- Secret 只以 envFrom/volume 引用，不渲染明文。

NetworkPolicy 仅允许：BFF/API → Gateway；SFTP ingress → SFTP；workers/reconciler → DB/MQ/storage/scanner。

- [ ] **Step 5: 验证并提交**

```powershell
cd deployments/helm/sparkops
helm lint .
helm template sparkops . -f values-production.yaml | kubectl apply --dry-run=client -f -
cd ..\..\..\backend
go test ./deployments/helm -count=1
git add ../deployments/helm backend/deployments/helm
git commit -m "deploy(filesystem): add production Helm workloads"
```

### Task 3: 增加指标、告警与 Trace

**Files:**
- Create: `backend/internal/modules/filesystem/infrastructure/observability/metrics.go`
- Create: `backend/internal/modules/filesystem/infrastructure/observability/metrics_test.go`
- Create: `deployments/helm/sparkops/templates/filesystem-alerts.yaml`
- Modify: `deployments/helm/sparkops/values.yaml`
- Modify: `docs/SparkOps部署与运维说明.md`

**Interfaces:**
- Consumes: Transfer、Provider、Quota、Scanner、Queue、Reconciler 状态。
- Produces: Prometheus metrics 与告警。

- [ ] **Step 1: 写 metric 名称测试**

```go
func TestFilesystemMetricNamesAreStable(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	require.NoError(t, observability.Register(registry))
	names := gatherNames(t, registry)
	assert.Contains(t, names, "sparkops_filesystem_transfer_active")
	assert.Contains(t, names, "sparkops_filesystem_reconcile_backlog")
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/modules/filesystem/infrastructure/observability -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现指标**

至少包含 active transfer、bytes、failure rate、state duration、scanner latency/reject、queue lag、provider errors/latency、quota reserved/used、replica inconsistency、GC backlog。

- [ ] **Step 4: 实现告警**

PrometheusRule 固定包含：

```text
FilesystemTransferFailureRateHigh
FilesystemScannerUnavailable
FilesystemProviderErrorRateHigh
FilesystemQuotaReservationStuck
FilesystemReconcileBacklogHigh
FilesystemReplicaMismatch
FilesystemOutboxLagHigh
```

- [ ] **Step 5: 验证 Trace**

集成测试断言同一 `trace_id` 出现在 BFF response、API log、Transfer Session、Worker log 和 Audit event。

- [ ] **Step 6: 提交**

```powershell
go test ./internal/modules/filesystem/infrastructure/observability -count=1
git add backend/internal/modules/filesystem/infrastructure/observability deployments/helm/sparkops docs/SparkOps部署与运维说明.md
git commit -m "feat(filesystem): add production observability"
```

### Task 4: 建立安全与一致性测试套件

**Files:**
- Create: `backend/tests/filesystem/security/tenant_isolation_test.go`
- Create: `backend/tests/filesystem/security/path_traversal_test.go`
- Create: `backend/tests/filesystem/security/presigned_url_test.go`
- Create: `backend/tests/filesystem/security/archive_bomb_test.go`
- Create: `backend/tests/filesystem/security/capability_test.go`
- Create: `backend/tests/filesystem/consistency/failure_injection_test.go`
- Create: `backend/tests/filesystem/integration/testenv_test.go`

**Interfaces:**
- Consumes: 完整 Compose 环境。
- Produces: 可在 CI 运行的 `integration` tagged tests。

- [ ] **Step 1: 写跨租户测试**

```go
func TestWorkspaceCannotReadAnotherOrganizationsBlob(t *testing.T) {
	env := NewTestEnv(t)
	file := env.UploadAs(t, env.OrganizationA.User, []byte("secret"))
	res := env.DownloadAs(t, env.OrganizationB.User, file.ID)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NotContains(t, readBody(t, res), "secret")
}
```

- [ ] **Step 2: 写故障注入测试**

固定覆盖：对象写完后 DB 失败、Outbox 暂停、Worker commit 前崩溃、multipart part 丢失、迁移中断、租约丢失。

- [ ] **Step 3: 运行套件**

```powershell
$env:REQUIRE_INTEGRATION="1"
go test -tags=integration ./tests/filesystem/security/... ./tests/filesystem/consistency/... -count=1
```

Expected: PASS；跨租户内容泄漏为 0。

- [ ] **Step 4: 提交**

```powershell
git add backend/tests/filesystem
git commit -m "test(filesystem): add security and failure injection suites"
```

### Task 5: 建立容量和断点续传压测

**Files:**
- Create: `backend/tests/filesystem/performance/transfer_load_test.go`
- Create: `backend/tests/filesystem/performance/metadata_load_test.go`
- Create: `backend/tests/filesystem/performance/report.go`
- Create: `scripts/run-filesystem-load.ps1`

**Interfaces:**
- Consumes: 目标环境 URL、Token、Workspace、20 GB fixture。
- Produces: JSON/Markdown 压测报告。

- [ ] **Step 1: 实现 20 GB 恢复场景**

测试流程固定：

```text
create session
→ upload 40%
→ terminate client
→ restart client
→ query completed parts
→ upload missing parts
→ verify final SHA-256
```

- [ ] **Step 2: 实现 200 并发**

每个 worker 创建独立 Idempotency-Key；记录 session P95、commit P95、吞吐、错误率和资源使用。验收要求无重复 Revision、无配额超卖、错误率低于 1%。

- [ ] **Step 3: 实现元数据负载**

生成足以模拟 100 TB 规模的 Node/Revision/Replica 行数分布，不实际写入 100 TB 内容；验证 100 条分页列表 P95 ≤500 ms。

- [ ] **Step 4: 运行压测**

```powershell
.\scripts\run-filesystem-load.ps1 `
  -BaseUrl https://api.example.com `
  -WorkspaceId $env:FS_LOAD_WORKSPACE `
  -Token $env:FS_LOAD_TOKEN `
  -ConcurrentTransfers 200 `
  -LargeFilePath D:\fixtures\20gb.bin
```

Expected: 生成 `artifacts/filesystem-load-report.md`，满足规格 SLO。

- [ ] **Step 5: 提交测试工具**

```powershell
git add backend/tests/filesystem/performance scripts/run-filesystem-load.ps1
git commit -m "test(filesystem): add capacity and resume load tests"
```

压测结果文件作为发布证据归档，不提交包含环境地址或 Token 的原始日志。

### Task 6: 执行备份、恢复与 Knowledge 回滚演练

**Files:**
- Create: `docs/runbooks/filesystem-backup-restore.md`
- Create: `docs/runbooks/filesystem-knowledge-migration.md`
- Create: `docs/verification/unified-file-infrastructure-disaster-recovery.md`

**Interfaces:**
- Consumes: PostgreSQL PITR、Provider 版本/副本、Knowledge migration commands。
- Produces: RPO/RTO 与回滚证据。

- [ ] **Step 1: 编写恢复 Runbook**

固定顺序：

```text
freeze writes
→ restore PostgreSQL to PITR target
→ validate schema version
→ reconcile Blob/Replica
→ rebuild search projection
→ verify sample checksums
→ resume workers
→ resume writes
```

- [ ] **Step 2: 执行灾备演练**

记录故障时间、最后可恢复事件、恢复完成时间：

```text
RPO = failure_time - latest_recovered_commit
RTO = service_restored_time - failure_time
```

Expected: RPO ≤15 分钟，RTO ≤2 小时。

- [ ] **Step 3: 执行 Knowledge rollback**

在已 switch 环境执行 rollback，验证全部文档从 legacy content 可读；再 switch 回 file_id，验证 SHA-256 不变。

- [ ] **Step 4: 提交 Runbook 与证据**

```powershell
git add docs/runbooks docs/verification/unified-file-infrastructure-disaster-recovery.md
git commit -m "docs(filesystem): add recovery and migration runbooks"
```

### Task 7: G5 与正式发布验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-g5.md`
- Modify: `docs/SparkOps技术架构说明.md`
- Modify: `docs/SparkOps部署与运维说明.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: G1–G5 全部证据。
- Produces: 可发布的 v1.0。

- [ ] **Step 1: 运行最终检查**

```powershell
make test
make helm-lint
$env:REQUIRE_INTEGRATION="1"
cd backend
go test -tags=integration ./tests/filesystem/... -count=1
```

Expected: PASS。

- [ ] **Step 2: 核对发布硬条件**

G5 文档必须包含：

```markdown
- [x] 生产副本数、HPA、PDB、Topology Spread 已验证
- [x] NetworkPolicy 与 Secret 引用已验证
- [x] 跨租户安全测试 0 泄漏
- [x] 故障注入后对账可收敛
- [x] 20 GB 恢复与 200 并发通过
- [x] Knowledge 迁移与 rollback 通过
- [x] RPO ≤15 分钟，RTO ≤2 小时
- [x] 所有关键指标和告警可触发
```

- [ ] **Step 3: 更新公开文档**

只描述已通过验收的能力；明确 Local/PVC、NFS、S3 Compatible 为首发原生 Provider，COS/OSS/Azure Blob 为 Adapter 扩展路线。

- [ ] **Step 4: 提交**

```powershell
git add docs/verification/unified-file-infrastructure-g5.md docs/SparkOps技术架构说明.md docs/SparkOps部署与运维说明.md README.md
git commit -m "feat(filesystem): complete v1.0 production readiness"
```
