import { describe, expect, it } from "vitest";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { DEFAULT_ENTITLEMENTS } from "./entitlements";
import {
  filterSidebarNavEntries,
  isNavHrefActive,
  menuTreeToNavEntries,
  menuTreeToNavGroups,
} from "./menu-tree-nav";

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
  it("keeps root leaf menus as direct entries", () => {
    const tree = [
      node({ path: "/knowledge", title: "知识库", sort: 95 }),
    ];

    expect(menuTreeToNavEntries(tree)).toEqual([
      { type: "link", id: "/knowledge", href: "/knowledge", label: "知识库" },
    ]);
  });

  it("drops files menu links when filesystem entitlement is disabled", () => {
    const tree = [
      node({ path: "/files", title: "文件", sort: 96 }),
    ];
    const entries = menuTreeToNavEntries(tree);
    const filtered = filterSidebarNavEntries(entries, {
      ...DEFAULT_ENTITLEMENTS,
      filesystem: false,
    });
    expect(filtered).toEqual([]);
  });

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

    const groups = menuTreeToNavGroups(tree);
    expect(groups[0]).toMatchObject({
      id: "/plugins/kubernetes",
      iconKey: "plugin-kubernetes",
      label: "Kubernetes",
      defaultCollapsed: false,
    });
    expect(groups[0]?.items.map((item) => [item.href, item.label])).toEqual([
      ["/plugins/kubernetes/clusters", "集群管理"],
      ["/plugins/kubernetes/nodes", "节点管理"],
      ["/plugins/kubernetes/backup", "Velero备份"],
    ]);
    expect(menuTreeToNavEntries(tree)).toContainEqual({
      type: "link",
      id: "/plugins/example",
      href: "/plugins/example",
      label: "Example Plugin",
    });
    expect(menuTreeToNavEntries(tree).some((entry) => entry.label === "插件")).toBe(false);
  });

  it("maps kubernetes menu folders to expandable sidebar submenus", () => {
    const tree = [
      node({
        path: "/plugins",
        title: "插件",
        children: [
          node({
            path: "/plugins/kubernetes",
            title: "Kubernetes",
            children: [
              node({
                path: "/plugins/kubernetes/ops-tools",
                title: "运维工具",
                children: [
                  node({ path: "/plugins/kubernetes/diagnosis", title: "应用诊断" }),
                  node({ path: "/plugins/kubernetes/cloudtty", title: "CloudTTY" }),
                ],
              }),
            ],
          }),
        ],
      }),
    ];

    const [group] = menuTreeToNavGroups(tree);
    const opsTools = group.items.find((item) => item.label === "运维工具");
    expect(opsTools?.children?.map((child) => child.href)).toEqual([
      "/plugins/kubernetes/diagnosis",
      "/plugins/kubernetes/cloudtty",
    ]);
  });

  it("keeps a consolidated entry when the member only has a deep menu role", () => {
    const tree = [
      node({
        path: "/plugins",
        title: "插件",
        children: [
          node({
            path: "/plugins/kubernetes",
            title: "Kubernetes",
            children: [
              node({ path: "/plugins/kubernetes/records", title: "终端审计" }),
              node({ path: "/plugins/kubernetes/chat", title: "AI Chat" }),
            ],
          }),
        ],
      }),
    ];

    const [group] = menuTreeToNavGroups(tree);
    expect(group.items.map((item) => item.href)).toEqual([
      "/plugins/kubernetes/records",
      "/plugins/kubernetes/chat",
    ]);
    expect(group.items[0]?.children).toBeUndefined();
    expect(group.items[1]?.children).toBeUndefined();
  });

  it("collapses duplicate empty plugin containers", () => {
    const tree = [
      node({ id: "menu-plugins-1", path: "/plugins", title: "插件", name: "" }),
      node({ id: "menu-plugins-2", path: "/plugins", title: "插件", name: "" }),
      node({ id: "menu-plugins-3", path: "/plugins", title: "插件", name: "" }),
    ];

    expect(menuTreeToNavEntries(tree)).toEqual([]);
  });

  it("merges duplicate plugin containers into one kubernetes group", () => {
    const tree = [
      node({
        id: "menu-plugins-1",
        path: "/plugins",
        title: "插件",
        children: [
          node({
            id: "menu-k8s",
            path: "/plugins/kubernetes",
            title: "Kubernetes",
            children: [
              node({ id: "menu-clusters", path: "/plugins/kubernetes/clusters", title: "集群管理" }),
            ],
          }),
        ],
      }),
      node({ id: "menu-plugins-2", path: "/plugins", title: "插件" }),
      node({ id: "menu-plugins-3", path: "/plugins", title: "插件" }),
    ];

    const groups = menuTreeToNavGroups(tree);
    expect(groups).toHaveLength(1);
    expect(groups[0]).toMatchObject({
      id: "menu-k8s",
      iconKey: "plugin-kubernetes",
      label: "Kubernetes",
    });
    expect(groups[0]?.items.map((item) => item.href)).toEqual(["/plugins/kubernetes/clusters"]);
  });

  it("promotes kubernetes when plugin container root is hidden", () => {
    const tree = [
      node({
        id: "menu-plugins",
        path: "/plugins",
        title: "插件",
        hidden: true,
        children: [
          node({
            id: "menu-k8s",
            path: "/plugins/kubernetes",
            title: "Kubernetes管理",
            children: [
              node({ id: "menu-clusters", path: "/plugins/kubernetes/clusters", title: "集群管理" }),
            ],
          }),
        ],
      }),
    ];

    const groups = menuTreeToNavGroups(tree);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe("Kubernetes");
    expect(groups[0]?.items.map((item) => item.href)).toContain("/plugins/kubernetes/clusters");
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
        id: "/delivery:leaves",
        iconKey: "ops-delivery",
        label: "持续交付",
        defaultCollapsed: false,
        items: [{ href: "/delivery/builds", label: "构建管理" }],
      },
    ]);
  });
});

describe("isNavHrefActive", () => {
  it("matches configured alias paths", () => {
    expect(isNavHrefActive(
      "/plugins/kubernetes/gateway",
      "/plugins/kubernetes/networks",
      ["/plugins/kubernetes/networks"],
      ["/plugins/kubernetes/gateway"],
    )).toBe(true);
  });

  it("prefers the longest matching href", () => {
    const hrefs = ["/plugins/kubernetes", "/plugins/kubernetes/clusters"];
    expect(isNavHrefActive("/plugins/kubernetes/clusters", "/plugins/kubernetes", hrefs)).toBe(false);
    expect(isNavHrefActive("/plugins/kubernetes/clusters", "/plugins/kubernetes/clusters", hrefs)).toBe(true);
  });

  it("keeps 产品管理 inactive on /products/workflows", () => {
    const hrefs = ["/products", "/products/workflows"];
    expect(isNavHrefActive("/products/workflows", "/products", hrefs)).toBe(false);
    expect(isNavHrefActive("/products/workflows", "/products/workflows", hrefs)).toBe(true);
    expect(isNavHrefActive("/products", "/products", hrefs)).toBe(true);
  });
});
