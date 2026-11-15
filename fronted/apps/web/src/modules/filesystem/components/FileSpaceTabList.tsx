"use client";

import Link from "next/link";
import { createElement } from "react";
import { FILE_SPACE_NAV_ITEMS } from "./file-space-nav-items";
import { hrefForDataWorkspace, type FileSpace, type FileView } from "./workbench-state";

/** 数据面文件空间切换，样式与控制面治理 Tab 一致 */
export function FileSpaceTabList({
  activeSpace,
  view,
}: {
  activeSpace: FileSpace;
  view: FileView;
}) {
  return createElement(
    "div",
    {
      role: "tablist",
      "aria-label": "文件空间",
      className: "filesystem-file-space-tabs flex max-w-full flex-wrap gap-2 overflow-x-auto",
    },
    FILE_SPACE_NAV_ITEMS.map((space) =>
      createElement(
        Link,
        {
          key: space.id,
          href: hrefForDataWorkspace(space.id, view),
          role: "tab",
          "aria-selected": activeSpace === space.id,
          "aria-current": activeSpace === space.id ? "page" : undefined,
          className:
            activeSpace === space.id
              ? "btn btn-secondary btn-sm filesystem-touch-target"
              : "btn btn-ghost btn-sm filesystem-touch-target",
        },
        space.label,
      ),
    ),
  );
}