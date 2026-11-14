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
import {
  abortTransfer,
  listTransferSessions,
} from "@sparkops/web-sdk/filesystem";
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
      className: "filesystem-progress-fill h-full bg-primary",
      style: { width: `${value}%` },
    }),
  );
}

function PersistedTransferCard({
  session,
  queue,
  onRefresh,
}: {
  session: TransferSessionVM;
  queue: TransferQueue;
  onRefresh: () => void;
}) {
  const completed = session.status === "COMPLETED";
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
  return createElement(
    "article",
    { className: "card stack-sm" },
    createElement(
      "div",
      { className: "flex flex-wrap items-start justify-between gap-3" },
      createElement(
        "div",
        { className: "min-w-0" },
        createElement("h3", { className: "truncate font-medium" }, session.target_name),
        createElement(
          "p",
          { className: "text-sm text-muted" },
          persistedStatusLabels[session.status],
        ),
      ),
      !terminal.has(session.status)
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
    ),
    createElement(
      "div",
      {
        role: "progressbar",
        "aria-label": `${session.target_name} 上传进度`,
        "aria-valuemin": 0,
        "aria-valuemax": 100,
        "aria-valuenow": completed ? 100 : 0,
        className: "h-1.5 overflow-hidden rounded-full bg-surface-muted",
      },
      createElement("div", {
        className: "filesystem-progress-fill h-full bg-primary",
        style: { width: completed ? "100%" : "0%" },
      }),
    ),
    createElement(
      "p",
      { className: "text-xs text-muted" },
      `${completed ? session.expected_size : 0} / ${session.expected_size} 字节`,
    ),
    session.error_code
      ? createElement(
          "p",
          { role: "alert", className: "text-sm text-danger" },
          session.error_code,
        )
      : null,
    session.status === "FAILED"
      ? createElement(
          "div",
          { className: "stack-xs" },
          createElement(
            "p",
            { className: "text-sm text-muted" },
            "旧会话已失败；重新选择匹配文件后将新建传输，不会恢复旧会话。",
          ),
          createElement(
            "label",
            { className: "btn btn-secondary btn-sm self-start" },
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
          { role: "alert", className: "text-sm text-danger" },
          actionError,
        )
      : null,
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
}: {
  queue: TransferQueue;
  workspaceId?: string;
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

  if (
    items.length === 0 &&
    visiblePersisted.length === 0 &&
    scopedPersisted.phase === "ready"
  ) {
    return createElement(
      "div",
      { className: "empty-state stack-sm" },
      createElement("h2", { className: "section-title" }, "暂无传输任务"),
      createElement(
        "p",
        { className: "text-sm text-muted" },
        "上传文件后，可在这里查看进度并处理失败任务。",
      ),
      workspaceId
        ? createElement(
            "button",
            {
              type: "button",
              className: "btn btn-secondary btn-sm self-start",
              onClick: () => beginFirstPageRequest(workspaceId, true),
            },
            "刷新",
          )
        : null,
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
    "section",
    { className: "stack-md", "aria-label": "传输任务" },
    createElement(
      "div",
      { className: "stack-sm" },
      createElement("h2", { className: "section-title" }, "传输"),
      createElement(
        "div",
        { className: "flex flex-wrap items-center justify-between gap-3" },
        createElement(
          "p",
          { className: "text-sm text-muted" },
          "任务会在后台继续运行，失败后可单独重试。",
        ),
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
              statusLabel(item),
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
    visiblePersisted.map((session) =>
      createElement(PersistedTransferCard, {
        key: session.session_id,
        session,
        queue,
        onRefresh: () => {
          if (workspaceId) beginFirstPageRequest(workspaceId, false);
        },
      }),
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
