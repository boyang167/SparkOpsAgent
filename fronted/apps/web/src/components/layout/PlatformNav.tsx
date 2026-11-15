"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { NavGroup } from "@/components/layout/NavGroup";
import {
  filterSidebarNavEntries,
  menuTreeToNavEntries,
  type SidebarNavEntry,
} from "@/components/layout/menu-tree-nav";
import {
  FILES_PLANE_NAV_GROUP,
  WORKPLACE_NAV_ITEM,
  filterBusinessDirectNavItems,
  filterBusinessNavGroups,
  filterNavTreeByEntitlements,
} from "@/components/layout/nav-tree";
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
  knowledge: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 5.5A2.5 2.5 0 016.5 3H11v16H6.5A2.5 2.5 0 004 21.5v-16Z" strokeLinejoin="round" />
      <path d="M20 5.5A2.5 2.5 0 0017.5 3H13v16h4.5a2.5 2.5 0 012.5 2.5v-16Z" strokeLinejoin="round" />
    </svg>
  ),
  filesystem: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 6a2 2 0 012-2h4l2 2h8a2 2 0 012 2v10a2 2 0 01-2 2H6a2 2 0 01-2-2V6z" strokeLinejoin="round" />
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
      <path d="M9 2v5M15 2v5" strokeLinecap="round" />
      <path d="M6 7h12v4a6 6 0 01-12 0V7z" strokeLinejoin="round" />
      <path d="M12 17v5" strokeLinecap="round" />
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

function toSidebarEntries(tree: MenuTreeVM[]): SidebarNavEntry[] {
  return menuTreeToNavEntries(tree).map((entry) => (
    entry.type === "group"
      ? {
          ...entry,
          defaultCollapsed: collapsedByDefault.has(entry.iconKey) || entry.defaultCollapsed,
        }
      : entry
  ));
}

function directNavLink(
  item: { id: string; href: string; label: string },
  pathname: string,
) {
  const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
  return (
    <Link
      key={item.id}
      href={item.href}
      className={`nav-group-trigger ${active ? "nav-group-trigger-active" : ""}`}
      aria-current={active ? "page" : undefined}
    >
      <span className="nav-icon">
        {item.href === WORKPLACE_NAV_ITEM.href
          ? icons.dashboard
          : item.href.startsWith("/files")
            ? icons.filesystem
            : icons.knowledge}
      </span>
      <span className="flex-1 text-left">{item.label}</span>
    </Link>
  );
}

export function PlatformNav({ workspaceId }: PlatformNavProps) {
  const pathname = usePathname();
  const { entitlements } = useShellContext();
  const [menuTree, setMenuTree] = useState<MenuTreeVM[] | null | undefined>(undefined);

  const loadNav = useCallback(async () => {
    if (!workspaceId) {
      setMenuTree(null);
      return;
    }
    setMenuTree(undefined);
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

  const dynamicEntries = useMemo(
    () => (
      menuTree === undefined
        ? []
        : menuTree
          ? toSidebarEntries(menuTree)
          : null
    ),
    [menuTree],
  );

  const fallbackEntries = useMemo(() => {
    const groups = filterBusinessNavGroups({ showSuperAdmin: false, entitlements });
    const primary = groups.filter((group) => group.id !== "ops-infra");
    const settings = groups.filter((group) => group.id === "ops-infra");
    const filesystemGroups = entitlements
      ? filterNavTreeByEntitlements([FILES_PLANE_NAV_GROUP], entitlements)
      : [FILES_PLANE_NAV_GROUP];
    const entries: SidebarNavEntry[] = primary.map((group) => ({
      type: "group",
      id: group.id,
      iconKey: group.id,
      label: group.label,
      items: group.items,
      defaultCollapsed: group.defaultCollapsed ?? false,
    }));
    for (const group of filesystemGroups) {
      entries.push({
        type: "group",
        id: group.id,
        iconKey: "filesystem",
        label: group.label,
        items: group.items,
        defaultCollapsed: false,
      });
    }
    for (const item of filterBusinessDirectNavItems(entitlements)) {
      entries.push({
        type: "link",
        id: item.href,
        href: item.href,
        label: item.label,
      });
    }
    entries.push(...settings.map((group) => ({
      type: "group" as const,
      id: group.id,
      iconKey: group.id,
      label: group.label,
      items: group.items,
      defaultCollapsed: group.defaultCollapsed ?? false,
    })));
    return entries;
  },
    [entitlements],
  );

  const rawEntries = dynamicEntries ?? fallbackEntries;
  const entries = useMemo(
    () => filterSidebarNavEntries(rawEntries, entitlements),
    [rawEntries, entitlements],
  );

  return (
    <>
      {directNavLink({ id: WORKPLACE_NAV_ITEM.href, ...WORKPLACE_NAV_ITEM }, pathname)}

      {entries.map((entry) => (
        entry.type === "link"
          ? directNavLink(entry, pathname)
          : (
              <NavGroup
                key={entry.id}
                label={entry.label}
                icon={resolveGroupIcon(entry.iconKey)}
                items={entry.items}
                pathname={pathname}
                defaultOpen={entry.iconKey === "application" || entry.iconKey === "filesystem"}
              />
            )
      ))}
    </>
  );
}
