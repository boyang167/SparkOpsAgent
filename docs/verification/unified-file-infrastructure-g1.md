# G1 Unified File Infrastructure 验收证据

> 状态：**PASS**。Copy 并发修复已在当前 HEAD 通过 fresh 验收；Backend `/ready` fresh HTTP 200。

## 验收上下文

- 验收时间：2026-11-13 08:54（UTC+8）
- 工作树：`A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- HEAD：`63e6fba866f5f39c3b24c05e0b96c41182d93456`
- HEAD subject：`fix(filesystem): retry copy transaction conflicts`
- Copy 修复证据：Task7 原失败并发组 fresh `-count=20` 通过，exit 0。
- PostgreSQL 集成条件：当前验收 shell 的 `DATABASE_DSN` 未设置；专门依赖该环境变量的 PostgreSQL 集成测试标记为 **SKIP**，不声称通过。运行中的后端使用本机既有 `CONFIG_PATH`，其 readiness 数据库检查独立为 up。
- 用户决策：G1 不提供 binary 传输端点；binary 上传/下载由 G2 承载。OpenAPI 只冻结 G1 元数据和操作接口。

## Fresh 命令证据

| 命令 | 结果 |
|---|---|
| `cd backend; go test ./internal/modules/filesystem/... -count=1` | PASS，exit 0；filesystem 各包均通过 |
| `cd backend; go test ./internal/platform/storage/... -count=1` | PASS，exit 0；Local Provider contract 通过 |
| `cd backend; go test ./internal/platform/authz ./internal/platform/entitlements -count=1` | PASS，exit 0 |
| `cd backend; go test ./internal/platform/messaging/... -count=1` | PASS，exit 0 |
| `cd backend; go test ./internal/platform/postgres/... -count=1` | PASS，exit 0 |
| `cd backend; go test ./api/openapi ./migrations/... ./internal/bootstrap/... -count=1` | PASS，exit 0 |
| `cd backend; go test ./... -count=1` | PASS，exit 0；全仓包级测试完成 |
| `cd backend; go test ./internal/modules/filesystem/application/command -run "Test(CreateNamespaceConcurrentSameWorkspaceReturnsSingleNamespace\|PutLocalFileConcurrentQuotaAllowsOnlyOneWinner\|PutLocalFileConcurrentDifferentCommandsAllocateContinuousUniqueRevisions\|MoveConcurrentOppositeMovesCannotCreateCycle\|MetadataMutationsUseCommandDedup\|CopyCommandDedupConcurrentReturnsAuthoritativeResult\|RenameCommandDedupConcurrentAuditsOnce)" -count=20` | PASS，exit 0；Task7 原失败并发组通过 |
| `cd backend; go test ./internal/modules/filesystem/infrastructure/persistence -run "Test(CreateNamespaceConcurrentSameWorkspaceReturnsSingleNamespace\|SaveRevisionConcurrentIdenticalRequestsAreIdempotent\|ClaimCommandConcurrentSQLiteSameAndDifferentHashesAreStable)" -count=20` | PASS，exit 0 |
| `cd backend; go vet ./internal/modules/filesystem/... ./internal/platform/storage/... ./internal/platform/authz/... ./internal/platform/entitlements/... ./internal/platform/messaging/... ./internal/platform/postgres/... ./api/openapi/... ./migrations/... ./internal/bootstrap/...` | PASS，exit 0 |
| `git diff --check` | PASS，exit 0 |
| `cd fronted; pnpm typecheck` | PASS；6/6 packages |
| `cd fronted; pnpm vitest run` | PASS；94 files，498 tests |
| `cd fronted; pnpm lint` | 工具限制，exit 1；`next lint` 进入交互式 ESLint 配置。该非计划门禁不计为测试通过，也不阻塞本次计划验收门禁 |

### PostgreSQL 集成与服务启动

- PostgreSQL 专门集成：**SKIP**，原因是当前验收 shell 的 `DATABASE_DSN` 缺失。
- 控制器使用本机既有 `CONFIG_PATH` 重启后端；进程链根 PID `22184`，本次 fresh listener PID `2056`。
- Fresh `curl.exe -sS -i --max-time 10 http://localhost:8080/ready`：HTTP `200`，响应 `{"code":0,"message":"ok","data":{"checks":{"database":"up","redis":"up"},"status":"ready"}}`。
- readiness 的 database `up` 证明运行中后端连接了本机配置数据库；不等同于 `DATABASE_DSN` 专门集成测试通过。

## Brief 六项证据

- [x] Workspace namespace 自动创建且唯一：filesystem 全量测试通过；并发 namespace 测试 `-count=20` 通过。
- [x] 同目录规范化名称冲突被拒绝：filesystem persistence 全量测试通过。
- [x] Revision 不可变：domain model 全量测试通过。
- [x] Local Provider 防路径逃逸并校验 SHA-256：storage 全量 contract 测试通过。
- [x] 配额事务与 Outbox 失败时完整回滚：filesystem command 全量测试通过，包含 publish 失败回滚测试；PostgreSQL 集成仍为 SKIP。
- [x] filesystem Entitlement 和 `file:*` 权限强制生效：authz/entitlements fresh 测试通过，前端 entitlement 测试包含在 498 个 Vitest 测试中。

上述勾选由本轮 fresh 测试证据支持。

## 额外验收记录

- Namespace/root：root directory 为 namespace 内部根节点，root parent 为 nil；列表接口默认 namespace root 且隐藏 root。
- Idempotency：metadata mutation、copy、rename 及 persistence claim 有测试覆盖；persistence 并发幂等 `-count=20` 通过。
- Outbox/audit：publish 失败回滚、成功/拒绝 mutation audit 和不泄露正文的测试通过。
- OpenAPI：OpenAPI filesystem schema 与 bootstrap/openapi 测试通过；G1 不暴露 binary content endpoint。
- Copy 修复复验：`63e6fba` 增加并发冲突后的完整事务重试；原失败 `TestCopyCommandDedupConcurrentReturnsAuthoritativeResult` 已在指定并发组 `-count=20` fresh 通过。
- 本报告不做无法由本轮命令证明的性能、HA 或 PostgreSQL 集成结论。
