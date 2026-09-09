"use client";

import { useEffect, useRef, useState } from "react";
import "@xterm/xterm/css/xterm.css";
import { BffError } from "@sparkops/contracts";
import {
  buildTerminalWebSocketURL,
  clampTerminalSize,
  encodeClientMessage,
  focusTerminalWithoutScroll,
  parseServerMessage,
  projectPodTerminalTicketPath,
  splitTerminalInput,
  type TerminalTicketResponse,
} from "../project-pod-terminal";

type TerminalStatus = "connecting" | "connected" | "failed" | "ended";

async function fetchTerminalTicket(
  path: string,
  body: { container: string; cols: number; rows: number },
  signal: AbortSignal,
): Promise<TerminalTicketResponse> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
    signal,
  });
  const payload = (await res.json()) as {
    code?: number;
    message?: string;
    data?: TerminalTicketResponse;
  };
  if (!res.ok || payload.code !== 0 || !payload.data) {
    throw new BffError(payload.message || "打开容器终端失败", "BFF_ERROR", res.status);
  }
  return payload.data;
}

export function ProjectPodTerminal({
  workspaceId,
  projectId,
  podId,
  container,
  active,
}: {
  workspaceId: string;
  projectId: string;
  podId: string;
  container: string;
  active: boolean;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<TerminalStatus>("connecting");
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!active) return;
    const currentHost = hostRef.current;
    if (!currentHost) return;
    const host: HTMLDivElement = currentHost;

    let cancelled = false;
    let socket: WebSocket | null = null;
    let terminal: import("@xterm/xterm").Terminal | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let inputSubscription: { dispose(): void } | null = null;
    let resizeSubscription: { dispose(): void } | null = null;
    const requestController = new AbortController();

    setStatus("connecting");
    setError(null);
    host.replaceChildren();

    async function connect() {
      try {
        const [{ Terminal }, { FitAddon }] = await Promise.all([
          import("@xterm/xterm"),
          import("@xterm/addon-fit"),
        ]);
        if (cancelled) return;

        terminal = new Terminal({
          convertEol: true,
          cursorBlink: true,
          cursorStyle: "bar",
          fontSize: 14,
          scrollback: 1000,
          theme: {
            background: "#000000",
            foreground: "#ffffff",
            cursor: "#ffffff",
          },
        });
        const fitAddon = new FitAddon();
        terminal.loadAddon(fitAddon);
        terminal.open(host);
        fitAddon.fit();
        terminal.writeln(`# connected to ${container}`);
        terminal.writeln("");

        const initialSize = clampTerminalSize(terminal.cols || 120, terminal.rows || 36);
        const ticket = await fetchTerminalTicket(
          projectPodTerminalTicketPath(workspaceId, projectId, podId),
          {
            container,
            cols: Math.trunc(initialSize.cols),
            rows: Math.trunc(initialSize.rows),
          },
          requestController.signal,
        );
        if (cancelled) return;

        socket = new WebSocket(
          buildTerminalWebSocketURL({
            ticket: ticket.ticket,
            websocketPath: ticket.websocket_path,
            configuredBase: process.env.NEXT_PUBLIC_KUBERNETES_WS_URL,
            location: window.location,
            isDev: process.env.NODE_ENV !== "production",
          }),
        );

        inputSubscription = terminal.onData((data) => {
          if (socket?.readyState === WebSocket.OPEN) {
            for (const chunk of splitTerminalInput(data)) {
              socket.send(encodeClientMessage({ type: "read", data: chunk }));
            }
          }
        });
        resizeSubscription = terminal.onResize(({ cols, rows }) => {
          if (socket?.readyState === WebSocket.OPEN) {
            socket.send(encodeClientMessage({ type: "resize", ...clampTerminalSize(cols, rows) }));
          }
        });
        resizeObserver = new ResizeObserver(() => {
          try {
            fitAddon.fit();
          } catch {
            // Terminal may be disposing while the observer callback runs.
          }
        });
        resizeObserver.observe(host);

        socket.onopen = () => {
          if (cancelled || !terminal || !socket) return;
          fitAddon.fit();
          socket.send(
            encodeClientMessage({
              type: "resize",
              ...clampTerminalSize(terminal.cols, terminal.rows),
            }),
          );
          focusTerminalWithoutScroll(terminal);
          setStatus("connected");
        };
        socket.onmessage = (event) => {
          if (!terminal || typeof event.data !== "string") return;
          const message = parseServerMessage(event.data);
          if (!message) return;
          if (message.type === "write") {
            terminal.write(message.data);
          } else if (message.type === "error") {
            terminal.writeln(`\r\n错误：${message.message}`);
            setError(message.message);
          } else if (message.type === "exit") {
            terminal.writeln(`\r\n会话已结束（退出码 ${message.code}）`);
            terminal.options.disableStdin = true;
            setStatus("ended");
          }
        };
        socket.onerror = () => {
          if (cancelled) return;
          setStatus("failed");
          setError("终端 WebSocket 连接失败");
        };
        socket.onclose = () => {
          if (cancelled) return;
          terminal?.blur();
          if (terminal) terminal.options.disableStdin = true;
          setStatus((current) => (current === "failed" ? current : "ended"));
        };
      } catch (err) {
        if (cancelled) return;
        setStatus("failed");
        setError(err instanceof BffError ? err.message : err instanceof Error ? err.message : "打开容器终端失败");
      }
    }

    void connect();
    return () => {
      cancelled = true;
      requestController.abort();
      resizeObserver?.disconnect();
      inputSubscription?.dispose();
      resizeSubscription?.dispose();
      if (socket && socket.readyState < WebSocket.CLOSING) socket.close();
      terminal?.dispose();
    };
  }, [active, attempt, container, podId, projectId, workspaceId]);

  const statusText = {
    connecting: "连接中…",
    connected: "已连接",
    failed: "连接失败",
    ended: "会话已结束",
  }[status];

  return (
    <div className="k8s-pod-terminal pod-detail-terminal">
      <div ref={hostRef} className="k8s-pod-terminal-host" />
      <div className="k8s-pod-terminal-status">
        <span>{statusText}</span>
        {status === "failed" ? (
          <button type="button" className="k8s-terminal-retry" onClick={() => setAttempt((value) => value + 1)}>
            重试
          </button>
        ) : null}
      </div>
      {error ? <p className="pod-detail-inline-error">{error}</p> : null}
    </div>
  );
}
