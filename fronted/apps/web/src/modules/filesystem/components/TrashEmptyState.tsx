import Link from "next/link";
import { createElement } from "react";
import { hrefForDataWorkspace, type FileSpace, type FileView } from "./workbench-state";

function TrashEmptyIllustration() {
  return createElement(
    "svg",
    {
      className: "filesystem-trash-empty-art",
      viewBox: "0 0 120 100",
      fill: "none",
      xmlns: "http://www.w3.org/2000/svg",
      "aria-hidden": true,
    },
    createElement("ellipse", { cx: "60", cy: "88", rx: "42", ry: "6", fill: "rgba(0,0,0,0.06)" }),
    createElement("path", {
      d: "M42 32h36l4 52H38l4-52z",
      fill: "color-mix(in srgb, var(--primary) 15%, white)",
      stroke: "color-mix(in srgb, var(--primary) 40%, transparent)",
      strokeWidth: "1.5",
    }),
    createElement("path", {
      d: "M48 32V26h24v6",
      stroke: "color-mix(in srgb, var(--primary) 50%, transparent)",
      strokeWidth: "1.5",
    }),
    createElement("path", {
      d: "M35 32h50",
      stroke: "color-mix(in srgb, var(--primary) 50%, transparent)",
      strokeWidth: "1.5",
      strokeLinecap: "round",
    }),
    createElement("ellipse", {
      cx: "28",
      cy: "22",
      rx: "10",
      ry: "5",
      fill: "rgba(0,0,0,0.04)",
    }),
    createElement("ellipse", {
      cx: "88",
      cy: "18",
      rx: "12",
      ry: "6",
      fill: "rgba(0,0,0,0.05)",
    }),
  );
}

const SPACE_EMPTY_HINT: Record<FileSpace, string> = {
  my: "当前个人空间没有已删除的文件或目录",
  workspace: "当前工作区没有已删除的文件或目录",
  organization: "当前租户没有已删除的文件或目录",
  shared: "与我共享的空间不支持回收站",
};

/** 回收站无内容时的空状态 */
export function TrashEmptyState({
  fileSpace = "workspace",
  view = "list",
}: {
  fileSpace?: FileSpace;
  view?: FileView;
}) {
  return createElement(
    "div",
    { className: "filesystem-trash-empty product-detail-empty" },
    createElement(
      "div",
      { className: "product-detail-empty-art" },
      createElement(TrashEmptyIllustration),
    ),
    createElement("p", { className: "product-detail-empty-title filesystem-trash-empty-title" }, "回收站为空"),
    createElement(
      "p",
      { className: "product-detail-empty-desc" },
      SPACE_EMPTY_HINT[fileSpace],
    ),
    createElement(
      "div",
      { className: "product-detail-empty-action" },
      createElement(
        Link,
        {
          href: hrefForDataWorkspace(fileSpace === "shared" ? "workspace" : fileSpace, view),
          className: "btn btn-secondary btn-sm filesystem-trash-back-link",
        },
        "返回文件浏览",
      ),
    ),
  );
}