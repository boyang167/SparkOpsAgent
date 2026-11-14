import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileActionMenuProps = {
  node: FileNodeVM;
  canRename: boolean;
  canMove: boolean;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
};

export function FileActionMenu({
  node,
  canRename,
  canMove,
  onRename,
  onMove,
}: FileActionMenuProps) {
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
  );
}
