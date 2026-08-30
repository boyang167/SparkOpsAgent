import type { MenuTreeVM } from "@sparkops/contracts/governance";

export type NavGroupFromMenu = {
  id: string;
  label: string;
  defaultCollapsed: boolean;
  items: Array<{ href: string; label: string }>;
};

function visibleChildren(node: MenuTreeVM): MenuTreeVM[] {
  return (node.children ?? []).filter((child) => !child.hidden && child.path.startsWith("/"));
}

function groupId(node: MenuTreeVM): string {
  if (node.path === "/plugins/kubernetes" || node.path.startsWith("/plugins/kubernetes/")) {
    return "plugin-kubernetes";
  }
  return node.icon || node.name || node.path;
}

function toLeaves(nodes: MenuTreeVM[]): Array<{ href: string; label: string }> {
  return nodes.map((node) => ({ href: node.path, label: node.title }));
}

/** 将治理菜单树转为侧栏分组。带子菜单的插件根提升为独立分组，避免只显示「Kubernetes管理」一层。 */
export function menuTreeToNavGroups(tree: MenuTreeVM[]): NavGroupFromMenu[] {
  const groups: NavGroupFromMenu[] = [];
  for (const root of tree) {
    if (root.hidden) continue;
    const children = visibleChildren(root);
    if (children.length === 0) continue;

    const nested = children.filter((child) => visibleChildren(child).length > 0);
    const leaves = children.filter((child) => visibleChildren(child).length === 0);

    for (const pluginRoot of nested) {
      groups.push({
        id: groupId(pluginRoot),
        label: pluginRoot.title,
        defaultCollapsed: false,
        items: toLeaves(visibleChildren(pluginRoot)),
      });
    }
    if (leaves.length > 0) {
      groups.push({
        id: groupId(root),
        label: root.title,
        defaultCollapsed: false,
        items: toLeaves(leaves),
      });
    }
  }
  return groups.filter((group) => group.items.length > 0);
}

export function isNavHrefActive(pathname: string, href: string, allHrefs: string[]): boolean {
  const matches = pathname === href || pathname.startsWith(`${href}/`);
  if (!matches) return false;
  return !allHrefs.some(
    (other) => other !== href && other.startsWith(`${href}/`) && (pathname === other || pathname.startsWith(`${other}/`)),
  );
}
