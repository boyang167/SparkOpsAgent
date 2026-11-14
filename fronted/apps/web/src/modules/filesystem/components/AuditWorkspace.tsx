"use client";

import { createElement, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import type { FileAuditEventVM, FileAuditQuery } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { listFileAuditEvents } from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";

export type AuditWorkspaceProps = {
  workspaceId: string;
  permissions: string[];
};

const PAGE_SIZE = 20;
type AuditLoadState = "loading" | "ready" | "error" | "forbidden";

function queryFromURL(params: URLSearchParams): FileAuditQuery {
  return {
    actor_id: params.get("actor_id") || undefined,
    action: params.get("action") || undefined,
    resource_id: params.get("resource_id") || undefined,
    result: params.get("result") || undefined,
    from: params.get("from") || undefined,
    to: params.get("to") || undefined,
    cursor: params.get("audit_cursor") || undefined,
    limit: PAGE_SIZE,
  };
}

export function AuditWorkspace({ workspaceId, permissions }: AuditWorkspaceProps) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlSignature = searchParams.toString();
  const initialQuery = useMemo(
    () => queryFromURL(new URLSearchParams(urlSignature)),
    [urlSignature],
  );
  const [draftState, setDraftState] = useState({
    source: urlSignature,
    value: initialQuery,
  });
  const [queryState, setQueryState] = useState({
    source: urlSignature,
    value: initialQuery,
  });
  const draft = draftState.source === urlSignature ? draftState.value : initialQuery;
  const query = queryState.source === urlSignature ? queryState.value : initialQuery;
  const requestScope = JSON.stringify([
    workspaceId,
    query.actor_id ?? "",
    query.action ?? "",
    query.resource_id ?? "",
    query.result ?? "",
    query.from ?? "",
    query.to ?? "",
    query.cursor ?? "",
  ]);
  const [viewState, setViewState] = useState<{
    scope: string;
    items: FileAuditEventVM[];
    nextCursor: string;
    state: AuditLoadState;
    selected: FileAuditEventVM | null;
  }>({
    scope: requestScope,
    items: [],
    nextCursor: "",
    state: "loading",
    selected: null,
  });
  const scopedView =
    viewState.scope === requestScope
      ? viewState
      : {
          scope: requestScope,
          items: [],
          nextCursor: "",
          state: "loading" as const,
          selected: null,
        };
  const { items, nextCursor, state, selected } = scopedView;
  const [advanced, setAdvanced] = useState(false);
  const [retry, setRetry] = useState(0);
  const requestGeneration = useRef(0);
  const filterScope = [
    workspaceId,
    query.actor_id ?? "",
    query.action ?? "",
    query.resource_id ?? "",
    query.result ?? "",
    query.from ?? "",
    query.to ?? "",
  ].join("|");
  const [historyState, setHistoryState] = useState({
    scope: filterScope,
    trail: [query.cursor ?? ""],
  });
  const cursorTrail =
    historyState.scope === filterScope ? historyState.trail : [query.cursor ?? ""];
  const currentCursorIndex = cursorTrail.lastIndexOf(query.cursor ?? "");
  const canRead = permissions.includes("file:admin");

  useEffect(() => {
    if (draftState.source !== urlSignature) {
      setDraftState({ source: urlSignature, value: initialQuery });
    }
    if (queryState.source !== urlSignature) {
      setQueryState({ source: urlSignature, value: initialQuery });
    }
  }, [draftState.source, initialQuery, queryState.source, urlSignature, workspaceId]);

  useEffect(() => {
    const generation = ++requestGeneration.current;
    if (!canRead) {
      setViewState({
        scope: requestScope,
        items: [],
        nextCursor: "",
        state: "forbidden",
        selected: null,
      });
      return;
    }
    const controller = new AbortController();
    setViewState({
      scope: requestScope,
      items: [],
      nextCursor: "",
      state: "loading",
      selected: null,
    });
    void listFileAuditEvents(
      workspaceId,
      { ...query, limit: PAGE_SIZE },
      { signal: controller.signal },
    )
      .then((page) => {
        if (controller.signal.aborted || generation !== requestGeneration.current) return;
        setViewState({
          scope: requestScope,
          items: page.items,
          nextCursor: page.next_cursor ?? "",
          state: "ready",
          selected: null,
        });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || generation !== requestGeneration.current) return;
        setViewState({
          scope: requestScope,
          items: [],
          nextCursor: "",
          state: error instanceof BffError && error.status === 403 ? "forbidden" : "error",
          selected: null,
        });
      });
    return () => controller.abort();
  }, [canRead, query, requestScope, retry, workspaceId]);

  function writeQuery(next: FileAuditQuery) {
    const params = new URLSearchParams(searchParams.toString());
    for (const key of ["actor_id", "action", "resource_id", "result", "from", "to"] as const) {
      const value = next[key];
      if (value) params.set(key, String(value));
      else params.delete(key);
    }
    if (next.cursor) params.set("audit_cursor", next.cursor);
    else params.delete("audit_cursor");
    params.set("tab", "audit");
    router.replace(`${pathname}?${params.toString()}`);
    setQueryState({
      source: urlSignature,
      value: { ...next, limit: PAGE_SIZE },
    });
  }

  if (!canRead) {
    return createElement("div", { className: "empty-state" }, "无权查看文件审计。");
  }

  const form = createElement(
    "form",
    {
      className: "stack-md",
      onSubmit: (event: React.FormEvent) => {
        event.preventDefault();
        writeQuery({ ...draft, cursor: undefined });
      },
    },
    createElement(
      "div",
      { className: "flex flex-wrap items-end gap-3" },
      createElement(
        "label",
        { className: "stack-xs min-w-56 flex-1" },
        createElement("span", null, "动作"),
        createElement("input", {
          className: "input",
          value: draft.action ?? "",
          onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
            setDraftState((current) => ({
              source: urlSignature,
              value: { ...current.value, action: event.target.value || undefined },
            })),
        }),
      ),
      createElement(
        "label",
        { className: "stack-xs min-w-40" },
        createElement("span", null, "结果"),
        createElement(
          "select",
          {
            className: "input",
            value: draft.result ?? "",
            onChange: (event: React.ChangeEvent<HTMLSelectElement>) =>
              setDraftState((current) => ({
                source: urlSignature,
                value: { ...current.value, result: event.target.value || undefined },
              })),
          },
          createElement("option", { value: "" }, "全部"),
          createElement("option", { value: "ALLOW" }, "ALLOW"),
          createElement("option", { value: "DENY" }, "DENY"),
        ),
      ),
      createElement(
        "button",
        { type: "button", className: "btn btn-ghost", onClick: () => setAdvanced((value) => !value) },
        "高级筛选",
      ),
      createElement("button", { type: "submit", className: "btn btn-secondary" }, "查询"),
    ),
    advanced
      ? createElement(
          "div",
          { className: "grid gap-3 sm:grid-cols-2 lg:grid-cols-3" },
          createElement(TextFilter, { label: "操作者 ID", value: draft.actor_id, onChange: (actor_id) => setDraftState((current) => ({ source: urlSignature, value: { ...current.value, actor_id } })), mono: true }),
          createElement(TextFilter, { label: "资源 ID", value: draft.resource_id, onChange: (resource_id) => setDraftState((current) => ({ source: urlSignature, value: { ...current.value, resource_id } })), mono: true }),
          createElement(DateFilter, { label: "起始时间", value: draft.from, onChange: (from) => setDraftState((current) => ({ source: urlSignature, value: { ...current.value, from } })) }),
          createElement(DateFilter, { label: "结束时间", value: draft.to, onChange: (to) => setDraftState((current) => ({ source: urlSignature, value: { ...current.value, to } })) }),
        )
      : null,
  );

  let content: React.ReactNode = null;
  if (state === "loading") {
    content = createElement("div", { role: "status", className: "empty-state" }, "正在加载审计记录…");
  } else if (state === "forbidden") {
    content = createElement("div", { className: "empty-state" }, "无权查看文件审计。");
  } else if (state === "error") {
    content = createElement(
      InlineAlert,
      {
        tone: "error",
        children: createElement(
          "div",
          { className: "flex flex-wrap items-center gap-2" },
          createElement("span", { role: "alert" }, "加载审计记录失败，请稍后重试。"),
          createElement(
            "button",
            { type: "button", className: "btn btn-ghost btn-sm", onClick: () => setRetry((value) => value + 1) },
            "重试加载审计",
          ),
        ),
      },
    );
  } else if (items.length === 0) {
    content = createElement("div", { className: "empty-state" }, "暂无审计记录。");
  } else {
    content = createElement(
      "div",
      { className: "table-wrap" },
      createElement(
        "table",
        { className: "data-table text-sm" },
        createElement(
          "thead",
          null,
          createElement("tr", null, ...["时间", "操作者", "动作", "资源", "决策", "Trace"].map((label) => createElement("th", { key: label }, label))),
        ),
        createElement(
          "tbody",
          null,
          items.map((item) =>
            createElement(
              "tr",
              { key: item.id },
              createElement("td", { className: "whitespace-nowrap text-muted" }, new Date(item.created_at).toLocaleString()),
              createElement("td", { className: "font-mono text-xs" }, shortID(item.actor_id)),
              createElement("td", null, createElement("button", {
                type: "button",
                className: "link-muted text-left",
                "aria-label": `查看审计事件 ${item.action}`,
                onClick: () =>
                  setViewState((current) =>
                    current.scope === requestScope
                      ? { ...current, selected: item }
                      : current,
                  ),
              }, item.action)),
              createElement("td", { className: "font-mono text-xs" }, `${item.resource_type}/${shortID(item.resource_id)}`),
              createElement("td", null, item.result),
              createElement("td", { className: "font-mono text-xs" }, item.trace_id || "—"),
            ),
          ),
        ),
      ),
    );
  }

  const pagination = state === "ready"
    ? createElement(
        "div",
        { className: "flex justify-end gap-2" },
        createElement("button", {
          type: "button",
          className: "btn btn-ghost btn-sm",
          disabled: currentCursorIndex <= 0,
          onClick: () => {
            const previous = cursorTrail[currentCursorIndex - 1];
            if (previous === undefined) return;
            writeQuery({ ...query, cursor: previous || undefined });
          },
        }, "上一页"),
        createElement("button", {
          type: "button",
          className: "btn btn-secondary btn-sm",
          disabled: !nextCursor,
          onClick: () => {
            const visited =
              currentCursorIndex >= 0
                ? cursorTrail.slice(0, currentCursorIndex + 1)
                : [query.cursor ?? ""];
            setHistoryState({
              scope: filterScope,
              trail: [...visited, nextCursor],
            });
            writeQuery({ ...query, cursor: nextCursor });
          },
        }, "下一页"),
      )
    : null;

  return createElement(
    "section",
    { className: "stack-lg" },
    form,
    content,
    pagination,
    createElement(AuditDetailDrawer, {
      item: selected,
      onClose: () =>
        setViewState((current) =>
          current.scope === requestScope
            ? { ...current, selected: null }
            : current,
        ),
    }),
  );
}

function TextFilter({ label, value, onChange, mono = false }: { label: string; value?: string; onChange: (value?: string) => void; mono?: boolean }) {
  return createElement(
    "label",
    { className: "stack-xs" },
    createElement("span", null, label),
    createElement("input", {
      "aria-label": label,
      className: mono ? "input font-mono text-xs" : "input",
      value: value ?? "",
      onChange: (event: React.ChangeEvent<HTMLInputElement>) => onChange(event.target.value || undefined),
    }),
  );
}

function DateFilter({ label, value, onChange }: { label: string; value?: string; onChange: (value?: string) => void }) {
  const localValue = value ? value.slice(0, 16) : "";
  return createElement(
    "label",
    { className: "stack-xs" },
    createElement("span", null, label),
    createElement("input", {
      "aria-label": label,
      type: "datetime-local",
      className: "input",
      value: localValue,
      onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
        onChange(event.target.value ? new Date(event.target.value).toISOString() : undefined),
    }),
  );
}

function AuditDetailDrawer({ item, onClose }: { item: FileAuditEventVM | null; onClose: () => void }) {
  if (!item) return null;
  return createElement(
    Drawer,
    {
      open: true,
      title: "审计事件详情",
      onClose,
      panelClassName: "w-full max-w-xl",
      children: createElement(
        "div",
        { className: "stack-lg" },
        createElement(
          "dl",
          { className: "grid gap-4 sm:grid-cols-2" },
          createElement(Detail, { label: "操作者", value: item.actor_id }),
          createElement(Detail, { label: "动作", value: item.action }),
          createElement(Detail, { label: "资源类型", value: item.resource_type }),
          createElement(Detail, { label: "资源 ID", value: item.resource_id }),
          createElement(Detail, { label: "决策", value: item.result }),
          createElement(Detail, { label: "Trace ID", value: item.trace_id || "—" }),
        ),
        createElement(
          "pre",
          { className: "max-w-full overflow-auto rounded-lg bg-surface-muted p-3 text-xs" },
          JSON.stringify(item.detail, null, 2),
        ),
      ),
    },
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return createElement(
    "div",
    { className: "stack-xs min-w-0" },
    createElement("dt", { className: "text-xs text-muted" }, label),
    createElement("dd", { className: "break-all font-mono text-xs" }, value),
  );
}

function shortID(value: string) {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value;
}
