"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { notification } from "@sparkops/web-sdk";
import { NOTIFICATIONS_CHANGED_EVENT, unreadBadgeText } from "./notification-events";

const POLL_INTERVAL_MS = 30_000;

export function NotificationBell({ workspaceId }: { workspaceId?: string }) {
  const pathname = usePathname();
  const [unread, setUnread] = useState(0);
  const inflight = useRef(false);

  const refresh = useCallback(async () => {
    if (!workspaceId || inflight.current) return;
    inflight.current = true;
    try {
      const data = await notification.listNotifications(workspaceId);
      setUnread(data.unread_count);
    } catch {
      // Keep the last known count; the badge is best-effort.
    } finally {
      inflight.current = false;
    }
  }, [workspaceId]);

  useEffect(() => {
    void refresh();
  }, [refresh, pathname]);

  useEffect(() => {
    if (!workspaceId) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, POLL_INTERVAL_MS);
    const onFocus = () => void refresh();
    const onChanged = (event: Event) => {
      const detail = (event as CustomEvent<{ unread?: number }>).detail;
      if (typeof detail?.unread === "number") setUnread(detail.unread);
      else void refresh();
    };
    window.addEventListener("focus", onFocus);
    window.addEventListener(NOTIFICATIONS_CHANGED_EVENT, onChanged);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", onFocus);
      window.removeEventListener(NOTIFICATIONS_CHANGED_EVENT, onChanged);
    };
  }, [workspaceId, refresh]);

  const badge = unreadBadgeText(unread);
  const label = unread > 0 ? `消息中心，${unread} 条未读` : "消息中心";

  return (
    <Link href="/notifications" className="topbar-tool-btn notif-bell" title={label} aria-label={label}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9" strokeLinecap="round" />
        <path d="M13.7 21a2 2 0 01-3.4 0" strokeLinecap="round" />
      </svg>
      {badge ? <span className="notif-bell-badge">{badge}</span> : null}
    </Link>
  );
}
