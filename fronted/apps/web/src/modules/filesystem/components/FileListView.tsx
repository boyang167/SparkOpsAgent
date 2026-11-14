import { createElement } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { FileActionMenu } from "./FileActionMenu";

export type FileListViewProps = {
  nodes: FileNodeVM[];
  selectedIds: ReadonlySet<string>;
  onSelectionChange: (nodeId: string, selected: boolean) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
  onDragStart: (event: DragEvent<HTMLElement>, node: FileNodeVM) => void;
  onDragEnd: () => void;
  canRename: boolean;
  canMove: boolean;
  organizationId?: string;
  workspaceId?: string;
  onShare?: (node: FileNodeVM, effectiveActions: GovernedPermission[]) => void;
};

export function FileListView({
  nodes,
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  onDragStart,
  onDragEnd,
  canRename,
  canMove,
  organizationId,
  workspaceId,
  onShare,
}: FileListViewProps) {
  return createElement(
    "div",
    { className: "card filesystem-table-wrap" },
    createElement(
      "table",
      { className: "data-table w-full" },
      createElement(
        "thead",
        null,
        createElement(
          "tr",
          null,
          createElement("th", { "aria-label": "选择" }),
          createElement("th", { className: "text-left" }, "名称"),
          createElement("th", { className: "text-left" }, "类型"),
          createElement("th", { className: "text-right" }, "操作"),
        ),
      ),
      createElement(
        "tbody",
        null,
        nodes.map((node) =>
          createElement(
            "tr",
            {
              key: node.id,
              draggable: canMove && node.state === "ACTIVE",
              onDragStart: (event: DragEvent<HTMLTableRowElement>) =>
                onDragStart(event, node),
              onDragEnd,
            },
            createElement(
              "td",
              null,
              createElement(
                "label",
                { className: "filesystem-selection-target" },
                createElement("input", {
                  type: "checkbox",
                  className: "filesystem-selection-checkbox",
                  "aria-label": `选择 ${node.name}`,
                  checked: selectedIds.has(node.id),
                  disabled: node.state !== "ACTIVE",
                  onChange: (event) =>
                    onSelectionChange(node.id, event.currentTarget.checked),
                }),
              ),
            ),
            createElement(
              "td",
              null,
              createElement(
                "button",
                {
                  type: "button",
                  className: "font-medium link filesystem-touch-target",
                  "aria-label": `打开 ${node.name}`,
                  onClick: () => onOpen(node),
                },
                node.name,
              ),
            ),
            createElement(
              "td",
              { className: "text-muted" },
              node.type === "DIRECTORY" ? "目录" : "文件",
            ),
            createElement(
              "td",
              { className: "text-right" },
              createElement(FileActionMenu, {
                node,
                canRename: canRename && node.state === "ACTIVE",
                canMove: canMove && node.state === "ACTIVE",
                organizationId,
                workspaceId,
                onShare,
                onRename,
                onMove,
              }),
            ),
          ),
        ),
      ),
    ),
  );
}
