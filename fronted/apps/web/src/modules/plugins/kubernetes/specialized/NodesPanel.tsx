"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  K8sActionIcons,
  K8sActionSheet,
  K8sActionSheetGroup,
  K8sActionSheetItem,
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sStatusBadge,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { humanizeK8sApiError, isKnownK8sApiError } from "../k8s-api-errors";
import { podOperationPath, readKubernetesSSE, toCommandQuery, waitForNodeTTYReady } from "../operations";
import { resourceCollectionUrl } from "../resource-url";
import { YamlEditor } from "../YamlEditor";
import { ChatPage } from "./ChatPage";
import { CloudTTYPage } from "./CloudTTYPage";
import { NodeDetailView } from "./NodeDetailView";
import {
  formatNodeAge,
  isEvictablePod,
  nodeInternalIP,
  nodeResourceView,
  nodeRoles,
  nodeStatus,
  type NodeMetrics,
  type NodeMetricView,
  type NodeObject,
  type PodObject,
} from "../node-view";

const nodeDef = CATALOG.find((item) => item.kind === "Node");
type Row = {
  node: NodeObject;
  cpu: NodeMetricView;
  memory: NodeMetricView;
};

export function NodesPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [rows, setRows] = useState<Row[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [moreNode, setMoreNode] = useState<NodeObject | null>(null);
  const [editNode, setEditNode] = useState<NodeObject | null>(null);
  const [yamlNode, setYamlNode] = useState<NodeObject | null>(null);
  const [detailNode, setDetailNode] = useState<NodeObject | null>(null);
  const [saving, setSaving] = useState(false);
  const [cloudOpen, setCloudOpen] = useState(false);
  const [chatOpen, setChatOpen] = useState(false);
  const [shellNode, setShellNode] = useState<string | null>(null);
  const [taintNode, setTaintNode] = useState<NodeObject | null>(null);
  const [monitorNode, setMonitorNode] = useState<string | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const load = useCallback(async () => {
    if (!clusterId || !nodeDef) return;
    const url = resourceCollectionUrl(clusterId, nodeDef, { page, pageSize, keyword });
    if (!url) return;
    setLoading(true);
    setError(null);
    try {
      const data = await k8sFetch<{ items?: NodeObject[]; total?: number }>(url);
      const items = data.items ?? [];
      const metricsMap = await loadNodeMetrics(clusterId);
      const enriched = await Promise.all(
        items.map(async (node) => {
          const name = node.metadata?.name ?? "";
          const pods = name ? await loadNodePods(clusterId, name) : [];
          const view = nodeResourceView(node, pods, metricsMap.get(name));
          return { node, cpu: view.cpu, memory: view.memory };
        }),
      );
      setRows(enriched);
      setTotal(data.total ?? items.length);
    } catch (err) {
      setRows([]);
      setTotal(0);
      setError(errorInfo(err, "加载节点失败"));
    } finally {
      setLoading(false);
    }
  }, [clusterId, keyword, page, pageSize]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setSelected([]);
    setPage(1);
  }, [clusterId]);

  async function refreshNode(name: string): Promise<NodeObject> {
    if (!nodeDef) throw new Error("missing node catalog");
    return k8sFetch<NodeObject>(proxyPath(clusterId, nodeDef, undefined, name));
  }

  async function saveNode(next: unknown) {
    const name = (next as NodeObject).metadata?.name;
    if (!nodeDef || !name) return;
    setSaving(true);
    try {
      await k8sFetch(proxyPath(clusterId, nodeDef, undefined, name), { method: "PUT", body: JSON.stringify(next) });
      setEditNode(null);
      setYamlNode(null);
      setTaintNode(null);
      await load();
    } catch (err) {
      setError(errorInfo(err, "保存节点失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function toggleSchedule(node: NodeObject) {
    const name = node.metadata?.name;
    if (!canManage || !name) return;
    const disable = !node.spec?.unschedulable;
    const ok = await ask({
      title: "调度设置",
      description: `确定将节点「${name}」设置为${disable ? "不可调度" : "可调度"}？`,
      confirmLabel: "确认",
      danger: false,
    });
    if (!ok) return;
    try {
      const latest = await refreshNode(name);
      const body = { ...latest, spec: { ...(latest.spec ?? {}), unschedulable: disable } };
      if (!nodeDef) return;
      await k8sFetch(proxyPath(clusterId, nodeDef, undefined, name), { method: "PUT", body: JSON.stringify(body) });
      await load();
    } catch (err) {
      setError(errorInfo(err, "调度设置失败"));
    }
  }

  async function drain(node: NodeObject) {
    const name = node.metadata?.name;
    if (!canManage || !name) return;
    const ok = await ask({
      title: "排空节点",
      description: `此操作将排空节点「${name}」上的 Pod，是否继续？`,
      confirmLabel: "继续排空",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      const latest = await refreshNode(name);
      if (!latest.spec?.unschedulable && nodeDef) {
        await k8sFetch(proxyPath(clusterId, nodeDef, undefined, name), {
          method: "PUT",
          body: JSON.stringify({ ...latest, spec: { ...(latest.spec ?? {}), unschedulable: true } }),
        });
      }
      const pods = (await loadNodePods(clusterId, name)).filter(isEvictablePod);
      for (const pod of pods) {
        const ns = pod.metadata?.namespace;
        const podName = pod.metadata?.name;
        if (!ns || !podName) continue;
        await k8sFetch(
          `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/namespaces/${encodeURIComponent(ns)}/pods/${encodeURIComponent(podName)}/eviction`,
          {
            method: "POST",
            body: JSON.stringify({
              apiVersion: "policy/v1",
              kind: "Eviction",
              metadata: { name: podName, namespace: ns },
            }),
          },
        ).catch(async () => {
          await k8sFetch(
            `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/namespaces/${encodeURIComponent(ns)}/pods/${encodeURIComponent(podName)}`,
            { method: "DELETE" },
          );
        });
      }
      await load();
    } catch (err) {
      setError(errorInfo(err, "节点排空失败"));
    }
  }

  async function deleteNode(node: NodeObject) {
    const name = node.metadata?.name;
    if (!canManage || !name || !nodeDef) return;
    const ok = await ask({
      title: "删除节点",
      description: `确定删除节点「${name}」？此操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    try {
      await k8sFetch(proxyPath(clusterId, nodeDef, undefined, name), { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除节点失败"));
    }
  }

  async function openDetail(nodeName: string) {
    try {
      setDetailNode(await refreshNode(nodeName));
    } catch (err) {
      setError(errorInfo(err, "加载节点详情失败"));
    }
  }

  function openLabelsTaints() {
    const name = selected[0] ?? rows[0]?.node.metadata?.name;
    const row = rows.find((item) => item.node.metadata?.name === name);
    if (!row) {
      setError({ code: "INVALID_REQUEST", message: "请先选择一个节点" });
      return;
    }
    setTaintNode(row.node);
  }

  if (!clusterId) {
    return (
      <K8sPanelCard>
        <K8sEmptyState variant="no-cluster" />
      </K8sPanelCard>
    );
  }

  const hasActiveFilters = keyword.trim().length > 0;

  return (
    <>
    <K8sPanelCard className="k8s-nodes-panel">
      <div className="k8s-clusters-toolbar k8s-nodes-toolbar">
        <div className="k8s-clusters-toolbar-filters">
          <label className="projects-search-wrap">
            <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M9 4a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z" stroke="currentColor" strokeWidth="1.5" />
              <path d="M13 13l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
            <input
              className="projects-search-input"
              value={keyword}
              placeholder="节点名称"
              aria-label="搜索节点"
              onChange={(e) => {
                setKeyword(e.target.value);
                setPage(1);
              }}
            />
          </label>
          {hasActiveFilters ? (
            <button
              type="button"
              className="btn-secondary projects-reset-btn"
              onClick={() => {
                setKeyword("");
                setPage(1);
              }}
            >
              重置
            </button>
          ) : null}
        </div>
        <div className="k8s-toolbar-actions">
          {!loading && total > 0 ? (
            <span className="k8s-clusters-count" role="status">
              共 {total} 个节点
            </span>
          ) : null}
          <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={openLabelsTaints}>
            标签与污点
          </button>
          <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={() => setCloudOpen(true)}>
            CloudShell
          </button>
          <button type="button" className="btn-secondary text-sm" onClick={() => setChatOpen(true)}>
            AI 助手
          </button>
          <button
            type="button"
            className="btn-secondary k8s-clusters-refresh-btn"
            disabled={loading}
            onClick={() => void load()}
            aria-label="刷新节点列表"
          >
            ↻
          </button>
        </div>
      </div>

      <ApiErrorBanner error={error} />

      <div className="table-wrap k8s-nodes-table">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th className="w-10">
                <input
                  type="checkbox"
                  aria-label="全选"
                  checked={rows.length > 0 && selected.length === rows.length}
                  onChange={(e) =>
                    setSelected(e.target.checked ? rows.map((item) => item.node.metadata?.name ?? "").filter(Boolean) : [])
                  }
                />
              </th>
              <th>名称</th>
              <th>Internal IP</th>
              <th>状态</th>
              <th>角色</th>
              <th>CPU</th>
              <th>内存</th>
              <th>版本</th>
              <th>存活时间</th>
              <th className="w-28">操作</th>
            </tr>
          </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={10}>
                    <K8sLoadingState compact />
                  </td>
                </tr>
              ) : null}
              {!loading &&
                rows.map((row) => {
                  const name = row.node.metadata?.name ?? "-";
                  const status = nodeStatus(row.node);
                  const ready = status.startsWith("Ready") && !status.includes("NotReady");
                  const info = row.node.status?.nodeInfo;
                  return (
                    <tr key={name}>
                      <td>
                        <input
                          type="checkbox"
                          checked={selected.includes(name)}
                          onChange={(e) =>
                            setSelected((current) =>
                              e.target.checked ? [...current, name] : current.filter((item) => item !== name),
                            )
                          }
                        />
                      </td>
                      <td>
                        <button type="button" className="k8s-cluster-name-link" onClick={() => void openDetail(name)}>
                          {name}
                        </button>
                        <LabelsHint labels={row.node.metadata?.labels} />
                      </td>
                      <td className="font-mono text-xs text-muted">{nodeInternalIP(row.node)}</td>
                      <td>
                        <K8sStatusBadge tone={ready ? "success" : "danger"}>{status}</K8sStatusBadge>
                      </td>
                      <td>
                        <div className="k8s-role-tags">
                          {nodeRoles(row.node.metadata?.labels).map((role) => (
                            <span key={role} className="k8s-meta-tag">
                              {role}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td>
                        <MetricCell view={row.cpu} />
                      </td>
                      <td>
                        <MetricCell view={row.memory} />
                      </td>
                      <td className="text-xs">
                        <div>{info?.kubeletVersion || "-"}</div>
                        {info?.osImage ? <div className="text-muted truncate max-w-[10rem]">{info.osImage}</div> : null}
                      </td>
                      <td className="text-muted text-xs">{formatNodeAge(row.node.metadata?.creationTimestamp)}</td>
                      <td>
                        <div className="k8s-row-actions">
                          <button
                            type="button"
                            className="btn-ghost text-sm"
                            disabled={!canManage}
                            onClick={() =>
                              void refreshNode(name)
                                .then(setEditNode)
                                .catch((err) => setError(errorInfo(err, "加载节点失败")))
                            }
                          >
                            编辑
                          </button>
                          <button type="button" className="btn-ghost text-sm" onClick={() => setMoreNode(row.node)}>
                            更多
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              {!loading && rows.length === 0 ? (
                <tr>
                  <td colSpan={10}>
                    <K8sEmptyState variant="empty" title="暂无节点" description="当前集群中没有可显示的节点。" compact />
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        <K8sPagination
          page={page}
          pageSize={pageSize}
          total={total}
          loading={loading}
          onPageChange={setPage}
          onPageSizeChange={(next) => {
            setPageSize(next);
            setPage(1);
          }}
        />
    </K8sPanelCard>

      <Modal
        nested
        open={Boolean(moreNode)}
        title={moreNode ? moreNode.metadata?.name ?? "节点操作" : "节点操作"}
        description={moreNode ? nodeActionDescription(moreNode) : undefined}
        size="sm"
        panelClassName="k8s-cluster-dialog-modal k8s-action-sheet-modal"
        onClose={() => setMoreNode(null)}
      >
        {moreNode ? (
          <K8sActionSheet>
            <K8sActionSheetGroup title="查看">
              <K8sActionSheetItem
                label="详情"
                icon={K8sActionIcons.detail}
                onClick={() => {
                  const n = moreNode.metadata?.name;
                  setMoreNode(null);
                  if (n) void openDetail(n);
                }}
              />
              <K8sActionSheetItem
                label="监控"
                icon={K8sActionIcons.monitor}
                onClick={() => {
                  setMonitorNode(moreNode.metadata?.name ?? null);
                  setMoreNode(null);
                }}
              />
            </K8sActionSheetGroup>
            {canManage ? (
              <K8sActionSheetGroup title="运维">
                <K8sActionSheetItem
                  label="Shell"
                  icon={K8sActionIcons.shell}
                  onClick={() => {
                    setShellNode(moreNode.metadata?.name ?? null);
                    setMoreNode(null);
                  }}
                />
                <K8sActionSheetItem
                  label="YAML"
                  icon={K8sActionIcons.yaml}
                  onClick={() => {
                    const name = moreNode.metadata?.name;
                    setMoreNode(null);
                    if (!name) return;
                    void refreshNode(name)
                      .then(setYamlNode)
                      .catch((err) => setError(errorInfo(err, "加载节点失败")));
                  }}
                />
              </K8sActionSheetGroup>
            ) : null}
            {canManage ? (
              <K8sActionSheetGroup title="节点">
                <K8sActionSheetItem
                  label="节点排空"
                  icon={K8sActionIcons.drain}
                  onClick={() => {
                    setMoreNode(null);
                    void drain(moreNode);
                  }}
                />
                <K8sActionSheetItem
                  label={moreNode.spec?.unschedulable ? "恢复调度" : "停止调度"}
                  icon={K8sActionIcons.schedule}
                  onClick={() => {
                    setMoreNode(null);
                    void toggleSchedule(moreNode);
                  }}
                />
              </K8sActionSheetGroup>
            ) : null}
            {canManage ? (
              <K8sActionSheetGroup danger>
                <K8sActionSheetItem
                  label="删除节点"
                  icon={K8sActionIcons.delete}
                  danger
                  onClick={() => {
                    setMoreNode(null);
                    void deleteNode(moreNode);
                  }}
                />
              </K8sActionSheetGroup>
            ) : null}
          </K8sActionSheet>
        ) : null}
      </Modal>
      <Modal
        open={Boolean(editNode)}
        title={editNode?.metadata?.name ?? "编辑"}
        description="标签与注解"
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => {
          if (!saving) setEditNode(null);
        }}
      >
        {editNode ? (
          <NodeEditForm
            node={editNode}
            canManage={canManage}
            saving={saving}
            onCancel={() => setEditNode(null)}
            onSave={saveNode}
          />
        ) : null}
      </Modal>
      <Modal
        open={Boolean(yamlNode)}
        title={yamlNode?.metadata?.name ?? "YAML"}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setYamlNode(null)}
      >
        {yamlNode ? (
          <YamlEditor
            title={`Node ${yamlNode.metadata?.name ?? ""}`}
            value={yamlNode}
            canSave={canManage}
            saving={saving}
            onSave={saveNode}
            onClose={() => setYamlNode(null)}
          />
        ) : null}
      </Modal>
      <Modal
        open={Boolean(detailNode)}
        title={detailNode?.metadata?.name ?? "节点详情"}
        description={detailNode ? nodeInternalIP(detailNode) : undefined}
        size="xl"
        panelClassName="k8s-node-detail-modal"
        onClose={() => setDetailNode(null)}
      >
        {detailNode ? <NodeDetailView clusterId={clusterId} node={detailNode} canManage={canManage} /> : null}
      </Modal>
      <Modal nested open={cloudOpen} title="CloudShell" size="xl" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal nested open={chatOpen} title="AI 助手" size="xl" onClose={() => setChatOpen(false)}>
        <ChatPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal
        nested
        open={Boolean(shellNode)}
        title="Shell"
        description={shellNode ?? undefined}
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => setShellNode(null)}
      >
        {shellNode ? <NodeShellForm clusterId={clusterId} node={shellNode} canManage={canManage} /> : null}
      </Modal>
      <Modal
        nested
        open={Boolean(monitorNode)}
        title="监控"
        description={monitorNode ?? undefined}
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => setMonitorNode(null)}
      >
        {monitorNode ? <NodeMonitorForm clusterId={clusterId} node={monitorNode} /> : null}
      </Modal>
      <Modal
        nested
        open={Boolean(taintNode)}
        title="标签与污点"
        description={taintNode?.metadata?.name}
        size="lg"
        panelClassName="k8s-pod-submodal"
        onClose={() => setTaintNode(null)}
      >
        {taintNode ? (
          <LabelsTaintsForm
            node={taintNode}
            canManage={canManage}
            onSave={saveNode}
          />
        ) : null}
      </Modal>
      {confirmDialog}
    </>
  );
}

function nodeActionDescription(node: NodeObject) {
  return [nodeInternalIP(node), ...nodeRoles(node.metadata?.labels), nodeStatus(node)].filter(Boolean).join(" · ");
}

function MetricCell({ view }: { view: NodeMetricView }) {
  return (
    <div className="k8s-metric-cell">
      <span className="k8s-metric-percent">{view.percent.toFixed(0)}%</span>
      <span className={`k8s-metric-detail${view.overcommit ? " is-overcommit" : ""}`}>{view.usedLabel}</span>
    </div>
  );
}

function LabelsHint({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {});
  if (entries.length === 0) return null;
  return (
    <span className="k8s-label-hint" title={entries.map(([k, v]) => `${k}: ${v}`).join("\n")}>
      ▸
    </span>
  );
}

type KvRow = { key: string; value: string };

type TaintRow = { key: string; value: string; effect: string };

const TAINT_EFFECTS = ["NoSchedule", "PreferNoSchedule", "NoExecute"] as const;

function mapToRows(map?: Record<string, string>): KvRow[] {
  return Object.entries(map ?? {}).map(([key, value]) => ({ key, value }));
}

function rowsToMap(rows: KvRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) continue;
    out[key] = row.value;
  }
  return out;
}

function mapToTaintRows(taints?: { key?: string; value?: string; effect?: string }[]): TaintRow[] {
  return (taints ?? []).map((item) => ({
    key: item.key ?? "",
    value: item.value ?? "",
    effect: item.effect ?? "NoSchedule",
  }));
}

function rowsToTaints(rows: TaintRow[]): { key: string; value: string; effect: string }[] {
  return rows
    .filter((row) => row.key.trim())
    .map((row) => ({
      key: row.key.trim(),
      value: row.value,
      effect: TAINT_EFFECTS.includes(row.effect as (typeof TAINT_EFFECTS)[number]) ? row.effect : "NoSchedule",
    }));
}

function KeyValueEditor({
  title,
  rows,
  onChange,
  disabled,
}: {
  title: string;
  rows: KvRow[];
  onChange: (next: KvRow[]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="stack-sm">
      <span className="label">{title}</span>
      <div className="k8s-kv-table">
        <div className="k8s-kv-head">
          <span>键</span>
          <span>值</span>
          <span />
        </div>
        {rows.map((row, index) => (
          <div key={index} className="k8s-kv-row">
            <input
              className="input"
              placeholder="请输入键"
              value={row.key}
              disabled={disabled}
              onChange={(e) =>
                onChange(rows.map((item, i) => (i === index ? { ...item, key: e.target.value } : item)))
              }
            />
            <input
              className="input"
              placeholder="请输入值"
              value={row.value}
              disabled={disabled}
              onChange={(e) =>
                onChange(rows.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))
              }
            />
            <button
              type="button"
              className="k8s-action-link k8s-kv-delete"
              disabled={disabled}
              onClick={() => onChange(rows.filter((_, i) => i !== index))}
            >
              删除
            </button>
          </div>
        ))}
        <button
          type="button"
          className="k8s-action-link"
          disabled={disabled}
          onClick={() => onChange([...rows, { key: "", value: "" }])}
        >
          + 添加
        </button>
      </div>
    </div>
  );
}

function TaintEditor({
  rows,
  onChange,
  disabled,
}: {
  rows: TaintRow[];
  onChange: (next: TaintRow[]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="stack-sm">
      <span className="label">污点</span>
      <div className="k8s-kv-table k8s-kv-table-taints">
        <div className="k8s-kv-head">
          <span>键</span>
          <span>值</span>
          <span>效果</span>
          <span />
        </div>
        {rows.length === 0 ? <p className="text-sm text-muted">暂无污点</p> : null}
        {rows.map((row, index) => (
          <div key={index} className="k8s-kv-row">
            <input
              className="input"
              placeholder="请输入键"
              value={row.key}
              disabled={disabled}
              onChange={(e) => onChange(rows.map((item, i) => (i === index ? { ...item, key: e.target.value } : item)))}
            />
            <input
              className="input"
              placeholder="可选"
              value={row.value}
              disabled={disabled}
              onChange={(e) => onChange(rows.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))}
            />
            <select
              className="input k8s-kv-effect-select"
              value={row.effect}
              disabled={disabled}
              onChange={(e) => onChange(rows.map((item, i) => (i === index ? { ...item, effect: e.target.value } : item)))}
            >
              {TAINT_EFFECTS.map((effect) => (
                <option key={effect} value={effect}>
                  {effect}
                </option>
              ))}
            </select>
            <button
              type="button"
              className="k8s-action-link k8s-kv-delete"
              disabled={disabled}
              onClick={() => onChange(rows.filter((_, i) => i !== index))}
            >
              删除
            </button>
          </div>
        ))}
        <button
          type="button"
          className="k8s-action-link"
          disabled={disabled}
          onClick={() => onChange([...rows, { key: "", value: "", effect: "NoSchedule" }])}
        >
          + 添加污点
        </button>
      </div>
    </div>
  );
}

function NodeEditForm({
  node,
  canManage,
  saving,
  onCancel,
  onSave,
}: {
  node: NodeObject;
  canManage: boolean;
  saving: boolean;
  onCancel: () => void;
  onSave: (next: NodeObject) => Promise<void>;
}) {
  const [labels, setLabels] = useState<KvRow[]>(() => mapToRows(node.metadata?.labels));
  const [annotations, setAnnotations] = useState<KvRow[]>(() => mapToRows(node.metadata?.annotations));
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function submit() {
    if (!canManage || saving) return;
    setError(null);
    const emptyKey = [...labels, ...annotations].find((row) => !row.key.trim() && row.value.trim());
    if (emptyKey) {
      setError({ code: "INVALID_REQUEST", message: "键不能为空" });
      return;
    }
    try {
      await onSave({
        ...node,
        metadata: {
          ...(node.metadata ?? {}),
          labels: rowsToMap(labels),
          annotations: rowsToMap(annotations),
        },
      });
    } catch (err) {
      setError(errorInfo(err, "保存节点失败"));
    }
  }

  return (
    <div className="stack-md">
      <ApiErrorBanner error={error} />
      <label>
        <span className="label">名称</span>
        <input className="input" value={node.metadata?.name ?? ""} disabled readOnly />
      </label>
      <KeyValueEditor title="标签" rows={labels} onChange={setLabels} disabled={!canManage || saving} />
      <KeyValueEditor title="注解" rows={annotations} onChange={setAnnotations} disabled={!canManage || saving} />
      <div className="modal-footer-actions">
        <button type="button" className="btn-secondary" disabled={saving} onClick={onCancel}>
          取消
        </button>
        <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void submit()}>
          {saving ? "保存中…" : "保存"}
        </button>
      </div>
    </div>
  );
}

function LabelsTaintsForm({
  node,
  canManage,
  onSave,
}: {
  node: NodeObject;
  canManage: boolean;
  onSave: (next: NodeObject) => Promise<void>;
}) {
  const [labels, setLabels] = useState<KvRow[]>(() => mapToRows(node.metadata?.labels));
  const [taints, setTaints] = useState<TaintRow[]>(() => mapToTaintRows(node.spec?.taints));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const disabled = !canManage || saving;

  async function save() {
    setError(null);
    const emptyKey = [...labels, ...taints.map((row) => ({ key: row.key, value: row.value }))].find(
      (row) => !row.key.trim() && row.value.trim(),
    );
    if (emptyKey) {
      setError({ code: "INVALID_REQUEST", message: "键不能为空" });
      return;
    }
    setSaving(true);
    try {
      await onSave({
        ...node,
        metadata: { ...(node.metadata ?? {}), labels: rowsToMap(labels) },
        spec: { ...(node.spec ?? {}), taints: rowsToTaints(taints) },
      });
    } catch (err) {
      setError(errorInfo(err, "保存标签与污点失败"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="stack-md">
      <ApiErrorBanner error={error} />
      <KeyValueEditor title="标签" rows={labels} onChange={setLabels} disabled={disabled} />
      <TaintEditor rows={taints} onChange={setTaints} disabled={disabled} />
      <div className="modal-footer-actions">
        <button type="button" className="btn" disabled={disabled} onClick={() => void save()}>
          {saving ? "保存中…" : "保存"}
        </button>
      </div>
    </div>
  );
}

function NodeShellForm({ clusterId, node, canManage }: { clusterId: string; node: string; canManage: boolean }) {
  const [command, setCommand] = useState("uname -a");
  const [output, setOutput] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [running, setRunning] = useState(false);
  const [phase, setPhase] = useState<string | null>(null);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);

  async function run() {
    if (running) return;
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    let createdPod: { name: string; namespace: string } | undefined;
    setRunning(true);
    setPhase("正在创建临时 Pod…");
    try {
      setError(null);
      setOutput([]);
      const session = await k8sFetch<{ pod?: { name: string; namespace: string } }>(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/nodetty`,
        { method: "POST", body: JSON.stringify({ node }), signal: controller.signal },
      );
      if (!session.pod) throw new Error("NodeTTY 未返回 Pod");
      createdPod = session.pod;
      const podURL =
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}` +
        `/proxy/api/v1/namespaces/${encodeURIComponent(session.pod.namespace)}` +
        `/pods/${encodeURIComponent(session.pod.name)}`;
      setPhase("等待 Pod 就绪…");
      await waitForNodeTTYReady({
        signal: controller.signal,
        load: () => k8sFetch(podURL, { signal: controller.signal }),
        onAttempt: (attempt, pod) => {
          const status = pod.status?.phase ?? "Unknown";
          setPhase(`等待 Pod 就绪…（${status}，第 ${attempt} 次检查）`);
        },
      });
      setPhase("执行命令中…");
      const url = podOperationPath(clusterId, session.pod.namespace, session.pod.name, "exec");
      const execURL = `${url}?node_tty=true&container=node-shell&${toCommandQuery(command.trim().split(/\s+/).filter(Boolean))}`;
      await readKubernetesSSE(execURL, setOutput, controller.signal);
      setPhase(null);
    } catch (err) {
      if (controller.signal.aborted) return;
      if (createdPod) {
        await k8sFetch(
          `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/namespaces/${encodeURIComponent(createdPod.namespace)}/pods/${encodeURIComponent(createdPod.name)}`,
          { method: "DELETE" },
        ).catch(() => undefined);
      }
      setError(errorInfo(err, "NodeTTY 创建或命令执行失败"));
      setPhase(null);
    } finally {
      if (active.current === controller) {
        active.current = null;
        setRunning(false);
      }
    }
  }

  const panelText = output.join("\n") || (!running ? "暂无输出" : "");

  return (
    <div className="k8s-stream-view stack-sm">
      <p className="text-sm text-muted">通过临时 Pod 在节点 {node} 上执行一次性命令。</p>
      <div className="k8s-stream-toolbar">
        <input
          className="input font-mono text-sm flex-1"
          value={command}
          disabled={running}
          onChange={(e) => setCommand(e.target.value)}
        />
        {canManage ? (
          <button
            type="button"
            className="btn-secondary text-sm"
            disabled={!command.trim() || running}
            onClick={() => void run()}
          >
            {running ? "执行中…" : "执行"}
          </button>
        ) : null}
      </div>
      <ApiErrorBanner error={error} />
      {running ? <K8sLoadingState compact label={phase ?? "处理中…"} /> : null}
      <pre className={`k8s-stream-panel${running ? " is-pending" : ""}`}>{panelText}</pre>
    </div>
  );
}

function NodeMonitorForm({ clusterId, node }: { clusterId: string; node: string }) {
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const humanizedError = error ? humanizeK8sApiError(error) : null;
  const prometheusMissing = isKnownK8sApiError(error, "PLUGIN_PROMETHEUS_NOT_CONFIGURED");

  async function query() {
    setLoading(true);
    setError(null);
    setResult("");
    try {
      const [cpu, memory] = await Promise.all(
        ["node_cpu", "node_memory"].map((template) =>
          k8sFetch<unknown>(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/metrics`, {
            method: "POST",
            body: JSON.stringify({ template, range: "1h", node }),
          }),
        ),
      );
      setResult(JSON.stringify({ cpu, memory }, null, 2));
    } catch (err) {
      setError(errorInfo(err, "指标查询失败"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void query();
  }, [clusterId, node]);

  return (
    <div className="k8s-stream-view stack-sm">
      <div className="k8s-stream-toolbar">
        <span className="text-sm text-muted">
          {loading ? "查询中…" : `节点 ${node} · CPU / 内存（近 1 小时）`}
        </span>
        <button type="button" className="btn-secondary text-sm" disabled={loading} onClick={() => void query()}>
          刷新
        </button>
      </div>
      {loading ? <K8sLoadingState compact label="正在查询指标…" /> : null}
      {prometheusMissing && humanizedError ? (
        <K8sEmptyState compact title={humanizedError.message} description={humanizedError.hint} />
      ) : null}
      {!prometheusMissing && error ? <ApiErrorBanner error={error} /> : null}
      {!prometheusMissing ? (
        <pre className="k8s-stream-panel">{result || (!loading && !error ? "暂无指标数据" : "")}</pre>
      ) : null}
    </div>
  );
}

async function loadNodePods(clusterId: string, nodeName: string): Promise<PodObject[]> {
  try {
    const data = await k8sFetch<{ items?: PodObject[] }>(
      `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/pods?fieldSelector=${encodeURIComponent(`spec.nodeName=${nodeName}`)}&page=1&pageSize=500`,
    );
    return data.items ?? [];
  } catch {
    return [];
  }
}

async function loadNodeMetrics(clusterId: string): Promise<Map<string, NodeMetrics>> {
  const map = new Map<string, NodeMetrics>();
  try {
    const data = await k8sFetch<{ items?: NodeMetrics[] }>(
      `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/apis/metrics.k8s.io/v1beta1/nodes`,
    );
    for (const item of data.items ?? []) {
      if (item.metadata?.name) map.set(item.metadata.name, item);
    }
  } catch {
    return map;
  }
  return map;
}
