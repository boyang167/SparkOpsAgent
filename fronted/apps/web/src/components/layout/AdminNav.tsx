"use client";

import { usePathname } from "next/navigation";
import { NavGroup } from "@/components/layout/NavGroup";
import { ADMIN_NAV_SECTIONS } from "@/components/layout/nav-tree";

const sectionIcons: Record<string, React.ReactNode> = {
  identity: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="8" r="3" />
      <path d="M5 20a7 7 0 0114 0" strokeLinecap="round" />
    </svg>
  ),
  policy: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
      <path d="M9 12l2 2 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  tenant: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 21h18M6 21V7l6-3 6 3v14M10 21v-4h4v4" strokeLinejoin="round" />
    </svg>
  ),
  compliance: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2" />
      <rect x="9" y="3" width="6" height="4" rx="1" />
      <path d="M9 14l2 2 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
};

const defaultSectionIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
  </svg>
);

export function AdminNav() {
  const pathname = usePathname();

  return (
    <>
      {ADMIN_NAV_SECTIONS.map((section) => (
        <NavGroup
          key={section.id}
          label={section.label}
          icon={sectionIcons[section.id] ?? defaultSectionIcon}
          items={section.items}
          pathname={pathname}
        />
      ))}
    </>
  );
}
