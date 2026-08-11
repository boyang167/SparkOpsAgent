import type { RoleTreeVM } from "@sparkops/contracts/governance";

export type RoleTreeNode = RoleTreeVM & { depth: number };

export function flattenRoleTree(nodes: RoleTreeVM[], depth = 0): RoleTreeNode[] {
  const rows: RoleTreeNode[] = [];
  for (const node of nodes) {
    rows.push({ ...node, depth });
    if (node.children?.length) {
      rows.push(...flattenRoleTree(node.children, depth + 1));
    }
  }
  return rows;
}

export function findRoleNode(nodes: RoleTreeVM[], id: string): RoleTreeVM | null {
  for (const node of nodes) {
    if (node.id === id) return node;
    if (node.children?.length) {
      const found = findRoleNode(node.children, id);
      if (found) return found;
    }
  }
  return null;
}

export function collectRoleParentOptions(
  nodes: RoleTreeVM[],
  excludeId?: string,
): Array<{ id: string; label: string; depth: number }> {
  const options: Array<{ id: string; label: string; depth: number }> = [];
  function walk(items: RoleTreeVM[], depth = 0) {
    for (const item of items) {
      if (item.id !== excludeId) {
        options.push({ id: item.id, label: item.name, depth });
        if (item.children?.length) walk(item.children, depth + 1);
      }
    }
  }
  walk(nodes);
  return options;
}

export function computeRoleStats(tree: RoleTreeVM[]) {
  const flat = flattenRoleTree(tree);
  const rootCount = tree.length;
  const systemCount = flat.filter((role) => role.name === "admin").length;
  return {
    total: flat.length,
    rootCount,
    systemCount,
    maxDepth: flat.reduce((max, role) => Math.max(max, role.depth), 0),
  };
}

export function parentRoleLabel(tree: RoleTreeVM[], parentId?: string) {
  if (!parentId) return "—";
  return findRoleNode(tree, parentId)?.name ?? parentId.slice(0, 8);
}
