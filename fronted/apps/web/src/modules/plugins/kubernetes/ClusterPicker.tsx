"use client";

import { useCallback, useEffect, useMemo, useState, type RefObject } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "./api";
import { selectStoredClusterId } from "./catalog";
import {
  ClusterSettingsForm,
  EMPTY_CLUSTER_SETTINGS,
  settingsFromCluster,
} from "./cluster-settings-form";
import type { KubernetesCluster } from "./cluster-types";
import { formatClusterServer, truncateClusterId } from "./cluster-view";
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
  createActionRef?: RefObject<(() => void) | null>;
};

export function ClusterPicker({
  value,
  onChange,
  onClustersChange,
  canManage,
  showSettings = false,
  layout = "picker",
  createActionRef,
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
        createActionRef={createActionRef}
        onClustersChange={(next) => {
          setClusters(next);
          onClustersChange?.(next);
        }}
      />
    );
  }

  return (
    <section className="k8s-cluster-bar">
      <div className="k8s-cluster-bar-row">
        <label className="k8s-cluster-bar-field">
          <span className="label">集群</span>
          <select
            className="input k8s-cluster-bar-select"
            value={value}
            disabled={loading || clusters.length === 0}
            onChange={(e) => choose(e.target.value)}
            aria-label="选择集群"
          >
            {clusters.length === 0 ? <option value="">暂无可用集群</option> : null}
            {clusters.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name || truncateClusterId(item.id)}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          className="btn-secondary k8s-clusters-refresh-btn"
          disabled={loading}
          onClick={() => void load()}
          aria-label="刷新集群列表"
        >
          {loading ? "…" : "↻"}
        </button>
      </div>
      {selected ? (
        <p className="k8s-cluster-summary">
          <span className="font-mono text-xs" title={selected.id}>
            {truncateClusterId(selected.id)}
          </span>
          <span className="k8s-page-sep">·</span>
          <span className="font-mono text-xs">{formatClusterServer(selected.server) || "未配置接口地址"}</span>
        </p>
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
