"use client";

import { useState } from "react";
import { K8sEmptyState, K8sLoadingState } from "@/components/k8s";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { cloudTTYProxyPath } from "../operations";
import { PodTargetPicker, type PodTargetValue } from "./PodTargetPicker";

const EMPTY_TARGET: PodTargetValue = { namespace: "default", pod: "", container: "" };

export function CloudTTYPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState<PodTargetValue>(EMPTY_TARGET);
  const [frameURL, setFrameURL] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [connecting, setConnecting] = useState(false);

  const canConnect = Boolean(clusterId && canManage && form.namespace && form.pod && form.container);

  async function start() {
    if (!canConnect) return;
    setConnecting(true);
    setError(null);
    setFrameURL("");
    try {
      const data = await k8sFetch<{
        id: string;
        cloudshell?: { name: string; namespace: string };
      }>(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/cloudtty`, {
        method: "POST",
        body: JSON.stringify(form),
      });
      if (!data.cloudshell) throw new Error("CloudTTY 未返回 CloudShell 引用");
      const servicePath =
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}` +
        `/proxy/api/v1/namespaces/${encodeURIComponent(data.cloudshell.namespace)}` +
        `/services/${encodeURIComponent(data.cloudshell.name)}`;
      let ready = false;
      for (let attempt = 0; attempt < 10; attempt += 1) {
        try {
          await k8sFetch(servicePath);
          ready = true;
          break;
        } catch {
          await new Promise((resolve) => window.setTimeout(resolve, 1000));
        }
      }
      if (!ready) throw new Error("CloudTTY 服务尚未就绪，请稍后重试");
      setFrameURL(cloudTTYProxyPath(clusterId, data.cloudshell.namespace, data.cloudshell.name));
    } catch (err) {
      setError(errorInfo(err, "创建 CloudShell 会话失败"));
    } finally {
      setConnecting(false);
    }
  }

  function disconnect() {
    setFrameURL("");
    setError(null);
  }

  if (!clusterId) {
    return (
      <div className="k8s-cloudshell">
        <K8sEmptyState variant="no-cluster" />
      </div>
    );
  }

  return (
    <div className="k8s-cloudshell stack-md">
      {!canManage ? (
        <p className="text-sm text-muted">需要 governance:admin 权限才能创建 CloudShell 会话。</p>
      ) : null}

      <PodTargetPicker clusterId={clusterId} value={form} onChange={setForm} disabled={!canManage || connecting} onError={setError} />

      <div className="k8s-cloudshell-actions">
        <span className="text-xs text-muted">基于 CloudTTY，会话约 30 分钟后自动过期</span>
        <div className="k8s-cloudshell-action-buttons">
          {frameURL ? (
            <button type="button" className="btn-secondary text-sm" disabled={connecting} onClick={disconnect}>
              断开
            </button>
          ) : null}
          <button type="button" className="btn text-sm" disabled={!canConnect || connecting} onClick={() => void start()}>
            {connecting ? "连接中…" : frameURL ? "重新连接" : "连接"}
          </button>
        </div>
      </div>

      <ApiErrorBanner error={error} />

      {connecting ? <K8sLoadingState label="正在创建 CloudShell 会话…" compact /> : null}

      {frameURL ? (
        <iframe
          className="k8s-cloudshell-frame"
          src={frameURL}
          sandbox="allow-scripts allow-forms allow-popups allow-downloads"
          title="CloudShell 终端"
        />
      ) : !connecting ? (
        <div className="k8s-cloudshell-placeholder">
          <p className="text-sm text-muted">选择目标并连接后，终端将显示在此处。</p>
        </div>
      ) : null}
    </div>
  );
}
