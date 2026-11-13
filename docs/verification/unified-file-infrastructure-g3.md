# G3 Unified File Infrastructure Storage Fabric 验收证据

> 状态：**PASS（含诚实 SKIP）**。计划 Task 8 fresh 测试矩阵通过；PostgreSQL 专项、`DATABASE_DSN` 持久化集成与 S3 MinIO contract 集成未实跑，不声称通过。Reconciler 后台收敛（`RepositoryWorkRunner` 全路径）无独立单元/E2E 证据。

## 验收上下文

- 验收时间：2026-11-13 20:52（UTC+8）
- 工作树：`A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- 被测实现提交：`9d14d8465e1970ba70067fd855ce0247be469f7f`
- 被测实现 subject：`feat(filesystem): expose storage fabric administration`
- G1 / G2 验收基线：见 `docs/verification/unified-file-infrastructure-g1.md`、`docs/verification/unified-file-infrastructure-g2.md`
- 验收 shell：`DATABASE_DSN` **未设置**；`S3_ENDPOINT` / `S3_BUCKET` **未设置**
- 计划来源：`docs/superpowers/plans/2026-11-12-unified-file-infrastructure-g3-storage-fabric.md` Task 8

## Fresh 命令证据

| 命令 | 结果 |
|---|---|
| `cd backend; go test ./internal/modules/filesystem/... ./internal/platform/storage/... ./cmd/file-reconciler -count=1` | **PASS**，exit 0；约 17s / 21s（复跑） |
| `cd backend; go test ./internal/platform/messaging/... -count=1` | **PASS**，exit 0 |
| `cd backend; go test ./... -count=1` | **PASS**，exit 0；全仓包级测试完成（约 206s） |
| `cd backend; $env:REQUIRE_INTEGRATION="1"; go test -tags=integration ./internal/platform/storage/adapters/s3 -count=1` | **PASS**（包级），exit 0；2 项 integration **SKIP**，3 项单元 PASS |

### S3 集成（MinIO）明细

| 测试 | 结果 |
|---|---|
| `TestS3ProviderIntegrationContract` | **SKIP** — `S3_ENDPOINT` and `S3_BUCKET required` |
| `TestS3ProviderIntegrationMultipartContract` | **SKIP** — 同上 |
| `TestS3ProviderRejectsInvalidConfig` | PASS |
| `TestS3ProviderCapabilitiesUnit` | PASS |
| `TestValidateObjectKey` | PASS |

### PostgreSQL 专项（`DATABASE_DSN`）

| 测试 | 结果 |
|---|---|
| `TestCreateTransferPostgresReplaysAuthoritativeExpiredSessionWhenConfigured` | **SKIP** — `DATABASE_DSN is not set` |
| persistence 包内依赖 `DATABASE_DSN` 的集成项 | **SKIP**（本轮未设置 DSN） |

## 计划 Step 2 固定验收项

- [x] 相同 Placement 输入得到相同决策与 policy_version  
  **证据：** `TestPlacementDeterministicForIdenticalInput`；`TestPlacementOrdersScopeComplianceCapabilityHealthFallback` 断言 `policy_version` 与 `MatchedRuleIDs`（含于 fresh `domain/service` PASS）

- [x] 不健康 Provider 不接收新写入  
  **证据：** `TestPlacementPrefersHealthyPrimaryOverUnhealthy` 在双 PRIMARY 下选择 `HEALTHY` Provider，`MatchedRuleIDs` 含 `health:prefer_healthy`（fresh PASS）

- [x] 新 Replica 校验前不会切换读取  
  **证据：** `TestReplicationResumesWithoutSwitchingBeforeVerification`：中断复制后 `assertSourceStillPreferred`，校验完成后才 `assertTargetAvailableAndPreferred`（fresh `application/operation` PASS）

- [x] 迁移中断后从 checkpoint 恢复  
  **证据：** 同上测试 `failAfterBytes(512)` → `assertCheckpoint(t, 512)` → 二次 `Execute` 完成（fresh PASS）

- [x] Legal Hold 和有效引用阻止 GC  
  **证据：** `TestGCRefusesBlobWithLiveRevisionOrLegalHold`（`ErrBlobStillReferenced` / `ErrRetentionHold`）；`TestLifecycleSelectorSkipsPurgeUnderLegalHold`（fresh PASS）

- [x] 两个 Reconciler 仅一个持有租约  
  **证据：** `TestOnlyLeaseOwnerReconciles`（第二实例 `ErrLeaseNotOwned`，仅 1 次 `RunPass`）；`cmd/file-reconciler` 的 `TestFileReconcilerUsesStableLeaseName`（fresh PASS）

- [x] 过期配额预占和临时对象最终收敛  
  **部分证据（非全链路 E2E）：** `TestAbortTransferReleasesQuotaReservation`、`TestCreateTransferReplaysAuthoritativeExpiredSession`、配额并发 `TestConcurrentReservationsDoNotOversell`（均含于 fresh `go test ./internal/modules/filesystem/...` PASS）。`RepositoryWorkRunner.releaseExpiredReservations` / `expireStaleTransfers` **无** dedicated 单元测试；`progress` 记录 Task 6 仍存在 orphan temp on expire 的 Important track — **不**声称生产多实例 Reconciler 长跑收敛已验收。

## 继承 G2 的诚实 defer / 未实跑项

- [ ] **S3 MinIO integration contract suite** — 本轮 SKIP；deferred 至具备 MinIO 的环境（同 G2）
- [ ] **`DATABASE_DSN` PostgreSQL 专项** — 本轮 SKIP；deferred 至 G5/集成环境
- [ ] **Reconciler `RepositoryWorkRunner` 批次收敛 E2E**（过期预占批量释放、stale transfer 批量过期、backlog claim）— 仅实现 + 租约单元测试；deferred 至集成/运维验收
- [ ] **多 Reconciler + Redis/真 DB 租约 failover** — 本轮 SQLite 内存夹具 only
- [ ] **Vault/K8s Secret resolver 真机轮换** — Registry 单元测试使用 fake secrets（Task 2 minor）

## 服务重启

- `A:\21-Agent\SparkOpsAgent\scripts\dev-restart.ps1 -Backend`（主仓脚本；G3 验收仅新增 docs，未改主工作区源码）
- Fresh listener PID `23356`（编译主仓 `backend` → `.dev/sparkops-server-dev.exe`）
- Fresh `curl.exe -sS -i --max-time 10 http://localhost:8080/ready`（12:52:13 UTC）：HTTP `200`，`database`/`redis` **up**
