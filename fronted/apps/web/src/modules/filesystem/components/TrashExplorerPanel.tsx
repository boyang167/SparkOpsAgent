"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement, useEffect, useMemo, useState } from "react";
import type { ChangeEvent } from "react";
import type { TrashItemVM } from "@sparkops/contracts/filesystem";
import {
  emptyTrash,
  listTrashItems,
  purgeFileNode,
  restoreFileNode,
} from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { DataPlaneSearchIcon, DataPlaneTrashIcon } from "./data-plane-icons";
import { FileBrowser } from "./FileBrowser";
import { runTrashBatch } from "./file-explorer-panel-trash-ops";
import { TrashEmptyState } from "./TrashEmptyState";
import type { FileSpace, FileView } from "./workbench-state";

type TrashDialog =
  | { type: "restore"; nodeIds: string[] }
  | { type: "purge"; nodeIds: string[] }
  | { type: "empty" }
  | null;

type TrashTypeFilter = "all" | "DIRECTORY" | "FILE";
type TrashSortOrder = "deleted_desc" | "deleted_asc" | "name";

export type TrashExplorerPanelProps = {
  workspaceId: string;
  namespaceId: string;
  initialItems: TrashItemVM[];
  initialNextCursor?: string;
  initialLoadError?: boolean;
  view?: FileView;
  fileSpace?: FileSpace;
};

export function TrashExplorerPanel({
  workspaceId,
  namespaceId,
  initialItems,
  initialNextCursor = "",
  initialLoadError = false,
  view = "list",
  fileSpace = "workspace",
}: TrashExplorerPanelProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [items, setItems] = useState(initialItems);
  const [searchQuery, setSearchQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState<TrashTypeFilter>("all");
  const [sortOrder, setSortOrder] = useState<TrashSortOrder>("deleted_desc");
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

  const filteredItems = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    let next = items;
    if (query) {
      next = next.filter(
        (item) =>
          item.node.name.toLowerCase().includes(query) ||
          item.original_path.toLowerCase().includes(query),
      );
    }
    if (typeFilter !== "all") {
      next = next.filter((item) => item.node.type === typeFilter);
    }
    const sorted = [...next];
    if (sortOrder === "name") {
      sorted.sort((a, b) => a.node.name.localeCompare(b.node.name, "zh-CN"));
    } else {
      sorted.sort((a, b) => {
        const left = new Date(a.deleted_at).getTime();
        const right = new Date(b.deleted_at).getTime();
        if (Number.isNaN(left) || Number.isNaN(right)) return 0;
        return sortOrder === "deleted_desc" ? right - left : left - right;
      });
    }
    return sorted;
  }, [items, searchQuery, sortOrder, typeFilter]);

  const trashDetails = useMemo(
    () =>
      Object.fromEntries(
        filteredItems.map((item) => [
          item.node.id,
          { original_path: item.original_path, deleted_at: item.deleted_at },
        ]),
      ),
    [filteredItems],
  );
  const nodes = useMemo(() => filteredItems.map((item) => item.node), [filteredItems]);

  function selectView(nextView: FileView) {
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.set("view", nextView);
    router.replace(`${pathname}?${nextSearchParams.toString()}`);
  }

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

  const listContent =
    items.length === 0
      ? createElement(TrashEmptyState, { fileSpace, view })
      : nodes.length === 0
        ? createElement("div", { className: "empty-state filesystem-trash-filter-empty" }, "没有符合筛选条件的项目。")
        : createElement(
            FileBrowser,
            {
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
              onPurge: (selected) =>
                setDialog({ type: "purge", nodeIds: selected.map((node) => node.id) }),
              workspaceId,
              namespaceId,
            },
          );

  return createElement(
    "div",
    { className: "filesystem-trash-panel stack-md min-w-0" },
    createElement(
      "div",
      { className: "filesystem-trash-info-banner", role: "status" },
      createElement(
        "p",
        { className: "filesystem-trash-info-text" },
        "当前空间内已删除的项目会保留在此，恢复或彻底删除后将不再显示。",
      ),
      createElement(
        "button",
        {
          type: "button",
          className: "btn btn-ghost btn-sm filesystem-trash-empty-btn filesystem-touch-target",
          disabled: pending || items.length === 0,
          onClick: () => setDialog({ type: "empty" }),
        },
        createElement(DataPlaneTrashIcon),
        "清空回收站",
      ),
    ),
    createElement(
      "div",
      { className: "filesystem-trash-command-bar" },
      createElement(
        "label",
        { className: "filesystem-trash-search" },
        createElement(DataPlaneSearchIcon),
        createElement("input", {
          type: "search",
          className: "input input-sm filesystem-trash-search-input",
          "aria-label": "搜索回收站",
          placeholder: "搜索回收站文件或目录",
          value: searchQuery,
          onChange: (event: ChangeEvent<HTMLInputElement>) => setSearchQuery(event.currentTarget.value),
        }),
      ),
      createElement(
        "div",
        { className: "filesystem-trash-command-meta" },
        createElement(
          "select",
          {
            className: "input input-sm filesystem-trash-filter-select",
            "aria-label": "类型筛选",
            value: typeFilter,
            onChange: (event: ChangeEvent<HTMLSelectElement>) =>
              setTypeFilter(event.currentTarget.value as TrashTypeFilter),
          },
          createElement("option", { value: "all" }, "全部类型"),
          createElement("option", { value: "DIRECTORY" }, "文件夹"),
          createElement("option", { value: "FILE" }, "文件"),
        ),
        createElement(
          "select",
          {
            className: "input input-sm filesystem-trash-filter-select",
            "aria-label": "删除时间排序",
            value: sortOrder,
            onChange: (event: ChangeEvent<HTMLSelectElement>) =>
              setSortOrder(event.currentTarget.value as TrashSortOrder),
          },
          createElement("option", { value: "deleted_desc" }, "删除时间"),
          createElement("option", { value: "deleted_asc" }, "最早删除"),
          createElement("option", { value: "name" }, "名称"),
        ),
        createElement(
          "div",
          {
            className: "filesystem-view-toggle filesystem-view-toggle--segment",
            role: "group",
            "aria-label": "视图切换",
          },
          createElement(
            "button",
            {
              type: "button",
              className: view === "list" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
              "aria-label": "列表视图",
              "aria-pressed": view === "list",
              onClick: () => selectView("list"),
            },
            "列表",
          ),
          createElement(
            "button",
            {
              type: "button",
              className: view === "grid" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
              "aria-label": "网格视图",
              "aria-pressed": view === "grid",
              onClick: () => selectView("grid"),
            },
            "网格",
          ),
        ),
      ),
    ),
    actionError
      ? createElement(InlineAlert, { tone: "error", children: actionError })
      : null,
    batchFailures.length > 0
      ? createElement(
          "div",
          { className: "text-sm text-muted px-3", role: "status" },
          batchFailures.map((failure) =>
            createElement("p", { key: failure.nodeId }, `${failure.nodeId.slice(0, 8)}：${failure.message}`),
          ),
        )
      : null,
    createElement("div", { className: "filesystem-trash-list-area" }, listContent),
    nextCursor
      ? createElement(
          "div",
          { className: "filesystem-trash-load-more px-3 pb-3" },
          createElement(
            "button",
            {
              type: "button",
              className: "btn btn-secondary btn-sm filesystem-touch-target",
              disabled: loadingMore,
              onClick: () => void loadMore(),
            },
            loadingMore ? "加载中…" : "加载更多",
          ),
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