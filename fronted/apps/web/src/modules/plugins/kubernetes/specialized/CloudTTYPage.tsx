"use client";

import { useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { cloudTTYProxyPath } from "../operations";
import { EndpointForm, TargetFields } from "./FormParts";

export function CloudTTYPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState({ namespace: "default", pod: "", container: "" });
  const [result, setResult] = useState("");
  const [frameURL, setFrameURL] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function start() {
    setError(null);
    setResult("");
    setFrameURL("");
    try {
      const data = await k8sFetch<{
        id: string;
        cloudshell?: { name: string; namespace: string };
      }>(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/cloudtty`, {
        method: "POST",
        body: JSON.stringify(form),
      });
      setResult(JSON.stringify(data, null, 2));
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
      if (!ready) throw new Error("CloudTTY Service 尚未就绪，请稍后重试");
      setFrameURL(cloudTTYProxyPath(clusterId, data.cloudshell.namespace, data.cloudshell.name));
    } catch (err) {
      setError(errorInfo(err, "创建 CloudTTY 失败"));
    }
  }

  return (
    <EndpointForm disabled={!clusterId || !canManage} submitLabel="创建 CloudTTY 会话" onSubmit={start}>
      <TargetFields form={form} setForm={setForm} />
      <div className="md:col-span-3">
        <ApiErrorBanner error={error} />
      </div>
      <pre className="input input-mono min-h-40 whitespace-pre-wrap text-xs md:col-span-3">{result}</pre>
      {frameURL ? (
        <iframe
          className="min-h-[32rem] w-full rounded border md:col-span-3"
          src={frameURL}
          sandbox="allow-scripts allow-forms allow-popups allow-downloads"
          title="CloudTTY"
        />
      ) : null}
    </EndpointForm>
  );
}
