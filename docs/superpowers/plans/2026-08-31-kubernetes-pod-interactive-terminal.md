# Kubernetes Pod Interactive Terminal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在节点详情「容器组 → 更多 → {容器} 终端」提供接近 `kubectl exec -it` 的浏览器交互终端。

**Architecture:** 浏览器经同源 BFF 用 Cookie 申请一次性票据；再直连 Go `GET /ws/plugins/kubernetes/terminal?ticket=` 升级 WebSocket。会话目标冻结在票据内。xterm 通过 `{type:read|resize}` / `{type:write}` 与 SPDY exec（TTY=true, Stdin=true）桥接。

**Tech Stack:** Go 1.26、Gin、gorilla/websocket v1.5.0、k8s client-go remotecommand、Next.js、`@xterm/xterm`、`@xterm/addon-fit`、vitest。

## Global Constraints

- 不在 WebSocket URL 中传递长期 JWT、cluster、pod、container 或 command。
- 不把 HttpOnly Cookie 或 JWT 复制到 JavaScript。
- 不依赖 Next.js Route Handler 代理 WebSocket Upgrade。
- 现有 SSE `/exec`、`/logs`、CloudTTY、NodeTTY 保持不变。
- 首版只覆盖 Pod 容器终端；不允许客户端自定义启动命令。
- 启动命令固定为 `/bin/sh -c 'clear 2>/dev/null || true; exec bash 2>/dev/null || exec sh'`。
- 票据有效期 30 秒、一次性、每成员最多 5 张未使用票据、进程内存存储。
- 尺寸范围 2–500 列、2–200 行；单条输入消息最多 64 KiB。
- 错误消息不暴露 kubeconfig、JWT、票据、内部 Kubernetes API 地址或原始堆栈。
- 审计沿用 `kubernetes.exec.started` / `kubernetes.exec.ended`，录制 kind 仍为 `exec`，只录制服务端输出。
- 用户未要求时不执行 git commit。

## File Structure

- Create: `backend/internal/plugins/kubernetes/terminal/ticket.go`
- Create: `backend/internal/plugins/kubernetes/terminal/ticket_test.go`
- Create: `backend/internal/plugins/kubernetes/terminal/protocol.go`
- Create: `backend/internal/plugins/kubernetes/terminal/protocol_test.go`
- Create: `backend/internal/plugins/kubernetes/terminal/session.go`
- Create: `backend/internal/plugins/kubernetes/terminal/session_test.go`
- Create: `backend/internal/plugins/kubernetes/terminal/handler.go`
- Create: `backend/internal/plugins/kubernetes/terminal/handler_test.go`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.test.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodTerminal.tsx`
- Modify: `backend/internal/plugins/kubernetes/http/router.go`
- Modify: `backend/internal/plugins/kubernetes/plugin.yaml`
- Modify: `backend/internal/plugins/kubernetes/plugin.go`
- Modify: `backend/go.mod`（gorilla/websocket 升为直接依赖）
- Modify: `fronted/apps/web/package.json`
- Modify: `fronted/apps/web/.env.example`
- Modify: `fronted/apps/web/src/components/ui/Modal.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodeDetailView.tsx`
- Modify: `fronted/apps/web/app/globals.css`

---

### Task 1: Ticket store

**Files:**
- Create: `backend/internal/plugins/kubernetes/terminal/ticket.go`
- Create: `backend/internal/plugins/kubernetes/terminal/ticket_test.go`

**Interfaces:**
- Consumes: `kernel.WorkspaceID`, `kernel.MemberID`, `kernel.ClusterInfoID`
- Produces: `type TicketStore struct`; `func NewTicketStore(now func() time.Time) *TicketStore`; `func (s *TicketStore) Issue(claims TicketClaims) (IssuedTicket, error)`; `func (s *TicketStore) Consume(ticket string) (TicketClaims, bool)`; `type TicketClaims struct`; `type IssuedTicket struct { Ticket string; ExpiresAt time.Time; WebsocketPath string }`; `const WebsocketPath = "/ws/plugins/kubernetes/terminal"`; `const TicketTTL = 30 * time.Second`; `const MaxTicketsPerMember = 5`

- [ ] **Step 1: Write the failing test**

```go
package terminal

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestTicketStoreIssuesOpaqueOneTimeTicket(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 59, 0, 0, time.UTC)
	store := NewTicketStore(func() time.Time { return now })
	claims := TicketClaims{
		WorkspaceID: kernel.NewWorkspaceID(),
		MemberID:    kernel.NewMemberID(),
		ClusterID:   kernel.NewClusterInfoID(),
		Namespace:   "kube-system",
		Pod:         "kube-proxy-xfkvs",
		Container:   "kube-proxy",
		Cols:        120,
		Rows:        36,
	}

	issued, err := store.Issue(claims)
	require.NoError(t, err)
	require.Len(t, issued.Ticket, 64)
	require.NotContains(t, issued.Ticket, claims.Pod)
	require.Equal(t, WebsocketPath, issued.WebsocketPath)
	require.Equal(t, now.Add(30*time.Second), issued.ExpiresAt)

	got, ok := store.Consume(issued.Ticket)
	require.True(t, ok)
	require.Equal(t, claims, got)
	_, ok = store.Consume(issued.Ticket)
	require.False(t, ok)
}

func TestTicketStoreRejectsExpiredAndOverQuota(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 59, 0, 0, time.UTC)
	store := NewTicketStore(func() time.Time { return now })
	member := kernel.NewMemberID()
	base := TicketClaims{WorkspaceID: kernel.NewWorkspaceID(), MemberID: member, ClusterID: kernel.NewClusterInfoID(), Namespace: "ns", Pod: "pod", Container: "c", Cols: 80, Rows: 24}

	expired, err := store.Issue(base)
	require.NoError(t, err)
	now = now.Add(31 * time.Second)
	_, ok := store.Consume(expired.Ticket)
	require.False(t, ok)

	for i := 0; i < MaxTicketsPerMember; i++ {
		_, err := store.Issue(base)
		require.NoError(t, err)
	}
	_, err = store.Issue(base)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/kubernetes/terminal -run TicketStore -count=1`

Working directory: `A:\21-Agent\SparkOpsAgent\backend`

Expected: FAIL because `NewTicketStore` is undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	WebsocketPath       = "/ws/plugins/kubernetes/terminal"
	TicketTTL           = 30 * time.Second
	MaxTicketsPerMember = 5
)

type TicketClaims struct {
	WorkspaceID kernel.WorkspaceID
	MemberID    kernel.MemberID
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Pod         string
	Container   string
	Cols        uint16
	Rows        uint16
}

type IssuedTicket struct {
	Ticket        string
	ExpiresAt     time.Time
	WebsocketPath string
}

type ticketRecord struct {
	claims    TicketClaims
	expiresAt time.Time
}

type TicketStore struct {
	now     func() time.Time
	mu      sync.Mutex
	tickets map[string]ticketRecord
}

func NewTicketStore(now func() time.Time) *TicketStore {
	if now == nil {
		now = time.Now
	}
	return &TicketStore{now: now, tickets: map[string]ticketRecord{}}
}

func (s *TicketStore) Issue(claims TicketClaims) (IssuedTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeLocked(now)
	count := 0
	for _, rec := range s.tickets {
		if rec.claims.MemberID == claims.MemberID {
			count++
		}
	}
	if count >= MaxTicketsPerMember {
		return IssuedTicket{}, errors.New("too many unused terminal tickets")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return IssuedTicket{}, err
	}
	token := hex.EncodeToString(raw)
	expires := now.Add(TicketTTL)
	s.tickets[token] = ticketRecord{claims: claims, expiresAt: expires}
	return IssuedTicket{Ticket: token, ExpiresAt: expires, WebsocketPath: WebsocketPath}, nil
}

func (s *TicketStore) Consume(ticket string) (TicketClaims, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeLocked(now)
	rec, ok := s.tickets[ticket]
	if !ok {
		return TicketClaims{}, false
	}
	delete(s.tickets, ticket)
	if !rec.expiresAt.After(now) {
		return TicketClaims{}, false
	}
	return rec.claims, true
}

func (s *TicketStore) purgeLocked(now time.Time) {
	for key, rec := range s.tickets {
		if !rec.expiresAt.After(now) {
			delete(s.tickets, key)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/kubernetes/terminal -run TicketStore -count=1`

Expected: PASS

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 2: Protocol and size validation

**Files:**
- Create: `backend/internal/plugins/kubernetes/terminal/protocol.go`
- Create: `backend/internal/plugins/kubernetes/terminal/protocol_test.go`

**Interfaces:**
- Consumes: none
- Produces: `type ClientMessage struct { Type string; Data string; Cols uint16; Rows uint16 }`; `type ServerMessage struct { Type string; Data string; Message string; Code *int }`; `func ParseClientMessage(raw []byte) (ClientMessage, error)`; `func ValidateSize(cols, rows uint16) error`; `func ShellCommand() []string`; `const MaxInputBytes = 64 << 10`; `const MinCols/MaxCols/MinRows/MaxRows`

- [ ] **Step 1: Write the failing test**

```go
package terminal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClientMessageRejectsInvalidInput(t *testing.T) {
	_, err := ParseClientMessage([]byte(`{"type":"unknown"}`))
	require.Error(t, err)
	_, err = ParseClientMessage([]byte(`{"type":"read","data":"` + strings.Repeat("a", MaxInputBytes+1) + `"}`))
	require.Error(t, err)
	require.Error(t, ValidateSize(1, 24))
	require.Error(t, ValidateSize(80, 201))
	require.NoError(t, ValidateSize(80, 24))
}

func TestParseClientMessageAcceptsReadAndResize(t *testing.T) {
	read, err := ParseClientMessage([]byte(`{"type":"read","data":"ls\r"}`))
	require.NoError(t, err)
	require.Equal(t, "read", read.Type)
	require.Equal(t, "ls\r", read.Data)
	resize, err := ParseClientMessage([]byte(`{"type":"resize","cols":120,"rows":36}`))
	require.NoError(t, err)
	require.Equal(t, uint16(120), resize.Cols)
	require.Equal(t, []string{"/bin/sh", "-c", "clear 2>/dev/null || true; exec bash 2>/dev/null || exec sh"}, ShellCommand())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/kubernetes/terminal -run 'ParseClientMessage|ShellCommand' -count=1`

Working directory: `A:\21-Agent\SparkOpsAgent\backend`

Expected: FAIL because `ParseClientMessage` is undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	MaxInputBytes = 64 << 10
	MinCols       = 2
	MaxCols       = 500
	MinRows       = 2
	MaxRows       = 200
)

type ClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type ServerMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Code    *int   `json:"code,omitempty"`
}

func ParseClientMessage(raw []byte) (ClientMessage, error) {
	if len(raw) > MaxInputBytes+256 {
		return ClientMessage{}, errors.New("message too large")
	}
	var msg ClientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ClientMessage{}, err
	}
	switch msg.Type {
	case "read":
		if len(msg.Data) > MaxInputBytes {
			return ClientMessage{}, errors.New("input too large")
		}
		return msg, nil
	case "resize":
		if err := ValidateSize(msg.Cols, msg.Rows); err != nil {
			return ClientMessage{}, err
		}
		return msg, nil
	default:
		return ClientMessage{}, fmt.Errorf("unknown message type")
	}
}

func ValidateSize(cols, rows uint16) error {
	if cols < MinCols || cols > MaxCols || rows < MinRows || rows > MaxRows {
		return errors.New("invalid terminal size")
	}
	return nil
}

func ShellCommand() []string {
	return []string{"/bin/sh", "-c", "clear 2>/dev/null || true; exec bash 2>/dev/null || exec sh"}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/kubernetes/terminal -count=1`

Expected: PASS

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 3: Session bridge

**Files:**
- Create: `backend/internal/plugins/kubernetes/terminal/session.go`
- Create: `backend/internal/plugins/kubernetes/terminal/session_test.go`

**Interfaces:**
- Consumes: `ParseClientMessage`, `ValidateSize`, `ClientMessage`
- Produces: `type Session struct`; `func NewSession(cols, rows uint16) *Session`; `func (s *Session) HandleClientJSON(raw []byte) error`; `func (s *Session) Read(p []byte) (int, error)`; `func (s *Session) Write(p []byte) (int, error)`; `func (s *Session) Next() *remotecommand.TerminalSize`; `func (s *Session) Close()`; `func (s *Session) TakeWrites() []ServerMessage`; `OnWrite func(ServerMessage)`

Use an in-memory `Session` that implements `io.Reader`, `io.Writer`, and `remotecommand.TerminalSizeQueue`. Tests do not open a real WebSocket.

- [ ] **Step 1: Write the failing test**

```go
package terminal

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
)

func TestSessionMapsReadResizeAndWrite(t *testing.T) {
	session := NewSession(80, 24)
	require.NoError(t, session.HandleClientJSON([]byte(`{"type":"read","data":"pwd\r"}`)))
	buf := make([]byte, 16)
	n, err := session.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "pwd\r", string(buf[:n]))

	require.NoError(t, session.HandleClientJSON([]byte(`{"type":"resize","cols":120,"rows":40}`)))
	size := session.Next()
	require.Equal(t, &remotecommand.TerminalSize{Width: 120, Height: 40}, size)

	n, err = session.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []ServerMessage{{Type: "write", Data: "hello"}}, session.TakeWrites())
}

func TestSessionCloseUnblocksReadAndNext(t *testing.T) {
	session := NewSession(80, 24)
	session.Close()
	_, err := session.Read(make([]byte, 8))
	require.ErrorIs(t, err, io.EOF)
	require.Nil(t, session.Next())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/kubernetes/terminal -run Session -count=1`

Working directory: `A:\21-Agent\SparkOpsAgent\backend`

Expected: FAIL because `NewSession` is undefined.

- [ ] **Step 3: Write minimal implementation**

Implement `Session` with:

- `stdin` channel of `[]byte`
- `sizeChan` of `remotecommand.TerminalSize`
- `done` channel
- mutex-protected `writes []ServerMessage` and optional `OnWrite`
- `Read` copies one stdin chunk or returns `io.EOF` after `Close`
- `HandleClientJSON` parses then either queues stdin or size
- `Next` returns queued size, initial size once, or nil after close
- `Write` appends `{Type:"write", Data:string(p)}` and calls `OnWrite` if set
- never write WebSocket from this type; handler owns the connection

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/kubernetes/terminal -count=1`

Expected: PASS

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 4: HTTP handlers, routes, and origin check

**Files:**
- Create: `backend/internal/plugins/kubernetes/terminal/handler.go`
- Create: `backend/internal/plugins/kubernetes/terminal/handler_test.go`
- Modify: `backend/internal/plugins/kubernetes/http/router.go`
- Modify: `backend/internal/plugins/kubernetes/plugin.yaml`
- Modify: `backend/internal/plugins/kubernetes/plugin.go`
- Modify: `backend/go.mod` — move `github.com/gorilla/websocket v1.5.0` to direct `require`

**Interfaces:**
- Consumes: `TicketStore`, `TicketClaims`, `IssuedTicket`, `ShellCommand`, `ValidateSize`, `NewSession`, `sparkplugin.PluginContext`
- Produces: `func NewHandler(ctx sparkplugin.PluginContext, tickets *TicketStore) *Handler`; `func (h *Handler) CreateTicket(c *gin.Context)`; `func (h *Handler) ServeWS(c *gin.Context)`; `func AllowedOrigin(origin, host string) bool`

`CreateTicket` body:

```json
{"container":"kube-proxy","cols":120,"rows":36}
```

Success data:

```json
{"ticket":"...","expires_at":"...","websocket_path":"/ws/plugins/kubernetes/terminal"}
```

`ServeWS` consumes ticket, checks Origin, upgrades, sends `{type:open}`, streams exec, then `{type:exit,code:N}` or `{type:error,message:"pod terminal unavailable"}`.

Origin rule: empty Origin allowed for same-site tools; otherwise Origin host must equal request Host, or `http://localhost:3000` / `http://127.0.0.1:3000` when Host is `localhost:8080` or `127.0.0.1:8080`.

Register `POST .../pods/:pod/terminal-ticket` on the existing workspace-member group. Register WebSocket outside `/api/v1/plugins/*`，避免浏览器 WebSocket 被 Bearer-only API 中间件拦截。`ServeWS` 只通过原子消费短期票据认证：

```go
engine.GET("/ws/plugins/kubernetes/terminal", terminalHandler.ServeWS)
```

Ticket POST stays on the member+auth group.

plugin.yaml add:

```yaml
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket
```

plugin.go InstalledFiles add the new `terminal/*.go` files.

Exec options:

```go
corev1.PodExecOptions{Container: claims.Container, Command: ShellCommand(), Stdin: true, Stdout: true, Stderr: false, TTY: true}
remotecommand.StreamOptions{Stdin: session, Stdout: io.MultiWriter(session, recorder.Stdout()), Tty: true, TerminalSizeQueue: session}
```

If recorder is nil, stdout is `session` only. Audit `kubernetes.exec.started` before stream and `kubernetes.exec.ended` after, with status `completed` / `failed` / `canceled`.

- [ ] **Step 1: Write the failing tests**

Cover:

1. `AllowedOrigin("http://localhost:3000", "localhost:8080") == true`
2. `AllowedOrigin("https://evil.example", "localhost:8080") == false`
3. `CreateTicket` without write permission returns 403 (mock `WriteAuthorizer`)
4. `CreateTicket` invalid container / size returns 400
5. `ServeWS` missing/expired ticket returns 401/400 JSON, no upgrade
6. `ServeWS` ticket workspace mismatch returns 403
7. After consume, second WS with same ticket fails

Use `httptest` + `gin.CreateTestContext`. For WS success path, inject a fake exec streamer:

```go
type ExecStreamer interface {
	Stream(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, claims TicketClaims, session *Session) error
}
```

The fake copies stdin to `session.Write` and returns nil. Test with `httptest.NewServer` and `websocket.DefaultDialer`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/kubernetes/terminal -count=1`

Working directory: `A:\21-Agent\SparkOpsAgent\backend`

Expected: FAIL because `Handler` is undefined.

- [ ] **Step 3: Write minimal implementation and wire routes**

In `router.go`:

```go
k8sterminal "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/terminal"

terminalHandler := k8sterminal.NewHandler(pctx, k8sterminal.NewTicketStore(nil))
group.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket", terminalHandler.CreateTicket)
engine.GET("/ws/plugins/kubernetes/terminal", terminalHandler.ServeWS)
```

Reuse DNS validation from stream package by duplicating the two small helpers in `handler.go` (same `validation.IsDNS1123Label` / `IsDNS1123Subdomain`) to avoid exporting stream internals.

CreateTicket: `requireWrite`, parse cluster/namespace/pod/container, `ValidateSize`, `tickets.Issue`, `httpx.OK`.

gorilla upgrader: `CheckOrigin` delegates to `AllowedOrigin(r.Header.Get("Origin"), r.Host)`.

On WS read loop: `session.HandleClientJSON`; on session `OnWrite`: marshal `ServerMessage` and `WriteMessage` under a mutex.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/plugins/kubernetes/terminal ./internal/plugins/kubernetes/http -count=1`

Expected: PASS. If `http` package has no tests, only terminal tests need to pass.

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 5: Frontend URL/protocol helpers

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.test.ts`
- Modify: `fronted/apps/web/.env.example`

**Interfaces:**
- Consumes: none
- Produces: `export function podTerminalTicketPath(clusterId, namespace, pod)`; `export function buildTerminalWebSocketURL(options: { ticket: string; websocketPath: string; configuredBase?: string; location: { protocol: string; hostname: string; host: string; origin: string }; isDev: boolean })`; `export function encodeClientMessage(...)`; `export function parseServerMessage(raw: string)`; `export type TerminalTicketResponse`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest";
import {
  buildTerminalWebSocketURL,
  encodeClientMessage,
  parseServerMessage,
  podTerminalTicketPath,
} from "./pod-terminal";

describe("pod terminal helpers", () => {
  it("builds the BFF ticket path without putting secrets in the URL", () => {
    expect(podTerminalTicketPath("dev", "kube-system", "kube-proxy-xfkvs")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/dev/namespaces/kube-system/pods/kube-proxy-xfkvs/terminal-ticket",
    );
  });

  it("builds a websocket URL that only includes the ticket", () => {
    const url = buildTerminalWebSocketURL({
      ticket: "abc123",
      websocketPath: "/ws/plugins/kubernetes/terminal",
      location: { protocol: "http:", hostname: "localhost", host: "localhost:3000", origin: "http://localhost:3000" },
      isDev: true,
    });
    expect(url).toBe("ws://localhost:8080/ws/plugins/kubernetes/terminal?ticket=abc123");
    expect(url).not.toContain("jwt");
    expect(url).not.toContain("kube-proxy");
  });

  it("uses configured base and current origin in production", () => {
    expect(buildTerminalWebSocketURL({
      ticket: "t",
      websocketPath: "/ws/plugins/kubernetes/terminal",
      configuredBase: "wss://ops.example/ws",
      location: { protocol: "https:", hostname: "ops.example", host: "ops.example", origin: "https://ops.example" },
      isDev: false,
    })).toBe("wss://ops.example/ws/ws/plugins/kubernetes/terminal?ticket=t");
    expect(buildTerminalWebSocketURL({
      ticket: "t",
      websocketPath: "/ws/plugins/kubernetes/terminal",
      location: { protocol: "https:", hostname: "ops.example", host: "ops.example", origin: "https://ops.example" },
      isDev: false,
    })).toBe("wss://ops.example/ws/plugins/kubernetes/terminal?ticket=t");
  });

  it("encodes client messages and parses server messages", () => {
    expect(JSON.parse(encodeClientMessage({ type: "read", data: "ls\r" }))).toEqual({ type: "read", data: "ls\r" });
    expect(JSON.parse(encodeClientMessage({ type: "resize", cols: 120, rows: 36 }))).toEqual({ type: "resize", cols: 120, rows: 36 });
    expect(parseServerMessage(`{"type":"write","data":"ok"}`)).toEqual({ type: "write", data: "ok" });
    expect(parseServerMessage("{")).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run apps/web/src/modules/plugins/kubernetes/pod-terminal.test.ts --reporter=dot`

Working directory: `A:\21-Agent\SparkOpsAgent\fronted`

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Write minimal implementation**

`podTerminalTicketPath` uses `podOperationPath(clusterId, namespace, pod, "terminal-ticket")` from `operations.ts`.

`buildTerminalWebSocketURL`:

1. If `configuredBase` is non-empty, strip trailing slash and append `websocketPath`.
2. Else if `isDev`, use `ws://${location.hostname}:8080` + path.
3. Else convert `location.origin` http→ws / https→wss + path.
4. Append `?ticket=` with `encodeURIComponent`.

Add to `.env.example`:

```
# Browser-reachable WebSocket base for Pod terminals (optional). Example: ws://localhost:8080
NEXT_PUBLIC_KUBERNETES_WS_URL=
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run apps/web/src/modules/plugins/kubernetes/pod-terminal.test.ts --reporter=dot`

Expected: PASS

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 6: xterm UI and NodeDetailView wiring

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodTerminal.tsx`
- Modify: `fronted/apps/web/package.json` — add `"@xterm/xterm"` and `"@xterm/addon-fit"`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodeDetailView.tsx`
- Modify: `fronted/apps/web/src/components/ui/Modal.tsx` — add optional `headerExtra?: React.ReactNode`
- Modify: `fronted/apps/web/app/globals.css`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/operations.ts` only if Task 5 did not reuse `podOperationPath`

**Interfaces:**
- Consumes: `podTerminalTicketPath`, `buildTerminalWebSocketURL`, `encodeClientMessage`, `parseServerMessage`, `k8sFetch`, `TerminalTicketResponse`
- Produces: `export function PodTerminal(props: { clusterId: string; namespace: string; pod: string; container: string; canManage: boolean })`

- [ ] **Step 1: Install packages**

Run in `A:\21-Agent\SparkOpsAgent\fronted\apps\web`:

`pnpm add @xterm/xterm @xterm/addon-fit`

- [ ] **Step 2: Implement Modal headerExtra**

In `Modal.tsx` header, after title block and before close button:

```tsx
{headerExtra}
```

Add `headerExtra?: React.ReactNode` to `ModalProps`.

- [ ] **Step 3: Implement PodTerminal**

Behavior:

1. `useEffect` on mount: if `!canManage` show error and return.
2. `k8sFetch<TerminalTicketResponse>(podTerminalTicketPath(...), { method: "POST", body: JSON.stringify({ container, cols: 120, rows: 36 }) })`.
3. Dynamic `import("@xterm/xterm")` and `import("@xterm/addon-fit")`, import `@xterm/xterm/css/xterm.css`.
4. Create Terminal: `cursorBlink: true`, `cursorStyle: "bar"`, `convertEol: true`, `scrollback: 1000`, theme `{ foreground: "#FFFFFF", background: "#000000" }`.
5. `writeln("欢迎使用运维系统终端")`.
6. Open on a ref div, FitAddon.fit(), send resize after open.
7. `new WebSocket(buildTerminalWebSocketURL({ ticket, websocketPath, configuredBase: process.env.NEXT_PUBLIC_KUBERNETES_WS_URL, location: window.location, isDev: process.env.NODE_ENV !== "production" }))`.
8. onData → send read if socket OPEN; onResize → send resize; FitAddon via ResizeObserver on container.
9. onmessage → parseServerMessage; write data; on error/exit disable stdin overlay text.
10. Cleanup: observer.disconnect, socket.close, terminal.dispose.
11. Status line: 连接中 / 已连接 / 连接失败（可重试按钮，重新跑 effect）/ 会话结束.
12. No automatic infinite reconnect.

CSS:

```css
.k8s-pod-terminal {
  display: flex;
  flex-direction: column;
  min-height: 24rem;
  background: #000000;
  color: #ffffff;
}
.k8s-pod-terminal-host {
  flex: 1;
  min-height: 20rem;
  padding: 0.5rem;
}
.k8s-pod-terminal-host .xterm {
  height: 100%;
}
.k8s-pod-terminal-status {
  padding: 0.35rem 0.75rem;
  font-size: 0.75rem;
  background: #111111;
  color: #e2e8f0;
}
```

- [ ] **Step 4: Wire NodeDetailView**

Change stream target so exec opens interactive terminal:

- Modal title: `容器组名: ${pod} 容器名: ${container} 终端`
- `size="xl"`
- `headerExtra`: fullscreen button toggling class `k8s-pod-terminal-modal-fullscreen` on the modal panel via `panelClassName`
- Children: `<PodTerminal clusterId namespace pod container canManage />`
- Remove `PodStreamPanel` exec command input, `toCommandQuery`, and the “非交互式” copy for exec mode. Keep logs SSE panel.

Fullscreen CSS:

```css
.modal-panel.k8s-pod-terminal-modal-fullscreen {
  width: 100vw;
  max-width: 100vw;
  height: 100vh;
  max-height: 100vh;
  border-radius: 0;
}
```

- [ ] **Step 5: Run frontend tests and typecheck**

Run from `A:\21-Agent\SparkOpsAgent\fronted`:

`pnpm exec vitest run apps/web/src/modules/plugins/kubernetes/pod-terminal.test.ts apps/web/src/modules/plugins/kubernetes/operations.test.ts apps/web/src/modules/plugins/kubernetes/node-view.test.ts --reporter=dot`

Run from `A:\21-Agent\SparkOpsAgent\fronted\apps\web`:

`pnpm typecheck`

Expected: tests PASS, typecheck exit 0.

- [ ] **Step 6: Restart Go backend** so the new route is loaded. Frontend `pnpm dev` hot-reloads. Manually open 节点详情 → 容器组 → 更多 → 终端, type a command, resize, close modal.

- [ ] **Step 7: Do not commit unless the user asks**

---

## Spec coverage

| Spec requirement | Task |
|---|---|
| Ticket + WS two-step auth | 1, 4, 5 |
| Frozen target, no JWT in URL | 1, 4, 5 |
| read/resize/write protocol | 2, 3, 5, 6 |
| Size and input limits | 2, 4 |
| SPDY TTY stdin/stdout, fixed shell | 4 |
| Origin check | 4 |
| Audit + stdout recording | 4 |
| xterm UI, title, fullscreen, light YAML unchanged | 6 |
| Logs/SSE exec unchanged | 6 |
| NodeTTY / custom command out of scope | none |

## Placeholder scan

Plan uses concrete types, commands, and file paths. Commit steps are explicitly skipped unless the user asks.
