# Task 13 实施与验证报告

## 范围与基线

- 日期：2026-11-15（UTC+8）。
- 分支：`master`。
- 基线：`5fa4680a6b8c395b30eee32b6dfa3fdc8d3fa7f6`。
- 开始时工作树干净。
- 仅增加测试、测试 harness 门禁与验证文档；未新增产品行为。

## 实施

1. Authorization matrix
   - 在 `tenant_isolation_test.go` 增加 11 行独立矩阵。
   - 每个 case 使用独立 SQLite DB、真实 filesystem repository、生产
     `NewPrincipalResolver` 与 `authorization.Service`。
   - fixture 明确创建 Organization、Workspace、Namespace、Node、typed Grant、Role、
     membership 和 admin authority。
   - 覆盖 Tenant Owner Organization admin、Tenant Owner 不可读他人个人盘、
     Workspace public/admin-only、A/B Workspace share、personal owner、继承、
     DENY precedence、revoke 立即生效与 cross-org。

2. Integration harness
   - `REQUIRE_INTEGRATION=1` 且缺少 `DATABASE_DSN` 时改为 `Fatal`，不再静默执行或 skip。
   - 未修改产品代码。

3. PostgreSQL migration rehearsal
   - 增加 `TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack`。
   - 一次性 PostgreSQL DB 中从 77 建到 82、插入 existing-data，再执行 83–87。
   - 自动断言 Namespace `WORKSPACE` 回填、Grant `USER` 回填、旧文件/旧 Grant 保留、
     typed Grant revoke active count 归零。
   - 在同一 disposable DB 执行 87 → 82 down，并断言旧数据保留及 DOWNLOAD Grant
     被 85 down 删除。
   - 测试输出现场 row counts；本机无 PostgreSQL，因此未产生实际 row counts。

4. Accessibility
   - accessibility suite 从 6 项扩至 10 项。
   - 真实渲染 FileSpaceNav、SharedWithMePanel、ShareFileModal、ShareApprovalPanel、
     FileDetailDrawer。
   - 覆盖四空间 nav、shared loading/error、Modal 两步、approval loading/action/reject、
     Drawer tabs/error/focus/dialog/tab/tabpanel ARIA；保留既有触控、对比度、focus、
     responsive 与 reduced-motion 断言。
   - 仓库无 axe 依赖，沿用 Testing Library role/name/state 的现有方式；未放宽规则。

5. Documentation
   - 新增 `docs/verification/enterprise-file-governance-sharing.md`。
   - 记录日期、基线 commit、命令/exit/result、授权矩阵、环境、兼容性自动/人工/未验证边界、
     rollout、监控、forward-fix、rollback 门禁及已知限制。
   - 明确 85 down 删除 DOWNLOAD grants；86 down 的 quota/transfer 门禁；87 down 的
     lineage 门禁。
   - `unified-file-infrastructure-v1.0.md` 增加版本关系与链接，保持原 RELEASE BLOCKED。

## 验证结果

- `go test ./internal/modules/filesystem/... -count=1`：exit 0。
- `go test ./migrations -count=1`：exit 0；PostgreSQL 条件测试因无 DSN skip。
- `REQUIRE_INTEGRATION=1 go test -tags=integration ./tests/filesystem/security/... -count=1`：
  exit 1，所有 suite 明确报 `DATABASE_DSN is required when REQUIRE_INTEGRATION=1`。
- authorization matrix 使用隔离 SQLite fixture 聚焦执行：exit 0，11/11 PASS。
- `go vet -tags=integration ./tests/filesystem/integration ./tests/filesystem/security ./migrations`：
  exit 0。
- `go test ./api/openapi ./internal/modules/filesystem/interface/http -count=1`：exit 0。
- `pnpm vitest run apps/web/src/modules/filesystem`：exit 0，26 files / 202 tests。
- 三 transport tests：exit 0，3 files / 33 tests。
- `pnpm typecheck`：exit 0，6/6 packages。
- IDE diagnostics：0。
- `git diff --check`：exit 0；仅 LF/CRLF checkout warning。
- filesystem migrations 77–87 连续，最新 `000087`。
- 全仓历史存在两个 `000061` up migration；为基线既有问题，本任务未修改。

## PostgreSQL 与 row counts

- `DATABASE_DSN`：未设置。
- Docker：命令不存在。
- 未安装软件，未连接开发/生产库。
- 真实 PostgreSQL 82 → 87 → 82：**NOT RUN / RELEASE BLOCKER**。
- 实际 row counts：**未取得**；文档没有把测试 fixture 预期值冒充现场计数。

## 兼容性分类

- 自动：旧 Workspace Grant handler、旧 `/files` transport/forwarding、OpenAPI 与旧 node
  migration retention。
- 人工：真实浏览器登录态 smoke 留给 controller 在已有可用会话时执行。
- 未验证：真实 PostgreSQL 上迁移后旧 `/files` API 读取。

## 已知限制汇总（Tasks 1–12）

- 历次任务均未取得本机真实 PostgreSQL migration/up/down/并发证据。
- 当前人类会话 resolver 不产生 AGENT/SERVICE_ACCOUNT。
- 未做真实登录态浏览器端到端；身份显示名仍有 nullable/fallback 边界。
- preview 无浏览器读取 capability；审计 offset cursor 在高并发下可能位移。
- 20 GiB、200 并发、Knowledge 真库迁移、RPO/RTO、灾备和真实 S3/MinIO 不因本任务提升为 PASS。
- 85 down 删除 DOWNLOAD grants，生产不得普通 rollback。
- 86 down 遇非 Workspace transfer 或活跃非 Workspace quota 时拒绝。
- 87 down 存在 lineage 时拒绝。

## 交付状态

实现和自动验证完成；真实 PostgreSQL 验证保持 release blocker。最终提交 hash 与提交后工作树状态
记录在 controller 交付回复中。
# Task 13 Report: Kubernetes 终端审计 records 与 Exec 录像

## 实现摘要

- 新增 `backend/internal/plugins/kubernetes/records`，提供 `Store`、`SessionRecorder` 与 records HTTP handler。
- `GET /records` 按 workspace 隔离分页，`page>=1`、`pageSize=1..100`、安全整数校验，按 `started_at DESC` 返回，列表显式 select 元数据不返回 blob。
- `GET /records/:recordId` 仅查询当前 workspace，未知与跨 workspace 均返回 404，响应为 `application/x-ndjson` 并设置 `Cache-Control: no-store`。
- `DELETE /records/:recordId` 在 DB delete 前执行 write guard，仅删除当前 workspace record，跨 workspace 与未知统一 404。
- Exec 会话开始后创建 `plugin_kubernetes.records` 记录；stdout、stderr、exit 写入简化 asciicast JSON Lines；结束、失败或取消均 defer 写入 `EndedAt` 与 blob。
- 录像内不写命令 argv、token 或 rest config；录像最大 5MiB，超过后写一次 `truncated` marker 并停止增长。
- 录像创建失败时 Exec 继续，只向 SSE 返回固定 audit warning，不泄漏 DB 细节；结束写回失败仅写内部 logger。
- 路由、manifest、bundled descriptor 和 records 查询索引已更新；新增 `plugin.NopAudit` 作为通用 no-op 审计实现。

## 测试

- 红灯：先运行 `go test ./internal/plugins/kubernetes/records ./internal/plugins/kubernetes/stream`，确认 records 新 API 缺失与 Exec 未持久化录像失败。
- 绿灯：`go test ./internal/plugins/kubernetes/records ./internal/plugins/kubernetes/stream ./internal/plugins/kubernetes/http ./internal/plugins/kubernetes ./internal/plugins/kubernetes/schema` 通过。
- 回归：`go test ./internal/platform/operationlog` 通过，确认 SSE response 仍省略。

## 自审要点

- records handler 所有读写查询均带 `workspace_id` 条件。
- 列表查询未 select `blob`。
- DELETE 权限检查发生在 DB delete 前。
- Exec 录像只记录 stdout/stderr/exit 内容，不记录命令、rest config 或平台 operation log 元数据。
- 报告文件未纳入提交。

## 审查阻断修复追加

- 严格 5MiB 上限：预生成固定合法 JSONL `truncated` marker，并在普通 frame 写入前为 marker 保留空间；触顶时只追加一次完整 marker，最终 blob 长度不超过 `MaxRecordingBytes`。
- Finalize 隔离：`SessionRecorder` 保存 `workspaceID`，结束写回使用 `WHERE id = ? AND workspace_id = ?`，且 `RowsAffected != 1` 返回错误。
- Manifest：移除 Kubernetes manifest 中 GET records 和其他 GET 的显式 `read_only: true`，由 binder 对 GET 默认只读处理；补实际 manifest parse 回归与 persistence seed 期望。
- 追加验证：`go test -count=1 ./internal/plugins/kubernetes/records ./internal/plugins/kubernetes/stream ./internal/plugins/kubernetes/http ./internal/plugins/kubernetes ./internal/modules/plugin/domain/manifest ./internal/modules/plugin/infrastructure/persistence ./internal/modules/plugin/application/install` 通过。
