# Filesystem Unified Workbench 最终全分支修复报告

## 范围

本轮修复最终审查的 1 Critical、6 Important、1 Minor，保持既有工作台、上传队列、治理、审计与可访问性能力不回退。每项均先添加失败测试确认 RED，再实施最小修复并确认 GREEN。

## 1. Critical：move 全链路转发

- RED：SDK 已请求 `/files/{nodeId}/move`，但 Next route 不存在，BFF module 与 upstream 也没有 move 方法；边界测试分别报模块不存在、方法不存在。
- GREEN：补齐 SDK → Next BFF route → BFF module → upstream → 后端链路，原样转发 `{ parent_id, name }` 和 `Idempotency-Key`。
- 契约取舍：采用真实 Go `moveRequest`，没有使用计划草案中的 `target_parent_id`。
- 测试：新增 Next route、BFF module、upstream 集成边界测试；SDK 继续验证请求体与幂等键。

## 2. `type=DIRECTORY` 全链路

- RED：SDK 能编码 `type`，但 Next route、BFF module、upstream 均丢弃。
- GREEN：三层查询类型均增加 `type` 并原样传至后端；OpenAPI 在文件列表接口声明 `FILE | DIRECTORY`。
- 测试：覆盖 SDK、Next route、BFF module、upstream 和后端 HTTP/持久化分页前类型过滤。

## 3. Transfer list 资源级 READ 授权

- RED：混合 ACL 测试证明列表直接返回无权目标父目录的 session。
- GREEN：对每个 session 的目标父目录执行资源级 READ；拒绝项不返回并记录拒绝审计。按“当前页剩余槽位”分批读取，保证填页时 cursor 正好落在已扫描边界，不重复、不漏项。
- 性能取舍：每个请求最多 20 个授权扫描批次；达到上限时返回短页及真实后续 cursor，避免无限循环和一次请求无界扫描。
- 测试：混合 allow/deny、跨批填满页、cursor 连续性以及原状态过滤测试通过。

## 4. Workspace 切换 scope 清理

- RED：切换 workspace 后同一 render frame 仍把旧 `parent/node/cursor` 传给子组件。
- GREEN：scope 由 workspace、namespace、root 组成；切换帧立即规范化到新根目录并清理 URL。DirectoryTree 与 FileExplorer key 包含 scope，旧节点/父目录不会等待 effect 才消失。
- 服务端：列表加载遇到 400/403/404 scope 错误时重定向到保留 tab/view/q 的根目录 URL；503 等真实服务错误不被吞掉。
- 测试：旧 parent/node/cursor、URL 清理、后续新 workspace node 打开及服务端错误分类通过。

## 5. `canWrite` 统一裁剪

- RED：只读 FileBrowser 仍显示重命名操作。
- GREEN：`file:write` 同时控制新建目录、上传、重命名、移动、拖拽与“移动到…”；读取和打开仍可用。后端授权保持最终安全边界。
- 测试：只读用户无 rename/move，写用户原有操作与触控目标测试保持通过。

## 6. `q/cursor` 真实搜索与分页

- RED：SDK、Next、BFF、upstream 丢弃 q；页面不传 cursor，也没有前后页。
- GREEN：`q` 与 cursor 串通页面、server loader、SDK、Next route、BFF、upstream 和 Go 列表接口。存储层对规范化名称执行参数化 LIKE，且 q/type/parent 均在 cursor 与 limit 前过滤。
- UI：搜索提交、parent 变化和 q 变化清 cursor；下一页使用后端 cursor，上一页只使用本次访问过的 cursor trail，不伪造总页数或前页。
- 安全取舍：LIKE 通配符被转义；q 最大 200 字符；不做前端首屏过滤。
- 测试：持久化稳定分页、HTTP 参数、OpenAPI、SDK/Next/BFF/upstream 与页面上一/下一页均通过。

## 7. 刷新后持久 transfer 操作

- RED：持久卡片只有展示，活动 session 无真实取消，失败 session 无安全重试入口。
- GREEN：非终态持久 session 的取消调用真实 abort SDK/BFF/后端，成功后刷新权威列表。FAILED 卡片明确说明“重新选择并新建”，只接受名称与大小均匹配的本地 File，再通过队列创建新 session。
- 安全取舍：浏览器刷新后没有 File 对象，绝不伪造 File、复用旧失败 session 或自动并行上传；只有服务端已明确为 FAILED 终态时才允许重新选择新建。
- 测试：真实 abort 调用、刷新、错误提示、文件名称/大小匹配和新队列入队通过。

## 8. View 与 URL/历史同步

- RED：浏览器历史把 URL view 改为 grid 后，组件仍显示本地 list。
- GREEN：view 的即时切换继续更新 URL；来自服务端/浏览器历史的 `initialState.view` 在 layout effect 中同步，不在 render phase 调用 setState。
- 测试：本地切换保留其他 query，历史前进/后退状态回灌通过。

## 最终验证

- 前端 filesystem 矩阵：23 files / 153 tests PASS。
- Go filesystem、filesystemgateway、sftpgateway、OpenAPI、migrations：PASS。
- `pnpm typecheck`：PASS。
- IDE diagnostics：修改范围无 linter errors。
- `git diff --check`：PASS（Windows 工作区仅报告既有 LF/CRLF 转换提示）。

## 剩余风险

- 资源授权扫描设置 20 批上限；极端稀疏 ACL 可能返回不足 limit 的短页，但会返回后续 cursor，继续翻页不会泄漏或跳过已授权项。
- 持久 FAILED 任务无法原地 resume，这是后端真实 API 限制；当前明确采用“确认旧终态后重新选择并新建”。
- Vitest 仍输出仓库既有 Vite/Rolldown deprecation warning，不影响结果。
