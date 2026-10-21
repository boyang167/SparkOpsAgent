# Kubernetes Pod 交互式终端设计

## 目标

在节点详情的「容器组」表格中，为每个容器提供接近 `kubectl exec -it` 的浏览器终端。终端支持持续输入输出、TTY、窗口缩放、粘贴、控制键和 ANSI 渲染。

入口保持为：

`节点详情 → 容器组 → 更多 → {容器名} 终端`

首版仅覆盖 Pod 容器终端。现有 SSE exec、日志、CloudTTY 和节点终端保持不变。

## 方案

采用原生 WebSocket 连接和 xterm 前端，参考 GVA 的 `terminal.vue`、`KubeShell` 与 `Pod.Exec`，但不在 WebSocket URL 中传递长期 JWT。

Next.js Route Handler 无法可靠处理 WebSocket Upgrade。因此连接分为两步：

1. 浏览器通过现有同源 BFF，使用 HttpOnly Cookie 发起 `POST terminal-ticket`。
2. BFF 将 Cookie 中的 JWT 转为上游 `Authorization`，后端完成工作区成员与写权限校验，返回一次性短期票据和 WebSocket 路径。
3. 浏览器使用票据建立 WebSocket。握手成功后票据立即失效。

票据是随机不可预测的临时凭据，不包含 JWT、集群凭据或 Kubernetes Token。

## API

### 创建终端票据

```text
POST /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket
Content-Type: application/json

{
  "container": "kube-proxy",
  "cols": 120,
  "rows": 36
}
```

响应：

```json
{
  "ticket": "<opaque-random-ticket>",
  "expires_at": "2026-08-31T15:00:30Z",
  "websocket_path": "/ws/plugins/kubernetes/terminal"
}
```

约束：

- 必须通过现有身份认证、工作区成员校验、插件 entitlement 和 write 权限。
- 校验 cluster、namespace、pod、container 及尺寸。
- 票据有效期 30 秒，只能使用一次。
- 每个成员最多保留 5 张未使用票据；过期票据惰性清理。
- 票据服务仅存于当前后端进程内。首版部署假定单实例或 WebSocket 粘性路由；多实例共享票据不在本次范围。

### WebSocket

```text
GET /ws/plugins/kubernetes/terminal?ticket=<opaque-random-ticket>
Upgrade: websocket
```

握手阶段：

- 原子消费票据，拒绝过期、重复或不存在的票据。
- 校验 `Origin` 与服务端允许来源，禁止任意站点发起跨站终端连接。
- 根据票据中冻结的 workspace、member、cluster、namespace、pod、container 和初始尺寸创建会话。
- URL 不接受 cluster、pod、container 或 command 参数，避免消费票据后被篡改目标。

客户端消息：

```json
{ "type": "read", "data": "ls\r" }
{ "type": "resize", "cols": 120, "rows": 36 }
```

服务端消息：

```json
{ "type": "open" }
{ "type": "write", "data": "..." }
{ "type": "error", "message": "..." }
{ "type": "exit", "code": 0 }
```

限制：

- 单条输入消息最多 64 KiB。
- 尺寸范围为 2–500 列、2–200 行。
- 服务端设置读写期限、ping/pong 和最大消息大小。
- 同一连接串行写 WebSocket，避免并发写导致数据帧损坏。
- 客户端断开、权限初始化失败或上游退出时立即取消 Kubernetes exec context。

## Kubernetes exec

使用 `remotecommand.NewSPDYExecutor`：

```text
PodExecOptions:
  Stdin: true
  Stdout: true
  Stderr: false
  TTY: true

StreamOptions:
  Stdin: WebSocket 输入流
  Stdout: WebSocket 输出流
  Tty: true
  TerminalSizeQueue: WebSocket resize 队列
```

TTY 模式下 stderr 与 stdout 合并，因此不单独设置 `Stderr`。

启动命令：

```sh
/bin/sh -c 'clear 2>/dev/null || true; exec bash 2>/dev/null || exec sh'
```

容器没有 bash 时使用 sh；没有 `/bin/sh` 时终端返回明确错误并关闭。首版不允许客户端传入启动命令。

## 后端组件

新增 `backend/internal/plugins/kubernetes/terminal/`：

- `ticket.go`：内存票据存储、签发、原子消费、过期和成员配额。
- `session.go`：WebSocket 与 Kubernetes SPDY 的双向桥接、TTY 尺寸队列、消息限制。
- `handler.go`：票据 HTTP handler、WebSocket Upgrade、权限与错误处理。
- 对应单元测试覆盖票据生命周期、消息验证、尺寸队列和目标冻结。

路由：

- `POST .../pods/:pod/terminal-ticket`
- `GET /ws/plugins/kubernetes/terminal`

同时更新插件 manifest 与 bundled descriptor 的 API/文件清单。

会话沿用现有终端审计语义：

- `kubernetes.exec.started`
- `kubernetes.exec.ended`
- 会话类型仍为 `exec`

输出写入现有 SessionRecorder。为控制隐私与存储规模，首版只录制服务端输出，不额外录制用户键盘输入。

## 前端组件

新增依赖：

- `@xterm/xterm`
- `@xterm/addon-fit`

新增 `PodTerminal.tsx`：

- 动态导入 xterm，避免 SSR 访问 `window`。
- 创建终端、加载 FitAddon、连接 WebSocket。
- `onData` 发送 `read`。
- `onResize` 发送 `resize`，使用 ResizeObserver 触发 fit。
- 收到 `write` 时调用 `terminal.write`。
- 打开后自动聚焦；关闭 Modal 时释放 observer、事件订阅、terminal 和 WebSocket。
- 状态覆盖：连接中、已连接、连接失败、会话结束。

`NodeDetailView` 的「终端」操作改为打开 `PodTerminal`，删除现有一次性命令输入框及“不提供交互式 stdin”提示。日志仍使用现有 SSE 面板。

弹窗标题：

`容器组名: {pod} 容器名: {container} 终端`

终端采用标准黑底主题；YAML 编辑器继续保持浅色，不受终端样式影响。弹窗支持关闭和全屏。

## WebSocket 地址

前端新增 `NEXT_PUBLIC_KUBERNETES_WS_URL`，值为浏览器可访问的后端或网关 WebSocket 基址。开发环境未配置时使用当前页面 hostname 和端口 `8080`；生产环境缺少配置时使用当前页面 origin 的 `ws://` 或 `wss://` 形式，并要求同站点反向代理转发终端路径。

推荐生产部署让前端和后端位于同一站点，由反向代理将 `/ws/plugins/kubernetes/terminal` 的 Upgrade 请求转发到 Go 服务。票据创建仍经 `/api/bff/v1/...`。WS 路径避开要求 Bearer Header 的全局 `/api/v1/plugins/*` 中间件，安全性由已鉴权签发、一次性消费且目标冻结的短期票据保证。

不依赖 Next.js Route Handler 代理 Upgrade，也不将 HttpOnly Cookie 或 JWT 复制到 JavaScript。

## 错误处理

- 票据创建失败：在弹窗中显示现有 `ApiErrorBanner`。
- WebSocket 握手失败：显示可重试状态，不自动无限重连。
- 上游容器不存在或 shell 缺失：向终端写入错误并结束连接。
- 浏览器断网：关闭本地终端状态，后端通过连接取消上游 exec。
- 会话退出：显示退出码，终端保留输出但停止输入。

错误消息不暴露 kubeconfig、JWT、票据、内部 Kubernetes API 地址或原始后端堆栈。

## 测试

后端采用测试驱动实现：

1. 票据仅消费一次、30 秒过期、目标不可变、成员配额生效。
2. 非法消息类型、超大输入和越界尺寸被拒绝。
3. WebSocket 输入映射到 stdin，resize 映射到 `TerminalSizeQueue`，输出映射到 `write`。
4. 断开连接取消 exec context。
5. 未授权用户无法签发票据，未配置集群凭据返回稳定错误。
6. 路由与 manifest 描述一致。

前端测试：

1. WebSocket URL 构建只包含票据，不含 JWT 和资源目标。
2. `read`、`resize` 和服务端消息解析协议。
3. Modal 关闭时释放终端和连接。
4. TypeScript typecheck 与 Kubernetes 现有测试通过。

## 非目标

- NodeTTY 交互终端。
- 将日志改为 xterm 或 WebSocket。
- 多标签终端、断线续连、会话共享。
- 文件传输、端口转发。
- Redis 或数据库共享票据。
- 客户端自定义 shell/命令。
