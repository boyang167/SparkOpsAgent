# G5 Unified File Infrastructure Production 验收证据

> 状态：**CONDITIONAL / RELEASE BLOCKED**。G5 生产化交付物（镜像、Compose、Helm、可观测性、安全/一致性集成套件、容量工具、Runbook）与本轮 fresh 静态/夹具测试均 **PASS**；主计划要求的现场容量、Knowledge 真库、灾备 RPO/RTO 与告警触发无目标环境，诚实标记 **SKIP**，不得签署 v1.0 正式发布。

## 验收上下文

- 验收时间：2026-11-13 22:10（UTC+8）
- 工作树：`A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- G5 代码基线提交：`c64a9b0786f634389ed53eb8b8131958eedc17b0`（`deploy(filesystem): harden production network policy`）
- 验收文档提交：`58b2eb4500318b13a4875065187a85218e0f28ca`（含 G5/v1.0 公开文档）
- G1–G4 证据：`docs/verification/unified-file-infrastructure-g1.md` … `g4.md`
- 计划来源：`docs/superpowers/plans/2026-11-12-unified-file-infrastructure-g5-production.md` Task 7
- 验收 shell：`DATABASE_DSN` **未设置**；`FS_LOAD_RUN` **未设置**；本机 **无 `make.exe`**、可用 `mingw32-make`、**无 `docker`**；Helm dependency 未 vendored

## Fresh 命令证据（Task 7 矩阵）

| 命令 | 结果 |
|---|---|
| `cd backend; go test ./... -count=1` | **PASS**，exit 0；约 189s |
| `cd backend; go test ./deployments/helm -count=1` | **PASS**，约 1.2s |
| `cd backend; go test ./internal/modules/filesystem/infrastructure/observability -count=1` | **PASS**，约 2s |
| `cd backend; $env:REQUIRE_INTEGRATION="1"; go test -tags=integration ./tests/filesystem/... -count=1` | **PASS**；performance 现场用例 **SKIP**（见下表） |
| `cd backend; go test ./internal/modules/filesystem/... ./internal/modules/knowledge/... ./internal/modules/mcp/... ./internal/sftpgateway/... -count=1` | **PASS**，约 25s（G4 回归子集） |
| `helm lint deployments/helm/sparkops` | **PASS**；WARNING 缺少 postgresql/redis/rabbitmq 子 chart |
| `cd fronted; pnpm --filter web typecheck` | **PASS** |
| `cd fronted; pnpm vitest run apps/web/src/modules/filesystem … StoragePoolPanel.test.tsx` | **PASS**，4 files / 8 tests |
| `mingw32-make test` / `mingw32-make helm-lint` | **PASS**；使用 Windows 可用的 Make 兼容实现执行同一 Makefile 目标 |

### Integration 明细（`REQUIRE_INTEGRATION=1`）

| 包 / 测试 | 结果 |
|---|---|
| `security` — `TestWorkspaceCannotReadAnotherOrganizationsBlob` 等 5 项 | **PASS** |
| `consistency` — `TestFailureInjectionRecoveryInvariants`（6 子场景） | **PASS** |
| `integration` — `TestEnvironmentRoundTripsAuthorizedContent` | **PASS** |
| `performance` — `TestPercentileUsesNearestRank` | **PASS** |
| `performance` — `TestFilesystemTwentyGiBResume` | **SKIP** — 需 `FS_LOAD_RUN=1` 与 20 GiB fixture |
| `performance` — `TestFilesystemConcurrentTransferCapacity` | **SKIP** — 同上 |
| `performance` — `TestFilesystemMetadataCapacity` | **SKIP** — 同上 |

### Helm 静态验收（G5 Task 2 / 3）

| 测试 | 结果 |
|---|---|
| `TestFilesystemProductionValuesRequireTwoReplicas` | **PASS** |
| `TestFilesystemWorkloadsUseRestrictedPodSecurity` | **PASS** |
| `TestFilesystemPodDisruptionBudgetsCoverWorkloads` | **PASS** |
| `TestFilesystemChartSecretCarriesCapabilityAndHostKey` | **PASS** |
| `TestFilesystemAlertsCoverProductionFailureModes` | **PASS** |
| `kubectl apply --dry-run` / 目标集群 HPA 行为 | **SKIP** — 无 kubectl |

## G5 Task 1–6 交付消费摘要

| Task | 提交 / 证据 | 本轮复跑 |
|---|---|---|
| 1 镜像与 Compose | `4f44484`；`go test ./deployments/helm` 断言 10 二进制 | Helm 测试 **PASS**；`make filesystem-up` **SKIP**（无 docker） |
| 2 Helm Workload | `01f8848`、`c64a9b0`；`filesystem_templates_test.go` | **PASS** |
| 3 可观测性 | `f4a99ad`；metrics + PrometheusRule | observability **PASS** |
| 4 安全与一致性 | `5202857`；`tests/filesystem/security|consistency` | integration **PASS** |
| 5 容量工具 | `162cdf1`；`run-filesystem-load.ps1` + performance 包 | 单元 **PASS**；现场压测 **SKIP** |
| 6 恢复与迁移 | `4a5ec38`；Runbook + `disaster-recovery.md` | 模板就绪；演练 **SKIP**（见灾备文档） |

## 发布硬条件（计划 Task 7）

- [x] 生产副本数、HPA、PDB、Topology Spread — **PASS（静态）**；`values-production.yaml` + Helm 模板测试；无集群扩缩容演练
- [x] NetworkPolicy 与 Secret 引用 — **PASS（静态）**；无现场网络探测
- [x] 跨租户安全测试 0 泄漏 — **PASS（夹具级）**；`TestWorkspaceCannotReadAnotherOrganizationsBlob`
- [x] 故障注入后对账可收敛 — **PASS（夹具级）**；consistency 六场景
- [ ] 20 GiB 恢复与 200 并发 — **SKIP**；未设置 `FS_LOAD_RUN`，无 `artifacts/filesystem-load-report.md`
- [ ] Knowledge 迁移与 rollback — **SKIP**；无 `DATABASE_DSN`；夹具级证据见 G4 `migration` 测试 **PASS**
- [ ] RPO ≤15 分钟，RTO ≤2 小时 — **SKIP**；见 `unified-file-infrastructure-disaster-recovery.md`
- [ ] 所有关键指标和告警可触发 — **SKIP**；PrometheusRule 名称已测，未在 Prometheus/Alertmanager 触发

## G1–G5 累计诚实 defer（节选）

| 项 | 最早 Gate | 本轮 |
|---|---|---|
| PostgreSQL `DATABASE_DSN` 专项 / Knowledge CLI 全链路 | G1/G4 | **SKIP** |
| S3 MinIO contract 集成（`S3_ENDPOINT`） | G2/G3 | **SKIP** |
| 跨组织 Export Worker 字节与 Preview Worker E2E | G4 | **SKIP** |
| SFTP 真 SSH / 生产 Transfer 接线 | G4 | **SKIP** |
| OpenSearch 投影、浏览器 File Explorer E2E | G4 | **SKIP** |
| 20 GiB / 200 并发 / 元数据 100 TB 分布 SLO | G5 | **SKIP** |
| PITR、Provider 快照、Knowledge 真库 rollback | G5 | **SKIP** |

完整 G4 defer 列表见 `unified-file-infrastructure-g4.md`「诚实 defer」节。

## Provider 发布范围

首发原生 Provider：**Local/PVC**、**NFS**、**S3 Compatible**。

**COS**、**OSS**、**Azure Blob** 为 Adapter 扩展路线，不作为 v1.0 已验收原生能力（见 `README.md`、`SparkOps技术架构说明.md`、`SparkOps部署与运维说明.md`）。

## G5 结论与 v1.0 关系

- **G5 门禁（工程交付 + 可复现测试）：** 通过，含诚实 SKIP。
- **v1.0 正式发布签署：** **未通过**；权威汇总见 `docs/verification/unified-file-infrastructure-v1.0.md`。

解除发布阻塞需：归档脱敏容量报告、Knowledge 真库对账、灾备 UTC 时间戳与 RPO/RTO、告警现场触发，并关闭影响数据面的 G4 defer。
