"use client";

import { PLATFORM_NAV_TREE } from "@/components/layout/nav-tree";

export function PermissionMenusPanel() {
  return (
    <div className="section stack-md">
      <div>
        <h2 className="section-title">菜单管理</h2>
        <p className="text-muted mt-1">
          当前平台菜单为静态配置（参照 gin-vue-admin 菜单管理 UI）。后续可接入动态菜单与角色可见性控制。
        </p>
      </div>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>分组</th>
              <th>菜单项</th>
              <th>路径</th>
            </tr>
          </thead>
          <tbody>
            {PLATFORM_NAV_TREE.flatMap((group) =>
              group.items.map((item) => (
                <tr key={item.href}>
                  <td>{group.label}</td>
                  <td>{item.label}</td>
                  <td className="font-mono text-xs">{item.href}</td>
                </tr>
              )),
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
