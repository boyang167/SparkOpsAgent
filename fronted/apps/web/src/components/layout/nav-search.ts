import {
  ADMIN_NAV_SECTIONS,
  BUSINESS_NAV_TREE,
  BUSINESS_DIRECT_NAV_ITEMS,
  PLATFORM_OPS_NAV_TREE,
} from "./nav-tree";
import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { isPathEntitled } from "./entitlements";

export interface NavSearchItem {
  href: string;
  label: string;
  group: string;
}

export function buildNavSearchItems(options?: {
  showAdmin?: boolean;
  showPlatform?: boolean;
  entitlements?: WorkspaceEntitlementsVM;
}) {
  const showAdmin = options?.showAdmin ?? true;
  const showPlatform = options?.showPlatform ?? true;
  const entitlements = options?.entitlements;

  const items: NavSearchItem[] = [
    { href: "/dashboard/workplace", label: "工作台", group: "工作台" },
    { href: "/settings/profile", label: "个人设置", group: "账户中心" },
    { href: "/notifications", label: "消息中心", group: "账户中心" },
  ];

  for (const group of BUSINESS_NAV_TREE) {
    for (const item of group.items) {
      if (entitlements && !isPathEntitled(item.href, entitlements)) continue;
      items.push({ href: item.href, label: item.label, group: group.label });
    }
  }
  for (const direct of BUSINESS_DIRECT_NAV_ITEMS) {
    if (!entitlements || isPathEntitled(direct.href, entitlements)) {
      const group = direct.href === "/knowledge" ? "知识库" : direct.label;
      items.push({ href: direct.href, label: direct.label, group });
    }
  }

  if (showAdmin) {
    for (const section of ADMIN_NAV_SECTIONS) {
      for (const item of section.items) {
        items.push({ href: item.href, label: item.label, group: `管理 · ${section.label}` });
      }
    }
  }

  if (showPlatform) {
    for (const group of PLATFORM_OPS_NAV_TREE) {
      for (const item of group.items) {
        items.push({ href: item.href, label: item.label, group: "平台运营" });
      }
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
