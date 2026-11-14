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
- `pnpm vitest run apps/web/src/modules/filesystem`：exit 0，27 files / 204 tests。
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

## 首轮审查修复

- 审查修复起点 HEAD：`a92c841aa28e6cb430704b5de5d93644073f7d5f`。
- migration helper 统一识别 `1`、`true`、`TRUE`（忽略首尾空格）；默认无 DSN 仍 SKIP，
  integration required 时缺 DSN 必须失败。子进程测试证明 `1/true/TRUE` 均 exit nonzero。
- 安全矩阵不再使用 map membership/role/admin 判定。SQLite authority adapter 从真实
  organization/workspace/member/role/binding/permission rows 查询；生产
  `PrincipalResolver`、`authorization.Service` 与 filesystem repository 保持真实。
- 由于 production tenant/governance persistence SQL 使用 PostgreSQL schema/cast，SQLite 矩阵没有伪称
  使用完整 production tenant/governance repository；仅这层 authority adapter 为测试适配器，且不是
  always-allow/deny stub。
- 顶层目录用例明确验证 membership audience Grant，与 classification 无关；Node 仍为 `INTERNAL`。
  admin-only 用例同时持久化 admin Role ALLOW 和当前 USER 显式 DENY，断言 `explicit_deny`。
- 新增真实 `Modal.tsx` 渲染测试，覆盖 ShareFileModal 与 approval Reject Modal 的 dialog ARIA、
  labelledby/description、打开焦点、Tab trap、Esc 与触发点焦点恢复。为适配现有 Vitest/Vite 组合，
  `Modal.tsx` 仅做 JSX → `createElement` 的等价语法重写，无行为变化。
- 删除本报告误追加的无关任务内容。

### 安全矩阵精确复现

命令：

```powershell
cd backend
$env:REQUIRE_INTEGRATION="1"
$env:DATABASE_DSN="sqlite-authority-fixture"
go test -tags=integration ./tests/filesystem/security -run EnterpriseAuthorizationMatrix -count=1 -v
```

结果：exit 0，以下 11 个 case 全部 PASS：

1. `tenant_owner_administers_organization_namespace`
2. `tenant_owner_cannot_read_another_users_private_namespace_without_grant`
3. `workspace_member_reads_top_level_directory_through_membership_audience_grant`
4. `workspace_member_is_explicitly_denied_admin_only_directory`
5. `workspace_A_member_reads_organization_directory_shared_to_A`
6. `workspace_B_member_cannot_read_directory_shared_only_to_A`
7. `personal_owner_reads_own_private_node`
8. `inherited_allow_reaches_descendant`
9. `direct_deny_overrides_inherited_allow`
10. `revoked_recipient_is_denied_immediately`
11. `cross_organization_workspace_recipient_is_denied`

`DATABASE_DSN` 在此命令只是 security harness 的 required sentinel；矩阵本身使用 SQLite authority rows，
不应被解释为真实 PostgreSQL 证据。

### Migration gate 精确复现

- 默认缺 DSN：聚焦 migration 命令 exit 0，并明确 `SKIP: DATABASE_DSN is not set`。
- `REQUIRE_INTEGRATION=1` 且缺 DSN：同一命令 exit 1，并明确
  `DATABASE_DSN is required when migration integration tests are required`。
- `TestMigrationIntegrationGateSubprocess` 证明 `1`、`true`、`TRUE` 均为 required 且子进程 nonzero。

### 审查修复验证

- `go test ./migrations ./internal/modules/filesystem/... ./api/openapi -count=1`：exit 0。
- security integration 全量（SQLite authority fixture）：exit 0。
- `go vet -tags=integration ./migrations ./tests/filesystem/integration ./tests/filesystem/security`：
  exit 0。
- 真实 Modal + Workbench + Modal focus 聚焦：3 files / 13 tests，exit 0。
- filesystem 前端全量：27 files / 204 tests，exit 0。
- 三 transport tests：3 files / 33 tests，exit 0。
- `pnpm typecheck`：6/6 packages，exit 0。

## 交付状态

实现与自动验证完成；真实 PostgreSQL 验证保持 release blocker。最终 fix commit hash 与提交后工作树
状态记录在交付回复中。
