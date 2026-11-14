# Task 9 实施报告

## 范围

- 基线：`39e1a2a51c35cb9e8aa10e5a326985d2768d417a`
- 实现四空间 URL 状态、Namespace 解析、服务端 loader 与 `FileSpaceNav`。
- 未实现 Task 10 浏览表格、Task 11 弹窗或 Task 12 治理 UI。

## TDD

1. 先增加 URL normalization/transition、空间解析、loader 状态和导航可访问性测试。
2. 首次红灯：5 个测试文件失败，原因均为 Task 9 API/组件尚不存在或行为未实现。
3. 最小实现后运行 Task 9、无障碍与 filesystem 回归测试并修复类型问题。

## 验证

- `pnpm vitest run "apps/web/src/modules/filesystem/components/workbench-state.test.ts" "apps/web/app/(platform)/files/page-state.test.ts" "apps/web/src/modules/filesystem/components/file-space-view.test.ts" "apps/web/src/server/loaders/filesystem.test.ts" "apps/web/src/modules/filesystem/components/FileSpaceNav.test.tsx" "apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx"`
  - 6 个文件、38 个测试通过。
- `pnpm vitest run "apps/web/src/modules/filesystem/components" "apps/web/src/server/loaders/filesystem.test.ts" "apps/web/app/(platform)/files/page-state.test.ts"`
  - 23 个文件、170 个测试通过。
- `pnpm typecheck`
  - 6 个 workspace package 全部通过。
- IDE lint
  - 修改范围无诊断。

## 结果

- `FileSpace` 固定为 `my / workspace / organization / shared`。
- 非法空间归一为 `workspace`；切空间清理 `parent / cursor / node`，保留 `view` 与搜索条件。
- `shared` 拒绝 `parent`；`source` 仅在 `shared` 下接受白名单值。
- 页面先加载可访问 Namespace summaries，再按权威 Organization/Workspace/User 上下文解析空间。
- 真实 Namespace 使用 governed children；shared 使用 shared-with-me，不伪造 Namespace。
- Namespace 缺失、工作区不可用、403、404、一般错误和空结果均为独立可渲染状态。
- 导航显示固定四项和权威组织/工作区名称；真实空间复用 `DirectoryTree`，shared 不渲染目录树。

## 未验证项

- 未连接真实后端进行浏览器端端到端验证。
- Task 10 才负责共享与多 Namespace 的正式浏览表格，本任务仅显示明确加载/空/错误状态。

## 首轮审查修复

- 修复起点：`a28ec72f21fd4e2468b0dc3dc2dbaa64e6d9e2a1`。
- `shared` source 白名单与 transport 统一为
  `organization / workspace / user / agent`，并以 `source_type` 精确转发
  loader、Next BFF、web-sdk 和 upstream。
- 节点详情加载后校验 `node.namespace_id` 必须等于当前解析出的真实
  Namespace；跨 Namespace 节点按 `not-found` 处理。
- 旧 workspace mutation 链路增加组合门禁：当前空间和 summary 均为当前
  Workspace Namespace、summary 包含 `WRITE`、全局权限包含 `file:write`
  时，才允许 upload/create/move/drag。
- 删除 Namespace 变化时主动清理 URL 的 effect；服务端确认的
  `parent / node / cursor` 在 history-like rerender 中保持，主动切空间仍由
  URL builder 清理这些参数并保留 `view`。

### 审查修复 TDD 与验证

- RED：7 个聚焦测试文件共观察到 13 个预期失败，分别覆盖 source 转发、
  跨 Namespace 节点、旧写入口和 history 状态。
- `pnpm vitest run "apps/web/src/modules/filesystem/components/workbench-state.test.ts" "apps/web/app/(platform)/files/page-state.test.ts" "apps/web/src/server/loaders/filesystem.test.ts" "apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx" "packages/web-sdk/src/filesystem.test.ts" "packages/upstream/src/filesystem.test.ts" "apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts"`
  - 7 个文件、88 个测试通过。
- `pnpm vitest run "apps/web/src/modules/filesystem" "apps/web/src/server/loaders/filesystem.test.ts" "apps/web/app/(platform)/files/page-state.test.ts" "apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts" "packages/web-sdk/src/filesystem.test.ts" "packages/upstream/src/filesystem.test.ts"`
  - 26 个文件、201 个测试通过，包含 accessibility。
- `pnpm typecheck`
  - 6 个 workspace package 全部通过，无缓存复用。
- IDE lint 与 `git diff --check`
  - 无诊断、无 whitespace 错误。

### 审查修复未验证项

- 未执行真实后端端到端浏览器验证。
- 未实现 Task 10 的多 Namespace 完整浏览、拖拽或 shared panel。
