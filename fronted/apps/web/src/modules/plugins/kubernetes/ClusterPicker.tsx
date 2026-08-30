"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "./api";
import { selectStoredClusterId } from "./catalog";
import {
  ClusterSettingsForm,
  EMPTY_CLUSTER_SETTINGS,
  settingsFromCluster,
} from "./cluster-settings-form";
import type { KubernetesCluster } from "./cluster-types";
import { ClustersPanel } from "./specialized/ClustersPanel";

export type { KubernetesCluster } from "./cluster-types";

export const CLUSTER_STORAGE_KEY = "sparkops.plugin.kubernetes.clusterId";

type ClusterListResponse = {
  items?: KubernetesCluster[];
};

type ClusterPickerProps = {
  value: string;
  onChange: (clusterId: string) => void;
  onClustersChange?: (clusters: KubernetesCluster[]) => void;
  canManage: boolean;
  showSettings?: boolean;
  layout?: "picker" | "table";
};

export function ClusterPicker({
  value,
  onChange,
  onClustersChange,
  canManage,
  showSettings = false,
  layout = "picker",
}: ClusterPickerProps) {
  const [clusters, setClusters] = useState<KubernetesCluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [settings, setSettings] = useState(EMPTY_CLUSTER_SETTINGS);

  const selected = useMemo(() => clusters.find((item) => item.id === value), [clusters, value]);

  const applySelection = useCallback(
    (items: KubernetesCluster[]) => {
      const stored = typeof window === "undefined" ? null : window.localStorage.getItem(CLUSTER_STORAGE_KEY);
      const next = selectStoredClusterId(stored, items);
      if (next) {
        window.localStorage.setItem(CLUSTER_STORAGE_KEY, next);
      } else {
        window.localStorage.removeItem(CLUSTER_STORAGE_KEY);
      }
      onChange(next);
    },
    [onChange],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await k8sFetch<ClusterListResponse>("/api/bff/v1/plugins/kubernetes/clusters");
      const items = data.items ?? [];
      setClusters(items);
      onClustersChange?.(items);
      applySelection(items);
    } catch (err) {
      setError(errorInfo(err, "加载集群失败"));
      setClusters([]);
      onClustersChange?.([]);
      onChange("");
    } finally {
      setLoading(false);
    }
  }, [applySelection, onChange, onClustersChange]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setSettings(settingsFromCluster(selected));
  }, [selected]);

  function choose(clusterId: string) {
    const allowed = clusters.some((item) => item.id === clusterId);
    const next = allowed ? clusterId : selectStoredClusterId(null, clusters);
    if (next) window.localStorage.setItem(CLUSTER_STORAGE_KEY, next);
    onChange(next);
  }

  async function saveSettings() {
    if (!selected || !canManage) return;
    setSaving(true);
    setError(null);
    try {
      const updated = await k8sFetch<KubernetesCluster>(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(selected.id)}/settings`,
        { method: "PUT", body: JSON.stringify(settings) },
      );
      const next = clusters.map((item) => (item.id === updated.id ? updated : item));
      setClusters(next);
      onClustersChange?.(next);
    } catch (err) {
      setError(errorInfo(err, "保存集群设置失败"));
    } finally {
      setSaving(false);
    }
  }

  if (layout === "table") {
    return (
      <ClustersPanel
        clusters={clusters}
        loading={loading}
        error={error}
        canManage={canManage}
        selectedId={value}
        onRefresh={() => void load()}
        onSelect={choose}
        onClustersChange={(next) => {
          setClusters(next);
          onClustersChange?.(next);
        }}
      />
    );
  }

  return (
    <section className="card stack-md p-4">
      <div className="flex flex-wrap items-end gap-3">
        <label className="min-w-72 flex-1">
          <span className="label">集群</span>
          <select className="input" value={value} disabled={loading || clusters.length === 0} onChange={(e) => choose(e.target.value)}>
            {clusters.length === 0 ? <option value="">暂无可用集群</option> : null}
            {clusters.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name || item.id}
              </option>
            ))}
          </select>
        </label>
        <button type="button" className="btn-secondary" disabled={loading} onClick={() => void load()}>
          {loading ? "刷新中…" : "刷新集群"}
        </button>
      </div>
      {selected ? (
        <div className="text-sm text-muted">
          {selected.id} · {selected.server || "未返回 server"}
        </div>
      ) : null}
      <ApiErrorBanner error={error} />
      {showSettings && selected ? (
        <div className="stack-md border-t pt-4">
          <div>
            <h2 className="text-lg font-semibold">集群设置</h2>
            <p className="text-sm text-muted mt-1">密钥字段返回为 *** 时可直接保存以保留原值。</p>
          </div>
          <ClusterSettingsForm settings={settings} onChange={setSettings} />
          {canManage ? (
            <div className="flex justify-end">
              <button type="button" className="btn" disabled={saving} onClick={() => void saveSettings()}>
                {saving ? "保存中…" : "保存设置"}
              </button>
            </div>
          ) : (
            <div className="alert alert-info">你没有 governance:admin 权限，集群设置只读。</div>
          )}
        </div>
      ) : null}
    </section>
  );
}
