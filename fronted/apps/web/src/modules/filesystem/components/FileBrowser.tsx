"use client";

import { createElement, useEffect, useMemo, useState } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { FileGridView } from "./FileGridView";
import { FileListView } from "./FileListView";
import { clearNodeDragSession, writeNodeDragPayload } from "./directory-dnd";
import type { FileView } from "./workbench-state";

export type FileBrowserMode = "browse" | "trash";

export type TrashNodeDetails = {
  original_path: string;
  deleted_at: string;
};

export type FileBrowserProps = {
  nodes: FileNodeVM[];
  workspaceId?: string;
  namespaceId?: string;
  view: FileView;
  mode?: FileBrowserMode;
  trashDetails?: Record<string, TrashNodeDetails>;
  selectedIds: string[];
  onSelectionChange: (selectedIds: string[]) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
  onDelete?: (nodes: FileNodeVM[]) => void;
  onRestore?: (nodes: FileNodeVM[]) => void;
  onPurge?: (nodes: FileNodeVM[]) => void;
  canRename?: boolean;
  canMove?: boolean;
  canDelete?: boolean;
  organizationId?: string;
  onShare?: (node: FileNodeVM, effectiveActions: GovernedPermission[]) => void;
  emptyMessage?: string;
  listVariant?: "classic" | "data-plane";
};

export function sortFileNodes(nodes: FileNodeVM[]) {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === "DIRECTORY" ? -1 : 1;
    return a.name.localeCompare(b.name, "zh-CN");
  });
}

function selectableState(mode: FileBrowserMode): FileNodeVM["state"] {
  return mode === "trash" ? "TRASHED" : "ACTIVE";
}

export function FileBrowser({
  nodes,
  workspaceId,
  namespaceId,
  view,
  mode = "browse",
  trashDetails = {},
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  onDelete,
  onRestore,
  onPurge,
  canRename = true,
  canMove = true,
  canDelete = false,
  organizationId,
  onShare,
  emptyMessage,
  listVariant = "classic",
}: FileBrowserProps) {
  const [selection, setSelection] = useState(() => new Set(selectedIds));
  const sortedNodes = useMemo(() => sortFileNodes(nodes), [nodes]);
  const expectedState = selectableState(mode);
  const selectableNodeIds = useMemo(
    () => new Set(nodes.filter((node) => node.state === expectedState).map((node) => node.id)),
    [expectedState, nodes],
  );
  const effectiveSelection = useMemo(
    () => new Set([...selection].filter((nodeId) => selectableNodeIds.has(nodeId))),
    [selectableNodeIds, selection],
  );
  const browseMode = mode === "browse";
  const effectiveCanRename = browseMode && canRename;
  const effectiveCanMove = browseMode && canMove;
  const effectiveCanDelete = browseMode && canDelete;

  useEffect(() => {
    const nextSelection = new Set(selectedIds.filter((nodeId) => selectableNodeIds.has(nodeId)));
    setSelection(nextSelection);
    if (
      nextSelection.size !== selectedIds.length ||
      selectedIds.some((nodeId) => !nextSelection.has(nodeId))
    ) {
      onSelectionChange([...nextSelection]);
    }
  }, [onSelectionChange, selectableNodeIds, selectedIds]);

  useEffect(() => clearNodeDragSession, []);

  function changeSelection(nodeId: string, selected: boolean) {
    const nextSelection = new Set(effectiveSelection);
    if (selected) {
      nextSelection.add(nodeId);
    } else {
      nextSelection.delete(nodeId);
    }
    setSelection(nextSelection);
    onSelectionChange([...nextSelection]);
  }

  function selectedNodes() {
    return nodes.filter(
      (node) => node.state === expectedState && effectiveSelection.has(node.id),
    );
  }

  function handleDragStart(event: DragEvent<HTMLElement>, node: FileNodeVM) {
    clearNodeDragSession();
    if (!workspaceId || !namespaceId || !effectiveCanMove || node.state !== "ACTIVE") {
      event.preventDefault();
      return;
    }
    const draggedNodes = effectiveSelection.has(node.id) ? selectedNodes() : [node];
    writeNodeDragPayload(
      event.dataTransfer,
      workspaceId,
      namespaceId,
      draggedNodes,
    );
  }

  if (sortedNodes.length === 0) {
    return createElement(
      "div",
      { className: "empty-state" },
      emptyMessage ?? (mode === "trash" ? "回收站为空。" : "此目录为空。"),
    );
  }

  const viewProps = {
    nodes: sortedNodes,
    mode,
    trashDetails,
    selectedIds: effectiveSelection,
    onSelectionChange: changeSelection,
    onOpen,
    onRename,
    onMove,
    onDelete,
    onRestore,
    onPurge,
    onDragStart: handleDragStart,
    onDragEnd: clearNodeDragSession,
    canRename: effectiveCanRename,
    canMove: effectiveCanMove,
    canDelete: effectiveCanDelete,
    organizationId,
    workspaceId,
    onShare: browseMode ? onShare : undefined,
    listVariant,
  };

  const showBrowseToolbar = browseMode && (effectiveCanMove || effectiveCanDelete) && effectiveSelection.size > 0;
  const showTrashToolbar = !browseMode && effectiveSelection.size > 0;

  return createElement(
    "div",
    { className: "stack-sm" },
    showBrowseToolbar
      ? createElement(
          "div",
          {
            role: "toolbar",
            "aria-label": "所选文件操作",
            className: "filesystem-selection-banner flex flex-wrap items-center justify-between gap-2",
          },
          createElement(
            "span",
            { className: "text-sm text-muted" },
            `已选择 ${effectiveSelection.size} 项`,
          ),
          createElement(
            "div",
            { className: "flex flex-wrap gap-2" },
            effectiveCanMove
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-secondary btn-sm filesystem-touch-target",
                    "aria-label": `移动所选 ${effectiveSelection.size} 项到…`,
                    onClick: () => onMove(selectedNodes()),
                  },
                  "移动到…",
                )
              : null,
            effectiveCanDelete && onDelete
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className:
                      "btn btn-ghost btn-sm filesystem-touch-target filesystem-file-action-danger",
                    "aria-label": `将所选 ${effectiveSelection.size} 项移入回收站`,
                    onClick: () => onDelete(selectedNodes()),
                  },
                  "移入回收站",
                )
              : null,
          ),
        )
      : null,
    showTrashToolbar
      ? createElement(
          "div",
          {
            role: "toolbar",
            "aria-label": "回收站所选操作",
            className: "filesystem-selection-banner flex flex-wrap items-center justify-between gap-2",
          },
          createElement(
            "span",
            { className: "text-sm text-muted" },
            `已选择 ${effectiveSelection.size} 项`,
          ),
          createElement(
            "div",
            { className: "flex flex-wrap gap-2" },
            onRestore
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className: "btn btn-secondary btn-sm filesystem-touch-target",
                    onClick: () => onRestore(selectedNodes()),
                  },
                  "恢复",
                )
              : null,
            onPurge
              ? createElement(
                  "button",
                  {
                    type: "button",
                    className:
                      "btn btn-ghost btn-sm filesystem-touch-target filesystem-file-action-danger",
                    onClick: () => onPurge(selectedNodes()),
                  },
                  "彻底删除",
                )
              : null,
          ),
        )
      : null,
    createElement(view === "grid" ? FileGridView : FileListView, viewProps),
  );
}