"use client";

import { createElement, useEffect, useRef, useState } from "react";
import type { DragEvent, KeyboardEvent, MouseEvent, ReactNode } from "react";
import { BffError } from "@sparkops/contracts";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import {
  listFileNodes,
  listGovernedChildren,
} from "@sparkops/web-sdk/filesystem";
import {
  clearNodeDragSession,
  getActiveNodeDragPayload,
  readNodeDragPayload,
  validateNodeDrop,
  type DropDecision,
  type FileNodeDragPayload,
} from "./directory-dnd";

export type DirectoryTreeProps = {
  workspaceId: string;
  namespaceId: string;
  rootNode: FileNodeVM;
  space?: string;
  currentId?: string;
  onNavigate: (node: FileNodeVM) => void;
  onDropNodes: (nodeIds: string[], target: FileNodeVM) => void;
  onSelectionAncestorsChange?: (ancestorIds: ReadonlySet<string>) => void;
  canWrite?: boolean;
  selectionOnly?: boolean;
  organizationId?: string;
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
  space = "workspace",
  ...props
}: DirectoryTreeProps) {
  const scopeKey = `${space}:${props.workspaceId}:${props.namespaceId}:${props.rootNode.id}`;
  return createElement(ScopedDirectoryTree, { key: scopeKey, ...props });
}

function ScopedDirectoryTree({
  workspaceId,
  namespaceId,
  rootNode,
  currentId,
  onNavigate,
  onDropNodes,
  onSelectionAncestorsChange,
  canWrite = false,
  selectionOnly = false,
  organizationId,
}: DirectoryTreeProps) {
  const [states, setStates] = useState<Record<string, NodeState>>({});
  const [focusedId, setFocusedId] = useState(rootNode.id);
  const [dropTarget, setDropTarget] = useState<{
    id: string;
    decision: DropDecision;
  } | null>(null);
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const expandTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function clearExpandTimer() {
    if (expandTimer.current) {
      clearTimeout(expandTimer.current);
      expandTimer.current = null;
    }
  }

  useEffect(
    () => () => {
      clearExpandTimer();
      clearNodeDragSession();
    },
    [],
  );

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
    append(rootNode);
    return visible;
  }

  function ancestorIdsFor(node: FileNodeVM): ReadonlySet<string> {
    const knownNodes = new Map<string, FileNodeVM>([[rootNode.id, rootNode]]);
    Object.values(states).forEach((state) => {
      state.children.forEach((child) => knownNodes.set(child.id, child));
    });
    const ancestorIds = new Set<string>();
    let parentId = node.parent_id;
    while (parentId) {
      ancestorIds.add(parentId);
      parentId = knownNodes.get(parentId)?.parent_id ?? null;
    }
    return ancestorIds;
  }

  function dropDecision(payload: FileNodeDragPayload, target: FileNodeVM): DropDecision {
    const knownNodes = new Map<string, FileNodeVM>([[rootNode.id, rootNode]]);
    Object.values(states).forEach((state) => {
      state.children.forEach((child) => knownNodes.set(child.id, child));
    });
    const draggedIds = payload.nodes.map((node) => node.id);
    const draggedSet = new Set(draggedIds);
    const descendantIds = new Set<string>();
    let current: FileNodeVM | undefined = target;
    while (current?.parent_id) {
      if (draggedSet.has(current.parent_id)) {
        descendantIds.add(target.id);
        break;
      }
      current = knownNodes.get(current.parent_id);
    }
    return validateNodeDrop({
      draggedIds,
      targetId: target.id,
      descendantIds,
      sameWorkspace: payload.workspaceId === workspaceId,
      sameNamespace: payload.namespace_id === namespaceId,
      canWrite,
      allActive: payload.nodes.every((node) => node.state === "ACTIVE"),
    });
  }

  function handleDragEnter(event: DragEvent<HTMLDivElement>, node: FileNodeVM) {
    event.preventDefault();
    event.stopPropagation();
    clearExpandTimer();
    const payload = readNodeDragPayload(event.dataTransfer);
    if (!payload) {
      setDropTarget(null);
      return;
    }
    const decision = dropDecision(payload, node);
    setDropTarget({ id: node.id, decision });
    if (decision.allowed && !stateFor(node.id).expanded) {
      const internalPayload = getActiveNodeDragPayload();
      expandTimer.current = setTimeout(() => {
        expandTimer.current = null;
        if (internalPayload && getActiveNodeDragPayload() !== internalPayload) return;
        void expand(node);
      }, 700);
    }
  }

  function handleDragOver(event: DragEvent<HTMLDivElement>, node: FileNodeVM) {
    event.preventDefault();
    event.stopPropagation();
    const payload = readNodeDragPayload(event.dataTransfer);
    if (!payload) {
      clearExpandTimer();
      setDropTarget(null);
      return;
    }
    const decision = dropDecision(payload, node);
    setDropTarget({ id: node.id, decision });
    if (decision.allowed) {
      event.dataTransfer.dropEffect = "move";
    } else {
      clearExpandTimer();
    }
  }

  function handleDragLeave(event: DragEvent<HTMLDivElement>, node: FileNodeVM) {
    event.stopPropagation();
    if (
      event.relatedTarget instanceof Node &&
      event.currentTarget.contains(event.relatedTarget)
    ) {
      return;
    }
    clearExpandTimer();
    setDropTarget((current) => (current?.id === node.id ? null : current));
  }

  function handleDrop(event: DragEvent<HTMLDivElement>, node: FileNodeVM) {
    event.preventDefault();
    event.stopPropagation();
    clearExpandTimer();
    const payload = readNodeDragPayload(event.dataTransfer);
    const decision = payload ? dropDecision(payload, node) : null;
    clearNodeDragSession();
    setDropTarget(null);
    if (payload && decision?.allowed) {
      onDropNodes(payload.nodes.map((item) => item.id), node);
    }
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
        const page =
          organizationId
            ? await listGovernedChildren(
                namespaceId,
                {
                  parentId: node.id,
                  nodeType: "DIRECTORY",
                  ...(cursor ? { cursor } : {}),
                },
                { organizationId, workspaceId },
              )
            : await listFileNodes(workspaceId, {
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
      if (selectionOnly) onSelectionAncestorsChange?.(ancestorIdsFor(node));
      onNavigate(node);
    }
  }

  function renderNode(node: FileNodeVM): ReactNode {
    const current = stateFor(node.id);
    const activeDrop = dropTarget?.id === node.id ? dropTarget.decision : null;
    const toggle = createElement(
      "button",
      {
        type: "button",
        className: "btn btn-ghost btn-sm filesystem-touch-target",
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
        "aria-selected": selectionOnly ? currentId === node.id : undefined,
        "data-drop-state": activeDrop
          ? activeDrop.allowed
            ? "allowed"
            : "rejected"
          : undefined,
        tabIndex: focusedId === node.id ? 0 : -1,
        className: `filesystem-tree-item${
          activeDrop ? (activeDrop.allowed ? " bg-surface-muted" : " opacity-60") : ""
        }`,
        ref: (element: HTMLDivElement | null) => {
          itemRefs.current[node.id] = element;
        },
        onClick: (event: MouseEvent<HTMLDivElement>) => {
          event.stopPropagation();
          focusNode(node.id);
          if (selectionOnly) onSelectionAncestorsChange?.(ancestorIdsFor(node));
          onNavigate(node);
        },
        onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => handleKeyDown(event, node),
        onDragEnter: (event: DragEvent<HTMLDivElement>) => handleDragEnter(event, node),
        onDragOver: (event: DragEvent<HTMLDivElement>) => handleDragOver(event, node),
        onDragLeave: (event: DragEvent<HTMLDivElement>) => handleDragLeave(event, node),
        onDrop: (event: DragEvent<HTMLDivElement>) => handleDrop(event, node),
      },
      row,
      activeDrop && !activeDrop.allowed
        ? createElement(
            "p",
            { role: "status", className: "px-2 text-sm text-muted" },
            activeDrop.reason,
          )
        : null,
      error,
      childGroup,
    );
  }

  return createElement(
    "div",
    { role: "tree", "aria-label": "目录", className: "filesystem-directory-tree stack-xs" },
    renderNode(rootNode),
  );
}
