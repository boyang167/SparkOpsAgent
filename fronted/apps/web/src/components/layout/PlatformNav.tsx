"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { NavGroup } from "@/components/layout/NavGroup";
import { menuTreeToNavGroups } from "@/components/layout/menu-tree-nav";
import { WORKPLACE_NAV_ITEM, filterBusinessNavGroups } from "@/components/layout/nav-tree";
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
      <path d="M4 7h10M18 7h2M4 17h4M12 17h8" strokeLinecap="round" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="10" cy="17" r="2" />
    </svg>
  ),
  "plugin-kubernetes": (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M7 15a5 5 0 111.7-9.7A6 6 0 0120 10a4 4 0 01-1 7.9H8" strokeLinecap="round" />
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

function toSidebarGroups(tree: MenuTreeVM[]) {
  return menuTreeToNavGroups(tree).map((group) => ({
    ...group,
    defaultCollapsed: collapsedByDefault.has(group.id) || group.defaultCollapsed,
  }));
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
    () => (menuTree ? toSidebarGroups(menuTree) : null),
    [menuTree],
  );

  const fallbackGroups = useMemo(
    () =>
      filterBusinessNavGroups({ showSuperAdmin: false, entitlements }).map((group) => ({
        ...group,
        defaultCollapsed: group.defaultCollapsed ?? false,
      })),
    [entitlements],
  );

  const groups = dynamicGroups ?? fallbackGroups;
  const workplaceActive =
    pathname === WORKPLACE_NAV_ITEM.href || pathname.startsWith(`${WORKPLACE_NAV_ITEM.href}/`);

  return (
    <>
      <Link
        href={WORKPLACE_NAV_ITEM.href}
        className={`nav-group-trigger ${workplaceActive ? "nav-group-trigger-active" : ""}`}
        aria-current={workplaceActive ? "page" : undefined}
      >
        <span className="nav-icon">{icons.dashboard}</span>
        <span className="flex-1 text-left">{WORKPLACE_NAV_ITEM.label}</span>
      </Link>

      {groups.map((group) => (
        <NavGroup
          key={group.id + group.label}
          label={group.label}
          icon={resolveGroupIcon(group.id)}
          items={group.items}
          pathname={pathname}
        />
      ))}
    </>
  );
}
