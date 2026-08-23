"use client";

import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { NavGroup } from "@/components/layout/NavGroup";
import { DASHBOARD_NAV_GROUP, filterBusinessNavGroups } from "@/components/layout/nav-tree";
import { useShellContext } from "@/components/layout/shell-context";

interface PlatformNavProps {
  workspaceId?: string;
}

const icons = {
  dashboard: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </svg>
  ),
  application: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 7h18M3 12h18M3 17h18" strokeLinecap="round" />
    </svg>
  ),
  "ops-delivery": (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3v4M12 17v4M3 12h4M17 12h4" strokeLinecap="round" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ),
  "ops-runtime": (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <path d="M9 9h6v6H9z" />
    </svg>
  ),
  "ops-observability": (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 19V5M4 19h16M8 15v-4M12 15V9M16 15v-2" strokeLinecap="round" />
    </svg>
  ),
  "ops-infra": (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
    </svg>
  ),
  default: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="12" r="2" />
      <path d="M12 2v4M12 18v4M2 12h4M18 12h4" strokeLinecap="round" />
    </svg>
  ),
} as const;

const collapsedByDefault = new Set(["ops-infra"]);

function resolveGroupIcon(key?: string) {
  if (!key) return icons.default;
  return icons[key as keyof typeof icons] ?? icons.default;
}

function menuTreeToNavGroups(tree: MenuTreeVM[]) {
  return tree
    .map((group) => ({
      id: group.icon || group.name,
      label: group.title,
      defaultCollapsed: collapsedByDefault.has(group.icon || group.name),
      items: (group.children ?? [])
        .filter((child) => !child.hidden && child.path.startsWith("/"))
        .map((child) => ({ href: child.path, label: child.title })),
    }))
    .filter((group) => group.items.length > 0);
}

export function PlatformNav({ workspaceId }: PlatformNavProps) {
  const pathname = usePathname();
  const { entitlements } = useShellContext();
  const [menuTree, setMenuTree] = useState<MenuTreeVM[] | null>(null);

  const loadNav = useCallback(async () => {
    if (!workspaceId) {
      setMenuTree(null);
      return;
    }
    try {
      setMenuTree(await governance.listNavMenus(workspaceId));
    } catch {
      setMenuTree(null);
    }
  }, [workspaceId]);

  useEffect(() => {
    void loadNav();
  }, [loadNav]);

  useEffect(() => {
    function onMenusUpdated() {
      void loadNav();
    }
    window.addEventListener("sparkops:menus-updated", onMenusUpdated);
    return () => window.removeEventListener("sparkops:menus-updated", onMenusUpdated);
  }, [loadNav]);

  const dynamicGroups = useMemo(
    () => (menuTree ? menuTreeToNavGroups(menuTree) : null),
    [menuTree],
  );

  const fallbackGroups = useMemo(
    () =>
      filterBusinessNavGroups({ showSuperAdmin: false, entitlements })
        .filter((group) => group.id !== DASHBOARD_NAV_GROUP.id)
        .map((group) => ({
          ...group,
          defaultCollapsed: group.defaultCollapsed ?? false,
        })),
    [entitlements],
  );

  const groups = dynamicGroups ?? fallbackGroups;

  return (
    <>
      <NavGroup
        label={DASHBOARD_NAV_GROUP.label}
        icon={icons.dashboard}
        items={[...DASHBOARD_NAV_GROUP.items]}
        pathname={pathname}
        defaultOpen
      />

      {groups.map((group) => (
        <NavGroup
          key={group.id + group.label}
          label={group.label}
          icon={resolveGroupIcon(group.id)}
          items={group.items}
          pathname={pathname}
          defaultOpen={!group.defaultCollapsed}
        />
      ))}
    </>
  );
}
