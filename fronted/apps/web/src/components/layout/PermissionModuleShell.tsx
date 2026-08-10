"use client";

import { usePathname } from "next/navigation";
import { Breadcrumb } from "@/components/ui/Breadcrumb";

const TITLES: Record<string, string> = {
  "/permission/roles": "角色管理",
  "/permission/members": "成员绑定",
  "/permission/menus": "菜单管理",
  "/permission/apis": "API 管理",
  "/permission/api-tokens": "API Token",
};

export function PermissionModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = TITLES[pathname] ?? "权限管理";

  return (
    <div className="stack-lg">
      <Breadcrumb
        items={[
          { label: "权限管理", href: "/permission/roles" },
          { label: title },
        ]}
      />
      <div>
        <p className="eyebrow">Permission</p>
        <h1 className="text-xl font-semibold mt-1">{title}</h1>
        <p className="text-sm text-muted mt-1">Workspace 级 RBAC，参照 gin-vue-admin 权限管理模块</p>
      </div>
      {children}
    </div>
  );
}
