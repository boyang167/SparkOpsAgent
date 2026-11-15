"use client";

import { createElement, useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import type {
  EffectiveAccessVM,
  FileAuditEventVM,
  FileNodeVM,
  FileRevisionVM,
  GovernedGrantVM,
  ShareRequestVM,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  listFileAuditEvents,
  listFileRevisions,
  getEffectiveAccess,
  listGovernedGrants,
  listShareRequests,
} from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";

export type FileDetailDrawerProps = {
  node: FileNodeVM | null;
  workspaceId: string;
  organizationId?: string;
  open: boolean;
  onClose: () => void;
  loadError?: "forbidden" | "not-found" | "error";
  onRetry?: () => void;
  projectionOnly?: boolean;
  projectionDetails?: Array<{ label: string; value: string }>;
};

type DetailTab = "details" | "access" | "revisions" | "audit";
type LoadState = "idle" | "loading" | "ready" | "error" | "forbidden";

export function FileDetailDrawer({
  node,
  workspaceId,
  organizationId,
  open,
  onClose,
  loadError,
  onRetry,
  projectionOnly = false,
  projectionDetails = [],
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
  const [accessRetry, setAccessRetry] = useState(0);
  const [accessData, setAccessData] = useState<{
    key: string;
    access?: EffectiveAccessVM;
    grants: GovernedGrantVM[];
    requests: ShareRequestVM[];
    grantCursor: string;
    requestCursor: string;
    metadataTruncated: boolean;
    state: LoadState;
    metadataRestricted: boolean;
    loadingMore: boolean;
    loadMoreError: boolean;
  }>({
    key: detailKey,
    grants: [],
    requests: [],
    grantCursor: "",
    requestCursor: "",
    metadataTruncated: false,
    state: "idle",
    metadataRestricted: false,
    loadingMore: false,
    loadMoreError: false,
  });
  const revisionGeneration = useRef(0);
  const auditGeneration = useRef(0);
  const accessGeneration = useRef(0);
  const accessMoreController = useRef<AbortController | null>(null);
  const revisions = revisionData.key === detailKey ? revisionData.items : [];
  const revisionState =
    revisionData.key === detailKey ? revisionData.state : open && node ? "loading" : "idle";
  const audits = auditData.key === detailKey ? auditData.items : [];
  const auditState =
    auditData.key === detailKey ? auditData.state : open && node ? "loading" : "idle";
  const scopedAccess = accessData.key === detailKey
    ? accessData
    : {
        key: detailKey,
        grants: [],
        requests: [],
        grantCursor: "",
        requestCursor: "",
        metadataTruncated: false,
        state: "loading" as const,
        metadataRestricted: false,
        loadingMore: false,
        loadMoreError: false,
      };

  useEffect(() => {
    const generation = ++revisionGeneration.current;
    if (!open || !node || projectionOnly) {
      setRevisionData({ key: detailKey, items: [], state: "idle" });
      return;
    }
    const controller = new AbortController();
    setRevisionData({ key: detailKey, items: [], state: "loading" });
    void listFileRevisions(
      workspaceId,
      node.id,
      { limit: 50, namespace_id: node.namespace_id },
      { signal: controller.signal },
    )
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
  }, [detailKey, node, open, projectionOnly, revisionRetry, workspaceId]);

  useEffect(() => {
    const generation = ++auditGeneration.current;
    if (!open || !node || projectionOnly) {
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
  }, [auditRetry, detailKey, node, open, projectionOnly, tab, workspaceId]);

  useEffect(() => {
    const generation = ++accessGeneration.current;
    accessMoreController.current?.abort();
    if (!open || !node || projectionOnly || tab !== "access") return;
    if (!organizationId) {
      setAccessData({
        key: detailKey,
        grants: [],
        requests: [],
        grantCursor: "",
        requestCursor: "",
        metadataTruncated: false,
        state: "error",
        metadataRestricted: false,
        loadingMore: false,
        loadMoreError: false,
      });
      return;
    }
    const controller = new AbortController();
    const key = detailKey;
    setAccessData({
      key,
      grants: [],
      requests: [],
      grantCursor: "",
      requestCursor: "",
      metadataTruncated: false,
      state: "loading",
      metadataRestricted: false,
      loadingMore: false,
      loadMoreError: false,
    });
    const options = { organizationId, workspaceId, signal: controller.signal };
    void Promise.allSettled([
      getEffectiveAccess(node.id, options),
      listGovernedGrants(node.id, options),
      listShareRequests(node.id, options),
    ]).then(([accessResult, grantsResult, requestsResult]) => {
      if (controller.signal.aborted || generation !== accessGeneration.current) return;
      if (accessResult.status === "rejected") {
        setAccessData({
          key,
          grants: [],
          requests: [],
          grantCursor: "",
          requestCursor: "",
          metadataTruncated: false,
          state: accessResult.reason instanceof BffError && accessResult.reason.status === 403
            ? "forbidden"
            : "error",
          metadataRestricted: false,
          loadingMore: false,
          loadMoreError: false,
        });
        return;
      }
      const metadataErrors = [grantsResult, requestsResult].filter(
        (result) => result.status === "rejected",
      );
      const metadataRestricted = metadataErrors.some(
        (result) =>
          result.status === "rejected" &&
          result.reason instanceof BffError &&
          result.reason.status === 403,
      );
      setAccessData({
        key,
        access: accessResult.value,
        grants: grantsResult.status === "fulfilled" ? grantsResult.value.items : [],
        requests: requestsResult.status === "fulfilled" ? requestsResult.value.items : [],
        grantCursor: grantsResult.status === "fulfilled" ? grantsResult.value.next_cursor : "",
        requestCursor: requestsResult.status === "fulfilled" ? requestsResult.value.next_cursor : "",
        metadataTruncated:
          (grantsResult.status === "fulfilled" && Boolean(grantsResult.value.next_cursor)) ||
          (requestsResult.status === "fulfilled" && Boolean(requestsResult.value.next_cursor)),
        state: metadataErrors.length > 0 && !metadataRestricted ? "error" : "ready",
        metadataRestricted,
        loadingMore: false,
        loadMoreError: false,
      });
    });
    return () => {
      controller.abort();
      accessMoreController.current?.abort();
    };
  }, [
    accessRetry,
    detailKey,
    node,
    open,
    organizationId,
    projectionOnly,
    tab,
    workspaceId,
  ]);

  function loadMoreGovernance() {
    if (
      !node ||
      !organizationId ||
      scopedAccess.loadingMore ||
      (!scopedAccess.grantCursor && !scopedAccess.requestCursor)
    ) return;
    const key = detailKey;
    const generation = accessGeneration.current;
    const grantCursor = scopedAccess.grantCursor;
    const requestCursor = scopedAccess.requestCursor;
    const controller = new AbortController();
    accessMoreController.current?.abort();
    accessMoreController.current = controller;
    setAccessData((current) =>
      current.key === key
        ? { ...current, loadingMore: true, loadMoreError: false }
        : current,
    );
    const options = { organizationId, workspaceId, signal: controller.signal };
    void Promise.all([
      grantCursor
        ? listGovernedGrants(node.id, options, { cursor: grantCursor })
        : Promise.resolve(null),
      requestCursor
        ? listShareRequests(node.id, options, { cursor: requestCursor })
        : Promise.resolve(null),
    ]).then(([grantPage, requestPage]) => {
      if (controller.signal.aborted || generation !== accessGeneration.current) return;
      setAccessData((current) => {
        if (current.key !== key) return current;
        const grants = grantPage
          ? appendUnique(current.grants, grantPage.items)
          : current.grants;
        const requests = requestPage
          ? appendUnique(current.requests, requestPage.items)
          : current.requests;
        const nextGrantCursor = grantPage?.next_cursor ?? "";
        const nextRequestCursor = requestPage?.next_cursor ?? "";
        return {
          ...current,
          grants,
          requests,
          grantCursor: nextGrantCursor,
          requestCursor: nextRequestCursor,
          metadataTruncated: Boolean(nextGrantCursor || nextRequestCursor),
          loadingMore: false,
          loadMoreError: false,
        };
      });
    }).catch(() => {
      if (controller.signal.aborted || generation !== accessGeneration.current) return;
      setAccessData((current) =>
        current.key === key
          ? { ...current, loadingMore: false, loadMoreError: true }
          : current,
      );
    });
  }

  useEffect(() => {
    if (!open) setTab("details");
  }, [open]);

  const tabDefinitions = [
    ["details", "详情"],
    ["access", "共享与权限"],
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
          className:
            tab === id
              ? "btn btn-secondary btn-sm filesystem-touch-target"
              : "btn btn-ghost btn-sm filesystem-touch-target",
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
      tabs: node && !projectionOnly ? tabs : undefined,
      panelClassName: "w-full max-w-xl",
      children: node
        ? createElement(
            "div",
            {
              id: projectionOnly ? undefined : panelID(tab),
              role: projectionOnly ? "region" : "tabpanel",
              "aria-label": projectionOnly ? "共享文件详情" : undefined,
              "aria-labelledby": projectionOnly ? undefined : tabID(tab),
            },
            projectionOnly
              ? createElement(ProjectionDetails, { node, details: projectionDetails })
              : tab === "details"
                ? createElement(Details, { node })
              : tab === "access"
                ? createElement(AccessDetails, {
                    data: scopedAccess,
                    onRetry: () => setAccessRetry((value) => value + 1),
                    onLoadMore: loadMoreGovernance,
                  })
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

function ProjectionDetails({
  node,
  details,
}: {
  node: FileNodeVM;
  details: Array<{ label: string; value: string }>;
}) {
  return createElement(
    "div",
    { className: "stack-lg" },
    createElement(
      "p",
      { className: "text-sm text-muted" },
      "此详情来自共享授权投影，不会请求源空间的工作区接口。",
    ),
    createElement(
      "dl",
      { className: "grid gap-4 sm:grid-cols-2" },
      createElement(Detail, {
        label: "类型",
        value: node.type === "DIRECTORY" ? "目录" : "文件",
      }),
      createElement(Detail, { label: "状态", value: node.state }),
      ...details.map((detail) =>
        createElement(Detail, {
          key: detail.label,
          label: detail.label,
          value: detail.value,
        }),
      ),
    ),
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

function AccessDetails({
  data,
  onRetry,
  onLoadMore,
}: {
  data: {
    access?: EffectiveAccessVM;
    grants: GovernedGrantVM[];
    requests: ShareRequestVM[];
    grantCursor: string;
    requestCursor: string;
    metadataTruncated: boolean;
    state: LoadState;
    metadataRestricted: boolean;
    loadingMore: boolean;
    loadMoreError: boolean;
  };
  onRetry: () => void;
  onLoadMore: () => void;
}) {
  if (data.state === "loading" || data.state === "idle") {
    return createElement("div", { role: "status", className: "empty-state" }, "正在加载共享与权限…");
  }
  if (data.state === "forbidden") {
    return createElement("div", { className: "empty-state" }, "无权查看有效权限。");
  }
  if (data.state === "error") {
    return createElement(
      InlineAlert,
      {
        tone: "error",
        children: createElement(
          "div",
          { className: "flex flex-wrap items-center gap-2" },
          createElement("span", { role: "alert" }, "加载共享与权限失败，请稍后重试。"),
          createElement(
            "button",
            { type: "button", className: "btn btn-ghost btn-sm", onClick: onRetry },
            "重试加载权限",
          ),
        ),
      },
    );
  }
  const pending = data.requests.filter((request) => request.status === "PENDING");
  return createElement(
    "div",
    { className: "stack-lg" },
    createElement(
      "section",
      { className: "stack-sm", "aria-labelledby": "effective-access-title" },
      createElement("h3", { id: "effective-access-title", className: "font-semibold" }, "我的有效权限"),
      createElement(
        "p",
        null,
        data.access?.effective_actions.length
          ? data.access.effective_actions.join("、")
          : "无有效操作权限",
      ),
      data.access?.decision === "DENY"
        ? createElement("p", { className: "text-sm text-muted" }, "当前访问结果：拒绝")
        : null,
    ),
    data.metadataRestricted
      ? createElement(
          "div",
          { className: "empty-state" },
          "治理元数据受限",
        )
      : createElement(
          "section",
          { className: "stack-sm", "aria-labelledby": "grant-list-title" },
          createElement("h3", { id: "grant-list-title", className: "font-semibold" }, "授权来源"),
          data.grants.length === 0
            ? createElement("div", { className: "empty-state" }, "暂无可查看的授权记录。")
            : data.grants.map((grant) =>
                createElement(
                  "article",
                  { key: grant.id, className: "card stack-xs" },
                  createElement(
                    "p",
                    { className: "font-medium" },
                    grant.effect === "DENY"
                      ? `显式拒绝 · ${grant.permission}`
                      : grant.permission,
                  ),
                  createElement(
                    "p",
                    { className: "text-sm text-muted" },
                    grant.source === "INHERITED" ? "继承授权" : "直接授权",
                  ),
                  createElement("p", { className: "text-sm" }, grant.principal_type),
                  createElement(
                    "p",
                    { className: "text-sm text-muted" },
                    grant.expires_at
                      ? new Date(grant.expires_at).toISOString().slice(0, 10)
                      : "永久有效",
                  ),
                ),
              ),
          pending.length > 0
            ? createElement(
                "div",
                { className: "stack-sm" },
                createElement("h3", { className: "font-semibold" }, "等待审批"),
                ...pending.map((request) =>
                  createElement(
                    "p",
                    { key: request.id, className: "text-sm text-muted" },
                    `${request.principal_type} · ${request.actions.join("、")}`,
                  ),
                ),
              )
            : null,
          data.metadataTruncated
            ? createElement(
                "button",
                {
                  type: "button",
                  className: "btn btn-secondary btn-sm self-start",
                  disabled: data.loadingMore,
                  onClick: onLoadMore,
                },
                data.loadingMore ? "正在加载…" : "加载更多",
              )
            : null,
          data.loadMoreError
            ? createElement(
                InlineAlert,
                {
                  tone: "error",
                  children: createElement(
                    "div",
                    { className: "flex flex-wrap items-center gap-2" },
                    createElement("span", { role: "alert" }, "加载更多治理记录失败。"),
                    createElement(
                      "button",
                      { type: "button", className: "btn btn-ghost btn-sm", onClick: onLoadMore },
                      "重试",
                    ),
                  ),
                },
              )
            : null,
        ),
  );
}

function appendUnique<T extends { id: string }>(current: T[], incoming: T[]): T[] {
  const ids = new Set(current.map((item) => item.id));
  return current.concat(incoming.filter((item) => {
    if (ids.has(item.id)) return false;
    ids.add(item.id);
    return true;
  }));
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
