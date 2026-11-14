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
