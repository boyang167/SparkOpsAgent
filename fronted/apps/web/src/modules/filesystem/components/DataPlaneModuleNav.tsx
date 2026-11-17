"use client";

import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { createElement, useMemo } from "react";
import {
  DataPlaneBrowseIcon,
  DataPlaneTransferIcon,
  DataPlaneTrashIcon,
} from "./data-plane-icons";
import { hrefForDataTrash, hrefForDataWorkspace } from "./workbench-state";

const DATA_PLANE_MODULES = [
  { id: "workspace", label: "文件浏览", href: "/files/data/workspace", Icon: DataPlaneBrowseIcon },
  { id: "trash", label: "回收站", href: "/files/data/trash", Icon: DataPlaneTrashIcon },
  { id: "transfers", label: "传输中心", href: "/files/data/transfers", Icon: DataPlaneTransferIcon },
] as const;

/** 数据面二级导航：文件浏览与传输中心 */
export function DataPlaneModuleNav() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const view = searchParams.get("view") === "grid" ? "grid" : "list";
  const spaceParam = searchParams.get("space");
  const space =
    spaceParam === "my" || spaceParam === "organization" || spaceParam === "shared"
      ? spaceParam
      : "workspace";
  const moduleHrefs = useMemo(
    () => ({
      workspace: hrefForDataWorkspace(space, view),
      trash: hrefForDataTrash(space, view),
      transfers: "/files/data/transfers",
    }),
    [space, view],
  );
  const active =
    DATA_PLANE_MODULES.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`))
      ?.id ??
    (pathname.startsWith("/files/data/trash")
      ? "trash"
      : pathname.startsWith("/files/data/transfers")
        ? "transfers"
        : "workspace");

  return createElement(
    "div",
    {
      role: "navigation",
      "aria-label": "数据面模块",
      className: "filesystem-data-module-nav",
    },
    DATA_PLANE_MODULES.map((item) =>
      createElement(
        Link,
        {
          key: item.id,
          href: moduleHrefs[item.id],
          "aria-current": active === item.id ? "page" : undefined,
          className:
            active === item.id
              ? "filesystem-data-module-tab is-active filesystem-touch-target"
              : "filesystem-data-module-tab filesystem-touch-target",
        },
        createElement(item.Icon),
        item.label,
      ),
    ),
  );
}