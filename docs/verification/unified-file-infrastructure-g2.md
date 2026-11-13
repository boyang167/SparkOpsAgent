# G2 Unified File Infrastructure Transfer 验收证据

> 状态：**PASS（含诚实 SKIP）**。计划 Task 7 fresh 测试矩阵通过；S3 MinIO 集成 contract 与 `DATABASE_DSN` 专项 PostgreSQL 测试未实跑，不声称通过。

## 验收上下文

- 验收时间：2026-11-13 19:53（UTC+8）
- 工作树：`A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- 被测实现提交：`2b4d0f1e58e4ffe9b74860a55eca719b656184c0`
- 被测实现 subject：`feat(filesystem): add transfer API and web SDK`
- G1 验收基线：见 `docs/verification/unified-file-infrastructure-g1.md`（G2 继承其 Global Constraints）
- 验收 shell：`DATABASE_DSN` **未设置**；`S3_ENDPOINT` / `S3_BUCKET` **未设置**
- 计划来源：`docs/superpowers/plans/2026-11-12-unified-file-infrastructure-g2-transfer.md` Task 7

## Fresh 命令证据

| 命令 | 结果 |
|---|---|
| `cd backend; go test ./internal/modules/filesystem/... ./internal/filesystemgateway/... ./internal/platform/storage/... -count=1` | **PASS**，exit 0；约 16.5s；含 `command`、`filesystemgateway`、`local`/`nfs`/`s3`（单元）包 |
| `cd backend; $env:REQUIRE_INTEGRATION="1"; go test -tags=integration ./internal/platform/storage/adapters/s3 -count=1` | **PASS**（包级），exit 0；2 项 integration **SKIP**，4 项单元 PASS（见下表） |
| `cd fronted; pnpm vitest run packages/web-sdk/src/filesystem.test.ts` | **PASS**；1 file，1 test |
| `cd fronted; pnpm typecheck` | **PASS**；6/6 packages（turbo cache hit） |

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

其余 Transfer 并发/幂等/配额用例在本轮通过 SQLite 夹具实跑（`application/command` 包全量 PASS）。

## 计划 Step 2 固定验收项

- [x] 相同 Idempotency-Key 不产生重复 Session  
  **证据：** `TestCreateTransferConcurrentSameKeyHasOneSideEffectSet`、`TestCreateTransferSameKeyDifferentParametersConflicts`（含于 fresh `go test ./internal/modules/filesystem/...` PASS）

- [x] 两个 60-byte 并发上传在 100-byte 配额下仅一个成功  
  **证据：** `TestConcurrentReservationsDoNotOversell`（fresh PASS）

- [x] 已完成分片不会在恢复时重复上传  
  **证据：** Vitest `uploads only missing parts when resuming`（`packages/web-sdk/src/filesystem.test.ts`，fresh PASS）

- [x] Checksum 不匹配不创建 Revision  
  **证据：** `TestVerifyTransferRejectsIntegrityMismatch` → `ErrTransferIntegrity`，未进入 Commit（fresh PASS）

- [x] Scanner REJECTED 不创建用户可见文件  
  **证据：** `TestVerifyTransferRejectsInfectedFile`、`TestVerifyTransferRestrictedScanFailureRejected` → Session `REJECTED`（fresh PASS）；非 Restricted 扫描不可用仍为可重试路径：`TestVerifyTransferScannerUnavailableFailsRetryably`

- [x] Commit 重试只创建一个 Revision 和一个 Outbox 事件  
  **证据：** `TestCommitTransferIsAtomicAndIdempotent`（`FileRevisionCreated` / `TransferCompleted` outbox 各 1；`assertQuotaCommittedOnce`）（fresh PASS）

- [x] Local/NFS Provider Contract Suite 通过  
  **证据：** `contracttest.Run` 于 `storage/adapters/local/provider_test.go`、`storage/adapters/nfs/provider_test.go`（fresh `go test ./internal/platform/storage/...` PASS）

- [x] S3 Provider 单元与配置校验通过；**against 真 endpoint 的 contract/multipart 集成 suite 为 SKIP**（无 MinIO）  
  **证据：** 上表；**不**将 integration SKIP 计为 contract suite 全绿。

## 诚实 defer / 未实跑项

- [ ] **S3 MinIO integration contract suite**（`REQUIRE_INTEGRATION=1` + `S3_ENDPOINT`/`S3_BUCKET`）— 本轮 SKIP；deferred 至具备 MinIO 的 CI/验收环境（计划 Task 3 Step 5）
- [ ] **`DATABASE_DSN` PostgreSQL 专项** — 本轮 SKIP；deferred 至 G5/集成环境
- [ ] **200 并发 Transfer / 20GB E2E 大文件** — 无自动化证据；deferred to Task 6+/G5
- [ ] **ClamAV 真机 TCP 扫描** — 本轮为 `port.NoopScanner` / scanner 包单元测试；真 ClamAV E2E 未声称

## 服务重启

- `A:\21-Agent\SparkOpsAgent\scripts\dev-restart.ps1 -Backend`（主仓脚本；未修改主工作区源码）
- Fresh listener PID `24400`（编译主仓 `backend` → `.dev/sparkops-server-dev.exe`）
- Fresh `curl.exe -sS -i --max-time 10 http://localhost:8080/ready`（11:54:45 UTC）：HTTP `200`，`database`/`redis` **up**
