# Task 4 报告：站内拖拽移动与键盘替代操作

## 状态

完成并已提交。

## 契约核对

- 前端路由使用 `POST /workspaces/{workspaceId}/files/{nodeId}/move`。
- 真实后端 `moveRequest` 接收 `parent_id` 与 `name`，因此 SDK 保留节点名称参数，并发送 `{ parent_id, name }`，未采用计划示例中与真实后端不一致的 `target_parent_id`。
- 每次移动请求均携带 `Idempotency-Key`，支持调用方显式传入。
- 后端路由由 `file:write` 权限保护；前端拖拽、单项“移动到…”和多选工具栏入口均由同一权限控制。

## TDD 记录

### 前任实现基线

1. `pnpm vitest run apps/web/src/modules/filesystem/components/directory-dnd.test.ts apps/web/src/modules/filesystem/components/MoveNodesModal.test.tsx apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx apps/web/src/modules/filesystem/components/FileBrowser.test.tsx packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0
   - 摘要：5 个测试文件、30 项测试通过；说明中断前代码已把原有 RED 测试推进到 GREEN，但仍有遗漏行为。

### RED

1. 新增“混合状态多选拖拽必须整体拒绝”和“深层后代目标必须拒绝”测试后运行：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FileBrowser.test.tsx apps/web/src/modules/filesystem/components/MoveNodesModal.test.tsx`
   - 退出码：1
   - 摘要：2 项按预期失败。拖拽载荷静默丢弃不可用节点，可能只移动部分选择；Modal 只识别直接子目录，未识别深层后代。
2. `pnpm --filter web typecheck`
   - 退出码：2
   - 摘要：`MoveNodesModal` 的 `Modal`、`InlineAlert` `createElement` 调用缺少类型要求的 `children`，随后显式返回类型也暴露并修复了不可命名的推导类型。

### GREEN

1. `pnpm vitest run apps/web/src/modules/filesystem/components/FileBrowser.test.tsx apps/web/src/modules/filesystem/components/MoveNodesModal.test.tsx apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx`
   - 退出码：0
   - 摘要：3 个测试文件、23 项测试通过。
2. `pnpm vitest run apps/web/src/modules/filesystem`
   `packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0
   - 摘要：文件系统相关 12 个测试文件、57 项测试全部通过。
3. `pnpm --filter web typecheck`
   - 退出码：0
   - 摘要：Web TypeScript 检查通过。
4. IDE diagnostics
   - 摘要：Task 4 组件、测试和 SDK 文件无 linter errors。
5. `git diff --check`
   - 退出码：0。

## 行为覆盖

- `validateNodeDrop` 为纯函数，拒绝无写权限、跨工作区、不可用项目、自身和后代目标。
- 文件列表与网格写入站内拖拽载荷；多选中含不可用节点时保留完整选择，让纯规则整体拒绝，避免部分移动。
- `DirectoryTree` 提供允许/拒绝目标样式，悬停 700ms 展开，并在 `dragleave`、`drop`、卸载时清理定时器。
- `FileActionMenu` 和多选工具栏提供键盘可达的“移动到…”。
- `MoveNodesModal` 复用 `DirectoryTree` 只选模式，并使用树提供的完整祖先链阻止移动到任意深度的后代。
- 多项移动逐项调用真实 SDK；任一失败均不提交成功状态，并刷新全部源目录和目标目录。
- 工作台对拖拽及键盘移动入口统一执行 `file:write` gating。
- 未实现复制、删除、回收站等 Task 5 功能。

## Lint

`pnpm --filter web lint` 已执行，但仓库当前没有 ESLint 配置，`next lint` 进入交互式初始化提示后以退出码 1 结束。为避免越界修改全局工具链，本任务未创建 ESLint 配置；IDE diagnostics 无错误。

## 提交

- Message: `feat(filesystem-ui): add drag and keyboard node moves`

## 风险

- Vitest 仍输出仓库既有的 Vite/Rolldown deprecated 配置警告，不影响测试结果。
- CLI lint 无法在当前仓库非交互执行；需由独立工具链任务补齐 ESLint 配置后恢复该质量门。
