import { createElement, type ReactNode } from "react";
import { DataPlaneTrashIcon } from "./data-plane-icons";
import { FileSpaceTabList } from "./FileSpaceTabList";
import type { FileSpace, FileView } from "./workbench-state";

/** 数据面回收站卡片：标题、空间 Tab 与内容区 */
export function DataPlaneTrashShell({
  activeSpace,
  view,
  children,
}: {
  activeSpace: FileSpace;
  view: FileView;
  children: ReactNode;
}) {
  return createElement(
    "section",
    {
      className: "filesystem-data-plane-card filesystem-trash-shell",
      "aria-labelledby": "filesystem-trash-shell-title",
    },
    createElement(
      "header",
      { className: "filesystem-trash-shell-head" },
      createElement(
        "div",
        { className: "filesystem-trash-shell-title-row" },
        createElement(
          "span",
          { className: "filesystem-trash-shell-icon", "aria-hidden": true },
          createElement(DataPlaneTrashIcon),
        ),
        createElement(
          "div",
          { className: "min-w-0" },
          createElement(
            "h2",
            { id: "filesystem-trash-shell-title", className: "filesystem-trash-shell-title" },
            "回收站",
          ),
          createElement(
            "p",
            { className: "filesystem-trash-shell-desc text-muted" },
            "管理已删除的文件，可恢复或永久删除",
          ),
        ),
      ),
    ),
    createElement(FileSpaceTabList, {
      activeSpace,
      view,
      mode: "trash",
      variant: "underline",
    }),
    createElement("div", { className: "filesystem-data-plane-card-divider", role: "presentation" }),
    createElement("div", { className: "filesystem-trash-shell-body" }, children),
    createElement(
      "footer",
      { className: "filesystem-trash-shell-foot text-muted" },
      "删除的文件会暂存在回收站，您可以在此恢复或永久删除",
    ),
  );
}