"use client";

import { createElement, Fragment, useEffect, useState } from "react";
import type { ReactElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { moveFileNode } from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { DirectoryTree } from "./DirectoryTree";
import { validateNodeDrop } from "./directory-dnd";

export type MoveNodesModalProps = {
  open: boolean;
  workspaceId: string;
  root: FileNodeVM;
  nodes: FileNodeVM[];
  onClose: () => void;
  onMoved: (nodes: FileNodeVM[], target: FileNodeVM) => void;
  onRefreshDirectories: (directoryIds: string[]) => void;
};

export function MoveNodesModal({
  open,
  workspaceId,
  root,
  nodes,
  onClose,
  onMoved,
  onRefreshDirectories,
}: MoveNodesModalProps): ReactElement {
  const [target, setTarget] = useState<FileNodeVM | null>(null);
  const [targetAncestorIds, setTargetAncestorIds] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setTarget(null);
      setTargetAncestorIds(new Set());
      setError(null);
      setPending(false);
    }
  }, [open]);

  const decision = target
    ? validateNodeDrop({
        draggedIds: nodes.map((node) => node.id),
        targetId: target.id,
        descendantIds: new Set(
          nodes.some((node) => targetAncestorIds.has(node.id)) ? [target.id] : [],
        ),
        sameWorkspace: true,
        canWrite: true,
        allActive: nodes.every((node) => node.state === "ACTIVE"),
      })
    : null;

  async function submit() {
    if (!target || !decision?.allowed || pending) return;
    setPending(true);
    setError(null);
    try {
      for (const node of nodes) {
        await moveFileNode(workspaceId, node.id, target.id, node.name);
      }
      onMoved(nodes, target);
      onClose();
    } catch {
      const directoryIds = new Set(
        nodes
          .map((node) => node.parent_id)
          .filter((id): id is string => Boolean(id)),
      );
      directoryIds.add(target.id);
      onRefreshDirectories([...directoryIds]);
      setError("移动失败，已刷新源目录和目标目录，请重试。");
    } finally {
      setPending(false);
    }
  }

  const footer = createElement(
    Fragment,
    null,
    createElement(
      "button",
      { type: "button", className: "btn btn-ghost", disabled: pending, onClick: onClose },
      "取消",
    ),
    createElement(
      "button",
      {
        type: "button",
        className: "btn btn-secondary",
        disabled: !decision?.allowed || pending,
        onClick: () => void submit(),
      },
      pending ? "移动中…" : "移动到此处",
    ),
  );

  const content = createElement(
    "div",
    { className: "stack-sm" },
    createElement(DirectoryTree, {
      workspaceId,
      root,
      currentId: target?.id,
      onNavigate: (node: FileNodeVM) => {
        setTarget(node);
        setError(null);
      },
      onSelectionAncestorsChange: setTargetAncestorIds,
      onDropNodes: () => {},
      canWrite: false,
      selectionOnly: true,
    }),
    createElement(
      "p",
      { className: "text-sm text-muted" },
      target ? `目标：${target.name}` : "请从目录树中选择目标目录。",
    ),
    decision && !decision.allowed
      ? createElement(
          "div",
          { role: "alert" },
          createElement(InlineAlert, { tone: "warning", children: decision.reason }),
        )
      : null,
    error
      ? createElement(
          "div",
          { role: "alert" },
          createElement(InlineAlert, { tone: "error", children: error }),
        )
      : null,
  );

  return createElement(Modal, {
    open,
    onClose: pending ? () => {} : onClose,
    title: "移动到…",
    description: `为 ${nodes.length} 项选择目标目录。`,
    size: "lg",
    footer,
    children: content,
  });
}
