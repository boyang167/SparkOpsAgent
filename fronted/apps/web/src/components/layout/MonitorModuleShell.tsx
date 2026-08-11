"use client";

import { usePathname } from "next/navigation";
import { Breadcrumb } from "@/components/ui/Breadcrumb";

const TITLES: Record<string, string> = {
  "/monitor/operation": "操作日志",
  "/monitor/log-viewer": "日志查看器",
  "/monitor/overview": "监控概览",
  "/monitor/statistics": "部署统计",
  "/monitor/audit": "操作日志",
  "/monitor/org-audit": "组织审计",
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
        <p className="text-sm text-muted mt-1">HTTP 操作历史、日志查看器、服务器状态与组织审计</p>
      </div>
      {children}
    </div>
  );
}
