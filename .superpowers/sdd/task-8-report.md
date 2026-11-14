# Task 8 报告：视觉、响应式、无障碍与自动验收

## 状态

Task 8 代码与自动验收完成。未启动或重启根工作区服务；浏览器手工矩阵保持 SKIP / PENDING，实时容量曲线明确不在范围。

## TDD 记录

### RED

1. 新增 `FilesystemWorkbench.accessibility.test.tsx` 后运行：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`
   - 退出码 1；4 项中 3 项按预期失败、1 项通过。
   - 失败原因分别为：缺少名为“文件系统功能”的 tablist、列表容器缺少横向溢出语义类、全局样式缺少工作台响应式/触控/focus/reduced-motion 契约。
2. 增加进度动效契约后复跑同一命令：
   - 退出码 1；4 项中 1 项按预期失败。
   - 失败原因：进度条仍使用 `duration-200`，没有统一到工作台 150ms 克制过渡类。

### GREEN

1. Task 8 定向测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`
   - 退出码 0；1 file / 4 tests。
2. Filesystem 前端完整矩阵：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 首轮退出码 0；19 files / 131 tests；Important 修复后矩阵为 19 files / 132 tests。
   - 无 React act warning 或未处理 Promise；存在仓库测试工具链既有的 Vite/esbuild/oxc deprecation warning。
3. Monorepo 类型检查：
   `pnpm typecheck`
   - 首次退出码 2：新测试把 Drawer children 作为 `createElement` 第三参数传入，不满足显式 props 类型。
   - 修正后退出码 0；6 packages。
4. 后端 brief 完整回归：
   `go test ./internal/modules/filesystem/... ./internal/filesystemgateway/... ./internal/sftpgateway/... ./api/openapi -count=1`
   - 退出码 0；18 个有测试的包通过，6 个包为 `[no test files]`。
5. IDE diagnostics：Task 8 修改范围无 linter errors。
6. `git diff --check`：退出码 0。

## 实现摘要

- 工作台使用语义化 CSS 类和既有 `--surface-*`、`--primary-soft`、`--primary`、shadow/radius token；未增加依赖或散落硬编码颜色。
- 桌面为 `14rem–18rem` 目录侧栏加弹性主区；900px 以下隐藏侧栏并通过现有 Drawer 打开目录，主区与表格均限制横向页面溢出。
- 列表表格在自身容器横向滚动；网格从三列降为两列、单列。关键图标和文件操作具备 44px 触控目标。
- tablist、tree/treeitem、list/grid、tabpanel、Drawer、progressbar 保持可访问语义；视图切换有明确 `aria-pressed` 与“切换到…”名称。
- 拖拽移动继续提供“移动到…”键盘替代流程；Drawer 保持 Esc、焦点圈和焦点管理。
- 工作台交互统一为 150ms ease；`prefers-reduced-motion` 下关闭动画和过渡。
- 近乎无色、内容优先；“上传文件”仍是文件区唯一 primary 视觉焦点，其余操作沿用 secondary/ghost。
- loading / error / empty / forbidden 继续由现有组件独立表达，自动矩阵覆盖相关分支。

## 自动验收与未执行项

### PASS

- tab / tabpanel 权限裁剪与键盘切换。
- tree / treeitem、列表 / 网格共享状态与键盘“移动到…”入口。
- Drawer Esc、焦点管理、上传 progressbar。
- 窄屏目录 Drawer、表格横向滚动、网格断点、44px 触控目标。
- focus-visible、reduced-motion、150ms 过渡、token 用色约束。
- Filesystem 前端完整矩阵、pnpm typecheck、后端 brief 回归。

### SKIP / PENDING（浏览器手工矩阵）

1. 普通成员只看到“文件”和“传输”；
2. 管理员看到四个标签；
3. 列表选择文件后切换网格，选择保持；
4. 目录树懒加载，拖入自身和子目录被拒绝；
5. 拖动文件到另一目录成功，失败时恢复；
6. 桌面拖入 4 个文件时最大 3 个并发；
7. 失败任务可重试，活跃任务可取消；
8. 文件详情 Drawer 显示版本，Esc 关闭；
9. 窄屏目录树通过 Drawer 使用；
10. 不使用鼠标通过“移动到…”完成移动。

以上项目有组件级自动测试证据，但本轮未运行真实浏览器矩阵，因此不记浏览器 PASS。

## 明确不在范围与未提升项

- 实时容量曲线：不在范围，未实现、未展示、未伪造。
- 20 GiB 断点续传、200 并发、Knowledge 真库迁移、RPO/RTO、灾备恢复：本轮无现场证据，验收文档继续保持 SKIP / PENDING。
- 根工作区服务重启与 `/ready`、`/files` 运行态探测：依用户约束未执行。

---

## 四个 Important 修复追加

### RED

运行：
`pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`

- 退出码 1；5 项中 4 项失败、1 项通过。
- 失败分别证明：tabs 未具备 44px 触控类；列表/网格 checkbox 没有可点击的 44px label 区域；上传按钮不响应编辑态主动作切换；CSS 缺少选择控件触控契约。
- 同一测试同时要求 `.filesystem-browser-main` 不再是 `main`、保留 `aria-labelledby="workbench-tab-files"`，并具体断言 focus-visible 为 `2px solid var(--primary)`、offset 2px；测试从现有 `--primary` / `--surface` token 解析颜色并验证对比度不低于 4.5。

### GREEN

1. Important 定向：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`
   - 退出码 0；1 file / 5 tests。
2. Filesystem 完整矩阵：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；19 files / 132 tests。
3. `pnpm typecheck`
   - 退出码 0；6 packages。
4. IDE diagnostics 与 `git diff --check`
   - 均无错误。
5. 后端
   - 本次仅修改前端组件、样式、测试与验收文档，未重复后端回归。

### 修复结果与主动作层级

- focus-visible 改用现有近黑 `--primary`，明确 2px outline + 2px offset；不再使用低对比 `--border-strong`。
- 文件浏览内容容器由嵌套 `main` 改为普通 `div`，继续由文件 tab ID 提供 `aria-labelledby`。
- 文件 tab：无编辑面板时“上传文件”为唯一 primary；新建/重命名面板打开后上传降为 secondary，面板提交成为唯一 primary。
- 治理 tab：Provider/存储池默认仅“新增存储池”为 primary；任一 Modal 打开后背景触发器降为 secondary，Modal 提交成为唯一 primary。作用域绑定同理；传输、审计无 competing primary，生命周期仅保存动作使用 primary。
- tabs、视图切换、目录入口、树展开和文件操作均有至少 44px 触控目标。列表/网格 checkbox 保持 16px 视觉尺寸，由 44px `label` 扩大点击区域，label 点击行为由 DOM 测试验证。

---

## 剩余核心触控目标修复追加

### RED

运行：
`pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx apps/web/src/modules/filesystem/components/GovernanceWorkspace.test.tsx`

- 退出码 1；3 files 中各有 1 项失败，合计 3 failed / 10 passed。
- 失败具体对应：UploadPanel 上传按钮缺少 44px utility；FileDetailDrawer 的 3 个内层 tabs 缺少该 utility；GovernanceWorkspace 的 3 个 tabs 缺少该 utility。
- FileExplorerPanel 新建目录按钮由源码契约测试单独定位（该 JSX panel 在现有 Vitest 工具链中不能直接组件渲染）；同级打开、选择、重命名、单项移动和多选移动均由 FileBrowser DOM 测试逐项检查。

### GREEN

1. Task 8 定向：
   - 上述命令退出码 0；3 files / 13 tests。
2. Filesystem 完整矩阵：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码 0；19 files / 132 tests。
3. `pnpm typecheck`
   - 退出码 0；6 packages。
4. IDE diagnostics
   - 本次修改范围无 linter errors。

### 已自动覆盖的核心交互清单

- DOM：工作台一级 tabs、列表/网格切换、移动端目录入口、目录树展开、上传按钮。
- DOM：列表/网格选择 label、文件打开、重命名、单项“移动到…”、多选“移动到…”。
- DOM：FileDetailDrawer 详情/版本/审计 tabs；GovernanceWorkspace 三个二级 tabs。
- 源码契约：FileExplorerPanel 新建目录按钮应用 `filesystem-touch-target`。
- CSS 契约：`filesystem-touch-target` 的 `min-height` 与 `min-width` 均为 `2.75rem`；未声明所有 Task 8 按钮均已覆盖，只声明上述清单中的核心交互。

---

# Task 8 追加：治理空间 Contracts 与传输路径

## 基线与范围

- 基线：`master` / `0f3b6b9d8d423e97098e091d1cb900f00ad83a51`，实施前工作树干净。
- 仅实现 contracts、upstream、BFF module、web-sdk 和 6 条 Next BFF route；未实现 Task 9–12 UI，未增加依赖。
- 保留原 `/api/bff/v1/workspaces/{workspaceId}/...` filesystem 客户端和路由。

## TDD 记录

### RED

先在以下三个文件增加测试：

- `packages/upstream/src/filesystem.test.ts`
- `packages/bff-modules/src/filesystem/index.test.ts`
- `packages/web-sdk/src/filesystem.test.ts`

运行 brief 指定命令后退出码 1：3 files / 7 failed / 13 passed。失败均因治理 schema、客户端和方法尚不存在，符合预期。

### GREEN

- brief 指定测试：3 files / 20 tests，退出码 0。
- 受影响 filesystem 回归：5 files / 35 tests，退出码 0。
- `pnpm typecheck`：6 packages，退出码 0。
- IDE diagnostics：修改范围无 linter errors。
- `git diff --check`：退出码 0。
- Vitest 仍输出仓库既有 Vite/esbuild/oxc deprecation warning；无测试失败。

## 实现摘要

- 按 Task 5/6 OpenAPI 和 Go JSON view 增加 Zod schema 与 inferred types：Namespace summary/page、typed Grant/page、effective access/cache key、shared item/page、ShareRequest、typed write command、即时/待审批写响应。
- 新治理客户端统一使用 `/api/bff/v1/filesystem`；personal/organization 未嵌入 workspace URL。
- children 完整转发 `parent_id/type/q/cursor/limit`，shared-with-me 完整转发 `q/cursor/limit`。
- 读写均转发 `X-Organization-ID` 和可选 `X-Workspace-ID`；写请求另转发原 body 与 `Idempotency-Key`。未新增或伪造 actor/user principal header，身份继续来自既有 BFF cookie/token 上下文。
- 动态 path 参数在 web-sdk 和 upstream 两层均使用 `encodeURIComponent`；Next route 使用当前版本的异步 `params`。
- 新增 6 条 route：namespaces、namespace children、shared-with-me、effective-access、grants POST、share-requests POST。
- 后端统一治理 grants 当前仅实现 POST，未虚构 GET/DELETE；share-requests 当前仅实现 POST。两类写响应均支持后端即时 `201` 与待审批 `202` 数据形态。

## 风险与未验证项

- 未做真实登录态浏览器/后端联调；本任务仅完成 contracts 与传输层自动测试。
- 未实现 OpenAPI 中尚未落到本 Task 8 六条 route 的 approve/reject UI/client，也未提前实现 Task 9–12。
