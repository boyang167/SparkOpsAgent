"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "../sse";
import { podOperationPath } from "../operations";
import { YamlEditor } from "../YamlEditor";
import { PodTerminal } from "./PodTerminal";
import { PodFilesBrowser } from "./PodFilesBrowser";
import {
  formatDateTime,
  formatNodeAge,
  nodeInternalIP,
  podContainerNames,
  podPhase,
  podResourceSummary,
  podRestartCount,
  type NodeObject,
  type PodObject,
} from "../node-view";

const podDef = CATALOG.find((item) => item.kind === "Pod");

async function loadNodePods(clusterId: string, nodeName: string): Promise<PodObject[]> {
  const data = await k8sFetch<{ items?: PodObject[] }>(
    `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/pods?fieldSelector=${encodeURIComponent(`spec.nodeName=${nodeName}`)}&page=1&pageSize=500`,
  );
  return data.items ?? [];
}

function MetaPills({ map, bordered }: { map?: Record<string, string>; bordered?: boolean }) {
  const entries = Object.entries(map ?? {});
  if (entries.length === 0) return <span className="text-muted">-</span>;
  return (
    <div className="k8s-role-badges">
      {entries.map(([key, value]) => (
        <span key={key} className={bordered ? "badge badge-neutral" : "badge badge-primary"} title={`${key}: ${value}`}>
          {key}: {value || '""'}
        </span>
      ))}
    </div>
  );
}

function InfoRow({ left, right }: { left: React.ReactNode; right?: React.ReactNode }) {
  return (
    <div className="k8s-detail-row">
      <div>{left}</div>
      {right !== undefined ? <div>{right}</div> : null}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="k8s-detail-field">
      <span className="k8s-detail-label">{label}</span>
      <span className="k8s-detail-sep">:</span>
      <span className="k8s-detail-value">{children}</span>
    </div>
  );
}

type StreamTarget = {
  pod: PodObject;
  container: string;
  mode: "exec" | "logs";
};

export function NodeDetailView({
  clusterId,
  node,
  canManage,
}: {
  clusterId: string;
  node: NodeObject;
  canManage: boolean;
}) {
  const [pods, setPods] = useState<PodObject[]>([]);
  const [loadingPods, setLoadingPods] = useState(true);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [editPod, setEditPod] = useState<PodObject | null>(null);
  const [podDetail, setPodDetail] = useState<PodObject | null>(null);
  const [morePod, setMorePod] = useState<PodObject | null>(null);
  const [stream, setStream] = useState<StreamTarget | null>(null);
  const [filesTarget, setFilesTarget] = useState<{ pod: PodObject; container: string } | null>(null);
  const [terminalFullscreen, setTerminalFullscreen] = useState(false);
  const [saving, setSaving] = useState(false);
  const info = node.status?.nodeInfo;
  const name = node.metadata?.name ?? "";

  useEffect(() => {
    let cancelled = false;
    async function run() {
      if (!clusterId || !name) return;
      setLoadingPods(true);
      setError(null);
      try {
        const items = await loadNodePods(clusterId, name);
        if (!cancelled) setPods(items);
      } catch (err) {
        if (!cancelled) {
          setPods([]);
          setError(errorInfo(err, "加载节点 Pod 失败"));
        }
      } finally {
        if (!cancelled) setLoadingPods(false);
      }
    }
    void run();
    return () => {
      cancelled = true;
    };
  }, [clusterId, name]);

  async function reloadPods() {
    if (!name) return;
    setPods(await loadNodePods(clusterId, name));
  }

  async function savePod(next: unknown) {
    const pod = next as PodObject;
    const podName = pod.metadata?.name;
    const ns = pod.metadata?.namespace;
    if (!podDef || !podName || !ns) return;
    setSaving(true);
    try {
      await k8sFetch(proxyPath(clusterId, podDef, ns, podName), { method: "PUT", body: JSON.stringify(next) });
      setEditPod(null);
      await reloadPods();
    } catch (err) {
      setError(errorInfo(err, "保存 Pod 失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function openPodYaml(pod: PodObject) {
    const podName = pod.metadata?.name;
    const ns = pod.metadata?.namespace;
    if (!podDef || !podName || !ns) return;
    try {
      setEditPod(await k8sFetch<PodObject>(proxyPath(clusterId, podDef, ns, podName)));
    } catch (err) {
      setError(errorInfo(err, "加载 Pod 失败"));
    }
  }

  async function deletePod(pod: PodObject) {
    const podName = pod.metadata?.name;
    const ns = pod.metadata?.namespace;
    if (!canManage || !podDef || !podName || !ns) return;
    if (!window.confirm(`确定删除 Pod「${ns}/${podName}」？`)) return;
    try {
      await k8sFetch(proxyPath(clusterId, podDef, ns, podName), { method: "DELETE" });
      await reloadPods();
    } catch (err) {
      setError(errorInfo(err, "删除 Pod 失败"));
    }
  }

  return (
    <div className="stack-lg k8s-node-detail">
      <ApiErrorBanner error={error} />

      <section className="stack-sm">
        <h3 className="k8s-detail-section-title">基本信息</h3>
        <div className="k8s-detail-grid">
          <InfoRow
            left={<Field label="名称">{node.metadata?.name || "-"}</Field>}
            right={<Field label="存活时间">{formatNodeAge(node.metadata?.creationTimestamp)}</Field>}
          />
          <InfoRow left={<Field label="UID">{node.metadata?.uid || "-"}</Field>} />
          <InfoRow
            left={<Field label="容器组 CIDR">{node.spec?.podCIDR || "-"}</Field>}
            right={<Field label="调度状态">{node.spec?.unschedulable ? "不可调度" : "可调度"}</Field>}
          />
          <InfoRow
            left={
              <Field label="IP 地址">
                <div className="stack-xs">
                  {(node.status?.addresses ?? []).length === 0 ? "-" : null}
                  {(node.status?.addresses ?? []).map((item) => (
                    <div key={`${item.type}-${item.address}`}>
                      {item.type}: {item.address}
                    </div>
                  ))}
                </div>
              </Field>
            }
          />
          <InfoRow
            left={
              <Field label="标签">
                <MetaPills map={node.metadata?.labels} />
              </Field>
            }
          />
          <InfoRow
            left={
              <Field label="注解">
                <MetaPills map={node.metadata?.annotations} bordered />
              </Field>
            }
          />
          <InfoRow
            left={<Field label="系统镜像">{info?.osImage || "-"}</Field>}
            right={<Field label="内核版本">{info?.kernelVersion || "-"}</Field>}
          />
          <InfoRow
            left={<Field label="Kubelet 版本">{info?.kubeletVersion || "-"}</Field>}
            right={<Field label="Kube-Proxy 版本">{info?.kubeProxyVersion || "-"}</Field>}
          />
          <InfoRow
            left={<Field label="机器 ID">{info?.machineID || "-"}</Field>}
            right={<Field label="系统 UUID">{info?.systemUUID || "-"}</Field>}
          />
          <InfoRow
            left={<Field label="启动 ID">{info?.bootID || "-"}</Field>}
            right={<Field label="容器运行时版本">{info?.containerRuntimeVersion || "-"}</Field>}
          />
          <InfoRow
            left={<Field label="操作系统">{info?.operatingSystem || "-"}</Field>}
            right={<Field label="架构">{info?.architecture || "-"}</Field>}
          />
          <InfoRow
            left={
              <Field label="污点（Taints）">
                <div className="k8s-role-badges">
                  {(node.spec?.taints ?? []).length === 0 ? <span className="text-muted">-</span> : null}
                  {(node.spec?.taints ?? []).map((taint, index) => (
                    <span key={`${taint.key}-${index}`} className="badge badge-neutral">
                      {taint.key}
                      {taint.value ? `: ${taint.value}` : ""} Effect: {taint.effect}
                    </span>
                  ))}
                </div>
              </Field>
            }
          />
        </div>
      </section>

      <section className="stack-sm">
        <h3 className="k8s-detail-section-title">状态</h3>
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>类型</th>
                <th>状态</th>
                <th>更新时间</th>
                <th>内容</th>
                <th>消息</th>
              </tr>
            </thead>
            <tbody>
              {(node.status?.conditions ?? []).map((item) => (
                <tr key={item.type}>
                  <td>{item.type || "-"}</td>
                  <td>{item.status || "-"}</td>
                  <td>{formatDateTime(item.lastTransitionTime)}</td>
                  <td>{item.reason || "-"}</td>
                  <td className="max-w-xs break-words">{item.message || "-"}</td>
                </tr>
              ))}
              {(node.status?.conditions ?? []).length === 0 ? (
                <tr>
                  <td colSpan={5}>
                    <div className="empty-state">暂无状态</div>
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="stack-sm">
        <h3 className="k8s-detail-section-title">容器组</h3>
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>容器</th>
                <th>CPU / 内存</th>
                <th>状态</th>
                <th>重启次数</th>
                <th>命名空间</th>
                <th>Pod IP</th>
                <th>调度节点</th>
                <th>存活时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {loadingPods ? (
                <tr>
                  <td colSpan={10}>
                    <div className="empty-state">加载中…</div>
                  </td>
                </tr>
              ) : null}
              {!loadingPods &&
                pods.map((pod) => {
                  const phase = podPhase(pod);
                  const running = phase === "Running";
                  const resources = podResourceSummary(pod);
                  return (
                    <tr key={`${pod.metadata?.namespace}/${pod.metadata?.name}`}>
                      <td>
                        <button type="button" className="k8s-action-link font-semibold" onClick={() => setPodDetail(pod)}>
                          {pod.metadata?.name}
                        </button>
                      </td>
                      <td>
                        <div className="flex flex-wrap gap-1">
                          {(pod.status?.containerStatuses ?? pod.spec?.containers ?? []).map((c, index) => {
                            const ready = "ready" in c ? Boolean(c.ready) : false;
                            const title =
                              "name" in c && c.name
                                ? `${c.name}${ready ? "（ready）" : ""}`
                                : `container-${index}`;
                            return (
                              <span
                                key={title}
                                title={title}
                                className={ready ? "k8s-container-dot is-ready" : "k8s-container-dot"}
                              />
                            );
                          })}
                          {(pod.status?.containerStatuses ?? pod.spec?.containers ?? []).length === 0 ? "-" : null}
                        </div>
                      </td>
                      <td className="text-xs leading-5">
                        <div>{resources.cpu}</div>
                        <div className="text-muted">{resources.memory}</div>
                      </td>
                      <td>
                        <span className={running ? "badge badge-success" : "badge badge-neutral"}>{phase}</span>
                      </td>
                      <td>{podRestartCount(pod)}</td>
                      <td>{pod.metadata?.namespace || "-"}</td>
                      <td className="font-mono text-xs">{pod.status?.podIP || "-"}</td>
                      <td className="font-mono text-xs">{pod.status?.hostIP || nodeInternalIP(node)}</td>
                      <td>{formatNodeAge(pod.metadata?.creationTimestamp)}</td>
                      <td className="whitespace-nowrap">
                        <button
                          type="button"
                          className="k8s-action-link"
                          disabled={!canManage}
                          onClick={() => void openPodYaml(pod)}
                        >
                          编辑
                        </button>
                        <span className="text-muted"> / </span>
                        <button type="button" className="k8s-action-link" onClick={() => setMorePod(pod)}>
                          更多
                        </button>
                      </td>
                    </tr>
                  );
                })}
              {!loadingPods && pods.length === 0 ? (
                <tr>
                  <td colSpan={10}>
                    <div className="empty-state">该节点暂无 Pod</div>
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <Modal nested open={Boolean(editPod)} title="编辑" size="xl" onClose={() => setEditPod(null)}>
        {editPod ? (
          <YamlEditor
            value={editPod}
            canSave={canManage}
            saving={saving}
            confirmLabel="确定"
            cancelLabel="取消"
            onSave={savePod}
            onClose={() => setEditPod(null)}
          />
        ) : null}
      </Modal>

      <Modal
        nested
        open={Boolean(morePod)}
        title={morePod ? `更多操作 · ${morePod.metadata?.name ?? ""}` : "更多操作"}
        description={morePod ? `${morePod.metadata?.namespace ?? ""} / ${podPhase(morePod)}` : undefined}
        size="md"
        onClose={() => setMorePod(null)}
      >
        {morePod ? (
          <div className="k8s-action-sheet">
            {podContainerNames(morePod).map((container) => (
              <button
                key={`files-${container}`}
                type="button"
                className="k8s-action-sheet-item"
                onClick={() => {
                  setMorePod(null);
                  setFilesTarget({ pod: morePod, container });
                }}
              >
                <span className="k8s-action-sheet-label">{container} 文件浏览</span>
                <span className="k8s-action-sheet-hint">列出、上传、下载或删除容器内文件</span>
              </button>
            ))}
            {podContainerNames(morePod).map((container) => (
              <button
                key={`exec-${container}`}
                type="button"
                className="k8s-action-sheet-item"
                disabled={!canManage}
                onClick={() => {
                  setMorePod(null);
                  setStream({ pod: morePod, container, mode: "exec" });
                }}
              >
                <span className="k8s-action-sheet-label">{container} 终端</span>
                <span className="k8s-action-sheet-hint">打开交互式容器终端</span>
              </button>
            ))}
            {podContainerNames(morePod).map((container) => (
              <button
                key={`logs-${container}`}
                type="button"
                className="k8s-action-sheet-item"
                onClick={() => {
                  setMorePod(null);
                  setStream({ pod: morePod, container, mode: "logs" });
                }}
              >
                <span className="k8s-action-sheet-label">{container} 终端日志</span>
                <span className="k8s-action-sheet-hint">查看容器标准输出日志</span>
              </button>
            ))}
            <button
              type="button"
              className="k8s-action-sheet-item is-danger"
              disabled={!canManage}
              onClick={() => {
                const target = morePod;
                setMorePod(null);
                void deletePod(target);
              }}
            >
              <span className="k8s-action-sheet-label">删除</span>
              <span className="k8s-action-sheet-hint">删除该 Pod 对象</span>
            </button>
          </div>
        ) : null}
      </Modal>

      <Modal nested open={Boolean(podDetail)} title={`Pod 详情 ${podDetail?.metadata?.name ?? ""}`} size="lg" onClose={() => setPodDetail(null)}>
        <pre className="k8s-yaml-panel font-mono text-xs min-h-80 overflow-auto whitespace-pre-wrap">
          {podDetail ? JSON.stringify(podDetail, null, 2) : ""}
        </pre>
      </Modal>

      <Modal
        nested
        open={Boolean(filesTarget)}
        title={
          filesTarget
            ? `Pod: ${filesTarget.pod.metadata?.name ?? ""} Container: ${filesTarget.container} 文件浏览`
            : "文件浏览"
        }
        size="xl"
        onClose={() => setFilesTarget(null)}
      >
        {filesTarget ? (
          <PodFilesBrowser
            clusterId={clusterId}
            pod={filesTarget.pod}
            container={filesTarget.container}
            canManage={canManage}
          />
        ) : null}
      </Modal>

      <Modal
        nested
        open={Boolean(stream)}
        title={
          stream
            ? stream.mode === "logs"
              ? `终端日志 · ${stream.pod.metadata?.name}/${stream.container}`
              : `容器组名: ${stream.pod.metadata?.name} 容器名: ${stream.container} 终端`
            : "容器输出"
        }
        size={stream?.mode === "exec" ? "xl" : "lg"}
        panelClassName={
          stream?.mode === "exec"
            ? terminalFullscreen
              ? "k8s-pod-terminal-modal-fullscreen"
              : "k8s-pod-terminal-modal"
            : undefined
        }
        headerExtra={
          stream?.mode === "exec" ? (
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
          setStream(null);
          setTerminalFullscreen(false);
        }}
      >
        {stream?.mode === "exec" ? (
          <PodTerminal
            clusterId={clusterId}
            namespace={stream.pod.metadata?.namespace ?? ""}
            pod={stream.pod.metadata?.name ?? ""}
            container={stream.container}
            canManage={canManage}
          />
        ) : stream ? (
          <PodLogPanel clusterId={clusterId} pod={stream.pod} container={stream.container} />
        ) : null}
      </Modal>
    </div>
  );
}

function PodLogPanel({
  clusterId,
  pod,
  container,
}: {
  clusterId: string;
  pod: PodObject;
  container: string;
}) {
  const [output, setOutput] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [running, setRunning] = useState(false);
  const ns = pod.metadata?.namespace ?? "";
  const podName = pod.metadata?.name ?? "";

  async function run() {
    if (!ns || !podName) return;
    setRunning(true);
    setError(null);
    setOutput([]);
    try {
      const base = podOperationPath(clusterId, ns, podName, "logs");
      const params = new URLSearchParams({ container });
      await readSSE(`${base}?${params.toString()}`, setOutput);
    } catch (err) {
      setError(errorInfo(err, "读取日志失败"));
    } finally {
      setRunning(false);
    }
  }

  useEffect(() => {
    void run();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId, ns, podName, container]);

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

async function readSSE(url: string, setOutput: (updater: (current: string[]) => string[]) => void) {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
  if (!res.body) throw new Error("SSE 响应为空");
  const parser = createSSEParser((event) => {
    setOutput((current) =>
      appendBoundedLines(current, `${event.event}: ${formatSSEPayload(event.data)}`, { maxLines: 1000, maxBytes: 1 << 20 }),
    );
  });
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    parser.push(value);
  }
  parser.flush();
}
