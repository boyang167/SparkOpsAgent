"use client";

import { createElement, useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
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
  node: FileNodeVM | null;
  workspaceId: string;
  open: boolean;
  onClose: () => void;
  loadError?: "forbidden" | "not-found" | "error";
  onRetry?: () => void;
};

type DetailTab = "details" | "revisions" | "audit";
type LoadState = "idle" | "loading" | "ready" | "error" | "forbidden";

export function FileDetailDrawer({
  node,
  workspaceId,
  open,
  onClose,
  loadError,
  onRetry,
}: FileDetailDrawerProps): React.ReactElement | null {
  const [tab, setTab] = useState<DetailTab>("details");
  const detailTabsId = useId();
  const detailKey = `${workspaceId}:${node?.id ?? ""}`;
  const [revisionData, setRevisionData] = useState<{
    key: string;
    items: FileRevisionVM[];
    state: LoadState;
  }>({ key: detailKey, items: [], state: "idle" });
  const [auditData, setAuditData] = useState<{
    key: string;
    items: FileAuditEventVM[];
    state: LoadState;
  }>({ key: detailKey, items: [], state: "idle" });
  const [revisionRetry, setRevisionRetry] = useState(0);
  const [auditRetry, setAuditRetry] = useState(0);
  const revisionGeneration = useRef(0);
  const auditGeneration = useRef(0);
  const revisions = revisionData.key === detailKey ? revisionData.items : [];
  const revisionState =
    revisionData.key === detailKey ? revisionData.state : open && node ? "loading" : "idle";
  const audits = auditData.key === detailKey ? auditData.items : [];
  const auditState =
    auditData.key === detailKey ? auditData.state : open && node ? "loading" : "idle";

  useEffect(() => {
    const generation = ++revisionGeneration.current;
    if (!open || !node) {
      setRevisionData({ key: detailKey, items: [], state: "idle" });
      return;
    }
    const controller = new AbortController();
    setRevisionData({ key: detailKey, items: [], state: "loading" });
    void listFileRevisions(workspaceId, node.id, { limit: 50 }, { signal: controller.signal })
      .then((page) => {
        if (controller.signal.aborted || generation !== revisionGeneration.current) return;
        setRevisionData({ key: detailKey, items: page.items, state: "ready" });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || generation !== revisionGeneration.current) return;
        setRevisionData({
          key: detailKey,
          items: [],
          state: error instanceof BffError && error.status === 403 ? "forbidden" : "error",
        });
      });
    return () => controller.abort();
  }, [detailKey, node, open, revisionRetry, workspaceId]);

  useEffect(() => {
    const generation = ++auditGeneration.current;
    if (!open || !node) {
      setAuditData({ key: detailKey, items: [], state: "idle" });
      return;
    }
    if (tab !== "audit") return;
    const controller = new AbortController();
    setAuditData({ key: detailKey, items: [], state: "loading" });
    void listFileAuditEvents(
      workspaceId,
      { resource_id: node.id, limit: 20 },
      { signal: controller.signal },
    )
      .then((page) => {
        if (controller.signal.aborted || generation !== auditGeneration.current) return;
        setAuditData({ key: detailKey, items: page.items, state: "ready" });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || generation !== auditGeneration.current) return;
        setAuditData({
          key: detailKey,
          items: [],
          state: error instanceof BffError && error.status === 403 ? "forbidden" : "error",
        });
      });
    return () => controller.abort();
  }, [auditRetry, detailKey, node, open, tab, workspaceId]);

  useEffect(() => {
    if (!open) setTab("details");
  }, [open]);

  const tabDefinitions = [
    ["details", "详情"],
    ["revisions", "版本"],
    ["audit", "审计"],
  ] as const;

  function tabID(id: DetailTab) {
    return `${detailTabsId}-tab-${id}`;
  }

  function panelID(id: DetailTab) {
    return `${detailTabsId}-panel-${id}`;
  }

  function onTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    const offset = event.key === "ArrowRight" ? 1 : -1;
    const next = tabDefinitions[(index + offset + tabDefinitions.length) % tabDefinitions.length];
    if (!next) return;
    setTab(next[0]);
    document.getElementById(tabID(next[0]))?.focus();
  }

  const tabs = createElement(
    "div",
    { role: "tablist", "aria-label": "文件详情" },
    tabDefinitions.map(([id, label], index) =>
      createElement(
        "button",
        {
          key: id,
          id: tabID(id),
          type: "button",
          role: "tab",
          "aria-selected": tab === id,
          "aria-controls": panelID(id),
          tabIndex: tab === id ? 0 : -1,
          className: tab === id ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
          onClick: () => setTab(id),
          onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) =>
            onTabKeyDown(event, index),
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
      title: node?.name ?? "文件详情",
      description: node?.type === "DIRECTORY" ? "目录详情" : "文件详情",
      tabs: node ? tabs : undefined,
      panelClassName: "w-full max-w-xl",
      children: node
        ? createElement(
            "div",
            {
              id: panelID(tab),
              role: "tabpanel",
              "aria-labelledby": tabID(tab),
            },
            tab === "details"
              ? createElement(Details, { node })
              : tab === "revisions"
                ? createElement(RevisionList, {
                    state: revisionState,
                    revisions,
                    onRetry: () => setRevisionRetry((value) => value + 1),
                  })
                : createElement(AuditList, {
                    state: auditState,
                    items: audits,
                    onRetry: () => setAuditRetry((value) => value + 1),
                  }),
          )
        : createElement(SelectionLoadState, { error: loadError, onRetry }),
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
  onRetry,
}: {
  state: LoadState;
  revisions: FileRevisionVM[];
  onRetry: () => void;
}) {
  if (state === "loading" || state === "idle") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载版本…");
  }
  if (state === "forbidden") return createElement("div", { className: "empty-state" }, "无权查看该文件的版本。");
  if (state === "error") {
    return createElement(
      InlineAlert,
      {
        tone: "error",
        children: createElement(
          "div",
          { className: "flex flex-wrap items-center gap-2" },
          createElement("span", { role: "alert" }, "加载版本失败，请稍后重试。"),
          createElement(
            "button",
            { type: "button", className: "btn btn-ghost btn-sm", onClick: onRetry },
            "重试加载版本",
          ),
        ),
      },
    );
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

function AuditList({
  state,
  items,
  onRetry,
}: {
  state: LoadState;
  items: FileAuditEventVM[];
  onRetry: () => void;
}) {
  if (state === "loading" || state === "idle") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载文件审计…");
  }
  if (state === "forbidden") return createElement("div", { className: "empty-state" }, "无权查看该文件的审计。");
  if (state === "error") {
    return createElement(
      InlineAlert,
      {
        tone: "error",
        children: createElement(
          "div",
          { className: "flex flex-wrap items-center gap-2" },
          createElement("span", { role: "alert" }, "加载审计失败，请稍后重试。"),
          createElement(
            "button",
            { type: "button", className: "btn btn-ghost btn-sm", onClick: onRetry },
            "重试加载审计",
          ),
        ),
      },
    );
  }
  if (items.length === 0) return createElement("div", { className: "empty-state" }, "暂无相关审计记录。");
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

function SelectionLoadState({
  error,
  onRetry,
}: {
  error?: "forbidden" | "not-found" | "error";
  onRetry?: () => void;
}) {
  const message =
    error === "forbidden"
      ? "无权查看该文件。"
      : error === "not-found"
        ? "文件不存在或已被删除。"
        : "加载文件详情失败，请稍后重试。";
  return createElement(
    "div",
    { className: "empty-state", role: error === "error" ? "alert" : undefined },
    createElement("p", null, message),
    onRetry
      ? createElement(
          "button",
          { type: "button", className: "btn btn-secondary btn-sm", onClick: onRetry },
          "重试",
        )
      : null,
  );
}
