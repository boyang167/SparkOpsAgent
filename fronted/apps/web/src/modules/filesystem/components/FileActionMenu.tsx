import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileActionMenuProps = {
  node: FileNodeVM;
  canMove: boolean;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
};

export function FileActionMenu({
  node,
  canMove,
  onRename,
  onMove,
}: FileActionMenuProps) {
  return createElement(
    "div",
    { className: "flex flex-wrap justify-end gap-1", "aria-label": `${node.name} 操作` },
    createElement(
      "button",
      {
        type: "button",
        className: "btn btn-ghost btn-sm",
        "aria-label": `重命名 ${node.name}`,
        onClick: () => onRename(node),
      },
      "重命名",
    ),
    canMove
      ? createElement(
          "button",
          {
            type: "button",
            className: "btn btn-ghost btn-sm",
            "aria-label": `移动到… ${node.name}`,
            onClick: () => onMove([node]),
          },
          "移动到…",
        )
      : null,
  );
}
