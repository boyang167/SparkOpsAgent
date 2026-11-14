"use client";

import { createElement, useEffect, useState } from "react";
import type { SharedItemVM } from "@sparkops/contracts/filesystem";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { FileDetailDrawer } from "./FileDetailDrawer";
import { ShareFileModal } from "./ShareFileModal";

export type SharedWithMePanelProps = {
  workspaceId: string;
  organizationId?: string;
  organizationLabel: string;
  workspaceLabel: string;
  source?: "organization" | "workspace" | "user" | "agent";
  items: SharedItemVM[];
  state: "loading" | "ready" | "empty" | "error" | "forbidden";
  members?: WorkspaceMemberVM[];
  memberDirectoryAvailable?: boolean;
};

const groups = [
  { id: "ORGANIZATION", label: "租户发布" },
  { id: "WORKSPACE", label: "工作区共享" },
  { id: "USER", label: "其他用户共享" },
  { id: "AGENT", label: "Agent / Workflow 产物" },
  { id: "OTHER", label: "其他授权来源" },
] as const;

type SourceFilter = SharedWithMePanelProps["source"];
type GroupID = (typeof groups)[number]["id"];

const knownSourceOrder = ["ORGANIZATION", "WORKSPACE", "USER", "AGENT"] as const;

function groupFor(item: SharedItemVM, source?: SourceFilter): GroupID {
  const sourceTypes = new Set(item.sources.map((entry) => entry.principal_type));
  const filteredType = source?.toUpperCase();
  if (
    filteredType &&
    knownSourceOrder.includes(filteredType as (typeof knownSourceOrder)[number]) &&
    sourceTypes.has(filteredType)
  ) {
    return filteredType as GroupID;
  }
  return knownSourceOrder.find((type) => sourceTypes.has(type)) ?? "OTHER";
}

function sourceLabel(
  item: SharedItemVM,
  workspaceId: string,
  organizationId: string | undefined,
  organizationLabel: string,
  workspaceLabel: string,
  source?: SourceFilter,
) {
  switch (groupFor(item, source)) {
    case "ORGANIZATION":
      return organizationId === item.source_namespace.organization_id
        ? organizationLabel
        : "组织空间（身份信息不可用）";
    case "WORKSPACE":
      return workspaceId === item.source_namespace.workspace_id
        ? workspaceLabel
        : "工作区空间（身份信息不可用）";
    case "USER":
      return "其他用户";
    case "AGENT":
      return "Agent / Workflow";
    default:
      return "其他授权主体";
  }
}

function sourceTypesLabel(item: SharedItemVM) {
  const labels: Record<string, string> = {
    ORGANIZATION: "租户",
    WORKSPACE: "工作区",
    USER: "其他用户",
    AGENT: "Agent / Workflow",
    ROLE: "角色",
    SERVICE_ACCOUNT: "服务账号",
  };
  const distinct = [...new Set(item.sources.map((entry) => entry.principal_type))];
  distinct.sort((left, right) => {
    const leftIndex = knownSourceOrder.indexOf(left as (typeof knownSourceOrder)[number]);
    const rightIndex = knownSourceOrder.indexOf(right as (typeof knownSourceOrder)[number]);
    const leftRank = leftIndex < 0 ? knownSourceOrder.length : leftIndex;
    const rightRank = rightIndex < 0 ? knownSourceOrder.length : rightIndex;
    return leftRank - rightRank || left.localeCompare(right);
  });
  return distinct.length > 0
    ? distinct.map((type) => labels[type] ?? `其他（${type}）`).join("、")
    : "未提供";
}

function hasExplicitShareSource(item: SharedItemVM) {
  return item.sources.some(
    (source) => source.effect === "ALLOW" && source.permission === "SHARE",
  );
}

function ownerAndSharerLabel(
  item: SharedItemVM,
  workspaceId: string,
  organizationId: string | undefined,
  organizationLabel: string,
  workspaceLabel: string,
) {
  const owner =
    item.source_namespace.namespace_type === "ORGANIZATION"
      ? organizationId === item.source_namespace.organization_id
        ? organizationLabel
        : "组织空间（身份信息不可用）"
      : item.source_namespace.namespace_type === "WORKSPACE"
        ? workspaceId === item.source_namespace.workspace_id
          ? workspaceLabel
          : "工作区空间（身份信息不可用）"
        : item.source_namespace.owner_user_id
          ? "个人空间所有者（身份信息不可用）"
          : "所有者信息不可用";
  return `${owner} · 共享者信息不可用`;
}

function expiry(item: SharedItemVM):
  | { kind: "date"; value: Date }
  | { kind: "permanent" }
  | { kind: "unavailable" } {
  const values = item.sources.map((source) => source.expires_at);
  if (values.length === 0 || values.some((value) => value === undefined)) {
    return { kind: "unavailable" };
  }
  if (values.some((value) => value === null)) return { kind: "permanent" };
  const dates = values.map((value) => new Date(value as string));
  if (dates.some((date) => Number.isNaN(date.getTime()))) {
    return { kind: "unavailable" };
  }
  return { kind: "date", value: dates.reduce((earliest, date) =>
    date.getTime() < earliest.getTime() ? date : earliest,
  ) };
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
  organizationId,
  organizationLabel,
  workspaceLabel,
  source,
  items,
  state,
  members = [],
  memberDirectoryAvailable = false,
}: SharedWithMePanelProps) {
  const [selection, setSelection] = useState<{
    scopeKey: string;
    namespaceId: string;
    nodeId: string;
  } | null>(null);
  const scopeKey = `${workspaceId}:${source ?? "all"}`;
  const [shareSelection, setShareSelection] = useState<SharedItemVM | null>(null);
  const selected =
    selection?.scopeKey === scopeKey
      ? items.find(
        (item) =>
          item.node.id === selection.nodeId &&
          item.node.namespace_id === selection.namespaceId,
      )
      : undefined;

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
      const groupItems = items.filter((item) => groupFor(item, source) === group.id);
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
                    onClick: () => setSelection({
                      scopeKey,
                      namespaceId: item.node.namespace_id,
                      nodeId: item.node.id,
                    }),
                  },
                  "查看详情",
                ),
                item.effective_actions.includes("SHARE") && hasExplicitShareSource(item)
                  ? createElement(
                      "button",
                      {
                        type: "button",
                        className: "btn btn-ghost btn-sm filesystem-touch-target",
                        "aria-label": `共享 ${item.node.name}`,
                        onClick: () => {
                          if (
                            item.effective_actions.includes("SHARE") &&
                            hasExplicitShareSource(item)
                          ) {
                            setShareSelection(item);
                          }
                        },
                      },
                      "共享",
                    )
                  : null,
              ),
              createElement(
                "dl",
                { className: "filesystem-shared-metadata" },
                createElement(Metadata, {
                  label: "来源",
                  value: sourceLabel(
                    item,
                    workspaceId,
                    organizationId,
                    organizationLabel,
                    workspaceLabel,
                    source,
                  ),
                }),
                createElement(Metadata, {
                  label: "授权来源类型",
                  value: sourceTypesLabel(item),
                }),
                createElement(Metadata, {
                  label: "所有者 / 分享者",
                  value: ownerAndSharerLabel(
                    item,
                    workspaceId,
                    organizationId,
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
              value: sourceLabel(
                selected,
                workspaceId,
                organizationId,
                organizationLabel,
                workspaceLabel,
                source,
              ),
            },
            {
              label: "授权来源类型",
              value: sourceTypesLabel(selected),
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
    shareSelection && organizationId
      ? createElement(ShareFileModal, {
          open: true,
          node: shareSelection.node,
          organizationId,
          organizationLabel,
          workspaceId,
          workspaceLabel,
          members,
          memberDirectoryAvailable,
          effectiveActions: shareSelection.effective_actions,
          reshareSourceGrantIds: shareSelection.sources
            .filter((source) => source.effect === "ALLOW" && source.permission === "SHARE")
            .map((source) => source.grant_id),
          onClose: () => setShareSelection(null),
        })
      : null,
  );
}

function ExpiryMetadata({ item }: { item: SharedItemVM }) {
  const result = expiry(item);
  return createElement(
    "div",
    { className: "stack-xs min-w-0" },
    createElement("dt", { className: "text-xs text-muted" }, "到期"),
    createElement(
      "dd",
      { className: "text-sm break-words" },
      result.kind === "date"
        ? createElement(
            "time",
            { dateTime: result.value.toISOString() },
            formatUTC(result.value),
          )
        : result.kind === "permanent"
          ? "永久有效"
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
