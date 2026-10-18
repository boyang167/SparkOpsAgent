import type { MenuTreeVM } from "@sparkops/contracts/governance";

export type NavGroupFromMenu = {
  /** 唯一 React key，来自治理菜单 id */
  id: string;
  /** 语义化分组标识，用于图标与默认展开 */
  iconKey: string;
  label: string;
  defaultCollapsed: boolean;
  items: NavLeafFromMenu[];
};

export type NavLeafFromMenu = {
  href: string;
  label: string;
  activeHrefs?: string[];
};

export type SidebarNavEntry =
  | ({ type: "group" } & NavGroupFromMenu)
  | { type: "link"; id: string; href: string; label: string };

const PLUGIN_CONTAINER_PATH = "/plugins";

const KUBERNETES_NAV_SECTIONS: NavLeafFromMenu[] = [
  { href: "/plugins/kubernetes/clusters", label: "集群管理" },
  { href: "/plugins/kubernetes/workloads", label: "工作负载" },
  { href: "/plugins/kubernetes/nodes", label: "节点管理" },
  { href: "/plugins/kubernetes/namespaces", label: "命名空间" },
  {
    href: "/plugins/kubernetes/networks",
    label: "网络与网关",
    activeHrefs: ["/plugins/kubernetes/gateway"],
  },
  {
    href: "/plugins/kubernetes/configs",
    label: "配置与存储",
    activeHrefs: ["/plugins/kubernetes/storages"],
  },
  {
    href: "/plugins/kubernetes/access-controls",
    label: "安全与访问",
    activeHrefs: ["/plugins/kubernetes/records"],
  },
  {
    href: "/plugins/kubernetes/diagnosis",
    label: "运维工具",
    activeHrefs: [
      "/plugins/kubernetes/backup",
      "/plugins/kubernetes/chat",
      "/plugins/kubernetes/cloudtty",
      "/plugins/kubernetes/openkruise",
      "/plugins/kubernetes/kruise-rollouts",
      "/plugins/kubernetes/kubeblocks",
      "/plugins/kubernetes/kubevela",
      "/plugins/kubernetes/velaux",
    ],
  },
];

function visibleChildren(node: MenuTreeVM): MenuTreeVM[] {
  return (node.children ?? []).filter((child) => !child.hidden && child.path.trim().startsWith("/"));
}

function isPluginContainer(node: MenuTreeVM): boolean {
  return node.path.trim() === PLUGIN_CONTAINER_PATH;
}

function isPluginPath(path: string): boolean {
  return path === PLUGIN_CONTAINER_PATH || path.startsWith(`${PLUGIN_CONTAINER_PATH}/`);
}

function iconKeyForNode(node: MenuTreeVM): string {
  if (node.path === "/plugins/kubernetes" || node.path.startsWith("/plugins/kubernetes/")) {
    return "plugin-kubernetes";
  }
  if (node.path === PLUGIN_CONTAINER_PATH) {
    return "plugins";
  }
  return node.icon || node.name || node.path;
}

function toLeaves(nodes: MenuTreeVM[]): NavLeafFromMenu[] {
  return nodes.map((node) => ({ href: node.path.trim(), label: node.title }));
}

function toKubernetesLeaves(nodes: MenuTreeVM[]): NavLeafFromMenu[] {
  const visiblePaths = new Set(nodes.map((node) => node.path.trim()));
  return KUBERNETES_NAV_SECTIONS.flatMap((section) => {
    const sectionPaths = [section.href, ...(section.activeHrefs ?? [])];
    const href = sectionPaths.find((path) => visiblePaths.has(path));
    if (!href) return [];
    return [{
      href,
      label: section.label,
      activeHrefs: sectionPaths.filter((path) => path !== href),
    }];
  });
}

function mergePluginContainers(roots: MenuTreeVM[]): MenuTreeVM[] {
  const mergedChildren: MenuTreeVM[] = [];
  const seenChildIds = new Set<string>();
  const nonPluginRoots: MenuTreeVM[] = [];
  let mergedContainer: MenuTreeVM | null = null;

  const appendChild = (child: MenuTreeVM) => {
    if (seenChildIds.has(child.id)) return;
    seenChildIds.add(child.id);
    mergedChildren.push(child);
  };

  for (const root of roots) {
    if (isPluginContainer(root)) {
      mergedContainer ??= root;
      for (const child of root.children ?? []) {
        appendChild(child);
      }
      continue;
    }
    if (root.path.trim().startsWith(`${PLUGIN_CONTAINER_PATH}/`)) {
      appendChild(root);
      continue;
    }
    nonPluginRoots.push(root);
  }

  if (mergedChildren.length === 0) {
    return nonPluginRoots;
  }

  const container = mergedContainer ?? {
    id: "plugin-container",
    name: "plugins",
    path: PLUGIN_CONTAINER_PATH,
    title: "插件",
    hidden: false,
    sort: 90,
    keep_alive: false,
    children: [],
  };

  return [
    ...nonPluginRoots,
    {
      ...container,
      children: mergedChildren,
    },
  ];
}

function appendPluginEntries(
  container: MenuTreeVM,
  entries: SidebarNavEntry[],
  seenIds: Set<string>,
) {
  const children = visibleChildren(container);
  if (children.length === 0) return;

  const nested = children.filter((child) => visibleChildren(child).length > 0);
  const leaves = children.filter((child) => visibleChildren(child).length === 0);

  for (const pluginRoot of nested) {
    const items = pluginRoot.path === "/plugins/kubernetes"
      ? toKubernetesLeaves(visibleChildren(pluginRoot))
      : toLeaves(visibleChildren(pluginRoot));
    if (items.length === 0 || seenIds.has(pluginRoot.id)) continue;
    seenIds.add(pluginRoot.id);
    entries.push({
      type: "group",
      id: pluginRoot.id,
      iconKey: iconKeyForNode(pluginRoot),
      label: pluginRoot.path === "/plugins/kubernetes" ? "Kubernetes" : pluginRoot.title,
      defaultCollapsed: false,
      items,
    });
  }

  for (const leaf of leaves) {
    const href = leaf.path.trim();
    if (!href.startsWith("/") || seenIds.has(leaf.id)) continue;
    seenIds.add(leaf.id);
    entries.push({
      type: "link",
      id: leaf.id,
      href,
      label: leaf.title,
    });
  }
}

function appendStandardRoot(
  root: MenuTreeVM,
  entries: SidebarNavEntry[],
  seenIds: Set<string>,
) {
  const children = visibleChildren(root);
  if (children.length === 0) {
    const href = root.path.trim();
    if (!href.startsWith("/") || seenIds.has(root.id)) return;
    seenIds.add(root.id);
    entries.push({
      type: "link",
      id: root.id,
      href,
      label: root.title,
    });
    return;
  }

  const nested = children.filter((child) => visibleChildren(child).length > 0);
  const leaves = children.filter((child) => visibleChildren(child).length === 0);

  for (const pluginRoot of nested) {
    const items = toLeaves(visibleChildren(pluginRoot));
    if (items.length === 0 || seenIds.has(pluginRoot.id)) continue;
    seenIds.add(pluginRoot.id);
    entries.push({
      type: "group",
      id: pluginRoot.id,
      iconKey: iconKeyForNode(pluginRoot),
      label: pluginRoot.title,
      defaultCollapsed: false,
      items,
    });
  }

  if (leaves.length > 0) {
    const leafItems = toLeaves(leaves);
    const groupId = `${root.id}:leaves`;
    if (leafItems.length > 0 && !seenIds.has(groupId)) {
      seenIds.add(groupId);
      entries.push({
        type: "group",
        id: groupId,
        iconKey: iconKeyForNode(root),
        label: root.title,
        defaultCollapsed: false,
        items: leafItems,
      });
    }
  }
}

/** 将治理菜单树转为侧栏分组。带子菜单的插件根提升为独立分组，避免只显示「Kubernetes管理」一层。 */
export function menuTreeToNavEntries(tree: MenuTreeVM[]): SidebarNavEntry[] {
  const entries: SidebarNavEntry[] = [];
  const seenIds = new Set<string>();
  const normalizedTree = mergePluginContainers(tree.filter((root) => !root.hidden));

  for (const root of normalizedTree) {
    if (isPluginContainer(root)) {
      appendPluginEntries(root, entries, seenIds);
      continue;
    }
    if (isPluginPath(root.path.trim())) {
      appendPluginEntries(
        { ...root, path: PLUGIN_CONTAINER_PATH, children: [root] },
        entries,
        seenIds,
      );
      continue;
    }
    appendStandardRoot(root, entries, seenIds);
  }

  return entries;
}

export function menuTreeToNavGroups(tree: MenuTreeVM[]): NavGroupFromMenu[] {
  return menuTreeToNavEntries(tree)
    .filter((entry): entry is { type: "group" } & NavGroupFromMenu => entry.type === "group")
    .map(({ type: _type, ...group }) => group)
    .filter((group) => group.items.length > 0);
}

export function isNavHrefActive(
  pathname: string,
  href: string,
  allHrefs: string[],
  activeHrefs: string[] = [],
): boolean {
  const candidates = [href, ...activeHrefs];
  const matches = candidates.some(
    (candidate) => pathname === candidate || pathname.startsWith(`${candidate}/`),
  );
  if (!matches) return false;
  return !allHrefs.some(
    (other) => other !== href && other.startsWith(`${href}/`) && (pathname === other || pathname.startsWith(`${other}/`)),
  );
}
