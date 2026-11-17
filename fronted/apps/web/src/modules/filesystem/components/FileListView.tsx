import { createElement } from "react";
import type { DragEvent } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { FileActionMenu } from "./FileActionMenu";
import type { FileBrowserMode, TrashNodeDetails } from "./FileBrowser";

function formatTrashDeletedAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { hour12: false });
}

export type FileListViewProps = {
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

export function FileListView({
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
}: FileListViewProps) {
  const trashMode = mode === "trash";
  const selectableState = trashMode ? "TRASHED" : "ACTIVE";

  return createElement(
    "div",
    { className: "filesystem-table-wrap" },
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
          trashMode ? createElement("th", { className: "text-left" }, "原路径") : null,
          trashMode ? createElement("th", { className: "text-left" }, "删除时间") : null,
          createElement("th", { className: "text-right" }, "操作"),
        ),
      ),
      createElement(
        "tbody",
        null,
        nodes.map((node) => {
          const trashMeta = trashDetails[node.id];
          return createElement(
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
                  disabled: node.state !== selectableState,
                  onChange: (event) =>
                    onSelectionChange(node.id, event.currentTarget.checked),
                }),
              ),
            ),
            createElement(
              "td",
              { className: "filesystem-name-cell" },
              trashMode
                ? createElement("span", { className: "filesystem-name-link" }, node.name)
                : createElement(
                    "button",
                    {
                      type: "button",
                      className: "filesystem-name-link filesystem-touch-target",
                      "aria-label": `打开 ${node.name}`,
                      onClick: () => onOpen(node),
                    },
                    node.name,
                  ),
              node.type === "FILE" && !trashMode
                ? createElement(
                    "span",
                    { className: "filesystem-node-id text-muted" },
                    node.id.slice(0, 8),
                  )
                : null,
            ),
            createElement(
              "td",
              { className: "text-muted" },
              node.type === "DIRECTORY" ? "目录" : "文件",
            ),
            trashMode
              ? createElement(
                  "td",
                  { className: "text-muted" },
                  trashMeta?.original_path ?? "—",
                )
              : null,
            trashMode
              ? createElement(
                  "td",
                  { className: "text-muted" },
                  trashMeta ? formatTrashDeletedAt(trashMeta.deleted_at) : "—",
                )
              : null,
            createElement(
              "td",
              { className: "text-right" },
              trashMode
                ? createElement(
                    "div",
                    { className: "filesystem-file-actions flex flex-wrap justify-end gap-1" },
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
                  }),
            ),
          );
        }),
      ),
    ),
  );
}