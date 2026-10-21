"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "../sse";
import { podOperationPath, toCommandQuery, waitForNodeTTYReady } from "../operations";
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
  const [draftKeyword, setDraftKeyword] = useState("");
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

  return (
    <div className="stack-md">
      <K8sPanelCard>
        <div>
          <h1 className="text-xl font-semibold">节点管理</h1>
          <p className="text-sm text-muted mt-1">查看节点状态、资源用量，并执行调度、排空与终端操作。</p>
        </div>
        <ApiErrorBanner error={error} />
        <div className="k8s-toolbar-row">
          <label>
            <span className="label">名称</span>
            <input
              className="input"
              placeholder="名称"
              value={draftKeyword}
              onChange={(e) => setDraftKeyword(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  setKeyword(draftKeyword);
                  setPage(1);
                }
              }}
            />
          </label>
          <button
            type="button"
            className="btn"
            disabled={loading}
            onClick={() => {
              setKeyword(draftKeyword);
              setPage(1);
            }}
          >
            查询
          </button>
          <button
            type="button"
            className="btn-secondary"
            onClick={() => {
              setDraftKeyword("");
              setKeyword("");
              setPage(1);
            }}
          >
            重置
          </button>
          <button type="button" className="btn" disabled={!canManage} onClick={openLabelsTaints}>
            标签与污点
          </button>
          <button type="button" className="btn" disabled={!canManage} onClick={() => setCloudOpen(true)}>
            CloudShell
          </button>
          <button type="button" className="btn" onClick={() => setChatOpen(true)}>
            AI 助手
          </button>
        </div>
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>
                  <input
                    type="checkbox"
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
                <th>CPU用量</th>
                <th>内存用量</th>
                <th>版本</th>
                <th>存活时间</th>
                <th>操作</th>
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
                        <button type="button" className="k8s-action-link font-semibold" onClick={() => void openDetail(name)}>
                          {name}
                        </button>
                        <LabelsHint labels={row.node.metadata?.labels} />
                      </td>
                      <td className="font-mono text-xs">{nodeInternalIP(row.node)}</td>
                      <td>
                        <span className={ready ? "badge badge-success" : "badge badge-danger"}>{status}</span>
                      </td>
                      <td>
                        <div className="k8s-role-badges">
                          {nodeRoles(row.node.metadata?.labels).map((role) => (
                            <span key={role} className="badge badge-neutral">
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
                      <td className="text-xs leading-5">
                        <div>{info?.kubeletVersion || "-"}</div>
                        <div className="text-muted">{info?.containerRuntimeVersion || "-"}</div>
                        <div className="text-muted">{info?.osImage || "-"}</div>
                      </td>
                      <td>{formatNodeAge(row.node.metadata?.creationTimestamp)}</td>
                      <td className="whitespace-nowrap">
                        <button
                          type="button"
                          className="k8s-action-link"
                          disabled={!canManage}
                          onClick={() =>
                            void refreshNode(name)
                              .then(setEditNode)
                              .catch((err) => setError(errorInfo(err, "加载节点失败")))
                          }
                        >
                          编辑
                        </button>
                        <span className="text-muted"> / </span>
                        <button
                          type="button"
                          className="k8s-action-link"
                          onClick={() => setMoreNode(row.node)}
                        >
                          更多
                        </button>
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
        title={moreNode ? `更多操作 · ${moreNode.metadata?.name ?? ""}` : "更多操作"}
        description={
          moreNode
            ? `${nodeInternalIP(moreNode)} · ${nodeStatus(moreNode)} · ${nodeRoles(moreNode.metadata?.labels).join(", ")}`
            : undefined
        }
        size="md"
        onClose={() => setMoreNode(null)}
      >
        {moreNode ? (
          <div className="k8s-action-sheet">
            {(
              [
                {
                  key: "detail",
                  label: "详情",
                  hint: "查看节点完整信息",
                  enabled: true,
                  run: () => {
                    const n = moreNode.metadata?.name;
                    if (n) void openDetail(n);
                  },
                },
                {
                  key: "shell",
                  label: "Shell",
                  hint: "在节点上执行一次性命令",
                  enabled: canManage,
                  run: () => setShellNode(moreNode.metadata?.name ?? null),
                },
                {
                  key: "monitor",
                  label: "监控",
                  hint: "查询节点 CPU / 内存指标",
                  enabled: true,
                  run: () => setMonitorNode(moreNode.metadata?.name ?? null),
                },
                {
                  key: "yaml",
                  label: "YAML",
                  hint: "以 YAML 编辑完整节点对象",
                  enabled: canManage,
                  run: () => {
                    const name = moreNode.metadata?.name;
                    if (!name) return;
                    void refreshNode(name)
                      .then(setYamlNode)
                      .catch((err) => setError(errorInfo(err, "加载节点失败")));
                  },
                },
                {
                  key: "drain",
                  label: "节点排空",
                  hint: "设置不可调度并驱逐工作负载 Pod",
                  enabled: canManage,
                  danger: false,
                  run: () => void drain(moreNode),
                },
                {
                  key: "schedule",
                  label: "调度设置",
                  hint: moreNode.spec?.unschedulable ? "恢复为可调度" : "设置为不可调度",
                  enabled: canManage,
                  run: () => void toggleSchedule(moreNode),
                },
                {
                  key: "delete",
                  label: "删除",
                  hint: "从集群删除该节点对象",
                  enabled: canManage,
                  danger: true,
                  run: () => void deleteNode(moreNode),
                },
              ] as const
            ).map((action) => (
              <button
                key={action.key}
                type="button"
                className={"danger" in action && action.danger ? "k8s-action-sheet-item is-danger" : "k8s-action-sheet-item"}
                disabled={!action.enabled}
                onClick={() => {
                  setMoreNode(null);
                  action.run();
                }}
              >
                <span className="k8s-action-sheet-label">{action.label}</span>
                <span className="k8s-action-sheet-hint">{action.hint}</span>
              </button>
            ))}
          </div>
        ) : null}
      </Modal>
      <Modal
        open={Boolean(editNode)}
        title="编辑"
        size="lg"
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
      <Modal open={Boolean(yamlNode)} title={`YAML ${yamlNode?.metadata?.name ?? ""}`} size="xl" onClose={() => setYamlNode(null)}>
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
      <Modal open={Boolean(detailNode)} title={`详情 ${detailNode?.metadata?.name ?? ""}`} size="xl" onClose={() => setDetailNode(null)}>
        {detailNode ? <NodeDetailView clusterId={clusterId} node={detailNode} canManage={canManage} /> : null}
      </Modal>
      <Modal nested open={cloudOpen} title="CloudShell" size="xl" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal nested open={chatOpen} title="AI 助手" size="xl" onClose={() => setChatOpen(false)}>
        <ChatPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal nested open={Boolean(shellNode)} title={`NodeShell ${shellNode ?? ""}`} size="lg" onClose={() => setShellNode(null)}>
        {shellNode ? <NodeShellForm clusterId={clusterId} node={shellNode} canManage={canManage} /> : null}
      </Modal>
      <Modal nested open={Boolean(monitorNode)} title={`节点监控 ${monitorNode ?? ""}`} size="lg" onClose={() => setMonitorNode(null)}>
        {monitorNode ? <NodeMonitorForm clusterId={clusterId} node={monitorNode} /> : null}
      </Modal>
      <Modal nested open={Boolean(taintNode)} title={`标签与污点 ${taintNode?.metadata?.name ?? ""}`} size="lg" onClose={() => setTaintNode(null)}>
        {taintNode ? (
          <LabelsTaintsForm
            node={taintNode}
            canManage={canManage}
            onSave={saveNode}
          />
        ) : null}
      </Modal>
      {confirmDialog}
    </div>
  );
}

function MetricCell({ view }: { view: NodeMetricView }) {
  return (
    <div className="k8s-percent-box">
      <div>{view.percent.toFixed(0)}%</div>
      <div className="k8s-percent-sep" />
      <div className={view.overcommit ? "text-red-600" : "text-xs text-muted"}>{view.usedLabel}</div>
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
      <div className="flex justify-end gap-2">
        <button type="button" className="btn-secondary" disabled={saving} onClick={onCancel}>
          取消
        </button>
        <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void submit()}>
          {saving ? "保存中…" : "确定"}
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
  const [labelsText, setLabelsText] = useState(
    Object.entries(node.metadata?.labels ?? {})
      .map(([k, v]) => `${k}=${v}`)
      .join("\n"),
  );
  const [taintsText, setTaintsText] = useState(
    (node.spec?.taints ?? []).map((item) => `${item.key ?? ""}=${item.value ?? ""}:${item.effect ?? "NoSchedule"}`).join("\n"),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function save() {
    setError(null);
    const labels: Record<string, string> = {};
    for (const line of labelsText.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const idx = trimmed.indexOf("=");
      if (idx <= 0) {
        setError({ code: "INVALID_REQUEST", message: `标签格式应为 key=value：${trimmed}` });
        return;
      }
      labels[trimmed.slice(0, idx)] = trimmed.slice(idx + 1);
    }
    const taints = [];
    for (const line of taintsText.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const [kv, effect] = trimmed.split(":");
      const eq = kv.indexOf("=");
      if (eq < 0 || !effect) {
        setError({ code: "INVALID_REQUEST", message: `污点格式应为 key=value:Effect：${trimmed}` });
        return;
      }
      taints.push({ key: kv.slice(0, eq), value: kv.slice(eq + 1), effect });
    }
    setSaving(true);
    try {
      await onSave({
        ...node,
        metadata: { ...(node.metadata ?? {}), labels },
        spec: { ...(node.spec ?? {}), taints },
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
      <p className="text-sm text-muted">每行一个。标签：key=value。污点：key=value:NoSchedule|PreferNoSchedule|NoExecute。</p>
      <label>
        <span className="label">标签</span>
        <textarea className="input font-mono text-xs" rows={8} value={labelsText} onChange={(e) => setLabelsText(e.target.value)} />
      </label>
      <label>
        <span className="label">污点</span>
        <textarea className="input font-mono text-xs" rows={6} value={taintsText} onChange={(e) => setTaintsText(e.target.value)} />
      </label>
      <div className="flex justify-end">
        <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void save()}>
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
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);

  async function run() {
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    let createdPod: { name: string; namespace: string } | undefined;
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
      await waitForNodeTTYReady({ signal: controller.signal, load: () => k8sFetch(podURL, { signal: controller.signal }) });
      const url = podOperationPath(clusterId, session.pod.namespace, session.pod.name, "exec");
      const res = await fetch(`${url}?node_tty=true&container=node-shell&${toCommandQuery(command.trim().split(/\s+/).filter(Boolean))}`, {
        cache: "no-store",
        signal: controller.signal,
      });
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
    } catch (err) {
      if (createdPod) {
        await k8sFetch(
          `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/api/v1/namespaces/${encodeURIComponent(createdPod.namespace)}/pods/${encodeURIComponent(createdPod.name)}`,
          { method: "DELETE" },
        ).catch(() => undefined);
      }
      setError(errorInfo(err, "NodeTTY 创建或命令执行失败"));
    }
  }

  return (
    <div className="stack-md">
      <p className="text-sm text-muted">创建特权临时 Pod 后执行一次性命令，不提供交互式终端。</p>
      <input className="input font-mono" value={command} onChange={(e) => setCommand(e.target.value)} />
      {canManage ? (
        <button type="button" className="btn" disabled={!command.trim()} onClick={() => void run()}>
          创建并执行
        </button>
      ) : null}
      <ApiErrorBanner error={error} />
      <pre className="input font-mono text-xs min-h-32 whitespace-pre-wrap">{output.join("\n")}</pre>
    </div>
  );
}

function NodeMonitorForm({ clusterId, node }: { clusterId: string; node: string }) {
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(false);

  async function query() {
    setLoading(true);
    setError(null);
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
    <div className="stack-md">
      <button type="button" className="btn-secondary" disabled={loading} onClick={() => void query()}>
        {loading ? "查询中…" : "刷新"}
      </button>
      <ApiErrorBanner error={error} />
      <pre className="input font-mono text-xs min-h-48 whitespace-pre-wrap">{result || "暂无指标（需配置 Prometheus）"}</pre>
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
