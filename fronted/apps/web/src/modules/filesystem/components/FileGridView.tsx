import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileGridViewProps = {
  nodes: FileNodeVM[];
  selectedIds: ReadonlySet<string>;
  onSelectionChange: (nodeId: string, selected: boolean) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (node: FileNodeVM) => void;
  canMove: boolean;
};

export function FileGridView({
  nodes,
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  canMove,
}: FileGridViewProps) {
  return createElement(
    "ul",
    { className: "grid gap-3 sm:grid-cols-2 lg:grid-cols-3", "aria-label": "文件网格" },
    nodes.map((node) =>
      createElement(
        "li",
        { key: node.id, className: "card card-hover stack-sm" },
        createElement(
          "div",
          { className: "flex items-start justify-between gap-3" },
          createElement(
            "button",
            {
              type: "button",
              className: "font-medium link text-left",
              "aria-label": `打开 ${node.name}`,
              onClick: () => onOpen(node),
            },
            node.name,
          ),
          createElement("input", {
            type: "checkbox",
            "aria-label": `选择 ${node.name}`,
            checked: selectedIds.has(node.id),
            onChange: (event) => onSelectionChange(node.id, event.currentTarget.checked),
          }),
        ),
        createElement(
          "span",
          { className: "text-sm text-muted" },
          node.type === "DIRECTORY" ? "目录" : "文件",
        ),
        createElement(
          "div",
          { className: "flex flex-wrap justify-end gap-2" },
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
                  "aria-label": `移动 ${node.name}`,
                  onClick: () => onMove(node),
                },
                "移动",
              )
            : null,
        ),
      ),
    ),
  );
}
