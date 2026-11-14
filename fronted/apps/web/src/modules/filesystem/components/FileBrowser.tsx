"use client";

import { createElement, useEffect, useMemo, useState } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { FileGridView } from "./FileGridView";
import { FileListView } from "./FileListView";
import { clearNodeDragSession, writeNodeDragPayload } from "./directory-dnd";
import type { FileView } from "./workbench-state";

export type FileBrowserProps = {
  nodes: FileNodeVM[];
  workspaceId?: string;
  view: FileView;
  selectedIds: string[];
  onSelectionChange: (selectedIds: string[]) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
  canMove?: boolean;
};

export function sortFileNodes(nodes: FileNodeVM[]) {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === "DIRECTORY" ? -1 : 1;
    return a.name.localeCompare(b.name, "zh-CN");
  });
}

export function FileBrowser({
  nodes,
  workspaceId,
  view,
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  canMove = true,
}: FileBrowserProps) {
  const [selection, setSelection] = useState(() => new Set(selectedIds));
  const sortedNodes = useMemo(() => sortFileNodes(nodes), [nodes]);
  const activeNodeIds = useMemo(
    () => new Set(nodes.filter((node) => node.state === "ACTIVE").map((node) => node.id)),
    [nodes],
  );
  const effectiveSelection = useMemo(
    () => new Set([...selection].filter((nodeId) => activeNodeIds.has(nodeId))),
    [activeNodeIds, selection],
  );

  useEffect(() => {
    const nextSelection = new Set(selectedIds.filter((nodeId) => activeNodeIds.has(nodeId)));
    setSelection(nextSelection);
    if (
      nextSelection.size !== selectedIds.length ||
      selectedIds.some((nodeId) => !nextSelection.has(nodeId))
    ) {
      onSelectionChange([...nextSelection]);
    }
  }, [activeNodeIds, onSelectionChange, selectedIds]);

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
      (node) => node.state === "ACTIVE" && effectiveSelection.has(node.id),
    );
  }

  function handleDragStart(event: DragEvent<HTMLElement>, node: FileNodeVM) {
    clearNodeDragSession();
    if (!workspaceId || !canMove || node.state !== "ACTIVE") {
      event.preventDefault();
      return;
    }
    const draggedNodes = effectiveSelection.has(node.id) ? selectedNodes() : [node];
    writeNodeDragPayload(
      event.dataTransfer,
      workspaceId,
      draggedNodes,
    );
  }

  if (sortedNodes.length === 0) {
    return createElement("div", { className: "empty-state" }, "此目录为空。");
  }

  const viewProps = {
    nodes: sortedNodes,
    selectedIds: effectiveSelection,
    onSelectionChange: changeSelection,
    onOpen,
    onRename,
    onMove,
    onDragStart: handleDragStart,
    onDragEnd: clearNodeDragSession,
    canMove,
  };

  return createElement(
    "div",
    { className: "stack-sm" },
    canMove && effectiveSelection.size > 0
      ? createElement(
          "div",
          {
            role: "toolbar",
            "aria-label": "所选文件操作",
            className: "flex flex-wrap items-center justify-between gap-2",
          },
          createElement(
            "span",
            { className: "text-sm text-muted" },
            `已选择 ${effectiveSelection.size} 项`,
          ),
          createElement(
            "button",
            {
              type: "button",
              className: "btn btn-secondary btn-sm",
              "aria-label": `移动所选 ${effectiveSelection.size} 项到…`,
              onClick: () => onMove(selectedNodes()),
            },
            "移动到…",
          ),
        )
      : null,
    createElement(view === "grid" ? FileGridView : FileListView, viewProps),
  );
}
