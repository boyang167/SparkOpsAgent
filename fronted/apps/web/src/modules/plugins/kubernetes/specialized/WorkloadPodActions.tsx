"use client";

import { useEffect, useRef, useState } from "react";
import {
  K8sActionIcons,
  K8sActionSheet,
  K8sActionSheetGroup,
  K8sActionSheetItem,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { podContainerNames, podPhase, type PodObject } from "../node-view";
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
  trigger = "link",
  showDetail = true,
}: {
  clusterId: string;
  pod: PodObject;
  canManage: boolean;
  onOpenDetail: (pod: PodObject) => void;
  onDeleted: () => Promise<void> | void;
  onError: (error: ApiErrorInfo) => void;
  trigger?: "link" | "ghost";
  showDetail?: boolean;
}) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const [view, setView] = useState<PodActionView | null>(null);
  const [terminalFullscreen, setTerminalFullscreen] = useState(false);
  const { ask, dialog: confirmDialog } = useK8sConfirm();
  const podName = pod.metadata?.name ?? "-";
  const namespace = pod.metadata?.namespace ?? "-";
  const title = `${namespace}/${podName}`;

  useEffect(() => {
    setSheetOpen(false);
    setView(null);
    setTerminalFullscreen(false);
  }, [clusterId, namespace, podName]);

  async function deletePod() {
    const ns = pod.metadata?.namespace;
    const name = pod.metadata?.name;
    if (!canManage || !podDef || !ns || !name) return;
    const ok = await ask({
      title: "删除 Pod",
      description: `确定删除 Pod「${ns}/${name}」？`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    try {
      await k8sFetch(proxyPath(clusterId, podDef, ns, name), { method: "DELETE" });
      await onDeleted();
    } catch (err) {
      onError(errorInfo(err, "删除 Pod 失败"));
    }
  }

  const containers = podContainerNames(pod);
  const triggerClass = trigger === "ghost" ? "btn-ghost text-sm" : "k8s-action-link";

  return (
    <>
      <button type="button" className={triggerClass} onClick={() => setSheetOpen(true)}>
        更多
      </button>

      <Modal
        nested
        open={sheetOpen}
        title={podName}
        description={`${namespace} · ${podPhase(pod)}`}
        size="sm"
        panelClassName="k8s-cluster-dialog-modal k8s-action-sheet-modal"
        onClose={() => setSheetOpen(false)}
      >
        <K8sActionSheet>
          {showDetail ? (
            <K8sActionSheetGroup title="查看">
              <K8sActionSheetItem
                label="详情"
                icon={K8sActionIcons.detail}
                onClick={() => {
                  setSheetOpen(false);
                  onOpenDetail(pod);
                }}
              />
              <K8sActionSheetItem
                label="监控"
                icon={K8sActionIcons.monitor}
                onClick={() => openView({ type: "monitor" })}
              />
            </K8sActionSheetGroup>
          ) : (
            <K8sActionSheetGroup>
              <K8sActionSheetItem
                label="监控"
                icon={K8sActionIcons.monitor}
                onClick={() => openView({ type: "monitor" })}
              />
            </K8sActionSheetGroup>
          )}
          {containers.map((container) => (
            <K8sActionSheetGroup key={container} title={container}>
              <K8sActionSheetItem
                label="文件"
                onClick={() => openView({ type: "files", container })}
              />
              {canManage ? (
                <K8sActionSheetItem
                  label="终端"
                  icon={K8sActionIcons.shell}
                  onClick={() => openView({ type: "terminal", container })}
                />
              ) : null}
              <K8sActionSheetItem
                label="日志"
                onClick={() => openView({ type: "logs", container })}
              />
            </K8sActionSheetGroup>
          ))}
          {canManage ? (
            <K8sActionSheetGroup danger>
              <K8sActionSheetItem
                label="删除 Pod"
                icon={K8sActionIcons.delete}
                danger
                onClick={() => {
                  setSheetOpen(false);
                  void deletePod();
                }}
              />
            </K8sActionSheetGroup>
          ) : null}
        </K8sActionSheet>
      </Modal>

      <Modal
        nested
        open={view?.type === "files"}
        title={view?.type === "files" ? "文件浏览" : "文件浏览"}
        description={view?.type === "files" ? `${title} · ${view.container}` : undefined}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setView(null)}
      >
        {view?.type === "files" ? (
          <PodFilesBrowser clusterId={clusterId} pod={pod} container={view.container} canManage={canManage} />
        ) : null}
      </Modal>

      <Modal
        nested
        open={view?.type === "monitor"}
        title="监控"
        description={title}
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => setView(null)}
      >
        {view?.type === "monitor" ? <PodMetricsPanel clusterId={clusterId} pod={pod} /> : null}
      </Modal>

      <Modal
        nested
        open={view?.type === "terminal"}
        title="终端"
        description={view?.type === "terminal" ? `${title} · ${view.container}` : undefined}
        size="xl"
        panelClassName={
          view?.type === "terminal"
            ? terminalFullscreen
              ? "k8s-pod-terminal-modal-fullscreen"
              : "k8s-pod-terminal-modal"
            : "k8s-pod-submodal"
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
        title="日志"
        description={view?.type === "logs" ? `${title} · ${view.container}` : undefined}
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => setView(null)}
      >
        {view?.type === "logs" ? <PodLogPanel clusterId={clusterId} pod={pod} container={view.container} /> : null}
      </Modal>
      {confirmDialog}
    </>
  );

  function openView(next: PodActionView) {
    setSheetOpen(false);
    setView(next);
  }
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
    <div className="k8s-stream-view stack-sm">
      <div className="k8s-stream-toolbar">
        <span className="text-sm text-muted">{running ? "接收中…" : "实时日志"}</span>
        <button type="button" className="btn-secondary text-sm" disabled={running} onClick={() => void run()}>
          刷新
        </button>
      </div>
      <ApiErrorBanner error={error} />
      <pre className="k8s-stream-panel">{output.join("\n") || "暂无输出"}</pre>
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
    <div className="k8s-stream-view stack-sm">
      <div className="k8s-stream-toolbar">
        <span className="text-sm text-muted">{loading ? "查询中…" : "Prometheus 指标"}</span>
        <button type="button" className="btn-secondary text-sm" disabled={loading} onClick={() => void query()}>
          刷新
        </button>
      </div>
      <ApiErrorBanner error={error} />
      <pre className="k8s-stream-panel">{result || "暂无指标（需配置 Prometheus）"}</pre>
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
