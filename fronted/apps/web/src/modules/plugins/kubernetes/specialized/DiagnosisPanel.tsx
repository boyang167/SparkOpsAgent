"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
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

  return (
    <div className="stack-lg">
      <section className="card stack-md p-4">
        <div>
          <h1 className="text-xl font-semibold">应用诊断</h1>
          <p className="text-sm text-muted mt-1">选择 Java 容器并安装 Arthas，查看 JVM 运行态与诊断命令结果。</p>
        </div>
        <div className="flex flex-wrap gap-3 items-end">
          <label>
            <span className="label">集群</span>
            <select
              className="input min-w-40"
              value={target.clusterId}
              onChange={(e) => {
                const clusterId = e.target.value;
                window.localStorage.setItem(CLUSTER_STORAGE_KEY, clusterId);
                setTarget({ clusterId, namespace: "", pod: "", container: "", processId: "" });
              }}
            >
              <option value="">请选择</option>
              {clusters.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="label">命名空间</span>
            <select
              className="input min-w-40"
              value={target.namespace}
              onChange={(e) => setTarget((current) => ({ ...current, namespace: e.target.value, pod: "", container: "", processId: "" }))}
            >
              {namespaces.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="label">Pod</span>
            <select
              className="input min-w-40"
              value={target.pod}
              onChange={(e) => setTarget((current) => ({ ...current, pod: e.target.value, container: "", processId: "" }))}
            >
              {pods.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="label">Container</span>
            <select
              className="input min-w-40"
              value={target.container}
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
            <label>
              <span className="label">进程</span>
              <select
                className="input min-w-48"
                value={target.processId}
                onChange={(e) => setTarget((current) => ({ ...current, processId: e.target.value }))}
              >
                <option value="">请选择</option>
                {processes.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.value} {item.name}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {!installed?.id ? (
            <button type="button" className="btn" disabled={!canManage || loading || !target.processId} onClick={() => void handleInstall()}>
              安装
            </button>
          ) : (
            <>
              <button type="button" className="btn btn-danger" disabled={!canManage || loading} onClick={() => void handleUninstall()}>
                卸载
              </button>
              <button type="button" className="btn" disabled={!canManage} onClick={() => setTerminalOpen(true)}>
                终端
              </button>
            </>
          )}
        </div>
        {!canManage ? <div className="alert alert-info">需要 governance:admin 权限才能安装或卸载 Arthas。</div> : null}
        <ApiErrorBanner error={error} />
      </section>

      <DiagnosisContext.Provider value={contextValue}>
        <section className="card stack-md p-4">
          <div className="flex flex-wrap gap-2 border-b border-border pb-3">
            {DIAGNOSIS_TABS.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`btn btn-sm ${tab === item.id ? "" : "btn-ghost"}`}
                onClick={() => setTab(item.id)}
              >
                {item.label}
              </button>
            ))}
          </div>
          {tab === "dashboard" ? (
            <DiagnosisDashboard onSessionChange={(nextSessionId, nextConsumerId) => {
              setSessionId(nextSessionId);
              setConsumerId(nextConsumerId);
            }} />
          ) : (
            <DiagnosisCommandTab tab={tab} />
          )}
        </section>
      </DiagnosisContext.Provider>

      <Modal open={terminalOpen} title={`Arthas 终端 · ${target.pod}/${target.container}`} onClose={() => setTerminalOpen(false)}>
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

  return (
    <div className="stack-md">
      <ApiErrorBanner error={error} />
      {!installed?.id ? <div className="alert alert-info">请先安装 Arthas 后再查看控制面板。</div> : null}
      <div className="stack-sm">
        <h2 className="font-medium">线程 TOP-10</h2>
        <div className="flex flex-wrap gap-2 text-sm">
          <span>总量: {summary.SUM}</span>
          {(["NEW", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "TERMINATED"] as const).map((state) => (
            <span key={state} className="badge">
              {state}: {summary[state]}
            </span>
          ))}
        </div>
        <DiagnosisTable rows={threads.slice(0, 10).map((item) => ({ name: item.name ?? "-", value: item.state ?? "-" }))} />
      </div>
      <div className="stack-sm">
        <h2 className="font-medium">内存</h2>
        <DiagnosisTable
          rows={memory.map((item) => ({
            name: item.name ?? item.type ?? "-",
            value: `${item.used ?? 0}/${item.total ?? item.max ?? 0}`,
          }))}
        />
      </div>
      <div className="stack-sm">
        <h2 className="font-medium">GC</h2>
        <DiagnosisTable
          rows={gcInfos.map((item) => ({
            name: item.name ?? "-",
            value: `${item.collectionCount ?? 0} / ${item.collectionTime ?? 0}ms`,
          }))}
        />
      </div>
      <div className="stack-sm">
        <h2 className="font-medium">运行时</h2>
        <DiagnosisTable
          rows={[
            { name: "OS", value: `${runtimeInfo.osName ?? "-"} ${runtimeInfo.osVersion ?? ""}`.trim() },
            { name: "Java", value: runtimeInfo.javaVersion ?? "-" },
            { name: "Java Home", value: runtimeInfo.javaHome ?? "-" },
            { name: "Uptime", value: String(runtimeInfo.uptime ?? "-") },
          ]}
        />
      </div>
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

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn btn-sm" onClick={() => void load()}>
          更新
        </button>
      </div>
      <ApiErrorBanner error={error} />
      {!installed?.id ? <div className="alert alert-info">请先安装 Arthas。</div> : null}
      <DiagnosisTable rows={rows} />
    </div>
  );
}

function DiagnosisTable({ rows }: { rows: NameValueRow[] }) {
  if (!rows.length) return <div className="text-sm text-muted">暂无数据</div>;
  return (
    <div className="overflow-auto">
      <table className="table table-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>值</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={`${row.name}-${index}`}>
              <td>{row.name}</td>
              <td className="font-mono text-xs whitespace-pre-wrap">{row.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
