"use client";

import { usePathname } from "next/navigation";
import { Breadcrumb } from "@/components/ui/Breadcrumb";

const TITLES: Record<string, string> = {
  "/permission/roles": "角色管理",
  "/permission/members": "成员绑定",
  "/permission/menus": "菜单管理",
  "/permission/apis": "API 管理",
  "/permission/data": "数据权限",
  "/permission/api-tokens": "API Token",
};

export function PermissionModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = TITLES[pathname] ?? "权限管理";
  const isMenusPage = pathname === "/permission/menus";

  if (isMenusPage) {
    return <div className="stack-lg">{children}</div>;
  }

  return (
    <div className="stack-lg">
      <Breadcrumb items={[{ label: title }]} />
      <h1 className="text-xl font-semibold">{title}</h1>
      {children}
    </div>
  );
}
