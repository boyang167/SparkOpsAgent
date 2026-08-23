"use client";

import { usePathname } from "next/navigation";
import {
  ADMIN_HOME_HREF,
  ADMIN_PAGE_DESCRIPTIONS,
  ADMIN_PAGE_TITLES,
} from "@/components/layout/nav-tree";
import { PageHeader } from "@/components/ui/PageHeader";

const FULL_BLEED_PATHS = new Set(["/admin/menus"]);

export function AdminModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = ADMIN_PAGE_TITLES[pathname] ?? "管理控制台";
  const description = ADMIN_PAGE_DESCRIPTIONS[pathname];

  if (FULL_BLEED_PATHS.has(pathname)) {
    return <div className="admin-page-full stack-lg">{children}</div>;
  }

  return (
    <div className="admin-page stack-lg">
      <PageHeader
        eyebrow="管理控制台"
        title={title}
        description={description}
      />
      {children}
    </div>
  );
}

export { ADMIN_HOME_HREF };
