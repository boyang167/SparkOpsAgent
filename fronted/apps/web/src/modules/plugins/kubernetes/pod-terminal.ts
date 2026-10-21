import { podOperationPath } from "./operations";

export type TerminalTicketResponse = {
  ticket: string;
  expires_at: string;
  websocket_path: string;
};

export type TerminalClientMessage =
  | { type: "read"; data: string }
  | { type: "resize"; cols: number; rows: number };

export type TerminalServerMessage =
  | { type: "open" }
  | { type: "write"; data: string }
  | { type: "error"; message: string }
  | { type: "exit"; code: number };

type BrowserLocation = {
  protocol: string;
  hostname: string;
  host: string;
  origin: string;
};

export function podTerminalTicketPath(clusterId: string, namespace: string, pod: string) {
  return podOperationPath(clusterId, namespace, pod, "terminal-ticket");
}

export function buildTerminalWebSocketURL(options: {
  ticket: string;
  websocketPath: string;
  configuredBase?: string;
  location: BrowserLocation;
  isDev: boolean;
}) {
  const configured = options.configuredBase?.trim().replace(/\/+$/, "");
  let base: string;
  if (configured) {
    base = configured.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
  } else if (options.isDev) {
    base = `ws://${options.location.hostname}:8080`;
  } else {
    base = options.location.origin.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
  }
  const path = options.websocketPath.startsWith("/") ? options.websocketPath : `/${options.websocketPath}`;
  return `${base}${path}?ticket=${encodeURIComponent(options.ticket)}`;
}

export function encodeClientMessage(message: TerminalClientMessage) {
  return JSON.stringify(message);
}

export function clampTerminalSize(cols: number, rows: number) {
  return {
    cols: Math.max(2, Math.min(500, Math.trunc(cols))),
    rows: Math.max(2, Math.min(200, Math.trunc(rows))),
  };
}

export function splitTerminalInput(data: string, maxBytes = 32 * 1024) {
  if (!data) return [];
  const encoder = new TextEncoder();
  const chunks: string[] = [];
  let current = "";
  let currentBytes = 0;
  for (const character of data) {
    const bytes = encoder.encode(character).byteLength;
    if (current && currentBytes + bytes > maxBytes) {
      chunks.push(current);
      current = "";
      currentBytes = 0;
    }
    current += character;
    currentBytes += bytes;
  }
  if (current) chunks.push(current);
  return chunks;
}

export function focusTerminalWithoutScroll(terminal: import("@xterm/xterm").Terminal) {
  const textarea = terminal.textarea;
  if (textarea) {
    textarea.focus({ preventScroll: true });
    return;
  }
  terminal.focus();
}

export function parseServerMessage(raw: string): TerminalServerMessage | null {
  try {
    const value = JSON.parse(raw) as Record<string, unknown>;
    switch (value.type) {
      case "open":
        return { type: "open" };
      case "write":
        return typeof value.data === "string" ? { type: "write", data: value.data } : null;
      case "error":
        return typeof value.message === "string" ? { type: "error", message: value.message } : null;
      case "exit":
        return typeof value.code === "number" ? { type: "exit", code: value.code } : null;
      default:
        return null;
    }
  } catch {
    return null;
  }
}
