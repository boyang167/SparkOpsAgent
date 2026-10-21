import { describe, expect, it } from "vitest";
import {
  buildTerminalWebSocketURL,
  clampTerminalSize,
  encodeClientMessage,
  parseServerMessage,
  podTerminalTicketPath,
  splitTerminalInput,
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
      location: {
        protocol: "http:",
        hostname: "localhost",
        host: "localhost:3000",
        origin: "http://localhost:3000",
      },
      isDev: true,
    });
    expect(url).toBe("ws://localhost:8080/ws/plugins/kubernetes/terminal?ticket=abc123");
    expect(url).not.toContain("jwt");
    expect(url).not.toContain("kube-proxy");
  });

  it("uses configured base and current origin in production", () => {
    expect(
      buildTerminalWebSocketURL({
        ticket: "t",
        websocketPath: "/ws/plugins/kubernetes/terminal",
        configuredBase: "wss://gateway.example",
        location: {
          protocol: "https:",
          hostname: "ops.example",
          host: "ops.example",
          origin: "https://ops.example",
        },
        isDev: false,
      }),
    ).toBe("wss://gateway.example/ws/plugins/kubernetes/terminal?ticket=t");
    expect(
      buildTerminalWebSocketURL({
        ticket: "t",
        websocketPath: "/ws/plugins/kubernetes/terminal",
        location: {
          protocol: "https:",
          hostname: "ops.example",
          host: "ops.example",
          origin: "https://ops.example",
        },
        isDev: false,
      }),
    ).toBe("wss://ops.example/ws/plugins/kubernetes/terminal?ticket=t");
  });

  it("encodes client messages and parses server messages", () => {
    expect(JSON.parse(encodeClientMessage({ type: "read", data: "ls\r" }))).toEqual({
      type: "read",
      data: "ls\r",
    });
    expect(JSON.parse(encodeClientMessage({ type: "resize", cols: 120, rows: 36 }))).toEqual({
      type: "resize",
      cols: 120,
      rows: 36,
    });
    expect(parseServerMessage(`{"type":"write","data":"ok"}`)).toEqual({
      type: "write",
      data: "ok",
    });
    expect(parseServerMessage("{")).toBeNull();
  });

  it("clamps resize values and chunks large unicode paste input", () => {
    expect(clampTerminalSize(1, 999)).toEqual({ cols: 2, rows: 200 });
    expect(clampTerminalSize(120, 36)).toEqual({ cols: 120, rows: 36 });
    const chunks = splitTerminalInput("🙂".repeat(20_000));
    expect(chunks.join("")).toBe("🙂".repeat(20_000));
    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks.every((chunk) => new TextEncoder().encode(chunk).byteLength <= 32 * 1024)).toBe(true);
  });
});
