"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { PLATFORM_OPS_NAV_TREE } from "@/components/layout/nav-tree";
import { NavGroup } from "@/components/layout/NavGroup";

const platformIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M3 21h18M6 21V7l6-3 6 3v14M9 21v-6h6v6" strokeLinejoin="round" />
  </svg>
);

export function PlatformOpsNav() {
  const pathname = usePathname();
  const group = PLATFORM_OPS_NAV_TREE[0];

  return (
    <>
      <Link
        href="/platform/accounts"
        className={`nav-link mb-1 ${pathname.startsWith("/platform") ? "nav-link-active" : ""}`}
      >
        <span className="nav-icon">{platformIcon}</span>
        平台运营中心
      </Link>
      <NavGroup label={group.label} icon={platformIcon} items={group.items} pathname={pathname} defaultOpen />
    </>
  );
}
