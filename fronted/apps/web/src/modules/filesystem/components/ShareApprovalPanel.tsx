"use client";

import { createElement, useCallback, useEffect, useRef, useState } from "react";
import type { ShareRequestVM } from "@sparkops/contracts/filesystem";
import {
  approveShareRequest,
  listOrganizationShareRequests,
  rejectShareRequest,
} from "@sparkops/web-sdk/filesystem";
import { Modal } from "@/components/ui/Modal";

type Props = {
  organizationId: string;
  workspaceId: string;
};

function stableKey(prefix: string) {
  return `${prefix}-${globalThis.crypto?.randomUUID?.() ?? Date.now().toString(36)}`;
}

export function ShareApprovalPanel({ organizationId, workspaceId }: Props) {
  const [items, setItems] = useState<ShareRequestVM[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [busy, setBusy] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState<ShareRequestVM | null>(null);
  const [reason, setReason] = useState("");
  const [mutationError, setMutationError] = useState("");
  const generation = useRef(0);
  const mutationKeys = useRef(new Map<string, string>());

  const load = useCallback(async () => {
    const current = ++generation.current;
    setState("loading");
    try {
      const page = await listOrganizationShareRequests(
        { status: "PENDING", limit: 50 },
        { organizationId, workspaceId },
      );
      if (current !== generation.current) return;
      setItems(page.items);
      setState("ready");
    } catch {
      if (current === generation.current) setState("error");
    }
  }, [organizationId, workspaceId]);

  useEffect(() => {
    void load();
    return () => {
      generation.current += 1;
    };
  }, [load]);

  function keyFor(action: "approve" | "reject", id: string) {
    const identity = `${action}:${id}`;
    const existing = mutationKeys.current.get(identity);
    if (existing) return existing;
    const value = stableKey(`share-${action}`);
    mutationKeys.current.set(identity, value);
    return value;
  }

  async function approve(item: ShareRequestVM) {
    if (busy) return;
    setBusy(item.id);
    setMutationError("");
    try {
      await approveShareRequest(item.id, {
        organizationId,
        workspaceId,
        idempotencyKey: keyFor("approve", item.id),
      });
      mutationKeys.current.delete(`approve:${item.id}`);
      await load();
    } catch {
      setMutationError("批准失败，请稍后重试。");
    } finally {
      setBusy(null);
    }
  }

  async function reject() {
    if (!rejecting || busy || reason.trim().length < 1 || reason.trim().length > 1000) return;
    setBusy(rejecting.id);
    setMutationError("");
    try {
      await rejectShareRequest(
        rejecting.id,
        { reason: reason.trim() },
        {
          organizationId,
          workspaceId,
          idempotencyKey: keyFor("reject", rejecting.id),
        },
      );
      mutationKeys.current.delete(`reject:${rejecting.id}`);
      setRejecting(null);
      setReason("");
      await load();
    } catch {
      setMutationError("拒绝失败，请稍后重试。");
    } finally {
      setBusy(null);
    }
  }

  let content: React.ReactNode;
  if (state === "loading") {
    content = createElement("div", { role: "status", className: "empty-state" }, "正在加载待审批申请…");
  } else if (state === "error") {
    content = createElement(
      "div",
      { className: "inline-alert inline-alert-error" },
      createElement(
        "div",
        { className: "flex flex-wrap items-center gap-2" },
        createElement("span", { role: "alert" }, "加载待审批申请失败。"),
        createElement("button", { type: "button", className: "btn btn-ghost btn-sm", onClick: () => void load() }, "重试"),
      ),
    );
  } else if (items.length === 0) {
    content = createElement("div", { className: "empty-state" }, "暂无待审批共享申请。");
  } else {
    content = createElement(
      "div",
      { className: "table-wrap" },
      createElement(
        "table",
        { className: "data-table text-sm" },
        createElement("thead", null, createElement("tr", null,
          ...["申请人", "目标主体", "权限", "申请时间", "操作"].map((label) => createElement("th", { key: label }, label)),
        )),
        createElement("tbody", null, items.map((item) => createElement(
          "tr",
          { key: item.id },
          createElement("td", { className: "font-mono text-xs" }, item.requested_by),
          createElement("td", null, `${item.principal_type} · ${item.principal_ids.length}`),
          createElement("td", null, item.actions.join("、")),
          createElement("td", { className: "whitespace-nowrap text-muted" }, new Date(item.created_at).toLocaleString()),
          createElement("td", null, createElement(
            "div",
            { className: "flex flex-wrap gap-2" },
            createElement("button", {
              type: "button", className: "btn btn-secondary btn-sm",
              "aria-label": "批准共享申请", disabled: busy !== null,
              onClick: () => void approve(item),
            }, busy === item.id ? "处理中…" : "批准"),
            createElement("button", {
              type: "button", className: "btn btn-ghost btn-sm",
              "aria-label": "拒绝共享申请", disabled: busy !== null,
              onClick: () => {
                setRejecting(item);
                setReason("");
              },
            }, "拒绝"),
          )),
        ))),
      ),
    );
  }

  const footer = createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-2" },
    createElement("button", {
      type: "button", className: "btn btn-secondary", disabled: Boolean(busy),
      onClick: () => setRejecting(null),
    }, "取消"),
    createElement("button", {
      type: "button", className: "btn btn-primary",
      disabled: Boolean(busy) || reason.trim().length < 1 || reason.trim().length > 1000,
      onClick: () => void reject(),
    }, busy ? "提交中…" : "确认拒绝"),
  );

  return createElement(
    "section",
    { className: "stack-md", "aria-label": "共享审批" },
    createElement(
      "div",
      { className: "section-header flex flex-wrap items-center justify-between gap-3" },
      createElement("div", null,
        createElement("h2", { className: "section-title" }, "共享审批"),
        createElement("p", { className: "text-sm text-muted" }, "审核需要组织文件管理员确认的共享范围。"),
      ),
      createElement("span", { className: "badge badge-neutral" }, `待审批 ${items.length}`),
    ),
    mutationError
      ? createElement("div", { className: "inline-alert inline-alert-error", role: "alert" }, mutationError)
      : null,
    content,
    createElement(Modal, {
      open: Boolean(rejecting),
      onClose: () => {
        if (!busy) setRejecting(null);
      },
      title: "拒绝共享申请",
      description: "请填写可供申请人理解的拒绝原因。",
      size: "sm",
      footer,
      children: createElement(
        "label",
        { className: "stack-xs" },
        createElement("span", null, "拒绝原因 *"),
        createElement("textarea", {
          "aria-label": "拒绝原因", className: "input min-h-28", maxLength: 1000,
          value: reason, onChange: (event: React.ChangeEvent<HTMLTextAreaElement>) => setReason(event.target.value),
        }),
        createElement("span", { className: "text-xs text-muted" }, `${reason.length}/1000`),
      ),
    }),
  );
}
