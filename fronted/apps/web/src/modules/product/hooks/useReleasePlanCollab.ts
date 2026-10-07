"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ReleasePlanCollabEditor } from "@sparkops/contracts/product";
import { product } from "@sparkops/web-sdk";
import { buildSprintBoardWebSocketURL } from "./sprint-board-ws";

type BrowserLocation = {
  protocol: string;
  hostname: string;
  host: string;
  origin: string;
};

export function useReleasePlanCollab(options: {
  enabled: boolean;
  workspaceId: string;
  productId: string;
  planId?: string;
  memberId?: string;
  section?: string;
  location: BrowserLocation;
  isDev: boolean;
}) {
  const [editors, setEditors] = useState<ReleasePlanCollabEditor[]>([]);
  const sectionRef = useRef(options.section ?? "plan");

  useEffect(() => {
    sectionRef.current = options.section ?? "plan";
  }, [options.section]);

  const notifySection = useCallback((socket: WebSocket, section: string) => {
    if (socket.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({ action: "join", section }));
  }, []);

  useEffect(() => {
    if (!options.enabled || !options.planId) {
      setEditors([]);
      return;
    }

    let cancelled = false;
    let socket: WebSocket | null = null;
    let heartbeat: ReturnType<typeof setInterval> | null = null;

    async function connect() {
      try {
        const config = await product.getReleasePlanCollabWsConfig(
          options.workspaceId,
          options.productId,
          options.planId!,
        );
        const url = buildSprintBoardWebSocketURL({
          websocketPath: config.websocket_path,
          accessToken: config.access_token,
          workspaceId: config.workspace_id,
          configuredBase: process.env.NEXT_PUBLIC_BACKEND_WS_BASE,
          location: options.location,
          isDev: options.isDev,
        });
        socket = new WebSocket(url);
        socket.onopen = () => notifySection(socket!, sectionRef.current);
        socket.onmessage = (event) => {
          try {
            const payload = JSON.parse(String(event.data)) as {
              event?: string;
              payload?: { editors?: ReleasePlanCollabEditor[] };
            };
            if (payload.event === "editors" && payload.payload?.editors) {
              if (!cancelled) setEditors(payload.payload.editors);
            }
          } catch {
            // ignore malformed messages
          }
        };
        heartbeat = setInterval(() => {
          notifySection(socket!, sectionRef.current);
        }, 20_000);
      } catch {
        if (!cancelled) setEditors([]);
      }
    }

    void connect();

    return () => {
      cancelled = true;
      if (heartbeat) clearInterval(heartbeat);
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ action: "leave" }));
      }
      socket?.close();
    };
  }, [
    options.enabled,
    options.workspaceId,
    options.productId,
    options.planId,
    options.memberId,
    options.location,
    options.isDev,
    notifySection,
  ]);

  useEffect(() => {
    // section changes are pushed via heartbeat / focus changes in parent
  }, [options.section]);

  const others = editors.filter((editor) => editor.member_id !== options.memberId);

  return { editors: others, allEditors: editors };
}
