"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SysConfigVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

export function SysConfigPanel() {
  const [config, setConfig] = useState<SysConfigVM | null>(null);
  const [grafanaUrl, setGrafanaUrl] = useState("");
  const [prometheusUrl, setPrometheusUrl] = useState("");
  const [jenkinsUrl, setJenkinsUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await platform.getSysConfig();
      setConfig(data);
      const cfg = (data.config ?? {}) as Record<string, string>;
      setGrafanaUrl(cfg.grafana_url ?? "");
      setPrometheusUrl(cfg.prometheus_url ?? "");
      setJenkinsUrl(cfg.jenkins_url ?? "");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await platform.updateSysConfig({
        kind: config?.kind ?? "platform",
        name: config?.name ?? "default",
        whether_share: config?.whether_share ?? true,
        config: {
          grafana_url: grafanaUrl,
          prometheus_url: prometheusUrl,
          jenkins_url: jenkinsUrl,
        },
      });
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <form onSubmit={onSave} className="stack-md max-w-xl">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <label className="stack-xs">
        <span className="label">Grafana URL</span>
        <input className="input" value={grafanaUrl} onChange={(e) => setGrafanaUrl(e.target.value)} placeholder="https://grafana.example.com" />
      </label>
      <label className="stack-xs">
        <span className="label">Prometheus URL</span>
        <input className="input" value={prometheusUrl} onChange={(e) => setPrometheusUrl(e.target.value)} placeholder="https://prometheus.example.com" />
      </label>
      <label className="stack-xs">
        <span className="label">Jenkins URL</span>
        <input className="input" value={jenkinsUrl} onChange={(e) => setJenkinsUrl(e.target.value)} placeholder="https://jenkins.example.com" />
      </label>
      <button type="submit" className="btn w-fit" disabled={saving}>{saving ? "保存中…" : "保存配置"}</button>
    </form>
  );
}
