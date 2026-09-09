"use client";

import { useEffect, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { podContainerNames, type PodObject } from "../node-view";
import { podOperationPath } from "../operations";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "../sse";
import { PodTerminal } from "./PodTerminal";
import { PodFilesBrowser } from "./PodFilesBrowser";
import { METRIC_TEMPLATES } from "./WorkloadPodActions.helpers";

const podDef = CATALOG.find((item) => item.kind === "Pod");
const STREAM_LIMIT = { maxLines: 1000, maxBytes: 1 << 20 };
type PodActionView =
  | { type: "files"; container: string }
  | { type: "monitor" }
  | { type: "terminal"; container: string }
  | { type: "logs"; container: string };

export function WorkloadPodActions({
  clusterId,
  pod,
  canManage,
  onOpenDetail,
  onDeleted,
  onError,
}: {
  clusterId: string;
  pod: PodObject;
  canManage: boolean;
  onOpenDetail: (pod: PodObject) => void;
  onDeleted: () => Promise<void> | void;
  onError: (error: ApiErrorInfo) => void;
}) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const [view, setView] = useState<PodActionView | null>(null);
  const [terminalFullscreen, setTerminalFullscreen] = useState(false);
  const title = `${pod.metadata?.namespace ?? "-"}/${pod.metadata?.name ?? "-"}`;

  useEffect(() => {
    setSheetOpen(false);
    setView(null);
    setTerminalFullscreen(false);
  }, [clusterId, pod.metadata?.namespace, pod.metadata?.name]);

  async function deletePod() {
    const namespace = pod.metadata?.namespace;
    const name = pod.metadata?.name;
    if (!canManage || !podDef || !namespace || !name) return;
    if (!window.confirm(`确定删除 Pod「${namespace}/${name}」？`)) return;
    try {
      await k8sFetch(proxyPath(clusterId, podDef, namespace, name), { method: "DELETE" });
      await onDeleted();
    } catch (err) {
      onError(errorInfo(err, "删除 Pod 失败"));
    }
  }

  const containers = podContainerNames(pod);

  return (
    <>
      <button type="button" className="k8s-action-link" onClick={() => setSheetOpen(true)}>
        更多
      </button>

      <Modal nested open={sheetOpen} title={`更多操作 · ${pod.metadata?.name ?? ""}`} description={title} size="md" onClose={() => setSheetOpen(false)}>
        <div className="k8s-action-sheet">
          <ActionItem label="详情" hint="查看 Pod 详情" onClick={openDetail} />
          {containers.map((container) => (
            <ActionItem
              key={`files-${container}`}
              label={`${container} 文件浏览`}
              hint="列出、上传、下载或删除容器内文件"
              onClick={() => openView({ type: "files", container })}
            />
          ))}
          <ActionItem label="监控" hint="查看 Pod CPU / 内存指标" onClick={() => openView({ type: "monitor" })} />
          {containers.map((container) => (
            <ActionItem
              key={`terminal-${container}`}
              label={`${container} 终端`}
              hint="打开交互式容器终端"
              disabled={!canManage}
              onClick={() => openView({ type: "terminal", container })}
            />
          ))}
          {containers.map((container) => (
            <ActionItem
              key={`logs-${container}`}
              label={`${container} 终端日志`}
              hint="查看容器标准输出日志"
              onClick={() => openView({ type: "logs", container })}
            />
          ))}
          <ActionItem label="删除" hint="删除该 Pod 对象" disabled={!canManage} danger onClick={() => void deletePod()} />
        </div>
      </Modal>

      <Modal
        nested
        open={view?.type === "files"}
        title={
          view?.type === "files"
            ? `Pod: ${pod.metadata?.name ?? ""} Container: ${view.container} 文件浏览`
            : "文件浏览"
        }
        size="xl"
        onClose={() => setView(null)}
      >
        {view?.type === "files" ? (
          <PodFilesBrowser clusterId={clusterId} pod={pod} container={view.container} canManage={canManage} />
        ) : null}
      </Modal>

      <Modal nested open={view?.type === "monitor"} title={`Pod 监控 ${pod.metadata?.name ?? ""}`} size="lg" onClose={() => setView(null)}>
        {view?.type === "monitor" ? <PodMetricsPanel clusterId={clusterId} pod={pod} /> : null}
      </Modal>

      <Modal
        nested
        open={view?.type === "terminal"}
        title={view?.type === "terminal" ? `容器组名: ${pod.metadata?.name} 容器名: ${view.container} 终端` : "容器终端"}
        size="xl"
        panelClassName={
          view?.type === "terminal"
            ? terminalFullscreen
              ? "k8s-pod-terminal-modal-fullscreen"
              : "k8s-pod-terminal-modal"
            : undefined
        }
        headerExtra={
          view?.type === "terminal" ? (
            <button
              type="button"
              className="modal-header-action"
              aria-label={terminalFullscreen ? "退出全屏" : "全屏"}
              title={terminalFullscreen ? "退出全屏" : "全屏"}
              onClick={() => setTerminalFullscreen((value) => !value)}
            >
              <span aria-hidden>{terminalFullscreen ? "⊙" : "⛶"}</span>
            </button>
          ) : undefined
        }
        onClose={() => {
          setView(null);
          setTerminalFullscreen(false);
        }}
      >
        {view?.type === "terminal" ? (
          <PodTerminal
            clusterId={clusterId}
            namespace={pod.metadata?.namespace ?? ""}
            pod={pod.metadata?.name ?? ""}
            container={view.container}
            canManage={canManage}
          />
        ) : null}
      </Modal>

      <Modal
        nested
        open={view?.type === "logs"}
        title={view?.type === "logs" ? `终端日志 · ${pod.metadata?.name}/${view.container}` : "终端日志"}
        size="lg"
        onClose={() => setView(null)}
      >
        {view?.type === "logs" ? <PodLogPanel clusterId={clusterId} pod={pod} container={view.container} /> : null}
      </Modal>
    </>
  );

  function openView(next: PodActionView) {
    setSheetOpen(false);
    setView(next);
  }

  function openDetail() {
    setSheetOpen(false);
    onOpenDetail(pod);
  }
}

function ActionItem({
  label,
  hint,
  disabled,
  danger,
  onClick,
}: {
  label: string;
  hint: string;
  disabled?: boolean;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={danger ? "k8s-action-sheet-item is-danger" : "k8s-action-sheet-item"}
      disabled={disabled}
      onClick={onClick}
    >
      <span className="k8s-action-sheet-label">{label}</span>
      <span className="k8s-action-sheet-hint">{hint}</span>
    </button>
  );
}

function PodLogPanel({ clusterId, pod, container }: { clusterId: string; pod: PodObject; container: string }) {
  const [output, setOutput] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [running, setRunning] = useState(false);
  const active = useRef<AbortController | null>(null);
  const namespace = pod.metadata?.namespace ?? "";
  const podName = pod.metadata?.name ?? "";

  useEffect(() => {
    void run();
    return () => active.current?.abort();
  }, [clusterId, namespace, podName, container]);

  async function run() {
    if (!namespace || !podName) return;
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    setRunning(true);
    setError(null);
    setOutput([]);
    try {
      const base = podOperationPath(clusterId, namespace, podName, "logs");
      const params = new URLSearchParams({ container, follow: "true" });
      await readSSE(`${base}?${params.toString()}`, setOutput, controller.signal);
    } catch (err) {
      if (!controller.signal.aborted) setError(errorInfo(err, "读取日志失败"));
    } finally {
      if (active.current === controller) {
        active.current = null;
        setRunning(false);
      }
    }
  }

  return (
    <div className="stack-md">
      <button type="button" className="btn-secondary" disabled={running} onClick={() => void run()}>
        {running ? "加载中…" : "刷新日志"}
      </button>
      <ApiErrorBanner error={error} />
      <pre className="k8s-yaml-panel font-mono text-xs min-h-64 whitespace-pre-wrap">{output.join("\n") || "暂无输出"}</pre>
    </div>
  );
}

function PodMetricsPanel({ clusterId, pod }: { clusterId: string; pod: PodObject }) {
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const active = useRef<AbortController | null>(null);

  useEffect(() => {
    void query();
    return () => active.current?.abort();
  }, [clusterId, pod.metadata?.namespace, pod.metadata?.name]);

  async function query() {
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    const namespace = pod.metadata?.namespace;
    const podName = pod.metadata?.name;
    if (!namespace || !podName) return;
    setLoading(true);
    setError(null);
    try {
      const endpoint = `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/metrics`;
      const responses = await Promise.allSettled(
        Object.entries(METRIC_TEMPLATES).map(async ([key, template]) => {
          const value = await k8sFetch<unknown>(endpoint, {
            method: "POST",
            signal: controller.signal,
            body: JSON.stringify({ template, range: "1h", namespace, pod: podName }),
          });
          return [key, value] as const;
        }),
      );
      const data: Record<string, unknown> = {};
      for (const item of responses) {
        if (item.status === "fulfilled") data[item.value[0]] = item.value[1];
      }
      if (!controller.signal.aborted) setResult(JSON.stringify(data, null, 2));
      if (!controller.signal.aborted && Object.keys(data).length === 0) {
        setError({ message: "暂无指标数据（需配置 Prometheus 或对应模板）" });
      }
    } catch (err) {
      if (!controller.signal.aborted) setError(errorInfo(err, "指标查询失败"));
    } finally {
      if (active.current === controller) {
        active.current = null;
        setLoading(false);
      }
    }
  }

  return (
    <div className="stack-md">
      <button type="button" className="btn-secondary" disabled={loading} onClick={() => void query()}>
        {loading ? "查询中…" : "刷新"}
      </button>
      <ApiErrorBanner error={error} />
      <pre className="k8s-yaml-panel font-mono text-xs min-h-48 whitespace-pre-wrap">{result || "暂无指标（需配置 Prometheus）"}</pre>
    </div>
  );
}

async function readSSE(
  url: string,
  setOutput: (updater: (current: string[]) => string[]) => void,
  signal: AbortSignal,
) {
  const res = await fetch(url, { cache: "no-store", signal });
  if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
  if (!res.body) throw new Error("SSE 响应为空");
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  const parser = createSSEParser((event) => {
    setOutput((current) => appendBoundedLines(current, `${event.event}: ${formatSSEPayload(event.data)}`, STREAM_LIMIT));
  });
  signal.addEventListener("abort", () => void reader.cancel().catch(() => undefined), { once: true });
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      parser.push(value);
    }
    parser.flush();
  } finally {
    reader.releaseLock();
  }
}
