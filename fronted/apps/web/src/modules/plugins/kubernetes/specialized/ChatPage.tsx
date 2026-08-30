"use client";

import { useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { KubernetesApiError, errorInfo, parseApiError, type ApiErrorInfo } from "../api";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "../sse";
import { EndpointForm } from "./FormParts";

const STREAM_LIMIT = { maxLines: 1000, maxBytes: 1024 * 1024 };

export function ChatPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [prompt, setPrompt] = useState("");
  const [lines, setLines] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function send() {
    setError(null);
    setLines([]);
    try {
      const res = await fetch(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/chat/query`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt }),
      });
      if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
      const parser = createSSEParser((event) => {
        if (event.event === "chunk" || event.event === "message") {
          setLines((prev) => appendBoundedLines(prev, formatSSEPayload(event.data), STREAM_LIMIT));
        }
      });
      const reader = res.body?.getReader();
      const decoder = new TextDecoder();
      while (reader) {
        const { value, done } = await reader.read();
        if (done) break;
        parser.push(decoder.decode(value, { stream: true }));
      }
      parser.flush();
    } catch (err) {
      setError(errorInfo(err, "Chat 请求失败"));
    }
  }

  return (
    <EndpointForm disabled={!clusterId || !canManage || !prompt.trim()} submitLabel="发送" onSubmit={send}>
      <label className="md:col-span-3">
        <span className="label">问题</span>
        <textarea className="input min-h-28" value={prompt} onChange={(e) => setPrompt(e.target.value)} />
      </label>
      <div className="md:col-span-3">
        <ApiErrorBanner error={error} />
      </div>
      <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs md:col-span-3">{lines.join("\n")}</pre>
    </EndpointForm>
  );
}
