"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { NavGroup } from "@/components/layout/NavGroup";
import { filterNavGroups } from "@/components/layout/nav-tree";

interface PlatformNavProps {
  showRbac: boolean;
  showSuperAdmin: boolean;
}

function NavItem({
  href,
  label,
  icon,
  active,
}: {
  href: string;
  label: string;
  icon: React.ReactNode;
  active: boolean;
}) {
  return (
    <Link href={href} className={`nav-link ${active ? "nav-link-active" : ""}`}>
      <span className="nav-icon">{icon}</span>
      {label}
    </Link>
  );
}

function isActive(pathname: string, href: string, prefix?: string) {
  if (pathname === href) return true;
  if (prefix && pathname.startsWith(prefix)) return true;
  return false;
}

const icons = {
  grid: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </svg>
  ),
  project: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 7h18M3 12h18M3 17h18" strokeLinecap="round" />
    </svg>
  ),
  delivery: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3v4M12 17v4M3 12h4M17 12h4" strokeLinecap="round" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ),
  infra: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
    </svg>
  ),
  resource: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M6 6h12v12H6zM9 6V4h6v2M9 18v2h6v-2" strokeLinejoin="round" />
    </svg>
  ),
  permission: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
    </svg>
  ),
  monitor: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 19V5M4 19h16M8 15v-4M12 15V9M16 15v-2" strokeLinecap="round" />
    </svg>
  ),
  platform: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 21h18M6 21V7l6-3 6 3v14M9 21v-6h6v6" strokeLinejoin="round" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="8" r="4" />
      <path d="M4 20c0-4 4-6 8-6s8 2 8 6" strokeLinecap="round" />
    </svg>
  ),
} as const;

const groupIcons: Record<string, React.ReactNode> = {
  delivery: icons.delivery,
  infra: icons.infra,
  resource: icons.resource,
  permission: icons.permission,
  monitor: icons.monitor,
  platform: icons.platform,
  settings: icons.settings,
};

export function PlatformNav({ showRbac, showSuperAdmin }: PlatformNavProps) {
  const pathname = usePathname();
  const groups = filterNavGroups({ showRbac, showSuperAdmin });

  return (
    <>
      <NavItem
        href="/dashboard/workplace"
        label="工作台"
        active={isActive(pathname, "/dashboard/workplace", "/dashboard/workplace")}
        icon={icons.grid}
      />
      <NavItem
        href="/dashboard"
        label="项目"
        active={pathname === "/dashboard" || pathname.startsWith("/projects")}
        icon={icons.project}
      />

      {groups.map((group) => (
        <NavGroup
          key={group.id}
          label={group.label}
          icon={groupIcons[group.id]}
          items={group.items}
          pathname={pathname}
        />
      ))}
    </>
  );
}
