# Enterprise File Governance and Sharing 验证记录

> 验证日期：2026-11-15（UTC+8）  
> 基线提交：`5fa4680a6b8c395b30eee32b6dfa3fdc8d3fa7f6`  
> 发布判定：**BLOCKED — 缺少真实 PostgreSQL 82 → 87 → 82 演练证据**

## 环境

- Windows 10 `10.0.19045`，PowerShell。
- `DATABASE_DSN`：未设置。
- `REQUIRE_INTEGRATION`：初始未设置；强制集成命令单独设置为 `1`。
- Docker：命令不存在，未安装软件，未连接或修改任何开发、测试、生产 PostgreSQL。
- 因无可用 PostgreSQL，本文没有把 migration SKIP/FAIL 写成 PASS，也没有填写虚构 row count。

## 自动授权矩阵

`backend/tests/filesystem/security/tenant_isolation_test.go` 的 11 个独立 fixture 均使用生产
`PrincipalResolver`、`authorization.Service`、filesystem repository 和真实 SQLite 行。测试专用
authority adapter 从 organization/workspace/member/role/binding/permission 表查询，不使用
always-allow/deny 或 map 判定。production tenant/governance persistence SQL 依赖 PostgreSQL schema/cast，
所以本矩阵不冒称使用完整 production tenant/governance repository；这一个 adapter 层是明确的测试边界。

- Tenant Owner → Organization Namespace `ADMIN`：ALLOW（organization admin authority）。
- Tenant Owner → 他人 User Namespace `READ` 且无 Grant：DENY。
- Workspace member → 顶层目录，membership audience 的 Workspace `READ ALLOW` Grant：ALLOW。
  这是 top-level visibility policy；Node classification 仍为 `INTERNAL`，不是 `PUBLIC` classification。
- Workspace member → admin-only 目录，admin Role ALLOW + 当前 USER 显式 DENY：DENY，
  理由 `explicit_deny`；`INTERNAL` classification 不代表 admin-only。
- Workspace A member → 分享给 Workspace A 的 Organization 目录：ALLOW。
- Workspace B member → 仅分享给 Workspace A 的同一 Organization 目录：DENY。
- Personal owner → 自己 User Namespace Node：ALLOW。
- 祖先 Workspace Grant 且 `inherit_to_children=true` → 子目录：ALLOW。
- 祖先 ALLOW + 子目录 USER DENY：DENY，理由 `explicit_deny`。
- USER Grant 撤销前 ALLOW，repository revoke 后下一次授权立即 DENY。
- Organization A 资源 + Organization B Workspace 上下文：DENY，理由 `organization_boundary`。

这套矩阵证明应用授权链路和持久化过滤行为；它不替代 PostgreSQL migration 演练。

精确复现命令（即使调用者环境残留 `REQUIRE_INTEGRATION=1` 也可稳定 exit 0）：

```powershell
cd backend
$previousDatabaseDsn = [Environment]::GetEnvironmentVariable("DATABASE_DSN", "Process")
$previousRequireIntegration = [Environment]::GetEnvironmentVariable("REQUIRE_INTEGRATION", "Process")
try {
  $env:REQUIRE_INTEGRATION = "1"
  $env:DATABASE_DSN = "sqlite-authority-fixture"
  go test -tags=integration ./tests/filesystem/security -run EnterpriseAuthorizationMatrix -count=1 -v
} finally {
  if ($null -eq $previousDatabaseDsn) {
    Remove-Item Env:DATABASE_DSN -ErrorAction SilentlyContinue
  } else {
    $env:DATABASE_DSN = $previousDatabaseDsn
  }
  if ($null -eq $previousRequireIntegration) {
    Remove-Item Env:REQUIRE_INTEGRATION -ErrorAction SilentlyContinue
  } else {
    $env:REQUIRE_INTEGRATION = $previousRequireIntegration
  }
}
```

这里的 `sqlite-authority-fixture` 只是启用 security harness 的 SQLite authority fixture 的 sentinel，
不是 PostgreSQL DSN，绝不能用于 migration test。命令的 `finally` 会清除本次设置或恢复调用前的
`DATABASE_DSN` / `REQUIRE_INTEGRATION`，避免污染后续命令。

## 2026-11-15 命令记录

- `go test ./internal/modules/filesystem/... -count=1`：exit 0，filesystem 全量 PASS。
- `go test ./migrations -count=1`：exit 0，静态/SQLite 测试 PASS；依赖 `DATABASE_DSN`
  的 PostgreSQL tests SKIP，因此不能据此声明 82 → 87 → 82 PASS。
- `$env:REQUIRE_INTEGRATION="1"; go test -tags=integration ./tests/filesystem/security/... -count=1`：
  exit 1，明确报错 `DATABASE_DSN is required when REQUIRE_INTEGRATION=1`；真实 PostgreSQL security
  集成未运行，发布阻断。
- 使用隔离 SQLite authority rows 并设置非连接用途的 harness sentinel 后运行矩阵聚焦命令：
  按上方命令显式设置 `DATABASE_DSN=sqlite-authority-fixture` 后执行
  `go test -tags=integration ./tests/filesystem/security -run EnterpriseAuthorizationMatrix -count=1 -v`，
  exit 0，11 行 authorization matrix PASS；该结果不冒充 PostgreSQL。
- `go vet -tags=integration ./tests/filesystem/integration ./tests/filesystem/security ./migrations`：
  exit 0。
- `go test ./api/openapi ./internal/modules/filesystem/interface/http -count=1`：exit 0，
  OpenAPI 与旧 Workspace Grant handler 自动契约 PASS。
- `pnpm vitest run apps/web/src/modules/filesystem`：exit 0，27 files / 204 tests。
- `pnpm vitest run packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts`：
  exit 0，3 files / 33 tests。
- `pnpm typecheck`：exit 0，6 packages。
- `git diff --check`：exit 0；仅 Git 提示两个 Go 文件未来 checkout 的 LF/CRLF 转换，没有 whitespace error。
- migration 检查：filesystem `000077` 至 `000087` 连续，最新为 `000087`。仓库历史上存在两个
  `000061` up 文件（`release_plan_audit_log_index`、`release_plan_hooks`），是本任务前已存在的全局编号重复；
  本任务未修改或掩盖。

## 迁移演练状态

已增加可重复执行的
`TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack`。测试会在
`openMigrationPostgres` 创建的一次性数据库中：

1. 执行 `000077` 至 `000082`，插入既有 Workspace Namespace、旧文件、旧 USER Grant 与 quota；
2. 执行 `000083` 至 `000087`；
3. 断言旧 Namespace 全部回填为 `WORKSPACE`、旧 Grant 全部回填为 `USER`；
4. 断言旧文件行和旧 Grant 仍存在；
5. 新建 typed Workspace Grant，软撤销后断言 active row count 立即为 0；
6. 插入一个 `DOWNLOAD` Grant；
7. 执行 `000087`、`000086` down 后，默认执行 `000085` down，断言门禁拒绝且
   `DOWNLOAD` Grant 与 `filesystem.share_requests` 表均保留；
8. 在同一显式事务中设置
   `SET LOCAL sparkops.allow_destructive_filesystem_000085_down = 'on'` 后执行 `000085` down，
   再完成 `000084`、`000083` down；
9. 断言旧文件和旧 Grant 仍存在，并断言只有显式授权后 `DOWNLOAD` Grant 才被删除。

本机未执行该 PostgreSQL 测试，因此本次实际 row counts 为：**未取得（NOT RUN）**。测试中预期的
fixture counts 不能冒充现场结果。发布前必须在提供 `DATABASE_DSN` 的隔离 PostgreSQL 上执行：

```powershell
cd backend
$previousRequireIntegration = [Environment]::GetEnvironmentVariable("REQUIRE_INTEGRATION", "Process")
try {
  $env:REQUIRE_INTEGRATION = "1"
  go test ./migrations -run TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack -count=1 -v
} finally {
  if ($null -eq $previousRequireIntegration) {
    Remove-Item Env:REQUIRE_INTEGRATION -ErrorAction SilentlyContinue
  } else {
    $env:REQUIRE_INTEGRATION = $previousRequireIntegration
  }
}
```

真实输出中的 row counts 必须归档后，才能解除 release blocker。

迁移门禁聚焦命令：

```powershell
$previousDatabaseDsn = [Environment]::GetEnvironmentVariable("DATABASE_DSN", "Process")
$previousRequireIntegration = [Environment]::GetEnvironmentVariable("REQUIRE_INTEGRATION", "Process")
try {
  # 默认：缺 DSN 时 exit 0 且明确 SKIP
  Remove-Item Env:REQUIRE_INTEGRATION -ErrorAction SilentlyContinue
  Remove-Item Env:DATABASE_DSN -ErrorAction SilentlyContinue
  go test ./migrations -run TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack -count=1 -v

  # 强制：缺 DSN 时预期 exit nonzero
  $env:REQUIRE_INTEGRATION = "1"
  go test ./migrations -run TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack -count=1 -v
} finally {
  if ($null -eq $previousDatabaseDsn) {
    Remove-Item Env:DATABASE_DSN -ErrorAction SilentlyContinue
  } else {
    $env:DATABASE_DSN = $previousDatabaseDsn
  }
  if ($null -eq $previousRequireIntegration) {
    Remove-Item Env:REQUIRE_INTEGRATION -ErrorAction SilentlyContinue
  } else {
    $env:REQUIRE_INTEGRATION = $previousRequireIntegration
  }
}
```

## 兼容性证据边界

- 自动：旧 Workspace Grant handler 契约由
  `internal/modules/filesystem/interface/http/access_test.go` 覆盖，旧
  `POST /api/v1/workspaces/{wsId}/files/{nodeId}/grants` 保持可用。
- 自动：旧 Workspace `/files` transport/forwarding 由 upstream、web-sdk 和
  `filesystem-forwarding.test.ts` 覆盖；migration 测试额外检查旧 node 行在 up/down 后仍存在。
- 自动：OpenAPI 本地引用、运行时路由一致性和 migration 连续编号由现有测试与本轮检查覆盖。
- 人工：真实浏览器登录态 `/files` smoke 本轮未执行；若 controller 已有可用本地会话，由
  controller 后续完成，不以 jsdom 测试替代。
- 未验证：真实 PostgreSQL 上旧 `/files` API 读取迁移后数据；这是同一 release blocker 的一部分。

## 无障碍验证

真实渲染测试覆盖四空间 navigation、shared loading/error panel、分享 Modal 两步、
approval panel loading/action/reject Modal、文件详情 Drawer tabs、loading/error、焦点、dialog/tab/tabpanel
ARIA，以及既有 tree/list/grid、触控目标、focus-visible、reduced-motion 和对比度契约。
`EnterpriseGovernanceModal.accessibility.test.tsx` 不 mock `Modal`，直接验证生产 `Modal.tsx` 的
`aria-modal`、`aria-labelledby`、`aria-describedby`、打开焦点、Tab trap、Esc 和触发点焦点恢复。
本包也调整了生产 `Modal.tsx`：为兼容真实 a11y 测试链路对 `inert` 属性语法的处理，将 Modal 标记从
JSX 等价改写为 `createElement`；组件没有新增 `inert` 行为，DOM 属性与交互语义不变。上述真实组件测试
覆盖并验证了这项等价调整。
仓库未安装 axe，因此沿用 Testing Library 的 role/name/state 查询与真实组件渲染；未削弱规则或屏蔽失败。

## Rollout 顺序

1. 先部署 `000083` typed Namespace 与 `000084` typed principal Grant，保持旧 Workspace route。
2. 再部署 `000085` sharing approval 和 `000086` Namespace quota；确认后台任务、API 与旧 route 健康。
3. 再部署 `000087` lineage 与 adoption。
4. Tasks 1–5 与 Task 8 后端/transport 同批可用后，才开启 Organization/User 导航。
5. Tasks 9–12 UI 最后开启；旧 Workspace route 至少保留一个 release。
6. 观察一个完整发布窗口后，再考虑移除兼容入口；本版本不移除。

## 监控指标

- 授权：`default_deny`、`explicit_deny`、`organization_boundary` 比率及跨组织拒绝数。
- 分享：Grant create/revoke、Share Request pending/approve/reject 量、审批时延、失败率。
- 读取：旧 Workspace route 与 governed route 的 4xx/5xx、p95 latency、空结果异常增幅。
- 配额：Namespace used/reserved bytes/files、reservation 泄漏、quota conflict。
- 迁移：migration job 状态、每步时长、回填行数、约束失败数。
- lineage：adoption 成功/冲突、lineage 写入失败、跨组织 FK 拒绝。

## Forward-fix 与 rollback 边界

- 默认策略是 forward-fix：停止新分享/采纳写入，保留读路径，修复后向前迁移；不要把生产库 down
  当作普通恢复动作。
- `000085` down **会删除全部 `DOWNLOAD` grants**，因为旧 schema 无法表示该权限。默认执行会在任何
  `DELETE`/`DROP` 前拒绝；生产应采用 forward-fix，不得把开关当作普通 rollback 手段。
- 仅一次性验证数据库可在同一个显式事务中执行（开关是 transaction-local，连接池或逐语句 runner
  不得把 `SET LOCAL` 与 down 脚本拆到不同事务）：

```sql
BEGIN;
SET LOCAL sparkops.allow_destructive_filesystem_000085_down = 'on';
\i backend/migrations/000085_filesystem_share_requests.down.sql
COMMIT;
```

  使用不支持 `\i` 的 runner 时，也必须在同一连接、同一事务中先执行 `SET LOCAL`，再执行完整 down
  脚本。执行前必须备份并清点 `DOWNLOAD` grants；生产事故一律优先 forward-fix。
- `000086` down 在存在非 Workspace transfer，或非 Workspace quota 有 used/reserved/activity 时拒绝执行；
  必须先停止写入并安全清空/迁移这些活动。
- `000087` down 在 `filesystem.file_lineage` 存在任何活动数据时拒绝执行；不得删除 lineage 来强行回滚
  生产数据，应采用 forward-fix。
- 只有一次性验证数据库允许执行 87 → 82 down。生产恢复优先使用备份/PITR 和 forward-fix，不以这次
  disposable down 证明可无损回滚。

## 已知要求与限制

- Tasks 1–12 多次记录本机缺少 `DATABASE_DSN`；真实 PostgreSQL 外键、并发、up/down 至今没有现场证据。
- 当前人类会话 resolver 不产生 AGENT/SERVICE_ACCOUNT；contract/read projection 可前向兼容，但真实执行主体
  接入仍是后续工作。
- 浏览器登录态联调、真实身份显示名、20 GiB、200 并发、Knowledge 真库迁移、RPO/RTO 均不由本记录提升为 PASS。
- preview 仍没有浏览器可读取 capability；审计 pagination 仍有既有 offset 并发位移边界。

结论：代码级自动验证可作为候选证据，但在真实 PostgreSQL 82 → 87 → 82 演练及 row-count 归档完成前，
Enterprise File Governance and Sharing 不可发布。
