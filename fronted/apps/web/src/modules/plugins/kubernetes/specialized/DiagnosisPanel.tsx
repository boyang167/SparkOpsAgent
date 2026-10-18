"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import {
  K8sEmptyState,
  K8sPanelCard,
  K8sResourceTabs,
  K8sStatusBadge,
} from "@/components/k8s";
import type { K8sStatusTone } from "@/components/k8s/K8sStatusBadge";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CLUSTER_STORAGE_KEY } from "../ClusterPicker";
import { CATALOG, proxyPath, selectStoredClusterId } from "../catalog";
import type { KubernetesCluster } from "../cluster-types";
import {
  DIAGNOSIS_TABS,
  dashboardFromProxy,
  diagnosisGetByPod,
  diagnosisInstall,
  diagnosisJps,
  diagnosisProxy,
  diagnosisUninstall,
  flattenArthasResults,
  isTargetReady,
  threadStateSummary,
  type DiagnosisRecord,
  type DiagnosisTab,
  type DiagnosisTarget,
  type NameValueRow,
  type ProcessOption,
} from "../diagnosis-view";
import { resourceCollectionUrl } from "../resource-url";
import { DiagnosisTerminal } from "./DiagnosisTerminal";

type PodObject = {
  metadata?: { name?: string; namespace?: string };
  spec?: { containers?: { name: string }[] };
};

type DiagnosisContextValue = {
  target: DiagnosisTarget;
  installed: DiagnosisRecord | null;
  sessionId: string;
  consumerId: string;
  canManage: boolean;
};

const DiagnosisContext = createContext<DiagnosisContextValue | null>(null);
const DIAGNOSIS_TAB_IDS = DIAGNOSIS_TABS.map((item) => item.id);

function useDiagnosisContext() {
  const ctx = useContext(DiagnosisContext);
  if (!ctx) throw new Error("DiagnosisContext missing");
  return ctx;
}

export function DiagnosisPanel({ canManage }: { canManage: boolean }) {
  const mountedRef = useRef(true);
  const [clusters, setClusters] = useState<KubernetesCluster[]>([]);
  const [target, setTarget] = useState<DiagnosisTarget>({
    clusterId: "",
    namespace: "",
    pod: "",
    container: "",
    processId: "",
  });
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [pods, setPods] = useState<string[]>([]);
  const [containers, setContainers] = useState<string[]>([]);
  const [processes, setProcesses] = useState<ProcessOption[]>([]);
  const [installed, setInstalled] = useState<DiagnosisRecord | null>(null);
  const [sessionId, setSessionId] = useState("");
  const [consumerId, setConsumerId] = useState("");
  const [tab, setTab] = useState<DiagnosisTab>("dashboard");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [terminalOpen, setTerminalOpen] = useState(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const data = await k8sFetch<{ items?: KubernetesCluster[] }>("/api/bff/v1/plugins/kubernetes/clusters");
        const items = data.items ?? [];
        if (!mountedRef.current) return;
        setClusters(items);
        const stored = typeof window === "undefined" ? null : window.localStorage.getItem(CLUSTER_STORAGE_KEY);
        const next = selectStoredClusterId(stored, items);
        if (next) {
          window.localStorage.setItem(CLUSTER_STORAGE_KEY, next);
          setTarget((current) => ({ ...current, clusterId: next }));
        }
      } catch (err) {
        if (!mountedRef.current) return;
        setError(errorInfo(err, "加载集群失败"));
      }
    })();
  }, []);

  const namespaceDef = useMemo(() => CATALOG.find((item) => item.kind === "Namespace"), []);
  const podDef = useMemo(() => CATALOG.find((item) => item.kind === "Pod"), []);

  const loadNamespaces = useCallback(async () => {
    if (!target.clusterId || !namespaceDef) return;
    const url = resourceCollectionUrl(target.clusterId, namespaceDef, { page: 1, pageSize: 500, keyword: "" });
    if (!url) return;
    const data = await k8sFetch<{ items?: { metadata?: { name?: string } }[] }>(url);
    const names = (data.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[];
    setNamespaces(names);
    setTarget((current) => ({ ...current, namespace: current.namespace || names[0] || "" }));
  }, [namespaceDef, target.clusterId]);

  const loadPods = useCallback(async () => {
    if (!target.clusterId || !podDef || !target.namespace) return;
    const url = resourceCollectionUrl(target.clusterId, podDef, {
      page: 1,
      pageSize: 500,
      keyword: "",
      namespace: target.namespace,
    });
    if (!url) return;
    const data = await k8sFetch<{ items?: PodObject[] }>(url);
    const names = (data.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[];
    setPods(names);
    setTarget((current) => ({ ...current, pod: current.pod && names.includes(current.pod) ? current.pod : names[0] || "" }));
  }, [podDef, target.clusterId, target.namespace]);

  const loadContainers = useCallback(async () => {
    if (!target.clusterId || !target.namespace || !target.pod || !podDef) return;
    const data = await k8sFetch<PodObject>(proxyPath(target.clusterId, podDef, target.namespace, target.pod));
    const names = (data.spec?.containers ?? []).map((item) => item.name);
    setContainers(names);
    setTarget((current) => ({
      ...current,
      container: current.container && names.includes(current.container) ? current.container : names[0] || "",
    }));
  }, [podDef, target.clusterId, target.namespace, target.pod]);

  const refreshInstalled = useCallback(async () => {
    if (!isTargetReady(target)) {
      setInstalled(null);
      return;
    }
    try {
      const data = await diagnosisGetByPod(target.clusterId, target);
      if (!mountedRef.current) return;
      const record = data.diagnosis?.id ? data.diagnosis : null;
      setInstalled(record);
    } catch {
      if (!mountedRef.current) return;
      setInstalled(null);
    }
  }, [target]);

  const refreshProcesses = useCallback(async () => {
    if (!isTargetReady(target) || installed?.id) return;
    try {
      const data = await diagnosisJps(target.clusterId, target);
      if (!mountedRef.current) return;
      setProcesses(data.diagnosis ?? []);
    } catch (err) {
      if (!mountedRef.current) return;
      setProcesses([]);
      setError(errorInfo(err, "加载 Java 进程失败"));
    }
  }, [installed?.id, target]);

  useEffect(() => {
    void loadNamespaces().catch((err) => setError(errorInfo(err, "加载命名空间失败")));
  }, [loadNamespaces]);

  useEffect(() => {
    void loadPods().catch((err) => setError(errorInfo(err, "加载 Pod 失败")));
  }, [loadPods]);

  useEffect(() => {
    void loadContainers().catch((err) => setError(errorInfo(err, "加载容器失败")));
  }, [loadContainers]);

  useEffect(() => {
    void refreshInstalled();
    const timer = window.setInterval(() => void refreshInstalled(), 3000);
    return () => window.clearInterval(timer);
  }, [refreshInstalled]);

  useEffect(() => {
    void refreshProcesses();
  }, [refreshProcesses]);

  async function handleInstall() {
    if (!canManage || !target.processId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await diagnosisInstall(target.clusterId, target);
      setInstalled(data.diagnosis ?? null);
    } catch (err) {
      setError(errorInfo(err, "安装 Arthas 失败"));
    } finally {
      setLoading(false);
    }
  }

  async function handleUninstall() {
    if (!canManage) return;
    setLoading(true);
    setError(null);
    try {
      await diagnosisUninstall(target.clusterId, target);
      setInstalled(null);
      setSessionId("");
      setConsumerId("");
    } catch (err) {
      setError(errorInfo(err, "卸载 Arthas 失败"));
    } finally {
      setLoading(false);
    }
  }

  const contextValue = useMemo(
    () => ({ target, installed, sessionId, consumerId, canManage }),
    [canManage, consumerId, installed, sessionId, target],
  );

  const targetReady = isTargetReady(target);
  const tabLabel = (id: DiagnosisTab) => DIAGNOSIS_TABS.find((item) => item.id === id)?.label ?? id;

  return (
    <div className="stack-md">
      <K8sPanelCard className="k8s-diagnosis-panel">
        <div className="k8s-clusters-toolbar k8s-workloads-toolbar k8s-diagnosis-target-toolbar">
          <div className="k8s-clusters-toolbar-filters">
            <label className="k8s-diagnosis-field">
              <span className="sr-only">集群</span>
              <select
                className="input"
                value={target.clusterId}
                aria-label="集群"
                onChange={(e) => {
                  const clusterId = e.target.value;
                  window.localStorage.setItem(CLUSTER_STORAGE_KEY, clusterId);
                  setTarget({ clusterId, namespace: "", pod: "", container: "", processId: "" });
                }}
              >
                <option value="">选择集群</option>
                {clusters.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="k8s-diagnosis-field">
              <span className="sr-only">命名空间</span>
              <select
                className="input"
                value={target.namespace}
                aria-label="命名空间"
                disabled={!target.clusterId}
                onChange={(e) =>
                  setTarget((current) => ({ ...current, namespace: e.target.value, pod: "", container: "", processId: "" }))
                }
              >
                {namespaces.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
            <label className="k8s-diagnosis-field">
              <span className="sr-only">Pod</span>
              <select
                className="input"
                value={target.pod}
                aria-label="Pod"
                disabled={!target.namespace}
                onChange={(e) => setTarget((current) => ({ ...current, pod: e.target.value, container: "", processId: "" }))}
              >
                {pods.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
            <label className="k8s-diagnosis-field">
              <span className="sr-only">Container</span>
              <select
                className="input"
                value={target.container}
                aria-label="Container"
                disabled={!target.pod}
                onChange={(e) => setTarget((current) => ({ ...current, container: e.target.value, processId: "" }))}
              >
                {containers.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
            {!installed?.id ? (
              <label className="k8s-diagnosis-field k8s-diagnosis-field-process">
                <span className="sr-only">Java 进程</span>
                <select
                  className="input"
                  value={target.processId}
                  aria-label="Java 进程"
                  disabled={!targetReady}
                  onChange={(e) => setTarget((current) => ({ ...current, processId: e.target.value }))}
                >
                  <option value="">选择进程</option>
                  {processes.map((item) => (
                    <option key={item.value} value={item.value}>
                      {item.value} {item.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
          </div>
          <div className="k8s-toolbar-actions">
            {installed?.id ? (
              <K8sStatusBadge tone="success" dot>
                Arthas 已安装
              </K8sStatusBadge>
            ) : targetReady ? (
              <span className="k8s-clusters-count" role="status">
                {processes.length > 0 ? `${processes.length} 个 Java 进程` : "未发现 Java 进程"}
              </span>
            ) : null}
            {!installed?.id ? (
              <>
                <button
                  type="button"
                  className="btn-secondary k8s-clusters-refresh-btn"
                  disabled={!targetReady || loading}
                  aria-label="刷新进程列表"
                  onClick={() => void refreshProcesses()}
                >
                  ↻
                </button>
                <button
                  type="button"
                  className="btn text-sm"
                  disabled={!canManage || loading || !target.processId}
                  onClick={() => void handleInstall()}
                >
                  安装 Arthas
                </button>
              </>
            ) : (
              <>
                <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={() => setTerminalOpen(true)}>
                  终端
                </button>
                <button
                  type="button"
                  className="btn-secondary text-sm is-danger"
                  disabled={!canManage || loading}
                  onClick={() => void handleUninstall()}
                >
                  卸载
                </button>
              </>
            )}
          </div>
        </div>

        {!canManage ? <p className="text-sm text-muted">需要 governance:admin 权限才能安装或卸载 Arthas。</p> : null}
        <ApiErrorBanner error={error} />
      </K8sPanelCard>

      <DiagnosisContext.Provider value={contextValue}>
        <K8sPanelCard className="k8s-diagnosis-panel">
          <K8sResourceTabs
            tabs={DIAGNOSIS_TAB_IDS}
            active={tab}
            ariaLabel="诊断视图"
            label={tabLabel}
            onChange={setTab}
          />
          {tab === "dashboard" ? (
            <DiagnosisDashboard
              onSessionChange={(nextSessionId, nextConsumerId) => {
                setSessionId(nextSessionId);
                setConsumerId(nextConsumerId);
              }}
            />
          ) : (
            <DiagnosisCommandTab tab={tab} />
          )}
        </K8sPanelCard>
      </DiagnosisContext.Provider>

      <Modal
        open={terminalOpen}
        title={target.pod ? `Arthas 终端 · ${target.namespace}/${target.pod}` : "Arthas 终端"}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setTerminalOpen(false)}
      >
        <DiagnosisTerminal target={target} onClose={() => setTerminalOpen(false)} />
      </Modal>
    </div>
  );
}

function DiagnosisDashboard({ onSessionChange }: { onSessionChange: (sessionId: string, consumerId: string) => void }) {
  const { target, installed, sessionId, consumerId } = useDiagnosisContext();
  const [dashboardReady, setDashboardReady] = useState(false);
  const [threads, setThreads] = useState(dashboardFromProxy().threads);
  const [memory, setMemory] = useState(dashboardFromProxy().memory);
  const [gcInfos, setGcInfos] = useState(dashboardFromProxy().gcInfos);
  const [runtimeInfo, setRuntimeInfo] = useState(dashboardFromProxy().runtimeInfo);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  const initSession = useCallback(async () => {
    if (!installed?.id || !isTargetReady(target)) return;
    const data = await diagnosisProxy(target.clusterId, target, { action: "init_session" });
    const nextSessionId = String(data.arthas?.sessionId ?? "");
    const nextConsumerId = String(data.arthas?.consumerId ?? "");
    onSessionChange(nextSessionId, nextConsumerId);
    if (!nextSessionId) return;
    const execData = await diagnosisProxy(target.clusterId, target, {
      action: "async_exec",
      command: "dashboard",
      sessionId: nextSessionId,
    });
    if (execData.arthas?.body?.jobStatus === "READY") {
      setDashboardReady(true);
    }
  }, [installed?.id, onSessionChange, target]);

  const pullDashboard = useCallback(async () => {
    if (!dashboardReady || !sessionId || !consumerId || !isTargetReady(target)) return;
    try {
      const data = await diagnosisProxy(target.clusterId, target, {
        action: "pull_results",
        sessionId,
        consumerId,
      });
      if (data.arthas?.state === "SUCCEEDED") {
        const parsed = dashboardFromProxy(data.arthas);
        setThreads(parsed.threads);
        setMemory(parsed.memory);
        setGcInfos(parsed.gcInfos);
        setRuntimeInfo(parsed.runtimeInfo);
      } else {
        setDashboardReady(false);
        await initSession();
      }
    } catch (err) {
      setError(errorInfo(err, "加载控制面板失败"));
    }
  }, [consumerId, dashboardReady, initSession, sessionId, target]);

  useEffect(() => {
    setDashboardReady(false);
    void initSession().catch((err) => setError(errorInfo(err, "初始化 Arthas 会话失败")));
  }, [initSession, target.clusterId, target.namespace, target.pod, target.container, installed?.id]);

  useEffect(() => {
    void pullDashboard();
    const timer = window.setInterval(() => void pullDashboard(), 10000);
    return () => window.clearInterval(timer);
  }, [pullDashboard]);

  const summary = threadStateSummary(threads);

  if (!installed?.id) {
    return (
      <K8sEmptyState
        compact
        title="尚未安装 Arthas"
        description="选择 Java 进程并安装 Arthas 后，即可查看 JVM 控制面板。"
      />
    );
  }

  return (
    <div className="stack-md k8s-diagnosis-dashboard">
      <ApiErrorBanner error={error} />
      <DiagnosisSection title="线程 TOP-10">
        <div className="k8s-diagnosis-thread-summary">
          <span className="k8s-clusters-count">总量 {summary.SUM}</span>
          {(["NEW", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "TERMINATED"] as const).map((state) => (
            <K8sStatusBadge key={state} tone={threadStateTone(state)} dot>
              {state} {summary[state]}
            </K8sStatusBadge>
          ))}
        </div>
        <DiagnosisTable rows={threads.slice(0, 10).map((item) => ({ name: item.name ?? "-", value: item.state ?? "-" }))} />
      </DiagnosisSection>
      <DiagnosisSection title="内存">
        <DiagnosisTable
          rows={memory.map((item) => ({
            name: item.name ?? item.type ?? "-",
            value: `${item.used ?? 0}/${item.total ?? item.max ?? 0}`,
          }))}
        />
      </DiagnosisSection>
      <DiagnosisSection title="GC">
        <DiagnosisTable
          rows={gcInfos.map((item) => ({
            name: item.name ?? "-",
            value: `${item.collectionCount ?? 0} / ${item.collectionTime ?? 0}ms`,
          }))}
        />
      </DiagnosisSection>
      <DiagnosisSection title="运行时">
        <DiagnosisTable
          rows={[
            { name: "OS", value: `${runtimeInfo.osName ?? "-"} ${runtimeInfo.osVersion ?? ""}`.trim() },
            { name: "Java", value: runtimeInfo.javaVersion ?? "-" },
            { name: "Java Home", value: runtimeInfo.javaHome ?? "-" },
            { name: "Uptime", value: String(runtimeInfo.uptime ?? "-") },
          ]}
        />
      </DiagnosisSection>
    </div>
  );
}

function DiagnosisCommandTab({ tab }: { tab: DiagnosisTab }) {
  const { target, installed } = useDiagnosisContext();
  const def = DIAGNOSIS_TABS.find((item) => item.id === tab);
  const [rows, setRows] = useState<NameValueRow[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  const load = useCallback(async () => {
    if (!installed?.id || !def?.command || !isTargetReady(target)) {
      setRows([]);
      return;
    }
    try {
      const data = await diagnosisProxy(target.clusterId, target, { action: "exec", command: def.command });
      setRows(flattenArthasResults(data.arthas));
    } catch (err) {
      setError(errorInfo(err, "加载诊断数据失败"));
    }
  }, [def?.command, installed?.id, target]);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 10000);
    return () => window.clearInterval(timer);
  }, [load]);

  if (!installed?.id) {
    return (
      <K8sEmptyState compact title="尚未安装 Arthas" description="安装 Arthas 后即可执行诊断命令并查看结果。" />
    );
  }

  return (
    <div className="stack-md">
      <div className="k8s-clusters-toolbar">
        <div />
        <div className="k8s-toolbar-actions">
          <span className="k8s-clusters-count" role="status">
            {tabLabel(tab)}
          </span>
          <button
            type="button"
            className="btn-secondary k8s-clusters-refresh-btn"
            aria-label="刷新数据"
            onClick={() => void load()}
          >
            ↻
          </button>
        </div>
      </div>
      <ApiErrorBanner error={error} />
      <DiagnosisTable rows={rows} emptyTitle={`暂无 ${tabLabel(tab)} 数据`} />
    </div>
  );
}

function tabLabel(tab: DiagnosisTab): string {
  return DIAGNOSIS_TABS.find((item) => item.id === tab)?.label ?? tab;
}

function DiagnosisSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="stack-sm k8s-diagnosis-section">
      <h2 className="k8s-diagnosis-section-title">{title}</h2>
      {children}
    </section>
  );
}

function DiagnosisTable({ rows, emptyTitle = "暂无数据" }: { rows: NameValueRow[]; emptyTitle?: string }) {
  if (!rows.length) {
    return <K8sEmptyState variant="empty" compact title={emptyTitle} description="当前命令未返回可展示的数据。" />;
  }
  return (
    <div className="table-wrap k8s-diagnosis-table">
      <table className="data-table text-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>值</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={`${row.name}-${index}`}>
              <td className="font-medium">{row.name}</td>
              <td className="text-muted font-mono text-xs whitespace-pre-wrap">{row.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function threadStateTone(state: string): K8sStatusTone {
  if (state === "RUNNABLE") return "success";
  if (state === "BLOCKED") return "danger";
  if (state === "WAITING" || state === "TIMED_WAITING") return "warning";
  if (state === "NEW") return "info";
  return "neutral";
}
