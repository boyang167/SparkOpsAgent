import { PLATFORM_NAV_TREE, filterNavGroups } from "./nav-tree";

export interface NavSearchItem {
  href: string;
  label: string;
  group: string;
}

export function buildNavSearchItems(options?: {
  showRbac?: boolean;
  showSuperAdmin?: boolean;
}) {
  const groups = filterNavGroups({
    showRbac: options?.showRbac ?? true,
    showSuperAdmin: options?.showSuperAdmin ?? true,
  });
  const items: NavSearchItem[] = [
    { href: "/dashboard", label: "工作台", group: "导航" },
    { href: "/dashboard/workplace", label: "工作台首页", group: "导航" },
    { href: "/settings/profile", label: "个人设置", group: "导航" },
  ];
  for (const group of groups) {
    for (const item of group.items) {
      items.push({ href: item.href, label: item.label, group: group.label });
    }
  }
  const seen = new Set<string>();
  return items.filter((item) => {
    if (seen.has(item.href)) return false;
    seen.add(item.href);
    return true;
  });
}

export function filterNavSearchItems(items: NavSearchItem[], keyword: string) {
  const q = keyword.trim().toLowerCase();
  if (!q) return items.slice(0, 12);
  return items
    .filter(
      (item) =>
        item.label.toLowerCase().includes(q) ||
        item.href.toLowerCase().includes(q) ||
        item.group.toLowerCase().includes(q),
    )
    .slice(0, 20);
}
