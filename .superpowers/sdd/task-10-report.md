# Task 10 实施报告

## 范围

- 基线：`3d8f4db5b5527dfd07609c1b2ac85c28d6557df2`
- 首次实现提交对象：`72f538fbdd7635fcbb038d7a93948abceff6c03f`
- 实现多 Namespace 浏览状态隔离、Namespace 安全拖放、与我共享投影及投影只读详情。
- 未实现 Task 11 分享弹窗或 Task 12 治理审批。

## TDD

1. 先增加 brief 四个聚焦测试文件中的空间切换、同 Node ID 隔离、跨 Namespace 拒绝、来源分组、fallback 分组和 original Node ID 详情测试。
2. 首次 RED：4 个测试文件失败；共享面板尚不存在，DirectoryTree 新接口尚未实现，DnD 未携带/校验 `namespace_id`。
3. 最小实现后补充 mutation handler 二次拒绝、shared projection-only drawer 不调用旧 workspace-only API 的回归测试。

## 结果

- `DirectoryTree` 显式接收 `namespaceId` 与 `rootNode`，以 `space:workspaceId:namespaceId:rootNodeId` remount 隔离 tree/cache/selection/expanded/loading。
- Workbench 的选择、分页游标和传输队列使用同一完整 scope；FileExplorer 也随 Namespace/父目录 scope 重置本地选择和编辑状态。
- DnD payload 包含 `namespace_id`；树判定、移动弹窗和最终 workbench mutation handler 均拒绝跨 Namespace，固定提示“跨空间请使用复制、转存或采纳”。
- shared 是独立授权投影，不渲染目录树、上传、新建、移动、重命名、删除或拖放目标。
- shared 按 `ORGANIZATION / USER / WORKSPACE / AGENT` 分组；`SERVICE_ACCOUNT / ROLE` 及未知类型进入“其他授权来源”，不会丢数据。
- shared 展示来源、owner/sharer、有效权限、到期和原路径；缺失字段明确显示“未提供”，不使用 raw ID 充当文案。
- shared item 使用 original Node ID 打开既有详情 drawer 的 projection-only 模式；该模式不调用旧 workspace-only 版本或审计接口。
- 页面与样式复用现有 token/class，提供加载、空、禁止、错误分态和窄屏布局。

## 验证

- brief 四项测试：4 个文件、67 个测试通过。
- `pnpm vitest run "apps/web/src/modules/filesystem"`：22 个文件、162 个测试通过。
- accessibility test：1 个文件、5 个测试通过。
- `pnpm typecheck`：6 个 workspace package 全部通过。
- IDE lint 与 `git diff --check`：无诊断、无 whitespace 错误。

## 未验证项

- 未连接真实后端执行浏览器端到端验证。
- 当前 `SharedItemVM` 后端 contract 仅保证 node、effective actions 和 grant references，不保证来源显示名、owner/sharer、expiry、original path。UI 已为后端后续返回这些投影字段提供安全展示，并对当前缺失字段明确降级；没有通过 raw ID 猜测文案。
- effective-access 通用详情读取按要求延后 Task 11，本任务未调用该接口。

## 首轮审查修复

- 修复起点 HEAD：`d8337621785518f7aaa962e6b612d8cf54bec276`。
- backend `SharedNode` 现在返回结构化 `sources`、后端计算的 `original_path` 和 `source_namespace`。`sources` 仅保留当前 principal 的有效 `ALLOW` grant，并与授权器实际参与 effective allow 的 grant IDs 取交集，不暴露 DENY、过期、撤销或未命中的 grant。
- original path 通过一次递归 CTE 批量读取候选节点祖先链；parent join 强制 Namespace 一致，并隐藏内部 `.root`，没有逐 item N+1。
- USER Namespace 仅返回数据库可证明的 nullable `owner_user_id`；不存在可靠 sharer/display-name 时不伪造身份。UI 不把 UUID 作为主文案，明确显示“共享者信息不可用”。
- `source_type` 在真实 effective source grant principal type 上过滤；共享游标 AAD 同时绑定 organization、workspace、actor、namespace、query 和 source type，跨 filter 游标重放被拒绝。
- OpenAPI、fronted Zod/inferred types、BFF validation、SDK transport fixture 与 SharedWithMePanel 已统一到同一投影结构；ROLE、SERVICE_ACCOUNT 和未知来源进入“其他授权来源”。
- SharedWithMePanel 的详情选择绑定 workspace scope；workspace 改变或 items 不再包含所选节点时自动关闭，避免显示旧投影。
- expiry 使用 UTC 确定性格式；合法值渲染 `<time dateTime>`，非法或 null 显示安全 fallback，不产生 `Invalid Date`。

## 首轮审查修复验证

- `go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/infrastructure/persistence ./internal/modules/filesystem/interface/http ./api/openapi -count=1`：4 个 package 全部通过。
- `go test ./internal/modules/filesystem/... -count=1`：filesystem 模块全部通过。
- `pnpm vitest run packages/contracts/src/filesystem/index.test.ts packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts apps/web/src/server/loaders/filesystem.test.ts apps/web/src/modules/filesystem`：28 个文件、218 个测试通过。
- `pnpm vitest run packages/web-sdk/src/filesystem.test.ts`：1 个文件、15 个测试通过（增加 SDK 真实 projection fixture 后复验）。
- `pnpm typecheck`：6 个 workspace package 全部通过。
- `pnpm --filter @sparkops/web-sdk typecheck`：通过（增加 SDK fixture 后复验）。
- IDE lint 与 `git diff --check`：无诊断、无 whitespace 错误。

## 首轮审查修复未验证项

- `DATABASE_DSN` 未配置，未执行依赖外部 PostgreSQL 的数据库集成测试；repository 的递归路径、Namespace 隔离和 grant 筛选已由 SQLite 聚焦测试覆盖。
- 未连接真实身份显示名服务；按审查要求保持 nullable/不可用语义，没有新增 migration 或伪造 owner/sharer。
- 修复提交 hash 见本次最终交付；Git commit 对象无法在其自身内容中自引用。

## 二次复审修复

- 修复起点 HEAD：`cb7727a190f528a47a1a5126721684151caf4098`。
- read projection 的 `sources[].principal_type` 改为非空、最长 64 字符的前向兼容字符串；OpenAPI 记录当前已知值但不以 enum 限制 read response。`CreateTypedGrantRequest` 与前端写 command 仍保持原 PrincipalType enum，未放宽写入面。
- shared 主分组使用固定业务顺序 `ORGANIZATION → WORKSPACE → USER → AGENT → OTHER`，不再依赖 source 数组顺序或 grant UUID；URL source filter 命中时优先作为主组。同一 node 只渲染一次，卡片和详情展示全部 distinct source 类型。
- drawer selection 只存 `scope + namespace_id + node_id`，每次从当前 items 派生 projection；同 identity 更新时自动刷新 node、permissions、path 和 sources，identity 移除或 workspace/source scope 改变时关闭。
- original path 使用批量读取到的 `namespace.root_node_id` 精确隐藏真实 root；合法用户目录名 `.root` 保留，不新增逐 item 查询。
- expiry 的 `null` 明确显示“永久有效”，缺失或非法值显示“到期信息不可用”，合法值继续使用 UTC `<time dateTime>`。

## AGENT / SERVICE_ACCOUNT resolver 审查

- 当前生产 `PrincipalResolver` 仅从认证 actor、经权威 workspace membership 校验后的 WORKSPACE/ORGANIZATION，以及权威 role binding 产生 principals；未发现可信的 Agent/Service Account impersonation 或 ownership execution context。
- 本次没有从 workspace membership 推断用户拥有 Agent/Service Account，也没有读取客户端 principal header，避免扩大授权面。
- 已增加 resolver stub → 真实 SharedWithMe handler → 真实 repository → 真实 authorization service 的链路测试，证明权威 resolver 一旦返回 AGENT/SERVICE_ACCOUNT，shared query 能正确候选、授权、投影，并可按 SERVICE_ACCOUNT source type 过滤。
- 已知限制：当前人类会话 resolver 不产生 AGENT/SERVICE_ACCOUNT；未来须由权威执行主体上下文接入。Task 10 read contract 和 UI 已能安全保留、展示并 fallback 分组这两类及未来未知类型。

## 二次复审验证

- RED 已确认：未知 source type 被 Zod 拒绝；分组随 sources 首项改变；drawer 保留 stale item；null expiry 被误判为不可用；路径按 `.root` 名称而非 root identity 隐藏；OpenAPI read source 仍为封闭 enum。
- `go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/infrastructure/persistence ./internal/modules/filesystem/interface/http ./api/openapi -count=1`：4 个 package 全部通过。
- `go test ./internal/modules/filesystem/... -count=1`：filesystem 模块全部通过。
- `pnpm vitest run packages/contracts/src/filesystem/index.test.ts packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts apps/web/src/server/loaders/filesystem.test.ts apps/web/src/modules/filesystem`：28 个文件、222 个测试通过。
- `pnpm typecheck`：6 个 workspace package 全部通过。
- IDE lint 与 `git diff --check`：无诊断、无 whitespace 错误。

## 二次复审未验证项

- `DATABASE_DSN` 未配置，未执行依赖外部 PostgreSQL 的数据库集成测试；root identity、合法 `.root` 子目录及 Agent/Service Account 生产链路由 SQLite repository 测试覆盖。
- 未连接真实浏览器或真实 Agent/Service Account 执行主体，因为当前系统不存在对应权威会话上下文。
- 修复提交 hash 见本次最终交付；Git commit 对象无法在其自身内容中自引用。
