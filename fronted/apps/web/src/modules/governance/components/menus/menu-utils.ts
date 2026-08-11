import type { MenuTreeVM } from "@sparkops/contracts/governance";

export type MenuTreeNode = MenuTreeVM & { depth: number };

export function flattenMenuTree(nodes: MenuTreeVM[], depth = 0): MenuTreeNode[] {
  const rows: MenuTreeNode[] = [];
  for (const node of nodes) {
    rows.push({ ...node, depth });
    if (node.children?.length) {
      rows.push(...flattenMenuTree(node.children, depth + 1));
    }
  }
  return rows;
}

export function collectMenuIds(nodes: MenuTreeVM[]): string[] {
  const ids: string[] = [];
  function walk(items: MenuTreeVM[]) {
    for (const item of items) {
      ids.push(item.id);
      if (item.children?.length) walk(item.children);
    }
  }
  walk(nodes);
  return ids;
}

export function collectParentOptions(
  nodes: MenuTreeVM[],
  excludeId?: string,
): Array<{ id: string; label: string; depth: number }> {
  const options: Array<{ id: string; label: string; depth: number }> = [];
  function walk(items: MenuTreeVM[], depth = 0) {
    for (const item of items) {
      if (item.id !== excludeId) {
        options.push({ id: item.id, label: item.title, depth });
        if (item.children?.length) walk(item.children, depth + 1);
      }
    }
  }
  walk(nodes);
  return options;
}

export function findMenuNode(nodes: MenuTreeVM[], id: string): MenuTreeVM | null {
  for (const node of nodes) {
    if (node.id === id) return node;
    if (node.children?.length) {
      const found = findMenuNode(node.children, id);
      if (found) return found;
    }
  }
  return null;
}

export function getMenuBreadcrumb(nodes: MenuTreeVM[], id: string): string[] {
  const path: string[] = [];
  function walk(items: MenuTreeVM[], trail: string[]): boolean {
    for (const item of items) {
      const next = [...trail, item.title];
      if (item.id === id) {
        path.push(...next);
        return true;
      }
      if (item.children?.length && walk(item.children, next)) return true;
    }
    return false;
  }
  walk(nodes, []);
  return path;
}

export function filterMenuTree(
  nodes: MenuTreeVM[],
  query: string,
  showHidden: boolean,
): MenuTreeVM[] {
  const q = query.trim().toLowerCase();
  function matches(node: MenuTreeVM) {
    if (!showHidden && node.hidden) return false;
    if (!q) return true;
    return (
      node.title.toLowerCase().includes(q) ||
      node.name.toLowerCase().includes(q) ||
      node.path.toLowerCase().includes(q) ||
      (node.component ?? "").toLowerCase().includes(q)
    );
  }
  function walk(items: MenuTreeVM[]): MenuTreeVM[] {
    const out: MenuTreeVM[] = [];
    for (const item of items) {
      const children = item.children?.length ? walk(item.children) : [];
      if (matches(item) || children.length > 0) {
        out.push({ ...item, children: children.length ? children : undefined });
      }
    }
    return out;
  }
  return walk(nodes);
}

export function computeMenuStats(nodes: MenuTreeVM[]) {
  const flat = flattenMenuTree(nodes);
  const roots = nodes.length;
  const hidden = flat.filter((n) => n.hidden).length;
  const leaves = flat.filter((n) => !n.children?.length).length;
  return { total: flat.length, roots, hidden, leaves };
}

export function collectExpandableIds(nodes: MenuTreeVM[]): string[] {
  const ids: string[] = [];
  function walk(items: MenuTreeVM[]) {
    for (const item of items) {
      if (item.children?.length) {
        ids.push(item.id);
        walk(item.children);
      }
    }
  }
  walk(nodes);
  return ids;
}

export function notifyMenusUpdated() {
  window.dispatchEvent(new Event("sparkops:menus-updated"));
}
