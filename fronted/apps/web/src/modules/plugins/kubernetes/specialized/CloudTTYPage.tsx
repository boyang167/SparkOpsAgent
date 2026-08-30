"use client";

import { useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { EndpointForm, TargetFields } from "./FormParts";

export function CloudTTYPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState({ namespace: "default", pod: "", container: "" });
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function start() {
    setError(null);
    setResult("");
    try {
      const data = await k8sFetch(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/cloudtty`, {
        method: "POST",
        body: JSON.stringify(form),
      });
      setResult(JSON.stringify(data, null, 2));
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
    </EndpointForm>
  );
}
