"use client";

import { useEffect, useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { appendBoundedLines, formatSSEPayload } from "../sse";
import { EndpointForm, Field, TargetFields } from "./FormParts";

const STREAM_LIMIT = { maxLines: 1000, maxBytes: 1024 * 1024 };

export function DiagnosisPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState({ namespace: "default", pod: "", container: "", pid: "1" });
  const [sessionId, setSessionId] = useState("");
  const [lines, setLines] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function start() {
    setError(null);
    setLines([]);
    try {
      const data = await k8sFetch<{ id: string; status: string }>(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/diagnosis`,
        { method: "POST", body: JSON.stringify({ ...form, pid: Number(form.pid) }) },
      );
      setSessionId(data.id);
    } catch (err) {
      setError(errorInfo(err, "启动诊断失败"));
    }
  }

  useEffect(() => {
    if (!sessionId) return;
    const source = new EventSource(`/api/bff/v1/plugins/kubernetes/diagnosis/${encodeURIComponent(sessionId)}/ws`);
    const append = (event: MessageEvent) => {
      setLines((prev) => appendBoundedLines(prev, safeEventData(event.data), STREAM_LIMIT));
    };
    source.onmessage = append;
    source.addEventListener("output", append);
    source.addEventListener("error", append);
    source.addEventListener("exit", (event) => {
      append(event as MessageEvent);
      source.close();
    });
    return () => source.close();
  }, [sessionId]);

  return (
    <EndpointForm disabled={!clusterId || !canManage} submitLabel="启动 Arthas 诊断" onSubmit={start}>
      <TargetFields form={form} setForm={setForm} />
      <Field label="PID">
        <input className="input" value={form.pid} onChange={(e) => setForm({ ...form, pid: e.target.value })} />
      </Field>
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs">{lines.join("\n")}</pre>
    </EndpointForm>
  );
}

function safeEventData(data: string): string {
  try {
    return formatSSEPayload(JSON.parse(data));
  } catch {
    return data || "诊断流错误或已结束";
  }
}
