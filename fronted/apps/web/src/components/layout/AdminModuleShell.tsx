"use client";

import { usePathname } from "next/navigation";
import { ADMIN_PAGE_DESCRIPTIONS, ADMIN_PAGE_TITLES } from "@/components/layout/nav-tree";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";

/** 自行渲染完整布局的管理控制台页面 */
const FULL_BLEED_PATHS = new Set(["/admin/menus", "/admin/members"]);

export function AdminModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = ADMIN_PAGE_TITLES[pathname];
  const description = ADMIN_PAGE_DESCRIPTIONS[pathname];

  if (FULL_BLEED_PATHS.has(pathname)) {
    return <div className="admin-page-full">{children}</div>;
  }

  if (!title) {
    return <div className="admin-page stack-lg">{children}</div>;
  }

  return (
    <div className="admin-page">
      <section className="platform-table-card admin-page-card">
        <PlatformPanelHead title={title} description={description} />
        <div className="admin-page-card-body">{children}</div>
      </section>
    </div>
  );
}
