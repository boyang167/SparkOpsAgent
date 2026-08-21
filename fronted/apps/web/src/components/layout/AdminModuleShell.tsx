"use client";

import { usePathname } from "next/navigation";
import { ADMIN_PAGE_TITLES } from "@/components/layout/nav-tree";
import { Breadcrumb } from "@/components/ui/Breadcrumb";

export function AdminModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = ADMIN_PAGE_TITLES[pathname] ?? "管理控制台";
  const isMenusPage = pathname === "/admin/menus";

  if (isMenusPage) {
    return <div className="stack-lg">{children}</div>;
  }

  return (
    <div className="stack-lg">
      <Breadcrumb
        items={[{ label: "管理控制台", href: "/admin/members" }, { label: title }]}
      />
      <h1 className="text-xl font-semibold">{title}</h1>
      {children}
    </div>
  );
}
