"use client";

import { useRouter } from "next/navigation";
import { createElement, useEffect, useMemo, useState } from "react";
import type { TrashItemVM } from "@sparkops/contracts/filesystem";
import {
  emptyTrash,
  listTrashItems,
  purgeFileNode,
  restoreFileNode,
} from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { FileBrowser } from "./FileBrowser";
import { runTrashBatch } from "./file-explorer-panel-trash-ops";
import type { FileView } from "./workbench-state";

type TrashDialog =
  | { type: "restore"; nodeIds: string[] }
  | { type: "purge"; nodeIds: string[] }
  | { type: "empty" }
  | null;

export type TrashExplorerPanelProps = {
  workspaceId: string;
  namespaceId: string;
  initialItems: TrashItemVM[];
  initialNextCursor?: string;
  initialLoadError?: boolean;
  view?: FileView;
};

export function TrashExplorerPanel({
  workspaceId,
  namespaceId,
  initialItems,
  initialNextCursor = "",
  initialLoadError = false,
  view = "list",
}: TrashExplorerPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState(initialItems);
  const [nextCursor, setNextCursor] = useState(initialNextCursor);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [dialog, setDialog] = useState<TrashDialog>(null);
  const [pending, setPending] = useState(false);
  const [loadError, setLoadError] = useState(initialLoadError);
  const [actionError, setActionError] = useState<string | null>(null);
  const [batchFailures, setBatchFailures] = useState<Array<{ nodeId: string; message: string }>>([]);
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    setItems(initialItems);
    setNextCursor(initialNextCursor);
    setLoadError(initialLoadError);
    setSelectedIds([]);
    setBatchFailures([]);
    setActionError(null);
  }, [initialItems, initialLoadError, initialNextCursor, namespaceId, workspaceId]);

  const trashDetails = useMemo(
    () =>
      Object.fromEntries(
        items.map((item) => [
          item.node.id,
          { original_path: item.original_path, deleted_at: item.deleted_at },
        ]),
      ),
    [items],
  );
  const nodes = useMemo(() => items.map((item) => item.node), [items]);

  async function reloadTrash() {
    setPending(true);
    setLoadError(false);
    setActionError(null);
    try {
      const page = await listTrashItems(workspaceId, { namespace_id: namespaceId });
      setItems(page.items);
      setNextCursor(page.next_cursor);
      setSelectedIds([]);
      setBatchFailures([]);
    } catch (err) {
      setLoadError(true);
      setActionError(err instanceof Error ? err.message : "回收站加载失败");
    } finally {
      setPending(false);
    }
  }

  async function loadMore() {
    if (!nextCursor) return;
    setLoadingMore(true);
    try {
      const page = await listTrashItems(workspaceId, {
        namespace_id: namespaceId,
        cursor: nextCursor,
      });
      setItems((current) => [...current, ...page.items]);
      setNextCursor(page.next_cursor);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "加载更多失败");
    } finally {
      setLoadingMore(false);
    }
  }

  function removeSucceeded(nodeIds: string[]) {
    const removed = new Set(nodeIds);
    setItems((current) => current.filter((item) => !removed.has(item.node.id)));
    setSelectedIds((current) => current.filter((nodeId) => !removed.has(nodeId)));
  }

  async function submitRestore(nodeIds: string[]) {
    setPending(true);
    setActionError(null);
    const result = await runTrashBatch(nodeIds, (nodeId) =>
      restoreFileNode(workspaceId, nodeId, { namespaceId }),
    );
    removeSucceeded(result.succeeded);
    if (result.failures.length > 0) {
      setBatchFailures(result.failures);
      setSelectedIds(result.failures.map((entry) => entry.nodeId));
      setActionError(result.failures.map((entry) => entry.message).join("；"));
      setDialog(null);
    } else {
      setBatchFailures([]);
      setDialog(null);
      router.refresh();
    }
    setPending(false);
  }

  async function submitPurge(nodeIds: string[]) {
    setPending(true);
    setActionError(null);
    const result = await runTrashBatch(nodeIds, (nodeId) =>
      purgeFileNode(workspaceId, nodeId, { namespaceId }),
    );
    removeSucceeded(result.succeeded);
    if (result.failures.length > 0) {
      setBatchFailures(result.failures);
      setSelectedIds(result.failures.map((entry) => entry.nodeId));
      setActionError(result.failures.map((entry) => entry.message).join("；"));
      setDialog(null);
    } else {
      setBatchFailures([]);
      setDialog(null);
      router.refresh();
    }
    setPending(false);
  }

  async function submitEmptyTrash() {
    setPending(true);
    setActionError(null);
    try {
      const batch = await emptyTrash(workspaceId, { namespaceId });
      const failed = batch.items
        .filter((item) => !item.succeeded)
        .map((item) => ({
          nodeId: item.node_id,
          message: item.error_code ?? "彻底删除失败",
        }));
      const succeeded = batch.items
        .filter((item) => item.succeeded)
        .map((item) => item.node_id);
      removeSucceeded(succeeded);
      if (failed.length > 0) {
        setBatchFailures(failed);
        setSelectedIds(failed.map((entry) => entry.nodeId));
        setActionError("部分项目未能彻底删除");
        setDialog(null);
      } else {
        setBatchFailures([]);
        setDialog(null);
        router.refresh();
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "清空回收站失败");
    } finally {
      setPending(false);
    }
  }

  const restoreDialogFooter = createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-2" },
    createElement(
      "button",
      { type: "button", className: "btn btn-ghost", disabled: pending, onClick: () => setDialog(null) },
      "取消",
    ),
    createElement(
      "button",
      {
        type: "button",
        className: "btn btn-primary",
        disabled: pending || dialog?.type !== "restore",
        onClick: () => {
          if (dialog?.type === "restore") void submitRestore(dialog.nodeIds);
        },
      },
      pending ? "恢复中…" : "恢复",
    ),
  );

  const purgeDialogFooter = createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-2" },
    createElement(
      "button",
      { type: "button", className: "btn btn-ghost", disabled: pending, onClick: () => setDialog(null) },
      "取消",
    ),
    createElement(
      "button",
      {
        type: "button",
        className: "btn-danger-solid",
        disabled: pending || dialog?.type !== "purge",
        onClick: () => {
          if (dialog?.type === "purge") void submitPurge(dialog.nodeIds);
        },
      },
      pending ? "删除中…" : "彻底删除",
    ),
  );

  const emptyDialogFooter = createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-2" },
    createElement(
      "button",
      { type: "button", className: "btn btn-ghost", disabled: pending, onClick: () => setDialog(null) },
      "取消",
    ),
    createElement(
      "button",
      {
        type: "button",
        className: "btn-danger-solid",
        disabled: pending,
        onClick: () => void submitEmptyTrash(),
      },
      pending ? "清空中…" : "清空回收站",
    ),
  );

  if (loadError) {
    return createElement(
      "div",
      { className: "stack-sm" },
      createElement(InlineAlert, {
        tone: "error",
        children: actionError ?? "回收站加载失败，请稍后重试。",
      }),
      createElement(
        "button",
        {
          type: "button",
          className: "btn btn-secondary btn-sm filesystem-touch-target",
          disabled: pending,
          onClick: () => void reloadTrash(),
        },
        pending ? "重试中…" : "重试",
      ),
    );
  }

  return createElement(
    "div",
    { className: "stack-md min-w-0" },
    createElement(
      "div",
      { className: "filesystem-trash-toolbar flex flex-wrap items-center justify-between gap-2" },
      createElement(
        "p",
        { className: "text-sm text-muted" },
        "当前空间内已删除的项目会保留在此，直到恢复或彻底删除；切换上方空间可查看其他回收站。",
      ),
      createElement(
        "button",
        {
          type: "button",
          className: "btn btn-ghost btn-sm filesystem-touch-target filesystem-file-action-danger",
          disabled: pending || items.length === 0,
          onClick: () => setDialog({ type: "empty" }),
        },
        "清空回收站",
      ),
    ),
    actionError
      ? createElement(InlineAlert, { tone: "error", children: actionError })
      : null,
    batchFailures.length > 0
      ? createElement(
          "div",
          { className: "text-sm text-muted", role: "status" },
          batchFailures.map((failure) =>
            createElement("p", { key: failure.nodeId }, `${failure.nodeId.slice(0, 8)}：${failure.message}`),
          ),
        )
      : null,
    createElement(FileBrowser, {
      nodes,
      mode: "trash",
      trashDetails,
      view,
      selectedIds,
      onSelectionChange: setSelectedIds,
      onOpen: () => undefined,
      onRename: () => undefined,
      onMove: () => undefined,
      onRestore: (selected) => {
        const nodeIds = selected.map((node) => node.id);
        if (nodeIds.length === 1) {
          void submitRestore(nodeIds);
          return;
        }
        setDialog({ type: "restore", nodeIds });
      },
      onPurge: (selected) => setDialog({ type: "purge", nodeIds: selected.map((node) => node.id) }),
      workspaceId,
      namespaceId,
      emptyMessage: "回收站为空。",
    }),
    nextCursor
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-secondary btn-sm filesystem-touch-target",
            disabled: loadingMore,
            onClick: () => void loadMore(),
          },
          loadingMore ? "加载中…" : "加载更多",
        )
      : null,
    createElement(Modal, {
      open: dialog?.type === "restore",
      title: "恢复所选项目",
      description: "恢复后项目会回到原目录。若原目录已有同名项目，恢复将失败。",
      size: "sm",
      onClose: () => {
        if (!pending) setDialog(null);
      },
      footer: restoreDialogFooter,
      children: null,
    }),
    createElement(Modal, {
      open: dialog?.type === "purge",
      title: "彻底删除",
      description: "彻底删除后无法恢复，相关存储配额将被释放。",
      size: "sm",
      onClose: () => {
        if (!pending) setDialog(null);
      },
      footer: purgeDialogFooter,
      children: null,
    }),
    createElement(Modal, {
      open: dialog?.type === "empty",
      title: "清空回收站",
      description: "将彻底删除回收站中的全部可见项目，且无法恢复。",
      size: "sm",
      onClose: () => {
        if (!pending) setDialog(null);
      },
      footer: emptyDialogFooter,
      children: null,
    }),
  );
}