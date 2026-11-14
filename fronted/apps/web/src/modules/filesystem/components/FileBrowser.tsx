"use client";

import { createElement, useEffect, useMemo, useState } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { FileGridView } from "./FileGridView";
import { FileListView } from "./FileListView";
import type { FileView } from "./workbench-state";

export type FileBrowserProps = {
  nodes: FileNodeVM[];
  view: FileView;
  selectedIds: string[];
  onSelectionChange: (selectedIds: string[]) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (node: FileNodeVM) => void;
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

  useEffect(() => {
    setSelection(new Set(selectedIds));
  }, [selectedIds]);

  function changeSelection(nodeId: string, selected: boolean) {
    const nextSelection = new Set(selection);
    if (selected) {
      nextSelection.add(nodeId);
    } else {
      nextSelection.delete(nodeId);
    }
    setSelection(nextSelection);
    onSelectionChange([...nextSelection]);
  }

  if (sortedNodes.length === 0) {
    return createElement("div", { className: "empty-state" }, "此目录为空。");
  }

  const viewProps = {
    nodes: sortedNodes,
    selectedIds: selection,
    onSelectionChange: changeSelection,
    onOpen,
    onRename,
    onMove,
    canMove,
  };

  return createElement(view === "grid" ? FileGridView : FileListView, viewProps);
}
