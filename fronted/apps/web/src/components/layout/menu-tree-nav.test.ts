import { describe, expect, it } from "vitest";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { isNavHrefActive, menuTreeToNavEntries, menuTreeToNavGroups } from "./menu-tree-nav";

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
      ["/plugins/kubernetes/backup", "运维工具"],
    ]);
    expect(menuTreeToNavEntries(tree)).toContainEqual({
      type: "link",
      id: "/plugins/example",
      href: "/plugins/example",
      label: "Example Plugin",
    });
    expect(menuTreeToNavEntries(tree).some((entry) => entry.label === "插件")).toBe(false);
  });

  it("maps kubernetes deep routes to consolidated entries", () => {
    const tree = [
      node({
        path: "/plugins",
        title: "插件",
        children: [
          node({
            path: "/plugins/kubernetes",
            title: "Kubernetes",
            children: [
              node({ path: "/plugins/kubernetes/networks", title: "网络与网关" }),
              node({ path: "/plugins/kubernetes/configs", title: "配置与存储" }),
              node({ path: "/plugins/kubernetes/access-controls", title: "安全与访问" }),
              node({ path: "/plugins/kubernetes/diagnosis", title: "运维工具" }),
            ],
          }),
        ],
      }),
    ];

    const [group] = menuTreeToNavGroups(tree);
    expect(group.items.find((item) => item.href.endsWith("/networks"))?.activeHrefs)
      .toContain("/plugins/kubernetes/gateway");
    expect(group.items.find((item) => item.href.endsWith("/configs"))?.activeHrefs)
      .toContain("/plugins/kubernetes/storages");
    expect(group.items.find((item) => item.href.endsWith("/access-controls"))?.activeHrefs)
      .toContain("/plugins/kubernetes/records");
    expect(group.items.find((item) => item.href.endsWith("/diagnosis"))?.activeHrefs)
      .toContain("/plugins/kubernetes/cloudtty");
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
    expect(group.items[0]?.activeHrefs).toContain("/plugins/kubernetes/access-controls");
    expect(group.items[1]?.activeHrefs).toContain("/plugins/kubernetes/diagnosis");
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
