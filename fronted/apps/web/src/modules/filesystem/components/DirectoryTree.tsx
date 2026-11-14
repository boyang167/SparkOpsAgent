"use client";

import { createElement, useState } from "react";
import type { KeyboardEvent, ReactNode } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { listFileNodes } from "@sparkops/web-sdk/filesystem";

export type DirectoryTreeProps = {
  workspaceId: string;
  root: FileNodeVM;
  currentId?: string;
  onNavigate: (node: FileNodeVM) => void;
  onDropNodes: (nodeIds: string[], target: FileNodeVM) => void;
};

type NodeState = {
  expanded: boolean;
  loading: boolean;
  childrenLoaded: boolean;
  children: FileNodeVM[];
  error?: string;
};

const initialNodeState: NodeState = {
  expanded: false,
  loading: false,
  childrenLoaded: false,
  children: [],
};

export function DirectoryTree({
  workspaceId,
  root,
  currentId,
  onNavigate,
}: DirectoryTreeProps) {
  const [states, setStates] = useState<Record<string, NodeState>>({});

  function stateFor(nodeId: string): NodeState {
    return states[nodeId] ?? initialNodeState;
  }

  async function expand(node: FileNodeVM) {
    const current = stateFor(node.id);
    if (current.loading) return;
    if (current.childrenLoaded) {
      setStates((value) => ({
        ...value,
        [node.id]: { ...current, expanded: true },
      }));
      return;
    }

    setStates((value) => ({
      ...value,
      [node.id]: { ...current, expanded: true, loading: true, error: undefined },
    }));
    try {
      const page = await listFileNodes(workspaceId, {
        parentId: node.id,
        nodeType: "DIRECTORY",
      });
      setStates((value) => ({
        ...value,
        [node.id]: {
          expanded: true,
          loading: false,
          childrenLoaded: true,
          children: page.items,
        },
      }));
    } catch {
      setStates((value) => ({
        ...value,
        [node.id]: {
          ...stateFor(node.id),
          expanded: true,
          loading: false,
          error: "目录加载失败",
        },
      }));
    }
  }

  function collapse(node: FileNodeVM) {
    const current = stateFor(node.id);
    setStates((value) => ({
      ...value,
      [node.id]: { ...current, expanded: false },
    }));
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>, node: FileNodeVM) {
    const current = stateFor(node.id);
    if (event.key === "ArrowRight") {
      event.preventDefault();
      void expand(node);
    } else if (event.key === "ArrowLeft" && current.expanded) {
      event.preventDefault();
      collapse(node);
    } else if (event.key === "Enter") {
      event.preventDefault();
      onNavigate(node);
    }
  }

  function renderNode(node: FileNodeVM): ReactNode {
    const current = stateFor(node.id);
    const toggle = createElement(
      "button",
      {
        type: "button",
        className: "btn btn-ghost btn-sm",
        "aria-label": `${current.expanded ? "收起" : "展开"} ${node.name}`,
        disabled: current.loading,
        onClick: () => {
          if (current.expanded) {
            collapse(node);
          } else {
            void expand(node);
          }
        },
      },
      createElement("span", { "aria-hidden": true }, current.expanded ? "▾" : "▸"),
    );
    const label = createElement(
      "button",
      {
        type: "button",
        className: "btn btn-ghost btn-sm justify-start",
        onClick: () => onNavigate(node),
      },
      node.name,
    );
    const row = createElement(
      "div",
      { className: "flex items-center gap-1" },
      toggle,
      label,
      current.loading
        ? createElement("span", { role: "status", className: "text-sm text-muted" }, "加载中…")
        : null,
    );
    const error = current.error
      ? createElement("p", { role: "alert", className: "text-sm text-danger" }, current.error)
      : null;
    const childGroup: ReactNode =
      current.expanded && current.childrenLoaded
        ? createElement(
            "div",
            { role: "group", className: "ml-5" },
            current.children.map(renderNode),
          )
        : null;

    return createElement(
      "div",
      {
        key: node.id,
        role: "treeitem",
        "aria-expanded": current.expanded,
        "aria-current": currentId === node.id ? "page" : undefined,
        tabIndex: currentId === node.id || (!currentId && node.id === root.id) ? 0 : -1,
        className: "rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary",
        onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => handleKeyDown(event, node),
      },
      row,
      error,
      childGroup,
    );
  }

  return createElement(
    "div",
    { role: "tree", "aria-label": "目录树", className: "stack-xs" },
    renderNode(root),
  );
}
