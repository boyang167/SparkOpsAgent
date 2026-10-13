"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { notification } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { NotificationVM } from "@sparkops/contracts/notification";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  groupNotificationsByDay,
  notificationKind,
  notificationTimeLabel,
  parseNotificationBody,
  type NotificationKind,
} from "./notification-view";

interface NotificationPanelProps {
  workspaceId: string;
  items: NotificationVM[];
  unreadCount: number;
}

type Filter = "all" | "unread";

function KindIcon({ kind }: { kind: NotificationKind }) {
  const common = {
    width: 16,
    height: 16,
    viewBox: "0 0 16 16",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.4,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
  switch (kind) {
    case "build":
      return (
        <svg {...common}>
          <path d="M8 1.75 13.5 4.75v6.5L8 14.25 2.5 11.25v-6.5L8 1.75Z" />
          <path d="M2.5 4.75 8 7.75l5.5-3M8 7.75v6.5" />
        </svg>
      );
    case "workflow":
      return (
        <svg {...common}>
          <circle cx="3.5" cy="8" r="1.75" />
          <circle cx="12.5" cy="3.75" r="1.75" />
          <circle cx="12.5" cy="12.25" r="1.75" />
          <path d="M5.25 8h2.5c1.2 0 1.5-4.25 3-4.25M7.75 8c1.2 0 1.5 4.25 3 4.25" />
        </svg>
      );
    case "agent":
      return (
        <svg {...common}>
          <path d="M8 1.75 9.4 6.6 14.25 8 9.4 9.4 8 14.25 6.6 9.4 1.75 8 6.6 6.6 8 1.75Z" />
        </svg>
      );
    default:
      return (
        <svg {...common}>
          <path d="M4 6.5a4 4 0 0 1 8 0c0 3.5 1.5 4.5 1.5 4.5h-11S4 10 4 6.5ZM6.75 13.5a1.4 1.4 0 0 0 2.5 0" />
        </svg>
      );
  }
}

export function NotificationPanel({ workspaceId, items, unreadCount }: NotificationPanelProps) {
  const router = useRouter();
  const [filter, setFilter] = useState<Filter>("all");
  const [readIds, setReadIds] = useState<Set<string>>(() => new Set());
  const [allRead, setAllRead] = useState(false);
  const [loadingAll, setLoadingAll] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isRead = (item: NotificationVM) => allRead || item.read || readIds.has(item.id);
  const localUnread = allRead ? 0 : items.filter((item) => !item.read && !readIds.has(item.id)).length;
  const displayUnread = allRead
    ? 0
    : Math.max(0, unreadCount - (items.filter((item) => !item.read).length - localUnread));

  const visible = useMemo(
    () => (filter === "unread" ? items.filter((item) => !(allRead || item.read || readIds.has(item.id))) : items),
    [filter, items, allRead, readIds],
  );
  const groups = useMemo(() => groupNotificationsByDay(visible), [visible]);

  async function onOpen(item: NotificationVM) {
    setError(null);
    if (!isRead(item)) {
      setReadIds((prev) => new Set(prev).add(item.id));
      try {
        await notification.markRead(item.id, workspaceId);
        router.refresh();
      } catch (err) {
        setReadIds((prev) => {
          const next = new Set(prev);
          next.delete(item.id);
          return next;
        });
        setError(err instanceof BffError ? err.message : "操作失败");
        return;
      }
    }
    if (item.link) router.push(item.link);
  }

  async function onMarkAllRead() {
    setLoadingAll(true);
    setError(null);
    try {
      await notification.markAllRead(workspaceId);
      setAllRead(true);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setLoadingAll(false);
    }
  }

  return (
    <div className="notif">
      <div className="notif-toolbar">
        <div className="segmented-control notif-filter" role="tablist" aria-label="筛选">
          <button
            type="button"
            role="tab"
            aria-selected={filter === "all"}
            className={`segmented-item${filter === "all" ? " segmented-item-active" : ""}`}
            onClick={() => setFilter("all")}
          >
            全部
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={filter === "unread"}
            className={`segmented-item${filter === "unread" ? " segmented-item-active" : ""}`}
            onClick={() => setFilter("unread")}
          >
            未读
            {displayUnread > 0 ? <span className="notif-filter-count">{displayUnread}</span> : null}
          </button>
        </div>
        {displayUnread > 0 ? (
          <button type="button" className="notif-mark-all" onClick={() => void onMarkAllRead()} disabled={loadingAll}>
            {loadingAll ? "处理中…" : "全部标为已读"}
          </button>
        ) : null}
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {visible.length === 0 ? (
        <div className="notif-empty">
          <p className="notif-empty-title">{filter === "unread" ? "没有未读通知" : "暂无通知"}</p>
          <p className="notif-empty-desc">构建、工作流或 Agent 运行完成后，会在这里提醒你。</p>
        </div>
      ) : (
        <div className="notif-groups">
          {groups.map((group) => (
            <section key={group.label} className="notif-group">
              <h2 className="notif-group-title">{group.label}</h2>
              <ul className="notif-list">
                {group.items.map((item) => {
                  const read = isRead(item);
                  const { status, text } = parseNotificationBody(item.body);
                  return (
                    <li key={item.id}>
                      <button
                        type="button"
                        className={`notif-item${read ? " is-read" : ""}`}
                        onClick={() => void onOpen(item)}
                      >
                        <span className={`notif-item-icon is-${status?.tone ?? "neutral"}`}>
                          <KindIcon kind={notificationKind(item.type)} />
                        </span>
                        <span className="notif-item-body">
                          <span className="notif-item-title">{item.title}</span>
                          {status || text ? (
                            <span className="notif-item-meta">
                              {status ? (
                                <span className={`notif-status is-${status.tone}`}>
                                  <span className="notif-status-dot" aria-hidden />
                                  {status.label}
                                </span>
                              ) : null}
                              {text ? <span className="notif-item-text">{text}</span> : null}
                            </span>
                          ) : null}
                        </span>
                        <span className="notif-item-side">
                          <time
                            className="notif-item-time"
                            dateTime={item.created_at}
                            suppressHydrationWarning
                            title={new Date(item.created_at).toLocaleString("zh-CN", { hour12: false })}
                          >
                            {notificationTimeLabel(item.created_at)}
                          </time>
                          {!read ? <span className="notif-unread-dot" aria-label="未读" /> : null}
                        </span>
                        {item.link ? (
                          <svg className="notif-item-chevron" width="14" height="14" viewBox="0 0 16 16" aria-hidden>
                            <path
                              d="m6 3.5 4.5 4.5L6 12.5"
                              fill="none"
                              stroke="currentColor"
                              strokeWidth="1.5"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                            />
                          </svg>
                        ) : null}
                      </button>
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
