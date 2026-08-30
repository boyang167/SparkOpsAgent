import { describe, expect, it } from "vitest";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { isNavHrefActive, menuTreeToNavGroups } from "./menu-tree-nav";

function node(partial: Partial<MenuTreeVM> & Pick<MenuTreeVM, "path" | "title">): MenuTreeVM {
  return {
    id: partial.id ?? partial.path,
    name: partial.name ?? partial.path,
    path: partial.path,
    title: partial.title,
    hidden: partial.hidden ?? false,
    sort: partial.sort ?? 0,
    keep_alive: false,
    icon: partial.icon,
    children: partial.children,
  };
}

describe("menuTreeToNavGroups", () => {
  it("promotes kubernetes plugin root so children appear as sidebar items", () => {
    const tree = [
      node({
        path: "/plugins",
        title: "插件",
        children: [
          node({
            path: "/plugins/kubernetes",
            title: "Kubernetes管理",
            children: [
              node({ path: "/plugins/kubernetes/clusters", title: "集群管理" }),
              node({ path: "/plugins/kubernetes/nodes", title: "节点管理" }),
              node({ path: "/plugins/kubernetes/backup", title: "Velero备份" }),
            ],
          }),
          node({ path: "/plugins/example", title: "Example Plugin" }),
        ],
      }),
    ];

    expect(menuTreeToNavGroups(tree)).toEqual([
      {
        id: "plugin-kubernetes",
        label: "Kubernetes管理",
        defaultCollapsed: false,
        items: [
          { href: "/plugins/kubernetes/clusters", label: "集群管理" },
          { href: "/plugins/kubernetes/nodes", label: "节点管理" },
          { href: "/plugins/kubernetes/backup", label: "Velero备份" },
        ],
      },
      {
        id: "/plugins",
        label: "插件",
        defaultCollapsed: false,
        items: [{ href: "/plugins/example", label: "Example Plugin" }],
      },
    ]);
  });

  it("keeps one-level groups unchanged", () => {
    const tree = [
      node({
        path: "/delivery",
        title: "持续交付",
        icon: "ops-delivery",
        children: [node({ path: "/delivery/builds", title: "构建管理" })],
      }),
    ];
    expect(menuTreeToNavGroups(tree)).toEqual([
      {
        id: "ops-delivery",
        label: "持续交付",
        defaultCollapsed: false,
        items: [{ href: "/delivery/builds", label: "构建管理" }],
      },
    ]);
  });
});

describe("isNavHrefActive", () => {
  it("prefers the longest matching href", () => {
    const hrefs = ["/plugins/kubernetes", "/plugins/kubernetes/clusters"];
    expect(isNavHrefActive("/plugins/kubernetes/clusters", "/plugins/kubernetes", hrefs)).toBe(false);
    expect(isNavHrefActive("/plugins/kubernetes/clusters", "/plugins/kubernetes/clusters", hrefs)).toBe(true);
  });
});
