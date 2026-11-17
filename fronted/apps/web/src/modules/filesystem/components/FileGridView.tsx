import { createElement } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { FileActionMenu } from "./FileActionMenu";
import type { FileBrowserMode, TrashNodeDetails } from "./FileBrowser";

export type FileGridViewProps = {
  nodes: FileNodeVM[];
  mode?: FileBrowserMode;
  trashDetails?: Record<string, TrashNodeDetails>;
  selectedIds: ReadonlySet<string>;
  onSelectionChange: (nodeId: string, selected: boolean) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove: (nodes: FileNodeVM[]) => void;
  onDelete?: (nodes: FileNodeVM[]) => void;
  onRestore?: (nodes: FileNodeVM[]) => void;
  onPurge?: (nodes: FileNodeVM[]) => void;
  onDragStart: (event: DragEvent<HTMLElement>, node: FileNodeVM) => void;
  onDragEnd: () => void;
  canRename: boolean;
  canMove: boolean;
  canDelete?: boolean;
  organizationId?: string;
  workspaceId?: string;
  onShare?: (node: FileNodeVM, effectiveActions: GovernedPermission[]) => void;
};

export function FileGridView({
  nodes,
  mode = "browse",
  trashDetails = {},
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
  onDelete,
  onRestore,
  onPurge,
  onDragStart,
  onDragEnd,
  canRename,
  canMove,
  canDelete = false,
  organizationId,
  workspaceId,
  onShare,
}: FileGridViewProps) {
  const trashMode = mode === "trash";
  const selectableState = trashMode ? "TRASHED" : "ACTIVE";

  return createElement(
    "ul",
    { className: "filesystem-file-grid", "aria-label": "文件网格" },
    nodes.map((node) => {
      const trashMeta = trashDetails[node.id];
      return createElement(
        "li",
        {
          key: node.id,
          className: "filesystem-grid-tile",
          draggable: canMove && node.state === "ACTIVE",
          onDragStart: (event: DragEvent<HTMLLIElement>) => onDragStart(event, node),
          onDragEnd,
        },
        createElement(
          "label",
          { className: "filesystem-grid-tile-select filesystem-selection-target" },
          createElement("input", {
            type: "checkbox",
            className: "filesystem-selection-checkbox",
            "aria-label": `选择 ${node.name}`,
            checked: selectedIds.has(node.id),
            disabled: node.state !== selectableState,
            onChange: (event) => onSelectionChange(node.id, event.currentTarget.checked),
          }),
        ),
        trashMode
          ? createElement(
              "div",
              { className: "filesystem-grid-tile-open" },
              createElement(
                "span",
                { className: "filesystem-grid-tile-icon", "aria-hidden": true },
                node.type === "DIRECTORY" ? "▦" : "◧",
              ),
              createElement("span", { className: "filesystem-grid-tile-name" }, node.name),
              createElement(
                "span",
                { className: "filesystem-grid-tile-kind text-muted" },
                trashMeta?.original_path ?? "—",
              ),
            )
          : createElement(
              "button",
              {
                type: "button",
                className: "filesystem-grid-tile-open filesystem-touch-target",
                "aria-label": `打开 ${node.name}`,
                onClick: () => onOpen(node),
              },
              createElement(
                "span",
                { className: "filesystem-grid-tile-icon", "aria-hidden": true },
                node.type === "DIRECTORY" ? "▦" : "◧",
              ),
              createElement("span", { className: "filesystem-grid-tile-name" }, node.name),
              createElement(
                "span",
                { className: "filesystem-grid-tile-kind" },
                node.type === "DIRECTORY" ? "目录" : "文件",
              ),
            ),
        createElement(
          "div",
          { className: "filesystem-grid-tile-actions" },
          trashMode
            ? createElement(
                "div",
                { className: "filesystem-file-actions flex flex-wrap justify-end gap-1 is-compact" },
                onRestore
                  ? createElement(
                      "button",
                      {
                        type: "button",
                        className: "btn btn-ghost btn-sm filesystem-touch-target",
                        onClick: () => onRestore([node]),
                      },
                      "恢复",
                    )
                  : null,
                onPurge
                  ? createElement(
                      "button",
                      {
                        type: "button",
                        className:
                          "btn btn-ghost btn-sm filesystem-touch-target filesystem-file-action-danger",
                        onClick: () => onPurge([node]),
                      },
                      "彻底删除",
                    )
                  : null,
              )
            : createElement(FileActionMenu, {
                node,
                canRename: canRename && node.state === "ACTIVE",
                canMove: canMove && node.state === "ACTIVE",
                canDelete: canDelete && node.state === "ACTIVE",
                organizationId,
                workspaceId,
                onShare,
                onRename,
                onMove,
                onDelete,
                density: "compact",
              }),
        ),
      );
    }),
  );
}