"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ADMIN_NAV_SECTIONS } from "@/components/layout/nav-tree";

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AdminNav() {
  const pathname = usePathname();

  return (
    <>
      <Link
        href="/admin/members"
        className={`nav-link mb-1 ${pathname.startsWith("/admin") ? "nav-link-active" : ""}`}
      >
        <span className="nav-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
          </svg>
        </span>
        系统管理
      </Link>

      {ADMIN_NAV_SECTIONS.map((section) => (
        <div key={section.id} className="nav-admin-section">
          <p className="nav-section-label">{section.label}</p>
          <div className="nav-admin-items">
            {section.items.map((item) => {
              const active = isActive(pathname, item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`nav-sublink ${active ? "nav-sublink-active" : ""}`}
                >
                  {item.label}
                </Link>
              );
            })}
          </div>
        </div>
      ))}
    </>
  );
}
