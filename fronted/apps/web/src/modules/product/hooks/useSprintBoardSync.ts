"use client";

import { useEffect, useRef } from "react";
import { product } from "@sparkops/web-sdk";
import { SprintBoardWsMessageSchema } from "@sparkops/contracts/product";
import { buildSprintBoardWebSocketURL } from "./sprint-board-ws";

interface UseSprintBoardSyncOptions {
  workspaceId: string;
  productId: string;
  sprintId: string | null;
  enabled?: boolean;
  onBoardChanged: () => void;
}

export function useSprintBoardSync({
  workspaceId,
  productId,
  sprintId,
  enabled = true,
  onBoardChanged,
}: UseSprintBoardSyncOptions) {
  const onBoardChangedRef = useRef(onBoardChanged);
  onBoardChangedRef.current = onBoardChanged;

  useEffect(() => {
    if (!enabled || !sprintId || typeof window === "undefined") return;

    let cancelled = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    async function connect() {
      if (cancelled) return;
      try {
        const config = await product.getSprintBoardWsConfig(workspaceId, productId, sprintId!);
        if (cancelled) return;

        const url = buildSprintBoardWebSocketURL({
          websocketPath: config.websocket_path,
          accessToken: config.access_token,
          workspaceId: config.workspace_id,
          configuredBase: process.env.NEXT_PUBLIC_API_WS_URL ?? process.env.NEXT_PUBLIC_KUBERNETES_WS_URL,
          location: window.location,
          isDev: process.env.NODE_ENV !== "production",
        });

        socket = new WebSocket(url);
        socket.onmessage = (event) => {
          try {
            const parsed = SprintBoardWsMessageSchema.safeParse(JSON.parse(String(event.data)));
            if (parsed.success && parsed.data.event === "board_changed") {
              onBoardChangedRef.current();
            }
          } catch {
            // ignore malformed messages
          }
        };
        socket.onclose = () => {
          if (!cancelled) {
            reconnectTimer = setTimeout(() => void connect(), 3000);
          }
        };
      } catch {
        if (!cancelled) {
          reconnectTimer = setTimeout(() => void connect(), 5000);
        }
      }
    }

    void connect();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      socket?.close();
    };
  }, [workspaceId, productId, sprintId, enabled]);
}
