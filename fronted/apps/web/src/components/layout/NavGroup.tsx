"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { isNavHrefActive } from "@/components/layout/menu-tree-nav";

export interface NavLeafItem {
  href: string;
  label: string;
  activeHrefs?: string[];
  children?: NavLeafItem[];
}

interface NavGroupProps {
  label: string;
  icon: React.ReactNode;
  items: NavLeafItem[];
  pathname: string;
  defaultOpen?: boolean;
}

function siblingHrefs(items: NavLeafItem[]): string[] {
  return items.flatMap((item) => (
    item.children?.length ? item.children.map((child) => child.href) : [item.href]
  ));
}

function isLeafItemActive(pathname: string, item: NavLeafItem, items: NavLeafItem[]): boolean {
  if (item.children?.length) {
    const childHrefs = item.children.map((child) => child.href);
    return item.children.some((child) =>
      isNavHrefActive(pathname, child.href, childHrefs, child.activeHrefs),
    );
  }
  return isNavHrefActive(pathname, item.href, siblingHrefs(items), item.activeHrefs);
}

function isGroupActive(pathname: string, items: NavLeafItem[]) {
  return items.some((item) => isLeafItemActive(pathname, item, items));
}

function NavSubGroup({
  item,
  pathname,
}: {
  item: NavLeafItem & { children: NavLeafItem[] };
  pathname: string;
}) {
  const active = isLeafItemActive(pathname, item, [item]);
  const [open, setOpen] = useState(active);

  useEffect(() => {
    if (active) setOpen(true);
  }, [active]);

  const childHrefs = item.children.map((child) => child.href);

  return (
    <div className="nav-subgroup">
      <button
        type="button"
        className={`nav-subgroup-trigger ${active ? "nav-subgroup-trigger-active" : ""}`}
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
      >
        <span className="flex-1 text-left">{item.label}</span>
        <svg
          className={`nav-group-chevron ${open ? "nav-group-chevron-open" : ""}`}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          aria-hidden
        >
          <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open ? (
        <div className="nav-subgroup-children">
          {item.children.map((child) => {
            const childActive = isNavHrefActive(
              pathname,
              child.href,
              childHrefs,
              child.activeHrefs,
            );
            return (
              <Link
                key={`${child.href}:${child.label}`}
                href={child.href}
                className={`nav-sublink nav-sublink-nested ${childActive ? "nav-sublink-active" : ""}`}
              >
                {child.label}
              </Link>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

export function NavGroup({ label, icon, items, pathname, defaultOpen = false }: NavGroupProps) {
  const active = isGroupActive(pathname, items);
  const [open, setOpen] = useState(defaultOpen);

  useEffect(() => {
    if (active) setOpen(true);
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
            if (item.children?.length) {
              return <NavSubGroup key={`${item.label}:${item.href}`} item={{ ...item, children: item.children }} pathname={pathname} />;
            }
            const itemActive = isLeafItemActive(pathname, item, items);
            return (
              <Link
                key={`${item.href}:${item.label}`}
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
