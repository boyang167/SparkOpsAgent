"use client";

import { useCallback, useEffect, useState } from "react";
import { K8sResourceTabs } from "@/components/k8s";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import type { KubernetesCluster } from "../cluster-types";
import { ClusterAuthorizeRoles } from "./ClusterAuthorizeRoles";
import { ClusterAuthorizeUsers } from "./ClusterAuthorizeUsers";

type Tab = "kubeconfig" | "users" | "roles";

export function ClusterAuthorizeDialog({
  cluster,
  canManage,
}: {
  cluster: KubernetesCluster;
  canManage: boolean;
}) {
  const [tab, setTab] = useState<Tab>("kubeconfig");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [kubeconfig, setKubeconfig] = useState("");

  const loadKubeconfig = useCallback(async () => {
    const data = await k8sFetch<{ kubeconfig?: string }>(
      `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(cluster.id)}/kubeconfig`,
    );
    setKubeconfig(data.kubeconfig ?? "");
  }, [cluster.id]);

  useEffect(() => {
    if (tab !== "kubeconfig") return;
    let cancelled = false;
    async function load() {
      setError(null);
      try {
        await loadKubeconfig();
      } catch (err) {
        if (!cancelled) setError(errorInfo(err, "加载授权信息失败"));
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [tab, loadKubeconfig]);

  async function copyKubeconfig() {
    if (!kubeconfig) return;
    await navigator.clipboard.writeText(kubeconfig);
  }

  function downloadKubeconfig() {
    const blob = new Blob([kubeconfig], { type: "text/yaml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${cluster.name || cluster.id}.kubeconfig`;
    link.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="k8s-auth-panel">
      <K8sResourceTabs
        tabs={["kubeconfig", "users", "roles"] as const}
        active={tab}
        ariaLabel="集群授权"
        label={(id) => (id === "kubeconfig" ? "连接" : id === "users" ? "用户" : "角色")}
        onChange={setTab}
      />
      {tab === "kubeconfig" ? (
        <>
          <ApiErrorBanner error={error} />
          <div className="k8s-dialog-toolbar">
            <span className="text-xs text-muted">平台凭证</span>
            <div className="k8s-dialog-toolbar-actions">
              <button type="button" className="btn-secondary text-sm" disabled={!kubeconfig} onClick={() => void copyKubeconfig()}>
                复制
              </button>
              <button type="button" className="btn-secondary text-sm" disabled={!kubeconfig} onClick={downloadKubeconfig}>
                下载
              </button>
            </div>
          </div>
          <textarea className="input font-mono text-xs k8s-kubeconfig-area" rows={14} readOnly value={kubeconfig} />
        </>
      ) : null}
      {tab === "users" ? <ClusterAuthorizeUsers cluster={cluster} canManage={canManage} /> : null}
      {tab === "roles" ? <ClusterAuthorizeRoles cluster={cluster} canManage={canManage} /> : null}
    </div>
  );
}
