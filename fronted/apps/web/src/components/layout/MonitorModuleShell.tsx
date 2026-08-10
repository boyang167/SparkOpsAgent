"use client";

import { usePathname } from "next/navigation";
import { Breadcrumb } from "@/components/ui/Breadcrumb";

const TITLES: Record<string, string> = {
  "/monitor/overview": "监控概览",
  "/monitor/statistics": "部署统计",
  "/monitor/audit": "操作历史",
  "/monitor/platform-audit": "平台审计",
  "/monitor/server": "服务器状态",
};

export function MonitorModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = TITLES[pathname] ?? "运维监控";

  return (
    <div className="stack-lg">
      <Breadcrumb
        items={[
          { label: "运维监控", href: "/monitor/overview" },
          { label: title },
        ]}
      />
      <div>
        <p className="eyebrow">Monitor</p>
        <h1 className="text-xl font-semibold mt-1">{title}</h1>
        <p className="text-sm text-muted mt-1">运维与审计能力，参照 gin-vue-admin 运维监控模块</p>
      </div>
      {children}
    </div>
  );
}
