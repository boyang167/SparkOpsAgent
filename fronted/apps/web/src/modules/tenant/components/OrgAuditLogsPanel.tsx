"use client";

import { useCallback, useEffect, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { OrganizationVM } from "@sparkops/contracts/tenant";
import { PlatformAuditLogsPanel } from "@/modules/tenant/components/PlatformAuditLogsPanel";
import { InlineAlert } from "@/components/ui/InlineAlert";

export function OrgAuditLogsPanel() {
  const [orgs, setOrgs] = useState<OrganizationVM[]>([]);
  const [orgId, setOrgId] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await tenant.listOrganizations();
      setOrgs(items);
      if (items.length > 0) {
        setOrgId((current) => current || items[0].id);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载组织失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <div className="empty-state">加载中…</div>;
  }

  if (orgs.length === 0) {
    return <div className="empty-state">暂无组织，无法查看组织审计</div>;
  }

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <label className="stack-xs max-w-sm">
        <span className="label">组织</span>
        <select className="input" value={orgId} onChange={(e) => setOrgId(e.target.value)}>
          {orgs.map((org) => (
            <option key={org.id} value={org.id}>
              {org.name}
            </option>
          ))}
        </select>
      </label>
      {orgId ? <PlatformAuditLogsPanel orgId={orgId} title="组织审计日志" /> : null}
    </div>
  );
}
