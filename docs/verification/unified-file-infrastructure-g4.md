# G4 Unified File Infrastructure Integrations 验收证据

> 状态：**PASS（含诚实 SKIP / defer）**。计划 Task 10 fresh 测试矩阵全部 exit 0；未对生产 PostgreSQL 跑 Knowledge CLI 迁移演练、未做 SFTP 真 SSH 往返、未验收跨组织 Export Worker 字节复制与 Preview Worker，不声称上述项已通过。

## 验收上下文

- 验收时间：2026-11-13 21:48（UTC+8）
- 工作树：`A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- 被测实现提交：`c9db9b683389c98ab597136080255dfa1f2c3d14`（`fix(filesystem): import sftp uploads from the landing zone`）
- G1–G3 验收基线：见 `docs/verification/unified-file-infrastructure-g1.md`、`g2.md`、`g3.md`
- 验收 shell：`DATABASE_DSN` **未设置**（Knowledge CLI 演练未执行）；主仓 `dev-restart -Backend` 使用既有本地 DB/Redis
- 计划来源：`docs/superpowers/plans/2026-11-12-unified-file-infrastructure-g4-integrations.md` Task 10

## Fresh 命令证据

| 命令 | 结果 |
|---|---|
| `cd backend; go test ./internal/modules/filesystem/... ./internal/modules/knowledge/... ./internal/modules/mcp/... ./internal/sftpgateway/... -count=1` | **PASS**，exit 0；约 25s |
| `cd backend; go test ./internal/sftpgateway ./cmd/sftp-gateway -count=1` | **PASS**，exit 0；约 4s（Task 7 补充矩阵） |
| `cd backend; go test ./cmd/migrate-knowledge-precopy ./cmd/migrate-knowledge-inventory ./cmd/migrate-knowledge-switch -count=1` | **PASS**（`precopy` 包有测试；`inventory`/`switch` 为 `[no test files]`） |
| `cd backend; go test ./internal/modules/knowledge/migration -count=1` | **PASS**，exit 0 |
| `cd backend; go test ./internal/modules/agent/... ./internal/modules/workflow/... -count=1` | **PASS**（`agent`/`workflow` 的 `infrastructure/acl` **无** test files） |
| `cd fronted; pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts` | **PASS**，4 files / 8 tests |
| `cd fronted; pnpm vitest run apps/web/src/components/layout/nav-tree.test.ts apps/web/src/components/layout/menu-tree-nav.test.ts` | **PASS**，2 files / 16 tests |
| `cd fronted; pnpm vitest run apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx` | **PASS**，2 tests |
| `cd fronted; pnpm typecheck` | **PASS**，6 packages（turbo cache hit） |

## 2026-11-14 Task 8 统一工作台自动验收（UTC+8）

| 项 | 结果 |
|---|---|
| Filesystem 前端完整矩阵 | **PASS** — `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`，fresh 结果为 19 files / 139 tests，exit 0 |
| 前端类型检查 | **PASS** — `pnpm typecheck`，6 packages，exit 0 |
| 后端全量回归 | **PASS** — 清除 shell 中残留 `DATABASE_DSN` / `DATABASE_URL` 后执行 `go test ./... -count=1`，exit 0 |
| 自动无障碍与响应式契约 | **PASS** — 覆盖 tab / tree / list / grid、键盘“移动到…”，Drawer Esc、progressbar、窄屏侧栏 Drawer、横向溢出；tabs/选择 label/关键按钮具备 44px 触控目标，checkbox 视觉尺寸保持 16px；focus-visible 具体断言为 `2px solid var(--primary)` + 2px offset，并自动验证 token 对比度 ≥4.5 |
| 单主动作与 landmark | **PASS（组件/源码契约）** — 文件编辑态上传降为 secondary、提交为唯一 primary；治理 Modal 打开时背景触发器降级；文件浏览容器已移除嵌套 `main` 并保留 tab `aria-labelledby` |
| loading / error / empty / forbidden | **PASS（组件自动测试）** — Filesystem 组件矩阵覆盖独立状态，不以空态替代加载或权限拒绝 |
| 浏览器手工矩阵 | **PASS（LOCAL 环境）** — 管理员四标签、新建目录、懒加载树、键盘移动、列表/网格保留选择、窄屏目录 Drawer、Esc 关闭、LOCAL Provider 治理、审计 ALLOW/DENY、传输取消、详情与版本均通过 |
| 多文件上传数据面 | **PASS（LOCAL 环境）** — 4 个文件经 Gateway PUT 返回 200，执行池实测最大并发 3；干净会话最终为 `COMPLETED`，文件详情显示版本 #1、131072 B |
| MinIO / S3 Compatible | **SKIP** — 未配置真实 MinIO endpoint，不把 LOCAL Provider 验收结果外推为 S3 通过 |
| 实时容量曲线 | **不在范围** — 未实现、未展示，也未伪造容量数据 |

Task 8 本轮只将上述真实自动和 LOCAL 浏览器执行项记为 PASS；20 GiB、200 并发、Knowledge 真库迁移、MinIO 与灾备状态不变。

### 迁移 CLI 演练（计划 Step 2）

| 步骤 | 结果 |
|---|---|
| `migrate-knowledge-inventory` → `precopy` → `switch` → `rollback` → `switch`（对真库输出报告） | **SKIP** — 本轮未设置 `DATABASE_DSN`，未归档 CLI JSON 报告 |
| 逻辑替代证据 | `internal/modules/knowledge/migration`：`TestInventoryReportsFixedCounters`、`TestPrecopy*`、`TestSwitchThenRollbackKeepsFileAndLegacyContent`、`TestSwitchBlockedUnlessFullyVerified` 等 **PASS** |

## 计划 Step 3 固定验收项

- [x] File Explorer 全部操作使用 Node ID  
  **证据：** `FileExplorerPanel.test.ts` — `keeps file identity stable after rename` 断言 `renameNodeKeepingIdentity` / `mockRename` 始终传 `"file-1"`；`submitPanelRename` 对空白 trim 仍保留 ID

- [x] 显式 Deny 覆盖目录继承 Allow  
  **证据：** `domain/service/authorization_test.go` — `TestExplicitDenyOverridesInheritedAllow`；`application/authorization` — `TestAuthorizeExplicitDenyWinsAcrossAncestors`（含于 fresh `filesystem/...` PASS）

- [x] Agent/MCP 不能访问 Capability Scope 外文件  
  **证据：** `mcp/application/filesystem_tools_test.go` — `TestReadFileToolCannotEscapeCapabilityScope`；`domain/service` — `TestCapabilityIsIntersectionNotDelegatorUnion`（含于 fresh PASS）

- [x] Artifact 跨 Workspace 引用被拒绝  
  **证据：** `application/command/create_artifact_test.go` — `TestArtifactCannotReferenceFileFromAnotherWorkspace`

- [x] Knowledge 行数、SHA-256、关联关系 100% 匹配（**夹具级，非生产全量**）  
  **证据：** `cmd/migrate-knowledge-precopy/main_test.go` — `TestPrecopyIsIdempotentAndRecordsChecksum`；`migration` — `TestPrecopyDoesNotVerifyMismatchedChecksum`、`TestInventoryReportsFixedCounters`  
  **未声称：** 生产库上 `legacy document count = verified mapping count` 的 CLI 对账；见 defer

- [x] Knowledge rollback 不复制或丢失内容  
  **证据：** `migration/migration_test.go` — `TestSwitchThenRollbackKeepsFileAndLegacyContent`（rollback 后 legacy `content` 与 file 计数不变）

- [x] SFTP 不能逃逸虚拟根且不直写生产存储（**单元/夹具级**）  
  **证据：** `internal/sftpgateway` — `TestSFTPRejectsEscapeFromVirtualRoot`、`TestSessionLandingDirIsIsolated`、`TestCloseCreatesAndCompletesTransferInLandingZone`、`TestNewFromConfigRejectsLandingZoneOverlappingStorage`  
  **未声称：** 真实 SFTP 客户端、已接线 Transfer Control、Vault 主机密钥；见 defer

- [x] 搜索结果在返回前重新授权  
  **证据：** `application/query/search_files_test.go` — `TestSearchReauthorizesEveryResult`

### 额外 G4 前端 / 授权项（矩阵外、本轮实跑）

- [x] `/files` 导航受 entitlement 控制 — `nav-tree.test.ts` / `menu-tree-nav.test.ts` PASS
- [x] 存储管理 UI 不脱敏 credential、无 `storage:admin` 时隐藏变更 — `StoragePoolPanel.test.tsx` PASS
- [x] 跨组织 Grant 拒绝 — `grant_resource_test.go` — `TestCrossOrganizationGrantIsRejected`

## 诚实 defer / 未实跑项

- [ ] **Knowledge CLI 全量迁移演练**（inventory / precopy / switch / rollback 对 PostgreSQL 连续执行并归档 JSON）— **SKIP**；deferred 至 G5 或集成环境（需 `DATABASE_DSN`）
- [ ] **Knowledge `FilesystemACL` 对接真 Filesystem Blob/Provider** — 仅有 SQLite 内存 + `ATTACH` 的 `TestStoreDocumentIsIdempotentAcrossInstances`；staging 表非生产 `filesystem` schema 全路径
- [ ] **跨组织 Controlled Export 字节面** — `TestControlledExportRecordsTaskWithoutGrant`、`TestImportCreatesNewFileIdentity` 覆盖任务/metadata/新 Node ID；**无** Worker 读源 Blob、目标 dedup、重扫描的 E2E
- [ ] **SFTP Gateway 生产接线** — `UnconfiguredTokenValidator` / `UnconfiguredTransferControl`、无 SSH 端到端；见 `.superpowers/sdd/g4-task-7-report.md`
- [ ] **Agent / Workflow `infrastructure/acl/filesystem.go`** — 无 dedicated 单元测试；仅依赖模块其余包级 PASS
- [ ] **Preview 派生 Blob** — `GeneratePreviewHandler` + subscriber 注册存在；**无** `generate_preview` 专项测试或 file-worker 集成证据
- [ ] **OpenSearch 全文投影** — 计划为可选；本轮未跑 OpenSearch 依赖测试
- [ ] **File Explorer 下载/回收站浏览器 E2E** — LOCAL 上传、浏览、移动、详情与版本已手工通过；下载和回收站仍无端到端证据

## 服务重启

- `A:\21-Agent\SparkOpsAgent\scripts\dev-restart.ps1 -Backend`（主仓脚本；编译主仓 `backend` → `.dev/sparkops-server-dev.exe`）
- Fresh listener PID `19704`
- Fresh `curl.exe -sS -i --max-time 10 http://localhost:8080/ready`（21:48:50 UTC）：HTTP `200`，`database`/`redis` **up**

## G5 生产化候选结论

- **可进入 G5 计划评审：** 包级测试矩阵与计划 Task 10 核心验收项（Node ID、授权、MCP scope、Artifact 边界、Knowledge switch/rollback 夹具、SFTP 虚拟根、搜索再授权）均有 fresh 证据。
- **G5 必须补齐：** Knowledge 真库迁移对账、Export Worker 字节与审计双边、SFTP/MCP/Agent 运行态接线、Preview Worker、`DATABASE_DSN` 集成与可选 OpenSearch。
