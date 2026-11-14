import { createElement, useEffect, useState } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { getEffectiveAccess } from "@sparkops/web-sdk/filesystem";

export type FileActionMenuProps = {
  node: FileNodeVM;
  canRename: boolean;
  canMove: boolean;
  organizationId?: string;
  workspaceId?: string;
  onShare?: (node: FileNodeVM, effectiveActions: GovernedPermission[]) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
};

export function FileActionMenu({
  node,
  canRename,
  canMove,
  organizationId,
  workspaceId,
  onShare,
  onRename,
  onMove,
}: FileActionMenuProps) {
  const [effectiveActions, setEffectiveActions] = useState<GovernedPermission[]>([]);

  useEffect(() => {
    if (!organizationId || !workspaceId || !onShare || node.state !== "ACTIVE") {
      setEffectiveActions([]);
      return;
    }
    const controller = new AbortController();
    setEffectiveActions([]);
    void getEffectiveAccess(node.id, {
      organizationId,
      workspaceId,
      signal: controller.signal,
    }).then(
      (result) => {
        if (!controller.signal.aborted) setEffectiveActions(result.effective_actions);
      },
      () => {
        if (!controller.signal.aborted) setEffectiveActions([]);
      },
    );
    return () => controller.abort();
  }, [node.id, node.state, onShare, organizationId, workspaceId]);

  const canShare = effectiveActions.includes("SHARE");
  return createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-1", "aria-label": `${node.name} 操作` },
    canRename
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-ghost btn-sm filesystem-touch-target",
            "aria-label": `重命名 ${node.name}`,
            onClick: () => onRename(node),
          },
          "重命名",
        )
      : null,
    canMove
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-ghost btn-sm filesystem-touch-target",
            "aria-label": `移动到… ${node.name}`,
            onClick: () => onMove([node]),
          },
          "移动到…",
        )
      : null,
    canShare
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-ghost btn-sm filesystem-touch-target",
            "aria-label": `共享 ${node.name}`,
            onClick: () => {
              if (effectiveActions.includes("SHARE")) onShare?.(node, effectiveActions);
            },
          },
          "共享",
        )
      : null,
  );
}
