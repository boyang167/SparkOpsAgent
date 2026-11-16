"use client";

import {
  createElement,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import type { ChangeEvent } from "react";
import type {
  TransferSessionVM,
  TransferStatus as PersistedStatus,
} from "@sparkops/contracts/filesystem";
import Link from "next/link";
import {
  abortTransfer,
  listTransferSessions,
} from "@sparkops/web-sdk/filesystem";
import { computeTransferMetrics, transferDisplayId } from "./transfer-center-stats";
import { transferErrorMessage } from "./transfer-error-message";
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

function ProgressBar({
  label,
  value,
}: {
  label: string;
  value: number;
}) {
  const clamped = Math.min(100, Math.max(0, value));
  return createElement(
    "div",
    { className: "filesystem-transfer-progress stack-xs min-w-[8rem]" },
    createElement(
      "div",
      {
        role: "progressbar",
        "aria-label": label,
        "aria-valuemin": 0,
        "aria-valuemax": 100,
        "aria-valuenow": clamped,
        className: "h-1.5 overflow-hidden rounded-full bg-surface-muted",
      },
      createElement("div", {
        className: "filesystem-progress-fill h-full bg-primary",
        style: { width: `${clamped}%` },
      }),
    ),
    createElement("span", { className: "text-xs text-muted tabular-nums" }, `${clamped}%`),
  );
}

function StatusBadge({ label, tone }: { label: string; tone: string }) {
  return createElement(
    "span",
    { className: `badge ${tone} text-xs` },
    label,
  );
}

function badgeToneForLocal(item: TransferItem) {
  if (item.status === "completed") return "badge-success";
  if (item.status === "failed") return "badge-danger";
  if (item.status === "canceled") return "badge-neutral";
  return "badge-primary";
}

function badgeToneForPersisted(status: PersistedStatus) {
  if (status === "COMPLETED") return "badge-success";
  if (status === "FAILED" || status === "REJECTED" || status === "EXPIRED") {
    return "badge-danger";
  }
  if (status === "PAUSED") return "badge-warning";
  if (status === "ABORTED") return "badge-neutral";
  return "badge-primary";
}

function persistedPercent(session: TransferSessionVM) {
  return session.status === "COMPLETED" ? 100 : 0;
}

function TransferPageHeader({ actions }: { actions: ReturnType<typeof createElement> }) {
  return createElement(
    "div",
    { className: "section-header !mb-0" },
    createElement(
      "div",
      null,
      createElement("h1", { className: "page-title mt-1" }, "传输中心"),
      createElement(
        "p",
        { className: "text-muted mt-2" },
        "统一管理上传、校验与提交状态。任务在后台持续运行，失败后可单独重试。",
      ),
    ),
    actions ?? null,
  );
}

function TransferStateMachineHint() {
  return createElement(
    "aside",
    { className: "card stack-xs text-sm text-muted", "aria-label": "传输状态机" },
    createElement("p", { className: "font-medium text" }, "传输状态机"),
    createElement(
      "p",
      null,
      "INITIATED → UPLOADING → PAUSED → VERIFYING → SCANNING → COMMITTING → COMPLETED",
    ),
    createElement(
      "p",
      { className: "text-xs" },
      "失败、中止或过期会终止当前会话；客户端重试将创建新的传输会话。",
    ),
  );
}

function TransferMetricStrip({ counts }: { counts: ReturnType<typeof computeTransferMetrics> }) {
  const cards = [
    { label: "进行中", value: counts.running, hint: "正在处理", tone: "metric-card-sky" },
    { label: "已完成", value: counts.completed, hint: "本会话", tone: "metric-card-emerald" },
    { label: "已暂停", value: counts.paused, hint: "可恢复", tone: "metric-card-amber" },
    { label: "失败任务", value: counts.failed, hint: "需处理", tone: "metric-card-violet" },
  ];
  return createElement(
    "div",
    {
      className: "filesystem-transfer-metrics grid gap-3 sm:grid-cols-2 xl:grid-cols-4",
      "aria-label": "传输概览",
    },
    cards.map((card) =>
      createElement(
        "article",
        { key: card.label, className: `metric-card ${card.tone}` },
        createElement(
          "div",
          { className: "metric-card-body" },
          createElement("p", { className: "metric-card-label" }, card.label),
          createElement("p", { className: "metric-card-value" }, card.value),
          createElement("p", { className: "metric-card-hint" }, card.hint),
        ),
      ),
    ),
  );
}

function PersistedTransferCard({
  session,
  queue,
  canWrite,
  onRefresh,
}: {
  session: TransferSessionVM;
  queue: TransferQueue;
  canWrite: boolean;
  onRefresh: () => void;
}) {
  const terminal = new Set<PersistedStatus>([
    "COMPLETED",
    "FAILED",
    "REJECTED",
    "ABORTED",
    "EXPIRED",
  ]);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  async function cancelPersisted() {
    setBusy(true);
    setActionError(null);
    try {
      await abortTransfer(session.session_id, session.workspace_id);
      onRefresh();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "取消失败，请重试。");
    } finally {
      setBusy(false);
    }
  }

  function selectReplacement(file: File | undefined) {
    if (!file) return;
    setActionError(null);
    if (
      file.name !== session.target_name ||
      file.size !== session.expected_size
    ) {
      setActionError("文件名称和大小必须与失败任务一致。");
      return;
    }
    if (!session.target_parent_id) {
      setActionError("原目标目录不可用，无法安全新建传输。");
      return;
    }
    queue.enqueue([file], session.target_parent_id);
  }
  const progress = persistedPercent(session);
  return createElement(
    "tr",
    null,
    createElement(
      "td",
      { className: "min-w-[12rem]" },
      createElement("p", { className: "font-mono text-xs text-muted" }, transferDisplayId(session.session_id)),
      createElement("p", { className: "truncate font-medium" }, session.target_name),
    ),
    createElement("td", null, "上传"),
    createElement(
      "td",
      null,
      createElement(ProgressBar, {
        label: `${session.target_name} 上传进度`,
        value: progress,
      }),
    ),
    createElement(
      "td",
      null,
      createElement(StatusBadge, {
        label: persistedStatusLabels[session.status],
        tone: badgeToneForPersisted(session.status),
      }),
      session.error_code
        ? createElement(
            "p",
            { role: "alert", className: "mt-1 text-xs text-danger" },
            transferErrorMessage(session.error_code),
          )
        : null,
    ),
    createElement("td", { className: "text-muted tabular-nums" }, "—"),
    createElement(
      "td",
      { className: "text-right" },
      createElement(
        "div",
        { className: "flex flex-col items-end gap-2" },
        canWrite && !terminal.has(session.status)
          ? createElement(
              "button",
              {
                type: "button",
                className: "btn btn-ghost btn-sm",
                "aria-label": `取消 ${session.target_name}`,
                disabled: busy,
                onClick: () => void cancelPersisted(),
              },
              busy ? "正在取消" : "取消",
            )
          : null,
        canWrite && session.status === "FAILED"
          ? createElement(
              "div",
              { className: "stack-xs items-end text-right" },
              createElement(
                "p",
                { className: "max-w-xs text-xs text-muted" },
                "旧会话已失败；重新选择匹配文件后将新建传输，不会恢复旧会话。",
              ),
              createElement(
                "label",
                { className: "btn btn-secondary btn-sm" },
                "重新选择并新建",
                createElement("input", {
                  type: "file",
                  className: "sr-only",
                  "aria-label": `重新选择并新建 ${session.target_name}`,
                  onChange: (event: ChangeEvent<HTMLInputElement>) => {
                    selectReplacement(event.currentTarget.files?.[0]);
                    event.currentTarget.value = "";
                  },
                }),
              ),
            )
          : null,
        actionError
          ? createElement(
              "p",
              { role: "alert", className: "text-xs text-danger" },
              actionError,
            )
          : null,
      ),
    ),
  );
}

function LocalTransferRow({
  item,
  canWrite,
  queue,
}: {
  item: TransferItem;
  canWrite: boolean;
  queue: TransferQueue;
}) {
  const progress = percent(item);
  const sessionId = item.session?.session_id ?? item.id;
  const speed =
    item.status === "uploading" || item.status === "verifying" ? "实时" : "—";
  return createElement(
    "tr",
    null,
    createElement(
      "td",
      { className: "min-w-[12rem]" },
      createElement("p", { className: "font-mono text-xs text-muted" }, transferDisplayId(sessionId)),
      createElement("p", { className: "truncate font-medium" }, item.file.name),
    ),
    createElement("td", null, "上传"),
    createElement(
      "td",
      null,
      createElement(ProgressBar, {
        label: `${item.file.name} 上传进度`,
        value: progress,
      }),
    ),
    createElement(
      "td",
      null,
      createElement(StatusBadge, {
        label: statusLabel(item),
        tone: badgeToneForLocal(item),
      }),
      item.error
        ? createElement(
            "p",
            { role: "alert", className: "mt-1 text-xs text-danger" },
            item.error,
          )
        : null,
    ),
    createElement("td", { className: "text-muted tabular-nums" }, speed),
    createElement(
      "td",
      { className: "text-right" },
      createElement(
        "div",
        { className: "flex flex-wrap justify-end gap-2" },
        canWrite && item.status === "failed"
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
        canWrite &&
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
  );
}

type PersistedLoadPhase =
  | "loading"
  | "refreshing"
  | "loadingMore"
  | "ready"
  | "error";

type PersistedState = {
  workspaceId?: string;
  items: TransferSessionVM[];
  nextCursor: string;
  phase: PersistedLoadPhase;
};

function mergePersisted(
  current: TransferSessionVM[],
  incoming: TransferSessionVM[],
) {
  const seen = new Set(current.map((session) => session.session_id));
  const merged = [...current];
  for (const session of incoming) {
    if (seen.has(session.session_id)) continue;
    seen.add(session.session_id);
    merged.push(session);
  }
  return merged;
}

export function TransferCenter({
  queue,
  workspaceId,
  canWrite = false,
}: {
  queue: TransferQueue;
  workspaceId?: string;
  canWrite?: boolean;
}) {
  const items = useTransfers(queue);
  const [persisted, setPersisted] = useState<PersistedState>({
    workspaceId,
    items: [],
    nextCursor: "",
    phase: workspaceId ? "loading" : "ready",
  });
  const requestGeneration = useRef(0);
  const controllers = useRef(new Set<AbortController>());
  const firstPageInFlight = useRef(false);
  const loadMoreInFlight = useRef(false);

  function abortRequests() {
    controllers.current.forEach((controller) => controller.abort());
    controllers.current.clear();
  }

  function beginFirstPageRequest(targetWorkspaceId: string, preserveItems: boolean) {
    if (firstPageInFlight.current && preserveItems) return;
    requestGeneration.current += 1;
    const generation = requestGeneration.current;
    abortRequests();
    firstPageInFlight.current = true;
    loadMoreInFlight.current = false;
    const controller = new AbortController();
    controllers.current.add(controller);
    setPersisted((current) => {
      const sameWorkspace = current.workspaceId === targetWorkspaceId;
      const retained = preserveItems && sameWorkspace ? current.items : [];
      return {
        workspaceId: targetWorkspaceId,
        items: retained,
        nextCursor: preserveItems && sameWorkspace ? current.nextCursor : "",
        phase: retained.length > 0 ? "refreshing" : "loading",
      };
    });
    void listTransferSessions(
      targetWorkspaceId,
      { limit: 100 },
      { signal: controller.signal },
    )
      .then((page) => {
        if (
          generation !== requestGeneration.current ||
          targetWorkspaceId !== workspaceId
        ) {
          return;
        }
        setPersisted({
          workspaceId: targetWorkspaceId,
          items: mergePersisted([], page.items),
          nextCursor: page.next_cursor,
          phase: "ready",
        });
      })
      .catch(() => {
        if (
          generation !== requestGeneration.current ||
          targetWorkspaceId !== workspaceId
        ) {
          return;
        }
        setPersisted((current) => ({
          ...current,
          phase: "error",
        }));
      })
      .finally(() => {
        controllers.current.delete(controller);
        if (generation === requestGeneration.current) {
          firstPageInFlight.current = false;
        }
      });
  }

  useEffect(() => {
    requestGeneration.current += 1;
    abortRequests();
    firstPageInFlight.current = false;
    loadMoreInFlight.current = false;
    if (!workspaceId) {
      setPersisted({
        workspaceId: undefined,
        items: [],
        nextCursor: "",
        phase: "ready",
      });
      return;
    }
    beginFirstPageRequest(workspaceId, false);
    return () => {
      requestGeneration.current += 1;
      abortRequests();
      firstPageInFlight.current = false;
      loadMoreInFlight.current = false;
    };
  }, [workspaceId]);

  const scopedPersisted =
    persisted.workspaceId === workspaceId
      ? persisted
      : {
          workspaceId,
          items: [],
          nextCursor: "",
          phase: workspaceId ? ("loading" as const) : ("ready" as const),
        };
  const localSessionIDs = new Set(
    items
      .map((item) => item.session?.session_id)
      .filter((id): id is string => Boolean(id)),
  );
  const visiblePersisted = scopedPersisted.items.filter(
    (session) => !localSessionIDs.has(session.session_id),
  );

  const metrics = computeTransferMetrics(items, visiblePersisted);
  const headerActions = createElement(
    "div",
    { className: "flex flex-wrap items-center gap-2" },
    workspaceId
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-ghost btn-sm",
            disabled:
              scopedPersisted.phase === "loading" ||
              scopedPersisted.phase === "refreshing",
            onClick: () => beginFirstPageRequest(workspaceId, true),
          },
          "刷新",
        )
      : null,
    createElement(
      Link,
      { href: "/files/data/workspace", className: "btn btn-primary btn-sm filesystem-touch-target" },
      "新建传输",
    ),
  );

  if (
    items.length === 0 &&
    visiblePersisted.length === 0 &&
    scopedPersisted.phase === "ready"
  ) {
    return createElement(
      "div",
      { className: "stack-lg", "aria-label": "传输中心" },
      createElement(TransferPageHeader, { actions: headerActions }),
      createElement(TransferMetricStrip, { counts: metrics }),
      createElement(
        "div",
        { className: "card empty-state stack-sm" },
        createElement("h2", { className: "section-title" }, "暂无传输任务"),
        createElement(
          "p",
          { className: "text-sm text-muted" },
          "在文件浏览中上传后，将在此展示进度与历史会话。",
        ),
      ),
      createElement(TransferStateMachineHint, null),
    );
  }

  async function loadMore() {
    const cursor = scopedPersisted.nextCursor;
    if (!workspaceId || !cursor || loadMoreInFlight.current) return;
    loadMoreInFlight.current = true;
    const generation = requestGeneration.current;
    const targetWorkspaceId = workspaceId;
    const controller = new AbortController();
    controllers.current.add(controller);
    setPersisted((current) => ({ ...current, phase: "loadingMore" }));
    try {
      const page = await listTransferSessions(
        targetWorkspaceId,
        { cursor, limit: 100 },
        { signal: controller.signal },
      );
      if (
        generation !== requestGeneration.current ||
        targetWorkspaceId !== workspaceId
      ) {
        return;
      }
      setPersisted((current) => ({
        ...current,
        items: mergePersisted(current.items, page.items),
        nextCursor: page.next_cursor,
        phase: "ready",
      }));
    } catch {
      if (
        generation === requestGeneration.current &&
        targetWorkspaceId === workspaceId
      ) {
        setPersisted((current) => ({ ...current, phase: "error" }));
      }
    } finally {
      controllers.current.delete(controller);
      if (generation === requestGeneration.current) {
        loadMoreInFlight.current = false;
      }
    }
  }

  return createElement(
    "div",
    { className: "stack-lg", "aria-label": "传输中心" },
    createElement(TransferPageHeader, { actions: headerActions }),
    createElement(TransferMetricStrip, { counts: metrics }),
    createElement(
      "section",
      { className: "card overflow-hidden stack-sm", "aria-label": "传输任务" },
      createElement(
        "div",
        { className: "flex items-center justify-between gap-3 border-b border-border px-4 py-3" },
        createElement("h2", { className: "section-title" }, "传输任务"),
        createElement(
          "p",
          { className: "text-xs text-muted" },
          `${items.length + visiblePersisted.length} 条记录`,
        ),
      ),
      createElement(
        "div",
        { className: "overflow-x-auto" },
        createElement(
          "table",
          { className: "data-table w-full text-sm" },
          createElement(
            "thead",
            null,
            createElement(
              "tr",
              null,
              ["任务 ID / 文件", "方向", "进度", "状态", "速度", "操作"].map((column) =>
                createElement("th", { key: column, scope: "col" }, column),
              ),
            ),
          ),
          createElement(
            "tbody",
            null,
            items.map((item) =>
              createElement(LocalTransferRow, {
                key: item.id,
                item,
                canWrite,
                queue,
              }),
            ),
            visiblePersisted.map((session) =>
              createElement(PersistedTransferCard, {
                key: session.session_id,
                session,
                queue,
                canWrite,
                onRefresh: () => {
                  if (workspaceId) beginFirstPageRequest(workspaceId, false);
                },
              }),
            ),
          ),
        ),
      ),
    ),
    scopedPersisted.phase === "loading"
      ? createElement(
          "p",
          { role: "status", className: "text-sm text-muted" },
          "正在加载持久传输任务…",
        )
      : null,
    scopedPersisted.phase === "refreshing"
      ? createElement(
          "p",
          { role: "status", className: "text-sm text-muted" },
          "正在刷新持久传输任务…",
        )
      : null,
    scopedPersisted.phase === "loadingMore"
      ? createElement(
          "p",
          { role: "status", className: "text-sm text-muted" },
          "正在加载更多传输任务…",
        )
      : null,
    scopedPersisted.phase === "error"
      ? createElement(
          "div",
          { className: "flex flex-wrap items-center gap-3" },
          createElement(
            "p",
            { role: "alert", className: "text-sm text-danger" },
            "无法加载持久传输任务，请稍后重试。",
          ),
          workspaceId
            ? createElement(
                "button",
                {
                  type: "button",
                  className: "btn btn-secondary btn-sm",
                  onClick: () => beginFirstPageRequest(workspaceId, true),
                },
                "重试",
              )
            : null,
        )
      : null,
    scopedPersisted.nextCursor &&
      scopedPersisted.phase !== "loading" &&
      scopedPersisted.phase !== "refreshing"
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-secondary btn-sm self-start",
            disabled: scopedPersisted.phase === "loadingMore",
            onClick: () => void loadMore(),
          },
          "加载更多",
        )
      : null,
    createElement(TransferStateMachineHint, null),
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
  const items = useTransfers(queue);
  const active = items
    .filter((item) => activeStatuses.has(item.status))
    .slice(0, 3);
  const failed = items.filter((item) => item.status === "failed").slice(0, 2);
  if (active.length === 0 && failed.length === 0) return null;

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
          createElement(
            "span",
            { className: "shrink-0 text-muted" },
            statusLabel(item),
          ),
        ),
        createElement(ProgressBar, {
          label: `${item.file.name} 上传进度`,
          value: percent(item),
        }),
      ),
    ),
    failed.map((item) =>
      createElement(
        "div",
        { key: item.id, className: "stack-xs", role: "alert" },
        createElement(
          "div",
          { className: "flex justify-between gap-3 text-sm" },
          createElement("span", { className: "truncate" }, item.file.name),
          createElement("span", { className: "shrink-0 text-danger" }, "上传失败"),
        ),
        item.error
          ? createElement("p", { className: "text-xs text-danger" }, item.error)
          : null,
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
