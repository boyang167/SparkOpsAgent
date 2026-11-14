"use client";

import { createElement, useState } from "react";
import type { SharedItemVM } from "@sparkops/contracts/filesystem";
import { FileDetailDrawer } from "./FileDetailDrawer";

export type SharedProjectionItem = SharedItemVM & {
  source_principal_type?:
    | "ORGANIZATION"
    | "USER"
    | "WORKSPACE"
    | "AGENT"
    | "SERVICE_ACCOUNT"
    | "ROLE"
    | string;
  source_label?: string | null;
  owner_label?: string | null;
  sharer_label?: string | null;
  expires_at?: string | null;
  original_path?: string | null;
};

export type SharedWithMePanelProps = {
  workspaceId: string;
  items: SharedProjectionItem[];
  state: "loading" | "ready" | "empty" | "error" | "forbidden";
};

const groups = [
  { id: "ORGANIZATION", label: "租户发布" },
  { id: "USER", label: "其他用户共享" },
  { id: "WORKSPACE", label: "工作区共享" },
  { id: "AGENT", label: "Agent / Workflow 产物" },
  { id: "OTHER", label: "其他授权来源" },
] as const;

function groupFor(item: SharedProjectionItem) {
  return groups.some((group) => group.id === item.source_principal_type)
    ? item.source_principal_type
    : "OTHER";
}

function safeLabel(value: string | null | undefined, fallback: string) {
  return value?.trim() || fallback;
}

export function SharedWithMePanel({
  workspaceId,
  items,
  state,
}: SharedWithMePanelProps) {
  const [selected, setSelected] = useState<SharedProjectionItem | null>(null);

  if (state === "loading") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载共享内容…");
  }
  if (state === "forbidden") {
    return createElement("div", { role: "alert", className: "empty-state" }, "无权读取与我共享。");
  }
  if (state === "error") {
    return createElement("div", { role: "alert", className: "empty-state" }, "共享内容加载失败，请稍后重试。");
  }
  if (state === "empty" || items.length === 0) {
    return createElement("div", { role: "status", className: "empty-state" }, "当前没有与你共享的文件。");
  }

  return createElement(
    "section",
    {
      "aria-label": "与我共享",
      className: "filesystem-shared-panel stack-lg",
    },
    createElement(
      "header",
      { className: "stack-xs" },
      createElement("h2", { className: "section-title" }, "共享入口"),
      createElement(
        "p",
        { className: "text-sm text-muted" },
        "这里展示授权投影，文件仍保留在原空间中。",
      ),
    ),
    groups.map((group) => {
      const groupItems = items.filter((item) => groupFor(item) === group.id);
      if (groupItems.length === 0) return null;
      return createElement(
        "section",
        {
          key: group.id,
          "aria-labelledby": `shared-group-${group.id.toLowerCase()}`,
          className: "stack-sm",
        },
        createElement(
          "h3",
          {
            id: `shared-group-${group.id.toLowerCase()}`,
            className: "font-semibold",
          },
          group.label,
        ),
        createElement(
          "ul",
          { className: "filesystem-shared-list", role: "list" },
          groupItems.map((item) =>
            createElement(
              "li",
              { key: `${item.node.namespace_id}:${item.node.id}`, className: "card stack-sm" },
              createElement(
                "div",
                { className: "flex min-w-0 flex-wrap items-start justify-between gap-3" },
                createElement(
                  "div",
                  { className: "stack-xs min-w-0" },
                  createElement("h4", { className: "font-medium break-words" }, item.node.name),
                  createElement(
                    "p",
                    { className: "text-sm text-muted break-words" },
                    safeLabel(item.original_path, "原路径未提供"),
                  ),
                ),
                createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-secondary btn-sm filesystem-touch-target",
                    "aria-label": `打开 ${item.node.name}`,
                    onClick: () => setSelected(item),
                  },
                  "查看详情",
                ),
              ),
              createElement(
                "dl",
                { className: "filesystem-shared-metadata" },
                createElement(Metadata, {
                  label: "来源",
                  value: safeLabel(item.source_label, "授权来源未提供"),
                }),
                createElement(Metadata, {
                  label: "所有者 / 分享者",
                  value: `${safeLabel(item.owner_label, "所有者未提供")} · ${safeLabel(item.sharer_label, "分享者未提供")}`,
                }),
                createElement(Metadata, {
                  label: "有效权限",
                  value: item.effective_actions.length > 0
                    ? item.effective_actions.join("、")
                    : "未提供",
                }),
                createElement(Metadata, {
                  label: "到期",
                  value: item.expires_at
                    ? new Date(item.expires_at).toLocaleString("zh-CN")
                    : "长期有效或未提供",
                }),
              ),
            ),
          ),
        ),
      );
    }),
    selected
      ? createElement(FileDetailDrawer, {
          open: true,
          node: selected.node,
          workspaceId,
          projectionOnly: true,
          projectionDetails: [
            { label: "来源", value: safeLabel(selected.source_label, "授权来源未提供") },
            {
              label: "有效权限",
              value: selected.effective_actions.length > 0
                ? selected.effective_actions.join("、")
                : "未提供",
            },
            { label: "原路径", value: safeLabel(selected.original_path, "原路径未提供") },
          ],
          onClose: () => setSelected(null),
        })
      : null,
  );
}

function Metadata({ label, value }: { label: string; value: string }) {
  return createElement(
    "div",
    { className: "stack-xs min-w-0" },
    createElement("dt", { className: "text-xs text-muted" }, label),
    createElement("dd", { className: "text-sm break-words" }, value),
  );
}
