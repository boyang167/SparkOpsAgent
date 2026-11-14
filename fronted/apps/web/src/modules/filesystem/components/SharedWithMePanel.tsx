"use client";

import { createElement, useEffect, useState } from "react";
import type { SharedItemVM } from "@sparkops/contracts/filesystem";
import { FileDetailDrawer } from "./FileDetailDrawer";

export type SharedWithMePanelProps = {
  workspaceId: string;
  organizationLabel: string;
  workspaceLabel: string;
  items: SharedItemVM[];
  state: "loading" | "ready" | "empty" | "error" | "forbidden";
};

const groups = [
  { id: "ORGANIZATION", label: "租户发布" },
  { id: "USER", label: "其他用户共享" },
  { id: "WORKSPACE", label: "工作区共享" },
  { id: "AGENT", label: "Agent / Workflow 产物" },
  { id: "OTHER", label: "其他授权来源" },
] as const;

function groupFor(item: SharedItemVM) {
  const principalType = item.sources[0]?.principal_type;
  return groups.some((group) => group.id === principalType)
    ? principalType
    : "OTHER";
}

function sourceLabel(
  item: SharedItemVM,
  organizationLabel: string,
  workspaceLabel: string,
) {
  switch (item.sources[0]?.principal_type) {
    case "ORGANIZATION":
      return organizationLabel;
    case "WORKSPACE":
      return workspaceLabel;
    case "USER":
      return "其他用户";
    case "AGENT":
      return "Agent / Workflow";
    default:
      return "其他授权主体";
  }
}

function ownerAndSharerLabel(
  item: SharedItemVM,
  organizationLabel: string,
  workspaceLabel: string,
) {
  const owner =
    item.source_namespace.namespace_type === "ORGANIZATION"
      ? organizationLabel
      : item.source_namespace.namespace_type === "WORKSPACE"
        ? workspaceLabel
        : item.source_namespace.owner_user_id
          ? "个人空间所有者（身份信息不可用）"
          : "所有者信息不可用";
  return `${owner} · 共享者信息不可用`;
}

function expiry(item: SharedItemVM) {
  const values = item.sources.map((source) => source.expires_at);
  if (values.length === 0 || values.some((value) => value == null)) return null;
  const dates = values.map((value) => new Date(value!));
  if (dates.some((date) => Number.isNaN(date.getTime()))) return null;
  return dates.reduce((earliest, date) =>
    date.getTime() < earliest.getTime() ? date : earliest,
  );
}

function formatUTC(date: Date) {
  const pad = (value: number) => String(value).padStart(2, "0");
  return [
    `${date.getUTCFullYear()}/${pad(date.getUTCMonth() + 1)}/${pad(date.getUTCDate())}`,
    `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`,
  ].join(" ");
}

export function SharedWithMePanel({
  workspaceId,
  organizationLabel,
  workspaceLabel,
  items,
  state,
}: SharedWithMePanelProps) {
  const [selection, setSelection] = useState<{
    scopeKey: string;
    item: SharedItemVM;
  } | null>(null);
  const selected =
    selection?.scopeKey === workspaceId &&
    items.some(
      (item) =>
        item.node.id === selection.item.node.id &&
        item.node.namespace_id === selection.item.node.namespace_id,
    )
      ? selection.item
      : null;

  useEffect(() => {
    if (selection && !selected) setSelection(null);
  }, [selected, selection]);

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
                    item.original_path || "原路径未提供",
                  ),
                ),
                createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-secondary btn-sm filesystem-touch-target",
                    "aria-label": `打开 ${item.node.name}`,
                    onClick: () => setSelection({ scopeKey: workspaceId, item }),
                  },
                  "查看详情",
                ),
              ),
              createElement(
                "dl",
                { className: "filesystem-shared-metadata" },
                createElement(Metadata, {
                  label: "来源",
                  value: sourceLabel(item, organizationLabel, workspaceLabel),
                }),
                createElement(Metadata, {
                  label: "所有者 / 分享者",
                  value: ownerAndSharerLabel(
                    item,
                    organizationLabel,
                    workspaceLabel,
                  ),
                }),
                createElement(Metadata, {
                  label: "有效权限",
                  value: item.effective_actions.length > 0
                    ? item.effective_actions.join("、")
                    : "未提供",
                }),
                createElement(ExpiryMetadata, { item }),
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
            {
              label: "来源",
              value: sourceLabel(selected, organizationLabel, workspaceLabel),
            },
            {
              label: "有效权限",
              value: selected.effective_actions.length > 0
                ? selected.effective_actions.join("、")
                : "未提供",
            },
            { label: "原路径", value: selected.original_path },
          ],
          onClose: () => setSelection(null),
        })
      : null,
  );
}

function ExpiryMetadata({ item }: { item: SharedItemVM }) {
  const date = expiry(item);
  return createElement(
    "div",
    { className: "stack-xs min-w-0" },
    createElement("dt", { className: "text-xs text-muted" }, "到期"),
    createElement(
      "dd",
      { className: "text-sm break-words" },
      date
        ? createElement(
            "time",
            { dateTime: date.toISOString() },
            formatUTC(date),
          )
        : "到期信息不可用",
    ),
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
