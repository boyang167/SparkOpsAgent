"use client";

import { useCallback, useEffect, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { AuditLogVM } from "@sparkops/contracts/tenant";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";

const ACTION_LABELS: Record<string, string> = {
  "org.create": "创建组织",
  "platform.org.create": "平台创建组织",
  "org.status.update": "更新组织状态",
  "org.member.invite": "邀请成员",
  "org.member.role_update": "更新成员角色",
  "org.member.remove": "移除成员",
  "org.owner.transfer": "转让 Owner",
};

interface PlatformAuditLogsPanelProps {
  orgId?: string;
  title?: string;
  showBreadcrumb?: boolean;
  variant?: "platform" | "default";
}

export function PlatformAuditLogsPanel({
  orgId,
  title = "平台审计日志",
  showBreadcrumb = true,
  variant = "default",
}: PlatformAuditLogsPanelProps) {
  const isPlatform = variant === "platform" && !orgId;
  const [logs, setLogs] = useState<AuditLogVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setLogs(orgId ? await tenant.listOrgAuditLogs(orgId) : await tenant.listPlatformAuditLogs());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    void load();
  }, [load]);

  const table = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>加载中…</div>
      ) : logs.length === 0 ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>暂无审计记录</div>
      ) : (
        <div className={isPlatform ? "platform-table-wrap" : "table-wrap"}>
          <table className={isPlatform ? "platform-table" : "data-table text-sm"}>
            <thead>
              <tr>
                <th>时间</th>
                <th>操作</th>
                <th>操作者</th>
                <th>详情</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr key={log.id}>
                  <td className="whitespace-nowrap text-muted">
                    {new Date(log.created_at).toLocaleString()}
                  </td>
                  <td>{ACTION_LABELS[log.action] ?? log.action}</td>
                  <td className="font-mono text-xs">{log.actor_id.slice(0, 8)}…</td>
                  <td className="text-xs text-muted max-w-md truncate">
                    {JSON.stringify(log.detail)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {!loading && logs.length > 0 && isPlatform ? (
        <div className="platform-table-footer">
          <span className="text-sm text-muted">共 {logs.length} 条记录</span>
        </div>
      ) : null}
    </>
  );

  if (isPlatform) {
    return (
      <div className="platform-orgs-page">
        <section className="platform-table-card platform-orgs-card">
          <PlatformPanelHead
            title="平台审计"
            description="跨租户操作与变更审计记录。"
          />
          <div className="platform-orgs-card-body">{table}</div>
        </section>
      </div>
    );
  }

  return (
    <div className="stack-lg">
      {!orgId && showBreadcrumb ? (
        <Breadcrumb items={[{ label: "设置", href: "/settings/profile" }, { label: "审计日志" }]} />
      ) : null}
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="text-sm text-muted mt-1">权限变更与平台操作记录</p>
      </div>
      {table}
    </div>
  );
}
