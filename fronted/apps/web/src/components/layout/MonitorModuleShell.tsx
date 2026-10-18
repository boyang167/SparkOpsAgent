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

const DESCRIPTIONS: Record<string, string> = {
  "/monitor/log-viewer": "结构化查询、本地日志文件、Trace 关联与运行时指标",
  "/monitor/operation": "工作区内 HTTP 与 API 操作审计记录",
  "/monitor/overview": "Grafana / Prometheus 外链与监控面板配置",
  "/monitor/statistics": "工作区部署频率与项目分布统计",
  "/monitor/audit": "工作区操作审计与变更追踪",
  "/monitor/org-audit": "组织级合规与变更审计",
  "/monitor/platform-audit": "平台级操作与合规审计",
  "/monitor/server": "平台服务健康与运行状态",
};

export function MonitorModuleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const title = TITLES[pathname] ?? "运维监控";
  const description = DESCRIPTIONS[pathname];

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
        {description ? (
          <p className="text-sm text-muted mt-1">{description}</p>
        ) : null}
      </div>
      {children}
    </div>
  );
}
