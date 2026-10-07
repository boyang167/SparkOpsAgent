"use client";

import { useCallback, useEffect, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { OrgQuotaUsageVM, OrgQuotaVM } from "@sparkops/contracts/tenant";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface OrgQuotaPanelProps {
  orgId: string;
  canEdit: boolean;
}

export function OrgQuotaPanel({ orgId, canEdit }: OrgQuotaPanelProps) {
  const [quota, setQuota] = useState<OrgQuotaVM | null>(null);
  const [usage, setUsage] = useState<OrgQuotaUsageVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState({ max_workspaces: 0, max_agents: 0, max_projects: 0, token_budget: 0 });

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [q, u] = await Promise.all([tenant.getOrgQuota(orgId), tenant.getOrgQuotaUsage(orgId)]);
      setQuota(q);
      setUsage(u);
      setForm({
        max_workspaces: q.max_workspaces,
        max_agents: q.max_agents,
        max_projects: q.max_projects,
        token_budget: q.token_budget,
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载配额失败");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    if (!canEdit) return;
    setSaving(true);
    setError(null);
    try {
      const updated = await tenant.updateOrgQuota(orgId, form);
      setQuota(updated);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="org-quota-skeleton">
        {[1, 2, 3].map((i) => (
          <div key={i} className="org-quota-skeleton-card" />
        ))}
      </div>
    );
  }

  return (
    <div className="org-quota">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {usage ? (
        <div className="org-quota-usage">
          <QuotaUsageCard label="工作区" used={usage.workspaces} max={quota?.max_workspaces ?? 0} />
          <QuotaUsageCard label="Agent" used={usage.agents} max={quota?.max_agents ?? 0} />
          <QuotaUsageCard label="项目" used={usage.projects} max={quota?.max_projects ?? 0} />
        </div>
      ) : null}

      {canEdit ? (
        <form onSubmit={onSave} className="org-quota-form">
          <h3 className="org-quota-form-title">配额设置</h3>
          <div className="org-quota-fields">
            <label className="org-quota-field">
              <span className="org-quota-field-label">最大工作区</span>
              <input
                className="org-quota-field-input"
                type="number"
                min={1}
                value={form.max_workspaces}
                onChange={(e) => setForm((f) => ({ ...f, max_workspaces: Number(e.target.value) }))}
              />
            </label>
            <label className="org-quota-field">
              <span className="org-quota-field-label">最大 Agent</span>
              <input
                className="org-quota-field-input"
                type="number"
                min={1}
                value={form.max_agents}
                onChange={(e) => setForm((f) => ({ ...f, max_agents: Number(e.target.value) }))}
              />
            </label>
            <label className="org-quota-field">
              <span className="org-quota-field-label">最大项目</span>
              <input
                className="org-quota-field-input"
                type="number"
                min={1}
                value={form.max_projects}
                onChange={(e) => setForm((f) => ({ ...f, max_projects: Number(e.target.value) }))}
              />
            </label>
            <label className="org-quota-field">
              <span className="org-quota-field-label">Token 预算</span>
              <input
                className="org-quota-field-input"
                type="number"
                min={0}
                value={form.token_budget}
                onChange={(e) => setForm((f) => ({ ...f, token_budget: Number(e.target.value) }))}
              />
            </label>
          </div>
          <button type="submit" className="org-invite-btn" disabled={saving} style={{ alignSelf: "flex-start" }}>
            {saving ? "保存中…" : "保存配额"}
          </button>
        </form>
      ) : null}
    </div>
  );
}

function QuotaUsageCard({ label, used, max }: { label: string; used: number; max: number }) {
  const pct = max > 0 ? Math.min(100, Math.round((used / max) * 100)) : 0;
  const isHigh = pct >= 80;
  return (
    <div className="org-quota-card">
      <div className="org-quota-card-header">
        <span className="org-quota-card-label">{label}</span>
        <span className={`org-quota-card-pct${isHigh ? " is-high" : ""}`}>{pct}%</span>
      </div>
      <p className="org-quota-card-value">
        <span className="org-quota-card-used">{used}</span>
        <span className="org-quota-card-sep">/</span>
        <span className="org-quota-card-max">{max}</span>
      </p>
      <div className="org-quota-bar">
        <div
          className={`org-quota-bar-fill${isHigh ? " is-high" : ""}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
