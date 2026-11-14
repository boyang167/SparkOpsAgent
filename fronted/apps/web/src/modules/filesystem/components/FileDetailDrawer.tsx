"use client";

import { createElement, useEffect, useState } from "react";
import type {
  FileAuditEventVM,
  FileNodeVM,
  FileRevisionVM,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  listFileAuditEvents,
  listFileRevisions,
} from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";

export type FileDetailDrawerProps = {
  node: FileNodeVM;
  workspaceId: string;
  open: boolean;
  onClose: () => void;
};

type DetailTab = "details" | "revisions" | "audit";
type LoadState = "idle" | "loading" | "ready" | "error" | "forbidden";

export function FileDetailDrawer({
  node,
  workspaceId,
  open,
  onClose,
}: FileDetailDrawerProps): React.ReactElement | null {
  const [tab, setTab] = useState<DetailTab>("details");
  const [revisions, setRevisions] = useState<FileRevisionVM[]>([]);
  const [revisionState, setRevisionState] = useState<LoadState>("idle");
  const [audits, setAudits] = useState<FileAuditEventVM[]>([]);
  const [auditState, setAuditState] = useState<LoadState>("idle");

  useEffect(() => {
    if (!open) {
      setTab("details");
      setRevisions([]);
      setRevisionState("idle");
      setAudits([]);
      setAuditState("idle");
      return;
    }
    const controller = new AbortController();
    setRevisionState("loading");
    void listFileRevisions(workspaceId, node.id, { limit: 50 }, { signal: controller.signal })
      .then((page) => {
        setRevisions(page.items);
        setRevisionState("ready");
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setRevisionState(
          error instanceof BffError && error.status === 403 ? "forbidden" : "error",
        );
      });
    return () => controller.abort();
  }, [node.id, open, workspaceId]);

  useEffect(() => {
    if (!open || tab !== "audit" || auditState !== "idle") return;
    const controller = new AbortController();
    setAuditState("loading");
    void listFileAuditEvents(
      workspaceId,
      { resource_id: node.id, limit: 20 },
      { signal: controller.signal },
    )
      .then((page) => {
        setAudits(page.items);
        setAuditState("ready");
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setAuditState(
          error instanceof BffError && error.status === 403 ? "forbidden" : "error",
        );
      });
    return () => controller.abort();
  }, [auditState, node.id, open, tab, workspaceId]);

  const tabs = createElement(
    "div",
    { role: "tablist", "aria-label": "文件详情" },
    ([
        ["details", "详情"],
        ["revisions", "版本"],
        ["audit", "审计"],
      ] as const).map(([id, label]) =>
      createElement(
        "button",
        {
          key: id,
          type: "button",
          role: "tab",
          "aria-selected": tab === id,
          className: tab === id ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
          onClick: () => setTab(id),
        },
        label,
      ),
    ),
  );

  return createElement(
    Drawer,
    {
      open,
      onClose,
      title: node.name,
      description: node.type === "DIRECTORY" ? "目录详情" : "文件详情",
      tabs,
      panelClassName: "w-full max-w-xl",
      children:
        tab === "details"
          ? createElement(Details, { node })
          : tab === "revisions"
            ? createElement(RevisionList, { state: revisionState, revisions })
            : createElement(AuditList, { state: auditState, items: audits }),
    },
  );
}

function Details({ node }: { node: FileNodeVM }) {
  return createElement(
    "div",
    { className: "stack-lg" },
    createElement(
      "dl",
      { className: "grid gap-4 sm:grid-cols-2" },
      createElement(Detail, { label: "类型", value: node.type === "DIRECTORY" ? "目录" : "文件" }),
      createElement(Detail, { label: "状态", value: node.state }),
      createElement(Detail, { label: "节点 ID", value: node.id, mono: true }),
      createElement(Detail, { label: "所有者", value: node.owner_id, mono: true }),
      createElement(Detail, { label: "当前版本 ID", value: node.current_revision_id ?? "—", mono: true }),
      createElement(Detail, { label: "父节点 ID", value: node.parent_id ?? "根目录", mono: true }),
    ),
    node.type === "FILE"
      ? createElement("div", { className: "empty-state" }, "当前后端未提供可直接展示的预览内容。")
      : null,
  );
}

function Detail({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return createElement(
    "div",
    { className: "stack-xs min-w-0" },
    createElement("dt", { className: "text-xs text-muted" }, label),
    createElement("dd", { className: mono ? "break-all font-mono text-xs" : "text-sm" }, value),
  );
}

function RevisionList({
  state,
  revisions,
}: {
  state: LoadState;
  revisions: FileRevisionVM[];
}) {
  if (state === "loading" || state === "idle") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载版本…");
  }
  if (state === "forbidden") return createElement("div", { className: "empty-state" }, "无权查看该文件的版本。");
  if (state === "error") {
    return createElement(InlineAlert, {
      tone: "error",
      children: createElement("span", { role: "alert" }, "加载版本失败，请稍后重试。"),
    });
  }
  if (revisions.length === 0) return createElement("div", { className: "empty-state" }, "暂无版本记录。");
  return createElement(
    "div",
    { className: "stack-sm" },
    revisions.map((revision) =>
      createElement(
        "article",
        { key: revision.id, className: "card stack-xs" },
        createElement("h3", { className: "font-medium" }, `版本 #${revision.revision_no}`),
        createElement("p", { className: "text-sm text-muted" }, `${revision.mime_type} · ${revision.size} B`),
        createElement("p", { className: "break-all font-mono text-xs text-muted" }, revision.id),
      ),
    ),
  );
}

function AuditList({ state, items }: { state: LoadState; items: FileAuditEventVM[] }) {
  if (state === "loading" || state === "idle") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载文件审计…");
  }
  if (state === "forbidden") return createElement("div", { className: "empty-state" }, "无权查看该文件的审计。");
  if (state === "error") {
    return createElement(InlineAlert, {
      tone: "error",
      children: "加载文件审计失败，请稍后重试。",
    });
  }
  if (items.length === 0) return createElement("div", { className: "empty-state" }, "暂无文件审计记录。");
  return createElement(
    "div",
    { className: "stack-sm" },
    items.map((item) =>
      createElement(
        "article",
        { key: item.id, className: "card stack-xs" },
        createElement("h3", { className: "font-medium" }, item.action),
        createElement("p", { className: "text-sm text-muted" }, `${item.result} · ${new Date(item.created_at).toLocaleString()}`),
        item.trace_id ? createElement("p", { className: "font-mono text-xs" }, item.trace_id) : null,
      ),
    ),
  );
}
