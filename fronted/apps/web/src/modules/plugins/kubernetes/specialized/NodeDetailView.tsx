"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { K8sEmptyState, K8sLoadingState, K8sStatusBadge } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { type PodEvent } from "../pod-detail-view";
import { podEventsUrl } from "../resource-url";
import { YamlEditor } from "../YamlEditor";
import {
  formatDateTime,
  formatNodeAge,
  nodeConditionTone,
  nodeInternalIP,
  nodeRoles,
  podPhase,
  podResourceSummary,
  podRestartCount,
  type NodeObject,
  type PodObject,
} from "../node-view";
import { formatMetaKey, formatMetaValue, splitMetaEntries } from "./node-detail-meta";
import { PodDetailView } from "./PodDetailView";
import { WorkloadPodActions } from "./WorkloadPodActions";

const podDef = CATALOG.find((item) => item.kind === "Pod");

async function loadNodePods(clusterId: string, nodeName: string): Promise<PodObject[]> {
  const data = await k8sFetch<{ items?: PodObject[] }>(
    `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/pods?fieldSelector=${encodeURIComponent(`spec.nodeName=${nodeName}`)}&page=1&pageSize=500`,
  );
  return data.items ?? [];
}

function dash(value: unknown): string {
  if (value === undefined || value === null || value === "") return "-";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}

function conditionTone(type?: string, status?: string) {
  return nodeConditionTone(type, status);
}

type PodDetailState = {
  pod: PodObject | null;
  loading: boolean;
  events: PodEvent[];
  eventsLoading: boolean;
  eventsError: ApiErrorInfo | null;
};

function InfoRow({ left, right }: { left: ReactNode; right?: ReactNode }) {
  return (
    <div className="k8s-pod-detail-row">
      <div>{left}</div>
      {right !== undefined ? <div>{right}</div> : null}
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="k8s-pod-detail-field">
      <span className="k8s-pod-detail-label">{label}</span>
      <span className="k8s-pod-detail-value">{children}</span>
    </div>
  );
}

function MetaPills({ entries }: { entries: Array<[string, string]> }) {
  if (entries.length === 0) return <span className="text-muted">-</span>;
  return (
    <div className="k8s-pod-detail-pills">
      {entries.map(([key, value]) => (
        <span key={key} className="k8s-pod-detail-pill" title={`${key}: ${value}`}>
          {formatMetaKey(key)}
          {value !== "" ? `: ${formatMetaValue(value)}` : ""}
        </span>
      ))}
    </div>
  );
}

function CollapsibleMetaBlock({
  title,
  entries,
  defaultOpen = false,
}: {
  title: string;
  entries: Array<[string, string]>;
  defaultOpen?: boolean;
}) {
  if (entries.length === 0) return null;
  return (
    <details className="k8s-node-meta-block" open={defaultOpen}>
      <summary className="k8s-node-meta-summary">
        <span>{title}</span>
        <span className="k8s-node-meta-count">{entries.length}</span>
      </summary>
      <MetaPills entries={entries} />
    </details>
  );
}

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
  const [podDetail, setPodDetail] = useState<PodDetailState | null>(null);
  const [saving, setSaving] = useState(false);
  const podDetailRequestSeq = useRef(0);
  const activePodDetailRequest = useRef<AbortController | null>(null);
  const info = node.status?.nodeInfo;
  const name = node.metadata?.name ?? "";
  const labelGroups = splitMetaEntries(node.metadata?.labels);
  const annotationGroups = splitMetaEntries(node.metadata?.annotations);
  const roles = nodeRoles(node.metadata?.labels);
  const addresses = node.status?.addresses ?? [];
  const internalIp = nodeInternalIP(node);
  const hostname = addresses.find((item) => item.type === "Hostname")?.address;
  const otherAddresses = addresses.filter((item) => item.type !== "InternalIP" && item.type !== "Hostname");

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

  function closePodDetail() {
    activePodDetailRequest.current?.abort();
    activePodDetailRequest.current = null;
    setPodDetail(null);
  }

  async function openPodDetail(pod: PodObject) {
    const namespace = pod.metadata?.namespace?.trim();
    const podName = pod.metadata?.name?.trim();
    const eventsUrl = namespace && podName ? podEventsUrl(clusterId, namespace, podName) : null;
    if (!podDef || !namespace || !podName || !eventsUrl) return;

    activePodDetailRequest.current?.abort();
    const controller = new AbortController();
    activePodDetailRequest.current = controller;
    const requestSeq = podDetailRequestSeq.current + 1;
    podDetailRequestSeq.current = requestSeq;
    const isCurrent = () =>
      activePodDetailRequest.current === controller && requestSeq === podDetailRequestSeq.current;

    setPodDetail({ pod: null, loading: true, events: [], eventsLoading: true, eventsError: null });
    const podRequest = k8sFetch<PodObject>(proxyPath(clusterId, podDef, namespace, podName), { signal: controller.signal });
    const eventsRequest = k8sFetch<{ items?: PodEvent[] }>(eventsUrl, { signal: controller.signal });

    void eventsRequest
      .then((data) => {
        if (!isCurrent()) return;
        setPodDetail((current) =>
          current ? { ...current, events: data.items ?? [], eventsLoading: false, eventsError: null } : current,
        );
      })
      .catch((err) => {
        if (!isCurrent()) return;
        setPodDetail((current) =>
          current
            ? { ...current, events: [], eventsLoading: false, eventsError: errorInfo(err, "加载 Pod 事件失败") }
            : current,
        );
      });

    void Promise.allSettled([podRequest, eventsRequest]).finally(() => {
      if (activePodDetailRequest.current === controller) activePodDetailRequest.current = null;
    });

    try {
      const latest = await podRequest;
      if (!isCurrent()) return;
      setPodDetail((current) => (current ? { ...current, pod: latest, loading: false } : current));
    } catch (err) {
      if (!isCurrent()) return;
      closePodDetail();
      setError(errorInfo(err, "加载 Pod 详情失败"));
    }
  }

  const conditions = node.status?.conditions ?? [];

  return (
    <div className="stack-lg k8s-node-detail-root">
      <ApiErrorBanner error={error} />

      <section className="k8s-node-detail-section stack-sm">
        <h3 className="k8s-node-detail-title">概览</h3>
        <div className="k8s-node-detail-grid">
          <InfoRow
            left={<Field label="名称">{dash(name)}</Field>}
            right={<Field label="存活时间">{formatNodeAge(node.metadata?.creationTimestamp)}</Field>}
          />
          <InfoRow
            left={
              <Field label="内部 IP">
                <span className="font-mono text-sm">{internalIp || "-"}</span>
              </Field>
            }
            right={
              <Field label="调度">
                <K8sStatusBadge tone={node.spec?.unschedulable ? "warning" : "success"}>
                  {node.spec?.unschedulable ? "不可调度" : "可调度"}
                </K8sStatusBadge>
              </Field>
            }
          />
          <InfoRow
            left={
              <Field label="角色">
                {roles.length > 0 ? (
                  <div className="k8s-role-tags">
                    {roles.map((role) => (
                      <span key={role} className="k8s-meta-tag">
                        {role}
                      </span>
                    ))}
                  </div>
                ) : (
                  <span className="text-muted">-</span>
                )}
              </Field>
            }
            right={
              <Field label="主机名">
                <span className="font-mono text-sm">{dash(hostname)}</span>
              </Field>
            }
          />
          <InfoRow
            left={<Field label="Kubelet">{dash(info?.kubeletVersion)}</Field>}
            right={<Field label="操作系统">{dash(info?.osImage)}</Field>}
          />
        </div>
      </section>

      <section className="k8s-node-detail-section stack-sm">
        <h3 className="k8s-node-detail-title">元数据</h3>
        <div className="k8s-node-meta-stack">
          {labelGroups.user.length > 0 ? (
            <div className="k8s-node-meta-group">
              <span className="k8s-node-meta-group-label">自定义标签</span>
              <MetaPills entries={labelGroups.user} />
            </div>
          ) : null}
          <CollapsibleMetaBlock title="系统标签" entries={labelGroups.system} />
          <CollapsibleMetaBlock title="注解" entries={[...annotationGroups.user, ...annotationGroups.system]} />
          {(node.spec?.taints ?? []).length > 0 ? (
            <div className="k8s-node-meta-group">
              <span className="k8s-node-meta-group-label">污点</span>
              <div className="k8s-role-tags">
                {(node.spec?.taints ?? []).map((taint, index) => (
                  <span key={`${taint.key}-${index}`} className="k8s-meta-tag" title={taint.effect}>
                    {taint.key}
                    {taint.value ? `: ${taint.value}` : ""}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
          {labelGroups.user.length === 0 &&
          labelGroups.system.length === 0 &&
          annotationGroups.user.length === 0 &&
          annotationGroups.system.length === 0 &&
          (node.spec?.taints ?? []).length === 0 ? (
            <span className="text-sm text-muted">暂无标签、注解或污点</span>
          ) : null}
        </div>
      </section>

      <details className="k8s-node-meta-block">
        <summary className="k8s-node-meta-summary">
          <span>系统信息</span>
        </summary>
        <div className="k8s-node-detail-grid k8s-node-detail-grid-nested">
          <InfoRow
            left={<Field label="Pod CIDR">{dash(node.spec?.podCIDR)}</Field>}
            right={<Field label="容器运行时">{dash(info?.containerRuntimeVersion)}</Field>}
          />
          <InfoRow
            left={<Field label="内核">{dash(info?.kernelVersion)}</Field>}
            right={<Field label="架构">{dash(info?.architecture)}</Field>}
          />
          <InfoRow
            left={<Field label="Kube-Proxy">{dash(info?.kubeProxyVersion)}</Field>}
            right={<Field label="UID">{dash(node.metadata?.uid)}</Field>}
          />
          {otherAddresses.length > 0 ? (
            <InfoRow
              left={
                <Field label="其他地址">
                  <div className="stack-xs text-sm">
                    {otherAddresses.map((item) => (
                      <div key={`${item.type}-${item.address}`} className="text-muted">
                        {item.type}: <span className="font-mono">{item.address}</span>
                      </div>
                    ))}
                  </div>
                </Field>
              }
            />
          ) : null}
          <InfoRow
            left={<Field label="机器 ID">{dash(info?.machineID)}</Field>}
            right={<Field label="系统 UUID">{dash(info?.systemUUID)}</Field>}
          />
        </div>
      </details>

      <details className="k8s-node-meta-block k8s-node-conditions-block" open={conditions.length > 0}>
        <summary className="k8s-node-meta-summary">
          <span>状态条件</span>
          {conditions.length > 0 ? <span className="k8s-node-meta-count">{conditions.length}</span> : null}
        </summary>
        <div className="table-wrap k8s-node-detail-table">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>类型</th>
                <th>状态</th>
                <th>更新时间</th>
                <th>原因</th>
              </tr>
            </thead>
            <tbody>
              {conditions.map((item) => (
                <tr key={item.type} title={item.message || undefined}>
                  <td>{dash(item.type)}</td>
                  <td>
                    <K8sStatusBadge tone={conditionTone(item.type, item.status)}>{dash(item.status)}</K8sStatusBadge>
                  </td>
                  <td className="text-muted text-xs whitespace-nowrap">{formatDateTime(item.lastTransitionTime)}</td>
                  <td className="text-muted text-xs">{dash(item.reason)}</td>
                </tr>
              ))}
              {conditions.length === 0 ? (
                <tr>
                  <td colSpan={4}>
                    <K8sEmptyState variant="empty" title="暂无状态" compact />
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </details>

      <details className="k8s-node-meta-block k8s-node-pods-block" open>
        <summary className="k8s-node-meta-summary">
          <span>容器组</span>
          {!loadingPods ? <span className="k8s-node-meta-count">{pods.length}</span> : null}
        </summary>
        <div className="table-wrap k8s-node-detail-table">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>命名空间</th>
                <th>状态</th>
                <th>资源</th>
                <th>重启</th>
                <th>存活时间</th>
                <th className="w-28">操作</th>
              </tr>
            </thead>
            <tbody>
              {loadingPods ? (
                <tr>
                  <td colSpan={7}>
                    <K8sLoadingState compact />
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
                      <td className="k8s-pod-name-cell">
                        <button
                          type="button"
                          className="k8s-cluster-name-link k8s-pod-name-link"
                          title={pod.metadata?.name}
                          onClick={() => void openPodDetail(pod)}
                        >
                          {pod.metadata?.name}
                        </button>
                      </td>
                      <td className="text-muted text-xs">{pod.metadata?.namespace || "-"}</td>
                      <td>
                        <K8sStatusBadge tone={running ? "success" : "neutral"}>{phase}</K8sStatusBadge>
                      </td>
                      <td className="k8s-resource-cell text-xs whitespace-nowrap font-mono">
                        <div>{resources.cpu}</div>
                        <div className="text-muted">{resources.memory}</div>
                      </td>
                      <td className="text-muted tabular-nums">{podRestartCount(pod)}</td>
                      <td className="text-muted text-xs whitespace-nowrap">{formatNodeAge(pod.metadata?.creationTimestamp)}</td>
                      <td>
                        <div className="k8s-row-actions">
                          <button
                            type="button"
                            className="btn-ghost text-sm"
                            disabled={!canManage}
                            onClick={() => void openPodYaml(pod)}
                          >
                            编辑
                          </button>
                          <WorkloadPodActions
                            clusterId={clusterId}
                            pod={pod}
                            canManage={canManage}
                            trigger="ghost"
                            onOpenDetail={(target) => void openPodDetail(target)}
                            onDeleted={reloadPods}
                            onError={setError}
                          />
                        </div>
                      </td>
                    </tr>
                  );
                })}
              {!loadingPods && pods.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <K8sEmptyState variant="empty" title="该节点暂无 Pod" compact />
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </details>

      <Modal
        nested
        open={Boolean(editPod)}
        title={editPod?.metadata?.name ?? "编辑"}
        description={editPod?.metadata?.namespace}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setEditPod(null)}
      >
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
        open={Boolean(podDetail)}
        title={podDetail?.pod?.metadata?.name ?? "Pod 详情"}
        description={podDetail?.pod?.metadata?.namespace}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={closePodDetail}
      >
        {podDetail?.loading || !podDetail?.pod ? (
          <K8sLoadingState label="Pod 详情加载中…" />
        ) : (
          <PodDetailView
            pod={podDetail.pod}
            events={podDetail.events}
            eventsError={podDetail.eventsError}
            eventsLoading={podDetail.eventsLoading}
          />
        )}
      </Modal>
    </div>
  );
}
