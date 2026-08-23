"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { PLATFORM_OPS_NAV_ITEMS } from "@/components/layout/nav-tree";

function isItemActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function PlatformOpsNav() {
  const pathname = usePathname();

  return (
    <div className="platform-ops-nav">
      {PLATFORM_OPS_NAV_ITEMS.map((item) => {
        const active = isItemActive(pathname, item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={`platform-ops-nav-link ${active ? "platform-ops-nav-link-active" : ""}`}
          >
            {item.label}
          </Link>
        );
      })}
    </div>
  );
}
