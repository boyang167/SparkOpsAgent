import { createElement, type ReactNode } from "react";
import { FileSpaceTabList } from "./FileSpaceTabList";
import type { FileSpace, FileView } from "./workbench-state";

/** 数据面「文件空间」白卡片：空间 Tab + 内容区 */
export function DataPlaneFileSpaceCard({
  activeSpace,
  view,
  mode = "browse",
  children,
}: {
  activeSpace: FileSpace;
  view: FileView;
  mode?: "browse" | "trash";
  children: ReactNode;
}) {
  return createElement(
    "section",
    {
      className: "filesystem-data-plane-card",
      "aria-labelledby": "filesystem-data-plane-card-title",
    },
    createElement(
      "header",
      { className: "filesystem-data-plane-card-head" },
      createElement(
        "div",
        { className: "min-w-0" },
        createElement(
          "h2",
          {
            id: "filesystem-data-plane-card-title",
            className: "filesystem-data-plane-card-title",
          },
          "文件空间",
        ),
        createElement(
          "p",
          { className: "filesystem-data-plane-card-desc text-muted" },
          "选择文件资源的归属范围",
        ),
      ),
    ),
    createElement(FileSpaceTabList, { activeSpace, view, mode, variant: "segment" }),
    createElement("div", { className: "filesystem-data-plane-card-divider", role: "presentation" }),
    createElement("div", { className: "filesystem-data-plane-card-body" }, children),
  );
}