"use client";

import { createElement, useEffect, useState, useSyncExternalStore } from "react";
import type { TransferSessionVM, TransferStatus as PersistedStatus } from "@sparkops/contracts/filesystem";
import { listTransferSessions } from "@sparkops/web-sdk/filesystem";
import type { TransferItem, TransferQueue, TransferStatus } from "./transfer-queue";

const statusLabels: Record<TransferStatus, string> = {
  queued: "等待上传",
  creating: "正在创建会话",
  uploading: "正在上传",
  verifying: "正在校验",
  completed: "已完成",
  failed: "上传失败",
  canceled: "已取消",
};

const persistedStatusLabels: Record<PersistedStatus, string> = {
  INITIATED: "等待上传",
  UPLOADING: "正在上传",
  PAUSED: "已暂停",
  VERIFYING: "正在校验",
  SCANNING: "正在扫描",
  COMMITTING: "正在提交",
  COMPLETED: "已完成",
  FAILED: "上传失败",
  REJECTED: "已拒绝",
  ABORTED: "已取消",
  EXPIRED: "已过期",
};

function useTransfers(queue: TransferQueue) {
  return useSyncExternalStore(queue.subscribe, queue.getSnapshot, queue.getSnapshot);
}

function percent(item: TransferItem) {
  if (item.progress.totalBytes === 0) {
    return item.status === "completed" ? 100 : 0;
  }
  const value = Math.min(
    100,
    Math.round((item.progress.uploadedBytes / item.progress.totalBytes) * 100),
  );
  return item.status === "verifying" ? Math.min(99, value) : value;
}

function statusLabel(item: TransferItem) {
  if (item.cancelRequested) return "正在取消";
  if (item.status === "failed" && item.error?.startsWith("取消失败：")) {
    return "取消失败";
  }
  return statusLabels[item.status];
}

function Progress({ item }: { item: TransferItem }) {
  const value = percent(item);
  return createElement(
    "div",
    {
      role: "progressbar",
      "aria-label": `${item.file.name} 上传进度`,
      "aria-valuemin": 0,
      "aria-valuemax": 100,
      "aria-valuenow": value,
      className: "h-1.5 overflow-hidden rounded-full bg-surface-muted",
    },
    createElement("div", {
      className: "h-full bg-primary transition-[width] duration-200",
      style: { width: `${value}%` },
    }),
  );
}

function PersistedTransferCard({ session }: { session: TransferSessionVM }) {
  const completed = session.status === "COMPLETED";
  return createElement(
    "article",
    { className: "card stack-sm" },
    createElement(
      "div",
      { className: "flex flex-wrap items-start justify-between gap-3" },
      createElement(
        "div",
        { className: "min-w-0" },
        createElement("h3", { className: "truncate font-medium" }, session.target_name),
        createElement(
          "p",
          { className: "text-sm text-muted" },
          persistedStatusLabels[session.status],
        ),
      ),
    ),
    createElement(
      "div",
      {
        role: "progressbar",
        "aria-label": `${session.target_name} 上传进度`,
        "aria-valuemin": 0,
        "aria-valuemax": 100,
        "aria-valuenow": completed ? 100 : 0,
        className: "h-1.5 overflow-hidden rounded-full bg-surface-muted",
      },
      createElement("div", {
        className: "h-full bg-primary transition-[width] duration-200",
        style: { width: completed ? "100%" : "0%" },
      }),
    ),
    createElement(
      "p",
      { className: "text-xs text-muted" },
      `${completed ? session.expected_size : 0} / ${session.expected_size} 字节`,
    ),
    session.error_code
      ? createElement(
          "p",
          { role: "alert", className: "text-sm text-danger" },
          session.error_code,
        )
      : null,
  );
}

export function TransferCenter({
  queue,
  workspaceId,
}: {
  queue: TransferQueue;
  workspaceId?: string;
}) {
  const items = useTransfers(queue);
  const [persisted, setPersisted] = useState<TransferSessionVM[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [loadState, setLoadState] = useState<"loading" | "ready" | "error">(
    workspaceId ? "loading" : "ready",
  );

  useEffect(() => {
    if (!workspaceId) return;
    let active = true;
    setLoadState("loading");
    void listTransferSessions(workspaceId, { limit: 100 })
      .then((page) => {
        if (!active) return;
        setPersisted(page.items);
        setNextCursor(page.next_cursor);
        setLoadState("ready");
      })
      .catch(() => {
        if (active) setLoadState("error");
      });
    return () => {
      active = false;
    };
  }, [workspaceId]);

  const localSessionIDs = new Set(
    items
      .map((item) => item.session?.session_id)
      .filter((id): id is string => Boolean(id)),
  );
  const visiblePersisted = persisted.filter(
    (session) => !localSessionIDs.has(session.session_id),
  );

  if (items.length === 0 && visiblePersisted.length === 0 && loadState === "ready") {
    return createElement(
      "div",
      { className: "empty-state stack-sm" },
      createElement("h2", { className: "section-title" }, "暂无传输任务"),
      createElement(
        "p",
        { className: "text-sm text-muted" },
        "上传文件后，可在这里查看进度并处理失败任务。",
      ),
    );
  }

  async function loadMore() {
    if (!workspaceId || !nextCursor) return;
    setLoadState("loading");
    try {
      const page = await listTransferSessions(workspaceId, {
        cursor: nextCursor,
        limit: 100,
      });
      setPersisted((current) => [...current, ...page.items]);
      setNextCursor(page.next_cursor);
      setLoadState("ready");
    } catch {
      setLoadState("error");
    }
  }

  return createElement(
    "section",
    { className: "stack-md", "aria-label": "传输任务" },
    createElement(
      "div",
      { className: "stack-sm" },
      createElement("h2", { className: "section-title" }, "传输"),
      createElement(
        "p",
        { className: "text-sm text-muted" },
        "任务会在后台继续运行，失败后可单独重试。",
      ),
    ),
    items.map((item) =>
      createElement(
        "article",
        { key: item.id, className: "card stack-sm" },
        createElement(
          "div",
          { className: "flex flex-wrap items-start justify-between gap-3" },
          createElement(
            "div",
            { className: "min-w-0" },
            createElement("h3", { className: "truncate font-medium" }, item.file.name),
            createElement(
              "p",
              { className: "text-sm text-muted" },
              statusLabel(item),
            ),
          ),
          createElement(
            "div",
            { className: "flex gap-2" },
            item.status === "failed"
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-secondary btn-sm",
                    "aria-label": `重试 ${item.file.name}`,
                    onClick: () => queue.retry(item.id),
                  },
                  "重试",
                )
              : null,
            !item.cancelRequested &&
            ["queued", "creating", "uploading", "verifying"].includes(item.status)
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-ghost btn-sm",
                    "aria-label": `取消 ${item.file.name}`,
                    onClick: () => queue.cancel(item.id),
                  },
                  "取消",
                )
              : null,
          ),
        ),
        createElement(Progress, { item }),
        createElement(
          "p",
          { className: "text-xs text-muted" },
          `${item.progress.uploadedBytes} / ${item.progress.totalBytes} 字节`,
        ),
        item.error
          ? createElement(
              "p",
              { role: "alert", className: "text-sm text-danger" },
              item.error,
            )
          : null,
      ),
    ),
    visiblePersisted.map((session) =>
      createElement(PersistedTransferCard, {
        key: session.session_id,
        session,
      }),
    ),
    loadState === "loading"
      ? createElement(
          "p",
          { role: "status", className: "text-sm text-muted" },
          "正在加载持久传输任务…",
        )
      : null,
    loadState === "error"
      ? createElement(
          "p",
          { role: "alert", className: "text-sm text-danger" },
          "无法加载持久传输任务，请稍后重试。",
        )
      : null,
    nextCursor && loadState !== "loading"
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-secondary btn-sm self-start",
            onClick: () => void loadMore(),
          },
          "加载更多",
        )
      : null,
  );
}

const activeStatuses = new Set<TransferStatus>([
  "queued",
  "creating",
  "uploading",
  "verifying",
]);

export function TransferSummary({
  queue,
  onViewAll,
}: {
  queue: TransferQueue;
  onViewAll: () => void;
}) {
  const active = useTransfers(queue)
    .filter((item) => activeStatuses.has(item.status))
    .slice(0, 3);
  if (active.length === 0) return null;

  return createElement(
    "section",
    { className: "card stack-sm", "aria-label": "正在传输" },
    active.map((item) =>
      createElement(
        "div",
        { key: item.id, className: "stack-xs" },
        createElement(
          "div",
          { className: "flex justify-between gap-3 text-sm" },
          createElement("span", { className: "truncate" }, item.file.name),
          createElement("span", { className: "shrink-0 text-muted" }, `${percent(item)}%`),
        ),
        createElement(Progress, { item }),
      ),
    ),
    createElement(
      "button",
      {
        type: "button",
        className: "btn btn-ghost btn-sm self-start",
        onClick: onViewAll,
      },
      "查看全部传输",
    ),
  );
}
