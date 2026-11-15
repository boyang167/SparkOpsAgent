"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { createElement } from "react";

const DATA_PLANE_MODULES = [
  { id: "workspace", label: "文件浏览", href: "/files/data/workspace" },
  { id: "transfers", label: "传输中心", href: "/files/data/transfers" },
] as const;

/** 数据面二级导航：文件浏览与传输中心 */
export function DataPlaneModuleNav() {
  const pathname = usePathname();
  const active =
    DATA_PLANE_MODULES.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`))
      ?.id ??
    (pathname.startsWith("/files/data/transfers") ? "transfers" : "workspace");

  return createElement(
    "div",
    {
      role: "navigation",
      "aria-label": "数据面模块",
      className: "filesystem-data-module-nav flex max-w-full flex-wrap gap-2 overflow-x-auto",
    },
    DATA_PLANE_MODULES.map((item) =>
      createElement(
        Link,
        {
          key: item.id,
          href: item.href,
          "aria-current": active === item.id ? "page" : undefined,
          className:
            active === item.id
              ? "btn btn-secondary btn-sm filesystem-touch-target"
              : "btn btn-ghost btn-sm filesystem-touch-target",
        },
        item.label,
      ),
    ),
  );
}