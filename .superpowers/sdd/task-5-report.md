# Task 5 报告：多文件上传与统一传输队列

## 状态

完成并随本报告提交；未生成 review package。

## 真实契约核对

- `CreateTransferCommand` 的 `provider_id` 可选；后端在未指定时通过 placement/default local Provider 决定存储位置，因此上传按钮不再依赖前端 Provider 配置。
- DIRECT 会话返回 `upload_url` 和 `capability_token`。客户端必须向 `upload_url` 发送带 `Authorization: Bearer <capability_token>` 的 `PUT`。
- 上传网关写入对象后调用 `RecordUploaded`，发布 `TransferUploaded` 事件；后端订阅者异步执行 verify/commit。前端不能把 PUT 完成等同于文件完成，必须通过 `getTransferSession` 轮询到 `COMPLETED`。
- brief 的 `uploadTransfer` 示例假定 SDK 已覆盖上述 DIRECT 协议，但原实现只遍历 multipart parts；本任务按真实后端契约补齐 DIRECT 上传、capability 鉴权、失败响应和取消 signal。

## TDD 记录

### RED

运行：

`pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts packages/web-sdk/src/filesystem.test.ts`

- 退出码：1。
- 新队列、DropZone、TransferCenter 尚不存在，对应测试按预期失败。
- DIRECT SDK 测试进入原 multipart 路径并以 `missing upload_url for part 1` 失败，确认真实契约缺口。
- 旧 `UploadPanel` 仍自行执行单文件上传并要求 `providerId`，不满足新的回调接口。

### GREEN

1. Task 5 核心与集成测试：
   `pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 6 个测试文件、30 项测试通过。
2. Filesystem 相关回归：
   `pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts`
   - 退出码：0。
   - 15 个测试文件、76 项测试通过。
3. Web 类型检查：
   `pnpm --filter web typecheck`
   - 退出码：0。
4. IDE diagnostics：
   - Task 5 组件、测试与 SDK 文件无 linter errors。
5. `git diff --check`
   - 退出码：0。

## 行为覆盖

- 多文件进入统一内存队列，固定最大并发数 3。
- 每项保存文件、目标目录、状态、字节进度、会话和错误；状态限定为 `queued | creating | uploading | verifying | completed | failed | canceled`。
- 失败项可单独重试；活动项取消时中止客户端请求，并在已有会话时调用后端 abort。
- 文件选择与桌面拖入共用队列；目录项明确拒绝，不实现递归上传。
- `application/x-sparkops-file-nodes` 站内节点拖拽优先排除，即使 DataTransfer 同时声明 `Files` 也绝不触发上传。
- 工作台持有队列，文件与传输标签只切换视图，不销毁正在运行的任务。
- 文件标签只显示最多三个活动任务摘要；传输标签展示完整进度、状态、错误、取消与重试。
- 上传入口与桌面投放统一受 `file:write` 权限控制。

## 提交

- Message: `feat(filesystem-ui): add multi-file transfer center`
- Commit: 本报告所在提交。

## 风险

- 浏览器 Fetch API 不提供上传字节流事件；DIRECT 单对象上传的进度会从 0 跳到完成，队列和 UI 已支持细粒度进度，后续若 SDK 改为可观测传输可直接接入。
- PUT 完成后的 verify/commit 是异步事件链，当前以前台每 500ms 轮询会话状态；任务仅保证在工作台标签切换期间持续运行，不承诺整页刷新后的持久恢复。
- Vitest 仍输出仓库既有的 Vite/Rolldown deprecated 配置警告，不影响测试结果。
