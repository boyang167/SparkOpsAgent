"use client";

import { useState } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "./api";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "./sse";
import { podOperationPath, toCommandQuery } from "./operations";

type Props = {
  clusterId: string;
  canManage: boolean;
  mode: "pods" | "nodes" | "metrics";
};

type Session = { pod?: { name: string; namespace: string } };

export function OperationsPanel({ clusterId, canManage, mode }: Props) {
  if (mode === "pods") return <PodOperations clusterId={clusterId} canManage={canManage} />;
  if (mode === "nodes") return <NodeOperations clusterId={clusterId} canManage={canManage} />;
  return <MetricsOperations clusterId={clusterId} />;
}

function PodOperations({ clusterId, canManage }: Omit<Props, "mode">) {
  const [target, setTarget] = useState({ namespace: "default", pod: "", container: "", path: "/tmp" });
  const [command, setCommand] = useState("sh -c date");
  const [output, setOutput] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const base = podOperationPath(clusterId, target.namespace, target.pod, "");
  const query = (extra: Record<string, string> = {}) => {
    const params = new URLSearchParams({ container: target.container, ...extra });
    return params.toString();
  };

  async function stream(operation: "logs" | "exec") {
    setError(null);
    setOutput([]);
    try {
      const suffix = operation === "exec" ? `${query()}&${toCommandQuery(splitCommand(command))}` : query();
      await readSSE(`${base}${operation}?${suffix}`, setOutput);
    } catch (err) {
      setError(errorInfo(err, `${operation} 请求失败`));
    }
  }

  async function listFiles() {
    try {
      const data = await k8sFetch<{ output: string }>(`${base}files?${query({ path: target.path })}`);
      setOutput(data.output.split("\n"));
    } catch (err) {
      setError(errorInfo(err, "读取文件列表失败"));
    }
  }

  async function upload(file?: File) {
    if (!file) return;
    const form = new FormData();
    form.append("file", file);
    try {
      const res = await fetch(`${base}files?${query({ path: target.path })}`, { method: "POST", body: form });
      if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
      await listFiles();
    } catch (err) {
      setError(errorInfo(err, "上传文件失败"));
    }
  }

  async function download() {
    try {
      const res = await fetch(`${base}files/download?${query({ path: target.path })}`, { cache: "no-store" });
      if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
      const blob = await res.blob();
      const href = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = href;
      anchor.download = target.path.split("/").pop() || "download";
      anchor.click();
      URL.revokeObjectURL(href);
    } catch (err) {
      setError(errorInfo(err, "下载文件失败"));
    }
  }

  async function remove() {
    try {
      await k8sFetch(`${base}files?${query({ path: target.path })}`, { method: "DELETE" });
      await listFiles();
    } catch (err) {
      setError(errorInfo(err, "删除文件失败"));
    }
  }

  return (
    <section className="card stack-sm p-4 mt-4">
      <h2 className="font-semibold">Pod 运维</h2>
      <p className="text-sm text-muted">Exec 仅执行一次性命令；当前后端不支持交互式 stdin 终端。</p>
      <div className="grid gap-2 md:grid-cols-4">
        {(["namespace", "pod", "container", "path"] as const).map((key) => (
          <input key={key} className="input" aria-label={key} placeholder={key} value={target[key]}
            onChange={(e) => setTarget({ ...target, [key]: e.target.value })} />
        ))}
      </div>
      <input className="input input-mono" value={command} onChange={(e) => setCommand(e.target.value)} />
      <div className="flex flex-wrap gap-2">
        <button className="btn-secondary" disabled={!clusterId || !target.pod} onClick={() => void stream("logs")}>读取日志 SSE</button>
        {canManage ? <button className="btn-secondary" disabled={!clusterId || !target.pod} onClick={() => void stream("exec")}>执行一次性命令</button> : null}
        <button className="btn-secondary" disabled={!clusterId || !target.pod} onClick={() => void listFiles()}>列出文件</button>
        <button className="btn-secondary" disabled={!clusterId || !target.pod} onClick={() => void download()}>下载</button>
        {canManage ? <label className="btn-secondary">上传<input className="hidden" type="file" onChange={(e) => void upload(e.target.files?.[0])} /></label> : null}
        {canManage ? <button className="btn-secondary" disabled={!clusterId || !target.pod} onClick={() => void remove()}>删除</button> : null}
      </div>
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-32 whitespace-pre-wrap text-xs">{output.join("\n")}</pre>
    </section>
  );
}

function MetricsOperations({ clusterId }: { clusterId: string }) {
  const [form, setForm] = useState({ template: "node_cpu", range: "1h", node: "", namespace: "", pod: "" });
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  async function query() {
    try {
      const data = await k8sFetch<unknown>(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/metrics`, {
        method: "POST", body: JSON.stringify(form),
      });
      setResult(JSON.stringify(data, null, 2));
    } catch (err) {
      setError(errorInfo(err, "指标查询失败"));
    }
  }
  return (
    <section className="card stack-sm p-4 mt-4">
      <h2 className="font-semibold">命名指标模板</h2>
      <div className="grid gap-2 md:grid-cols-5">
        <select className="input" value={form.template} onChange={(e) => setForm({ ...form, template: e.target.value })}>
          {["node_cpu", "node_memory", "pod_cpu", "pod_memory"].map((v) => <option key={v}>{v}</option>)}
        </select>
        {(["range", "node", "namespace", "pod"] as const).map((key) => (
          <input key={key} className="input" placeholder={key} value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} />
        ))}
      </div>
      <button className="btn-secondary" disabled={!clusterId} onClick={() => void query()}>查询指标</button>
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-24 whitespace-pre-wrap text-xs">{result}</pre>
    </section>
  );
}

function NodeOperations({ clusterId, canManage }: Omit<Props, "mode">) {
  const [node, setNode] = useState("");
  const [command, setCommand] = useState("uname -a");
  const [output, setOutput] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  async function run() {
    try {
      const session = await k8sFetch<Session>(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/nodetty`, {
        method: "POST", body: JSON.stringify({ node }),
      });
      if (!session.pod) throw new Error("NodeTTY 未返回 Pod");
      const url = podOperationPath(clusterId, session.pod.namespace, session.pod.name, "exec");
      await readSSE(`${url}?container=node-shell&${toCommandQuery(splitCommand(command))}`, setOutput);
    } catch (err) {
      setError(errorInfo(err, "NodeTTY 创建或命令执行失败"));
    }
  }
  return (
    <section className="card stack-sm p-4 mt-4">
      <h2 className="font-semibold">NodeTTY 一次性命令</h2>
      <p className="text-sm text-muted">创建特权临时 Pod 后执行一次性命令，不提供交互式终端。</p>
      <input className="input" placeholder="节点名" value={node} onChange={(e) => setNode(e.target.value)} />
      <input className="input input-mono" value={command} onChange={(e) => setCommand(e.target.value)} />
      {canManage ? <button className="btn-secondary" disabled={!clusterId || !node} onClick={() => void run()}>创建并执行</button> : null}
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-24 whitespace-pre-wrap text-xs">{output.join("\n")}</pre>
    </section>
  );
}

async function readSSE(url: string, setOutput: (updater: (current: string[]) => string[]) => void) {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
  if (!res.body) throw new Error("SSE 响应为空");
  const parser = createSSEParser((event) => {
    setOutput((current) => appendBoundedLines(current, `${event.event}: ${formatSSEPayload(event.data)}`, { maxLines: 1000, maxBytes: 1 << 20 }));
  });
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    parser.push(value);
  }
  parser.flush();
}

function splitCommand(raw: string) {
  return raw.trim().split(/\s+/).filter(Boolean);
}
