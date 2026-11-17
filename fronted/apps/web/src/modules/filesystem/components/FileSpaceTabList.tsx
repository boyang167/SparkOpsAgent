"use client";

import Link from "next/link";
import { createElement } from "react";
import { FILE_SPACE_NAV_ITEMS, TRASH_FILE_SPACE_NAV_ITEMS } from "./file-space-nav-items";
import {
  hrefForDataTrash,
  hrefForDataWorkspace,
  type FileSpace,
  type FileView,
} from "./workbench-state";

/** 数据面文件空间切换，样式与控制面治理 Tab 一致 */
export function FileSpaceTabList({
  activeSpace,
  view,
  mode = "browse",
  variant = "buttons",
}: {
  activeSpace: FileSpace;
  view: FileView;
  mode?: "browse" | "trash";
  variant?: "buttons" | "segment" | "underline";
}) {
  const items = mode === "trash" ? TRASH_FILE_SPACE_NAV_ITEMS : FILE_SPACE_NAV_ITEMS;
  const hrefFor = mode === "trash" ? hrefForDataTrash : hrefForDataWorkspace;
  const listClass =
    variant === "segment"
      ? "filesystem-file-space-tabs filesystem-space-segments"
      : variant === "underline"
        ? "filesystem-file-space-tabs filesystem-trash-space-tabs"
        : "filesystem-file-space-tabs flex max-w-full flex-wrap gap-2 overflow-x-auto";

  return createElement(
    "div",
    {
      role: "tablist",
      "aria-label": mode === "trash" ? "回收站所属空间" : "文件空间",
      className: listClass,
    },
    items.map((space) =>
      createElement(
        Link,
        {
          key: space.id,
          href: hrefFor(space.id, view),
          role: "tab",
          "aria-selected": activeSpace === space.id,
          "aria-current": activeSpace === space.id ? "page" : undefined,
          className:
            variant === "segment"
              ? activeSpace === space.id
                ? "filesystem-space-segment is-active filesystem-touch-target"
                : "filesystem-space-segment filesystem-touch-target"
              : variant === "underline"
                ? activeSpace === space.id
                  ? "filesystem-trash-space-tab is-active filesystem-touch-target"
                  : "filesystem-trash-space-tab filesystem-touch-target"
                : activeSpace === space.id
                  ? "btn btn-secondary btn-sm filesystem-touch-target"
                  : "btn btn-ghost btn-sm filesystem-touch-target",
        },
        space.label,
      ),
    ),
  );
}