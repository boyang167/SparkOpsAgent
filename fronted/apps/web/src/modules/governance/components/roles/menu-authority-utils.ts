import type { MenuTreeVM } from "@sparkops/contracts/governance";

export function buildMenuParentMap(nodes: MenuTreeVM[]): Map<string, string | null> {
  const map = new Map<string, string | null>();
  function walk(items: MenuTreeVM[], parentId: string | null) {
    for (const item of items) {
      map.set(item.id, parentId);
      if (item.children?.length) walk(item.children, item.id);
    }
  }
  walk(nodes, null);
  return map;
}

export function collectDescendantIds(nodes: MenuTreeVM[], menuId: string): string[] {
  const target = findMenuNode(nodes, menuId);
  if (!target) return [];
  const ids: string[] = [];
  function walk(node: MenuTreeVM) {
    ids.push(node.id);
    node.children?.forEach(walk);
  }
  walk(target);
  return ids;
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

export function toggleMenuSelection(
  tree: MenuTreeVM[],
  selected: Set<string>,
  menuId: string,
  checked: boolean,
): Set<string> {
  const next = new Set(selected);
  const parentMap = buildMenuParentMap(tree);

  if (checked) {
    next.add(menuId);
    for (const id of collectDescendantIds(tree, menuId)) {
      next.add(id);
    }
    let parentId = parentMap.get(menuId) ?? null;
    while (parentId) {
      next.add(parentId);
      parentId = parentMap.get(parentId) ?? null;
    }
  } else {
    for (const id of collectDescendantIds(tree, menuId)) {
      next.delete(id);
    }
  }
  return next;
}

export function setAllMenuSelection(tree: MenuTreeVM[], checked: boolean): Set<string> {
  if (!checked) return new Set();
  const ids = new Set<string>();
  function walk(items: MenuTreeVM[]) {
    for (const item of items) {
      ids.add(item.id);
      if (item.children?.length) walk(item.children);
    }
  }
  walk(tree);
  return ids;
}

export function menuSelectionState(
  tree: MenuTreeVM[],
  selected: Set<string>,
  menuId: string,
): "checked" | "unchecked" | "indeterminate" {
  const node = findMenuNode(tree, menuId);
  if (!node) return "unchecked";
  const descendants = collectDescendantIds(tree, menuId);
  const selectedCount = descendants.filter((id) => selected.has(id)).length;
  if (selectedCount === 0) return "unchecked";
  if (selectedCount === descendants.length) return "checked";
  return "indeterminate";
}
