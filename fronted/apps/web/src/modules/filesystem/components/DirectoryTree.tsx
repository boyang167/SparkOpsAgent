"use client";

import { createElement, useRef, useState } from "react";
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { BffError } from "@sparkops/contracts";
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
  const [focusedId, setFocusedId] = useState(root.id);
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({});

  function stateFor(nodeId: string): NodeState {
    return states[nodeId] ?? initialNodeState;
  }

  function focusNode(nodeId: string) {
    setFocusedId(nodeId);
    itemRefs.current[nodeId]?.focus();
  }

  function visibleNodes(): FileNodeVM[] {
    const visible: FileNodeVM[] = [];
    function append(node: FileNodeVM) {
      visible.push(node);
      const state = stateFor(node.id);
      if (state.expanded && state.childrenLoaded) {
        state.children.forEach(append);
      }
    }
    append(root);
    return visible;
  }

  async function expand(node: FileNodeVM) {
    const current = stateFor(node.id);
    if (current.loading) return;
    if (current.childrenLoaded) {
      setStates((value) => ({
        ...value,
        [node.id]: { ...(value[node.id] ?? initialNodeState), expanded: true },
      }));
      return;
    }

    setStates((value) => ({
      ...value,
      [node.id]: {
        ...(value[node.id] ?? initialNodeState),
        expanded: true,
        loading: true,
        error: undefined,
      },
    }));
    try {
      const children: FileNodeVM[] = [];
      const seenCursors = new Set<string>();
      let cursor = "";
      do {
        const page = await listFileNodes(workspaceId, {
          parentId: node.id,
          nodeType: "DIRECTORY",
          ...(cursor ? { cursor } : {}),
        });
        children.push(...page.items);
        cursor = page.next_cursor;
        if (cursor && seenCursors.has(cursor)) {
          throw new Error("repeated directory cursor");
        }
        if (cursor) seenCursors.add(cursor);
      } while (cursor);

      setStates((value) => ({
        ...value,
        [node.id]: {
          ...(value[node.id] ?? initialNodeState),
          loading: false,
          childrenLoaded: true,
          children,
        },
      }));
    } catch (error) {
      setStates((value) => ({
        ...value,
        [node.id]: {
          ...(value[node.id] ?? initialNodeState),
          loading: false,
          error:
            error instanceof BffError && error.status === 403
              ? "无权访问此目录"
              : "目录加载失败",
        },
      }));
    }
  }

  function collapse(node: FileNodeVM) {
    const nodeElement = itemRefs.current[node.id];
    const focusedElement = itemRefs.current[focusedId];
    if (
      focusedId !== node.id &&
      nodeElement &&
      focusedElement &&
      nodeElement.contains(focusedElement)
    ) {
      focusNode(node.id);
    }
    setStates((value) => ({
      ...value,
      [node.id]: { ...(value[node.id] ?? initialNodeState), expanded: false },
    }));
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>, node: FileNodeVM) {
    if (
      ["ArrowDown", "ArrowUp", "ArrowRight", "ArrowLeft", "Enter", " "].includes(event.key)
    ) {
      event.stopPropagation();
    }
    const current = stateFor(node.id);
    const visible = visibleNodes();
    const index = visible.findIndex((item) => item.id === node.id);
    if (event.key === "ArrowDown") {
      event.preventDefault();
      const next = visible[index + 1];
      if (next) focusNode(next.id);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      const previous = visible[index - 1];
      if (previous) focusNode(previous.id);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      if (current.expanded && current.childrenLoaded && current.children[0]) {
        focusNode(current.children[0].id);
      } else if (!current.expanded) {
        void expand(node);
      }
    } else if (event.key === "ArrowLeft") {
      event.preventDefault();
      if (current.expanded) {
        collapse(node);
      } else if (node.parent_id) {
        focusNode(node.parent_id);
      }
    } else if (event.key === "Enter" || event.key === " ") {
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
        tabIndex: -1,
        onClick: (event: MouseEvent) => {
          event.stopPropagation();
          if (current.expanded) {
            collapse(node);
          } else {
            void expand(node);
          }
        },
      },
      createElement("span", { "aria-hidden": true }, current.expanded ? "▾" : "▸"),
    );
    const label = createElement("span", { className: "px-2 text-sm" }, node.name);
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
            current.children.length > 0
              ? current.children.map(renderNode)
              : createElement("p", { role: "status", className: "px-2 text-sm text-muted" }, "空目录"),
          )
        : null;

    return createElement(
      "div",
      {
        key: node.id,
        role: "treeitem",
        "aria-label": node.name,
        "aria-expanded": current.expanded,
        "aria-current": currentId === node.id ? "page" : undefined,
        tabIndex: focusedId === node.id ? 0 : -1,
        className: "rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary",
        ref: (element: HTMLDivElement | null) => {
          itemRefs.current[node.id] = element;
        },
        onClick: (event: MouseEvent<HTMLDivElement>) => {
          event.stopPropagation();
          focusNode(node.id);
          onNavigate(node);
        },
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
