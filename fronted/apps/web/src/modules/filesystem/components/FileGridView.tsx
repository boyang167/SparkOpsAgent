import { createElement } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { FileActionMenu } from "./FileActionMenu";

export type FileGridViewProps = {
  nodes: FileNodeVM[];
  selectedIds: ReadonlySet<string>;
  onSelectionChange: (nodeId: string, selected: boolean) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
  onDragStart: (event: DragEvent<HTMLElement>, node: FileNodeVM) => void;
  onDragEnd: () => void;
  canMove: boolean;
};

export function FileGridView({
  nodes,
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  onDragStart,
  onDragEnd,
  canMove,
}: FileGridViewProps) {
  return createElement(
    "ul",
    { className: "filesystem-file-grid", "aria-label": "文件网格" },
    nodes.map((node) =>
      createElement(
        "li",
        {
          key: node.id,
          className: "card card-hover stack-sm",
          draggable: canMove && node.state === "ACTIVE",
          onDragStart: (event: DragEvent<HTMLLIElement>) => onDragStart(event, node),
          onDragEnd,
        },
        createElement(
          "div",
          { className: "flex items-start justify-between gap-3" },
          createElement(
            "button",
            {
              type: "button",
              className: "font-medium link text-left filesystem-touch-target",
              "aria-label": `打开 ${node.name}`,
              onClick: () => onOpen(node),
            },
            node.name,
          ),
          createElement(
            "label",
            { className: "filesystem-selection-target" },
            createElement("input", {
              type: "checkbox",
              className: "filesystem-selection-checkbox",
              "aria-label": `选择 ${node.name}`,
              checked: selectedIds.has(node.id),
              disabled: node.state !== "ACTIVE",
              onChange: (event) => onSelectionChange(node.id, event.currentTarget.checked),
            }),
          ),
        ),
        createElement(
          "span",
          { className: "text-sm text-muted" },
          node.type === "DIRECTORY" ? "目录" : "文件",
        ),
        createElement(
          "div",
          { className: "flex flex-wrap justify-end gap-2" },
          createElement(FileActionMenu, {
            node,
            canMove: canMove && node.state === "ACTIVE",
            onRename,
            onMove,
          }),
        ),
      ),
    ),
  );
}
