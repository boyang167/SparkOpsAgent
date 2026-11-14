"use client";

import { createElement, useSyncExternalStore } from "react";
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

function useTransfers(queue: TransferQueue) {
  return useSyncExternalStore(queue.subscribe, queue.getSnapshot, queue.getSnapshot);
}

function percent(item: TransferItem) {
  if (item.progress.totalBytes === 0) {
    return item.status === "completed" ? 100 : 0;
  }
  return Math.min(
    100,
    Math.round((item.progress.uploadedBytes / item.progress.totalBytes) * 100),
  );
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

export function TransferCenter({ queue }: { queue: TransferQueue }) {
  const items = useTransfers(queue);
  if (items.length === 0) {
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
              statusLabels[item.status],
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
