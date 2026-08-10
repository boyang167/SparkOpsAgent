"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { notification } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { NotificationVM } from "@sparkops/contracts/notification";

interface NotificationPanelProps {
  workspaceId: string;
  items: NotificationVM[];
  unreadCount: number;
}

function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString("zh-CN");
  } catch {
    return iso;
  }
}

export function NotificationPanel({ workspaceId, items, unreadCount }: NotificationPanelProps) {
  const router = useRouter();
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [loadingAll, setLoadingAll] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onMarkRead(item: NotificationVM) {
    if (item.read) {
      if (item.link) {
        router.push(item.link);
      }
      return;
    }
    setLoadingId(item.id);
    setError(null);
    try {
      await notification.markRead(item.id, workspaceId);
      router.refresh();
      if (item.link) {
        router.push(item.link);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onMarkAllRead() {
    setLoadingAll(true);
    setError(null);
    try {
      await notification.markAllRead(workspaceId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setLoadingAll(false);
    }
  }

  return (
    <div className="stack-md">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted">
          {unreadCount > 0 ? `${unreadCount} 条未读` : "全部已读"}
        </p>
        {unreadCount > 0 ? (
          <button className="btn-secondary" type="button" onClick={onMarkAllRead} disabled={loadingAll}>
            {loadingAll ? "处理中..." : "全部标为已读"}
          </button>
        ) : null}
      </div>

      {error ? <p className="error">{error}</p> : null}

      {items.length === 0 ? (
        <div className="empty-state">暂无站内信。完成 Agent 对话后会收到通知。</div>
      ) : (
        <div className="stack-md">
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`card text-left w-full transition-opacity ${
                item.read ? "opacity-80" : "border-[var(--primary)]/30"
              }`}
              onClick={() => onMarkRead(item)}
              disabled={loadingId === item.id}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="font-semibold">{item.title}</p>
                  <p className="text-muted mt-1 text-sm">{item.body}</p>
                  <p className="text-xs text-muted mt-2">{formatTime(item.created_at)}</p>
                </div>
                {!item.read ? (
                  <span className="badge badge-primary shrink-0">未读</span>
                ) : (
                  <span className="badge badge-neutral shrink-0">已读</span>
                )}
              </div>
              {item.link ? (
                <p className="text-xs text-[var(--primary)] mt-3 truncate">点击查看详情</p>
              ) : null}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
