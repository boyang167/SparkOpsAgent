import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileListViewProps = {
  nodes: FileNodeVM[];
  selectedIds: ReadonlySet<string>;
  onSelectionChange: (nodeId: string, selected: boolean) => void;
  onOpen: (node: FileNodeVM) => void;
  onRename: (node: FileNodeVM) => void;
  onMove?: (node: FileNodeVM) => void;
};

export function FileListView({
  nodes,
  selectedIds,
  onSelectionChange,
  onOpen,
  onRename,
  onMove,
}: FileListViewProps) {
  return createElement(
    "div",
    { className: "card overflow-hidden" },
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
            { key: node.id },
            createElement(
              "td",
              null,
              createElement("input", {
                type: "checkbox",
                "aria-label": `选择 ${node.name}`,
                checked: selectedIds.has(node.id),
                onChange: (event) => onSelectionChange(node.id, event.currentTarget.checked),
              }),
            ),
            createElement(
              "td",
              null,
              createElement(
                "button",
                {
                  type: "button",
                  className: "font-medium link",
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
              onMove
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
      ),
    ),
  );
}
