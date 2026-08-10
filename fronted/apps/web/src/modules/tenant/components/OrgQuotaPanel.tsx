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

  if (loading) return <div className="empty-state">加载配额中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {usage ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <QuotaUsageCard label="工作区" used={usage.workspaces} max={quota?.max_workspaces ?? 0} />
          <QuotaUsageCard label="Agent" used={usage.agents} max={quota?.max_agents ?? 0} />
          <QuotaUsageCard label="项目" used={usage.projects} max={quota?.max_projects ?? 0} />
        </div>
      ) : null}
      {canEdit ? (
        <form onSubmit={onSave} className="stack-md">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label className="stack-xs">
              <span className="label">最大工作区</span>
              <input
                className="input"
                type="number"
                min={1}
                value={form.max_workspaces}
                onChange={(e) => setForm((f) => ({ ...f, max_workspaces: Number(e.target.value) }))}
              />
            </label>
            <label className="stack-xs">
              <span className="label">最大 Agent</span>
              <input
                className="input"
                type="number"
                min={1}
                value={form.max_agents}
                onChange={(e) => setForm((f) => ({ ...f, max_agents: Number(e.target.value) }))}
              />
            </label>
            <label className="stack-xs">
              <span className="label">最大项目</span>
              <input
                className="input"
                type="number"
                min={1}
                value={form.max_projects}
                onChange={(e) => setForm((f) => ({ ...f, max_projects: Number(e.target.value) }))}
              />
            </label>
            <label className="stack-xs">
              <span className="label">Token 预算</span>
              <input
                className="input"
                type="number"
                min={0}
                value={form.token_budget}
                onChange={(e) => setForm((f) => ({ ...f, token_budget: Number(e.target.value) }))}
              />
            </label>
          </div>
          <button type="submit" className="btn w-fit" disabled={saving}>
            {saving ? "保存中…" : "保存配额"}
          </button>
        </form>
      ) : null}
    </div>
  );
}

function QuotaUsageCard({ label, used, max }: { label: string; used: number; max: number }) {
  const pct = max > 0 ? Math.min(100, Math.round((used / max) * 100)) : 0;
  return (
    <div className="card p-4">
      <p className="text-sm text-muted">{label}</p>
      <p className="text-xl font-semibold mt-1">
        {used} / {max}
      </p>
      <div className="mt-2 h-2 rounded bg-[var(--border)] overflow-hidden">
        <div className="h-full bg-[var(--accent)]" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
