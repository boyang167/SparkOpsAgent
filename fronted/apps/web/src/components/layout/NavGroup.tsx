"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

export interface NavLeafItem {
  href: string;
  label: string;
}

interface NavGroupProps {
  label: string;
  icon: React.ReactNode;
  items: NavLeafItem[];
  pathname: string;
  defaultOpen?: boolean;
}

function isGroupActive(pathname: string, items: NavLeafItem[]) {
  return items.some(
    (item) => pathname === item.href || pathname.startsWith(`${item.href}/`),
  );
}

export function NavGroup({ label, icon, items, pathname, defaultOpen }: NavGroupProps) {
  const active = isGroupActive(pathname, items);
  const [open, setOpen] = useState(defaultOpen ?? active);

  useEffect(() => {
    if (active) {
      setOpen(true);
    }
  }, [active]);

  return (
    <div className="nav-group">
      <button
        type="button"
        className={`nav-group-trigger ${active ? "nav-group-trigger-active" : ""}`}
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
      >
        <span className="nav-icon">{icon}</span>
        <span className="flex-1 text-left">{label}</span>
        <svg
          className={`nav-group-chevron ${open ? "nav-group-chevron-open" : ""}`}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open ? (
        <div className="nav-group-children">
          {items.map((item) => {
            const itemActive =
              pathname === item.href || pathname.startsWith(`${item.href}/`);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`nav-sublink ${itemActive ? "nav-sublink-active" : ""}`}
              >
                {item.label}
              </Link>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
