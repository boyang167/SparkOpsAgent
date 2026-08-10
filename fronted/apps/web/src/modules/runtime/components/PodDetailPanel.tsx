"use client";

import { useEffect, useRef, useState } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PodVM } from "@sparkops/contracts/cluster";

interface PodDetailPanelProps {
  workspaceId: string;
  projectId: string;
  pod: PodVM;
  onClose: () => void;
}

export function PodDetailPanel({ workspaceId, projectId, pod, onClose }: PodDetailPanelProps) {
  const [logs, setLogs] = useState<string>("");
  const [terminal, setTerminal] = useState<string>("");
  const [logsLoading, setLogsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const terminalRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    setLogsLoading(true);
    setError(null);
    void cluster
      .getPodLogs(projectId, pod.id, workspaceId)
      .then((data) => setLogs(data.logs))
      .catch((err) => setError(err instanceof BffError ? err.message : "加载日志失败"))
      .finally(() => setLogsLoading(false));
  }, [projectId, pod.id, workspaceId]);

  useEffect(() => {
    setTerminal("");
    const stop = cluster.streamPodTerminal(projectId, pod.id, workspaceId, {
      onDelta: (delta) => {
        setTerminal((prev) => {
          const next = prev + delta;
          if (terminalRef.current) {
            terminalRef.current.textContent = next;
            terminalRef.current.scrollTop = terminalRef.current.scrollHeight;
          }
          return next;
        });
      },
      onDone: () => {},
    });
    return stop;
  }, [projectId, pod.id, workspaceId]);

  return (
    <section className="card stack-md border-[var(--border-strong)]">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="font-medium">{pod.name}</h3>
          <p className="text-xs text-muted mt-1">
            {pod.namespace} · {pod.phase} · {pod.node}
          </p>
        </div>
        <button className="btn-secondary text-sm" type="button" onClick={onClose}>
          关闭
        </button>
      </div>

      <dl className="grid gap-2 sm:grid-cols-2 text-sm">
        <div>
          <dt className="text-muted">容器</dt>
          <dd>{pod.container}</dd>
        </div>
        <div>
          <dt className="text-muted">镜像</dt>
          <dd className="font-mono text-xs break-all">{pod.image}</dd>
        </div>
        <div>
          <dt className="text-muted">重启次数</dt>
          <dd>{pod.restarts}</dd>
        </div>
        <div>
          <dt className="text-muted">启动时间</dt>
          <dd>{new Date(pod.started_at).toLocaleString("zh-CN")}</dd>
        </div>
      </dl>

      {error ? <p className="error text-sm">{error}</p> : null}

      <div className="stack-sm">
        <h4 className="text-sm font-medium">Pod 日志</h4>
        {logsLoading ? (
          <p className="text-sm text-muted">加载中…</p>
        ) : (
          <pre className="text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-48 overflow-y-auto font-mono">
            {logs || "暂无日志"}
          </pre>
        )}
      </div>

      <div className="stack-sm">
        <div className="flex items-center gap-2">
          <h4 className="text-sm font-medium">Web 终端（Mock）</h4>
          <span className="text-xs text-muted">SSE 模拟 shell 输出</span>
        </div>
        <pre
          ref={terminalRef}
          className="text-xs bg-[#1e1e1e] text-[#d4d4d4] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-48 overflow-y-auto font-mono"
        >
          {terminal || "$ connecting…\n"}
        </pre>
      </div>
    </section>
  );
}
