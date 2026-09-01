"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath, type ResourceDef } from "../catalog";
import {
  formatCores,
  formatGi,
  formatNodeAge,
  podResourceSummary,
  podRestartCount,
  sumPodResources,
  type PodObject,
} from "../node-view";
import { type PodEvent } from "../pod-detail-view";
import { podEventsUrl, resourceCollectionUrl } from "../resource-url";
import {
  WORKLOAD_TABS,
  clampPageToTotal,
  cronJobView,
  daemonSetCounts,
  emptyWorkloadTemplate,
  jobStatus,
  podPhaseBadge,
  replicaBadge,
  validateCreateIdentity,
  validateExistingIdentity,
} from "../workload-view";
import { YamlEditor } from "../YamlEditor";
import { ChatPage } from "./ChatPage";
import { CloudTTYPage } from "./CloudTTYPage";
import { PodDetailView } from "./PodDetailView";
import { WorkloadPodActions } from "./WorkloadPodActions";
import { containerReadinessDots, loadPodMetricCell, withLimitedConcurrency } from "./WorkloadPodActions.helpers";

type WorkloadTab = (typeof WORKLOAD_TABS)[number];

type K8sObject = {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
  [key: string]: unknown;
};

type K8sList<T> = {
  items?: T[];
  total?: number;
};

type QueryState = {
  draftKeyword: string;
  keyword: string;
  page: number;
  pageSize: number;
};

type WorkloadTarget = {
  def: ResourceDef;
  value: K8sObject;
};

type PodDetailState = {
  pod: PodObject | null;
  loading: boolean;
  events: PodEvent[];
  eventsLoading: boolean;
  eventsError: ApiErrorInfo | null;
};

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");
const podDef = CATALOG.find((item) => item.kind === "Pod");
const PAGE_SIZES = [10, 30, 50, 100];
const POD_METRIC_CONCURRENCY = 6;

const initialQuery = (): QueryState => ({
  draftKeyword: "",
  keyword: "",
  page: 1,
  pageSize: 10,
});

const initialQueries = (): Record<WorkloadTab, QueryState> =>
  Object.fromEntries(WORKLOAD_TABS.map((item) => [item, initialQuery()])) as Record<WorkloadTab, QueryState>;

export function WorkloadsPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const mountedRef = useRef(false);
  const namespaceRequestSeq = useRef(0);
  const listRequestSeq = useRef(0);
  const objectRequestSeq = useRef(0);
  const podDetailRequestSeq = useRef(0);
  const activeObjectRequest = useRef<AbortController | null>(null);
  const activePodDetailRequest = useRef<AbortController | null>(null);
  const [namespace, setNamespace] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [tab, setTab] = useState<WorkloadTab>("Deployment");
  const [queries, setQueries] = useState<Record<WorkloadTab, QueryState>>(() => initialQueries());
  const [items, setItems] = useState<K8sObject[]>([]);
  const [podMetrics, setPodMetrics] = useState<Record<string, string>>({});
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [detail, setDetail] = useState<K8sObject | null>(null);
  const [podDetail, setPodDetail] = useState<PodDetailState | null>(null);
  const [yamlTarget, setYamlTarget] = useState<WorkloadTarget | null>(null);
  const [editTarget, setEditTarget] = useState<WorkloadTarget | null>(null);
  const [createTarget, setCreateTarget] = useState<{ title: string; def: ResourceDef; value: K8sObject } | null>(null);
  const [cloudOpen, setCloudOpen] = useState(false);
  const [chatOpen, setChatOpen] = useState(false);

  const defByKind = useMemo(
    () => new Map(CATALOG.filter((item) => WORKLOAD_TABS.includes(item.kind as WorkloadTab)).map((item) => [item.kind, item])),
    [],
  );
  const selectedDef = defByKind.get(tab);
  const query = queries[tab];

  const abortObjectRequest = useCallback(() => {
    activeObjectRequest.current?.abort();
    activeObjectRequest.current = null;
    objectRequestSeq.current += 1;
  }, []);

  const abortPodDetailRequest = useCallback(() => {
    activePodDetailRequest.current?.abort();
    activePodDetailRequest.current = null;
    podDetailRequestSeq.current += 1;
  }, []);

  const closeWorkloadModals = useCallback(() => {
    setDetail(null);
    setPodDetail(null);
    setYamlTarget(null);
    setEditTarget(null);
    setCreateTarget(null);
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      abortObjectRequest();
      abortPodDetailRequest();
    };
  }, [abortObjectRequest, abortPodDetailRequest]);

  const updateCurrentQuery = useCallback((patch: Partial<QueryState> | ((current: QueryState) => QueryState)) => {
    setQueries((current) => ({
      ...current,
      [tab]: typeof patch === "function" ? patch(current[tab]) : { ...current[tab], ...patch },
    }));
  }, [tab]);

  const loadNamespaces = useCallback(async (signal?: AbortSignal) => {
    if (!clusterId || !namespaceDef) return;
    const requestSeq = namespaceRequestSeq.current + 1;
    namespaceRequestSeq.current = requestSeq;
    const isCurrent = () => mountedRef.current && !signal?.aborted && requestSeq === namespaceRequestSeq.current;
    try {
      const pageSize = 500;
      let page = 1;
      let totalItems: number | undefined;
      const allItems: K8sObject[] = [];
      for (;;) {
        const url = resourceCollectionUrl(clusterId, namespaceDef, { page, pageSize }) ?? proxyPath(clusterId, namespaceDef);
        const data = await k8sFetch<K8sList<K8sObject>>(url, { signal });
        if (!isCurrent()) return;
        const pageItems = data.items ?? [];
        allItems.push(...pageItems);
        totalItems = data.total ?? totalItems;
        if (pageItems.length < pageSize || (totalItems !== undefined && allItems.length >= totalItems)) break;
        page += 1;
      }
      if (!isCurrent()) return;
      setNamespaces(allItems.map((item) => item.metadata?.name).filter(Boolean) as string[]);
    } catch {
      if (!isCurrent()) return;
      setNamespaces([]);
    }
  }, [clusterId]);

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!clusterId || !selectedDef) return;
    const url = resourceCollectionUrl(clusterId, selectedDef, {
      namespace,
      page: query.page,
      pageSize: query.pageSize,
      keyword: query.keyword,
      allowAllNamespaces: !namespace,
    });
    if (!url) return;
    const requestSeq = listRequestSeq.current + 1;
    listRequestSeq.current = requestSeq;
    const isCurrent = () => mountedRef.current && !signal?.aborted && requestSeq === listRequestSeq.current;
    setLoading(true);
    setError(null);
    try {
      const data = await k8sFetch<K8sList<K8sObject>>(url, { signal });
      if (!isCurrent()) return;
      const nextItems = data.items ?? [];
      const nextTotal = data.total ?? data.items?.length ?? 0;
      const nextPage = clampPageToTotal(query.page, nextTotal, query.pageSize);
      if (nextPage !== query.page) {
        setItems([]);
        setTotal(nextTotal);
        setPodMetrics({});
        setQueries((current) => ({ ...current, [tab]: { ...current[tab], page: nextPage } }));
        return;
      }
      setItems(nextItems);
      setTotal(nextTotal);
      setPodMetrics({});
      if (tab === "Pod") {
        void withLimitedConcurrency(
          nextItems as PodObject[],
          POD_METRIC_CONCURRENCY,
          async (pod) => [podKey(pod), await loadPodMetricCell(clusterId, pod, signal)] as const,
          signal,
        )
          .then((entries) => {
            if (!isCurrent()) return;
            setPodMetrics(Object.fromEntries(entries));
          })
          .catch(() => {
            if (isCurrent()) setPodMetrics({});
          });
      }
    } catch (err) {
      if (!isCurrent()) return;
      setItems([]);
      setTotal(0);
      setError(errorInfo(err, `加载 ${tab} 失败`));
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [clusterId, namespace, query.keyword, query.page, query.pageSize, selectedDef, tab]);

  useEffect(() => {
    const controller = new AbortController();
    void loadNamespaces(controller.signal);
    return () => controller.abort();
  }, [loadNamespaces]);

  useEffect(() => {
    setNamespace("");
    setQueries(initialQueries());
    setSelected([]);
    closeWorkloadModals();
    abortObjectRequest();
    abortPodDetailRequest();
  }, [abortObjectRequest, abortPodDetailRequest, closeWorkloadModals, clusterId]);

  useEffect(() => {
    setSelected([]);
    closeWorkloadModals();
    abortObjectRequest();
    abortPodDetailRequest();
  }, [abortObjectRequest, abortPodDetailRequest, closeWorkloadModals, namespace, tab]);

  useEffect(() => {
    setSelected([]);
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const totalPages = Math.max(1, Math.ceil(total / query.pageSize));

  function rowKey(item: K8sObject, index: number): string {
    return `${item.metadata?.namespace ?? "_"}-${item.metadata?.name ?? index}`;
  }

  function defForObject(item: K8sObject, fallbackDef = selectedDef): ResourceDef | undefined {
    return typeof item.kind === "string" ? defByKind.get(item.kind) ?? fallbackDef : fallbackDef;
  }

  async function fetchLatest(item: K8sObject, def: ResourceDef, signal: AbortSignal): Promise<K8sObject | null> {
    if (!item.metadata?.name) return null;
    return k8sFetch<K8sObject>(proxyPath(clusterId, def, item.metadata.namespace, item.metadata.name), { signal });
  }

  async function openLatest(
    item: K8sObject,
    onLoaded: (latest: K8sObject, def: ResourceDef) => void,
    fallbackMessage: string,
  ) {
    const requestDef = defForObject(item);
    if (!requestDef) return;
    abortObjectRequest();
    const controller = new AbortController();
    activeObjectRequest.current = controller;
    const requestSeq = objectRequestSeq.current + 1;
    objectRequestSeq.current = requestSeq;
    const isCurrent = () =>
      mountedRef.current &&
      !controller.signal.aborted &&
      activeObjectRequest.current === controller &&
      requestSeq === objectRequestSeq.current;
    try {
      const latest = await fetchLatest(item, requestDef, controller.signal);
      if (latest && isCurrent()) onLoaded(latest, defForObject(latest, requestDef) ?? requestDef);
    } catch (err) {
      if (!controller.signal.aborted && isCurrent()) setError(errorInfo(err, fallbackMessage));
    } finally {
      if (activeObjectRequest.current === controller) activeObjectRequest.current = null;
    }
  }

  async function openDetail(item: K8sObject) {
    await openLatest(item, (latest) => setDetail(latest), "加载工作负载详情失败");
  }

  async function openPodDetail(pod: PodObject) {
    const namespace = pod.metadata?.namespace?.trim();
    const name = pod.metadata?.name?.trim();
    const eventsUrl = namespace && name ? podEventsUrl(clusterId, namespace, name) : null;
    if (!podDef || !namespace || !name || !eventsUrl) return;

    abortPodDetailRequest();
    const controller = new AbortController();
    activePodDetailRequest.current = controller;
    const requestSeq = podDetailRequestSeq.current + 1;
    podDetailRequestSeq.current = requestSeq;
    const isCurrent = () =>
      mountedRef.current &&
      !controller.signal.aborted &&
      activePodDetailRequest.current === controller &&
      requestSeq === podDetailRequestSeq.current;

    setError(null);
    setPodDetail({ pod: null, loading: true, events: [], eventsLoading: true, eventsError: null });
    const podRequest = k8sFetch<PodObject>(proxyPath(clusterId, podDef, namespace, name), { signal: controller.signal });
    const eventsRequest = k8sFetch<K8sList<PodEvent>>(eventsUrl, { signal: controller.signal });

    void eventsRequest
      .then((data) => {
        if (!isCurrent()) return;
        setPodDetail((current) => (current ? { ...current, events: data.items ?? [], eventsLoading: false, eventsError: null } : current));
      })
      .catch((err) => {
        if (!isCurrent()) return;
        setPodDetail((current) =>
          current ? { ...current, events: [], eventsLoading: false, eventsError: errorInfo(err, "加载 Pod 事件失败") } : current,
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
      abortPodDetailRequest();
      setPodDetail(null);
      setError(errorInfo(err, "加载 Pod 详情失败"));
    }
  }

  function closePodDetail() {
    abortPodDetailRequest();
    setPodDetail(null);
  }

  async function openYaml(item: K8sObject) {
    await openLatest(item, (latest, def) => setYamlTarget({ def, value: latest }), "加载 YAML 失败");
  }

  async function openEdit(item: K8sObject) {
    await openLatest(item, (latest, def) => setEditTarget({ def, value: latest }), "加载工作负载失败");
  }

  async function saveExisting(next: unknown) {
    const obj = next as K8sObject;
    const target = yamlTarget ?? editTarget;
    const requestDef = target?.def;
    const identityError = target ? validateExistingIdentity(target.value, obj) : "缺少原始资源身份，无法保存。";
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    const name = obj.metadata?.name ?? target?.value.metadata?.name;
    const ns = obj.metadata?.namespace ?? target?.value.metadata?.namespace;
    if (!requestDef || !name) return;
    setSaving(true);
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, requestDef, ns, name), { method: "PUT", body: JSON.stringify(next) });
      setYamlTarget(null);
      setEditTarget(null);
      await load();
    } catch (err) {
      setError(errorInfo(err, "保存工作负载失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function createWorkload(next: unknown) {
    const obj = next as K8sObject;
    const requestDef = createTarget?.def;
    if (!requestDef) return;
    const identityError = validateCreateIdentity(requestDef, obj);
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    const ns = obj.metadata?.namespace?.trim() || namespace;
    if (requestDef.namespaced && !ns) {
      const message = "All Namespaces 下新增工作负载时，请在 metadata.namespace 中指定命名空间。";
      setError({ message });
      throw new Error(message);
    }
    setSaving(true);
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, requestDef, ns), { method: "POST", body: JSON.stringify(next) });
      setCreateTarget(null);
      updateCurrentQuery({ page: 1 });
      await load();
    } catch (err) {
      setError(errorInfo(err, "新增工作负载失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function deleteItem(item: K8sObject) {
    const requestDef = defForObject(item);
    if (!canManage || !requestDef || !item.metadata?.name) return;
    const label = item.metadata.namespace ? `${item.metadata.namespace}/${item.metadata.name}` : item.metadata.name;
    if (!window.confirm(`确定要删除 ${requestDef.kind} ${label} 吗？`)) return;
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, requestDef, item.metadata.namespace, item.metadata.name), { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除工作负载失败"));
    }
  }

  function openCreate(title: string) {
    if (!selectedDef) return;
    setCreateTarget({
      title,
      def: selectedDef,
      value: emptyWorkloadTemplate(tab, namespace) as K8sObject,
    });
  }

  if (!clusterId) {
    return (
      <section className="card p-4">
        <div className="empty-state">请先选择集群。</div>
      </section>
    );
  }

  if (!selectedDef) {
    return (
      <section className="card p-4">
        <div className="empty-state">缺少 {tab} 资源定义。</div>
      </section>
    );
  }

  return (
    <div className="stack-md">
      <section className="card stack-md p-4">
        <ApiErrorBanner error={error} />
        <div className="flex flex-wrap items-end gap-2">
          <label className="min-w-52">
            <span className="label">命名空间</span>
            <select
              className="input"
              value={namespace}
              onChange={(e) => {
                setNamespace(e.target.value);
                setQueries((current) => resetPages(current));
              }}
            >
              <option value="">All Namespaces</option>
              {namespaces.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          <button type="button" className="btn" disabled={!canManage} onClick={() => setCloudOpen(true)}>
            CloudShell
          </button>
          <button type="button" className="btn" onClick={() => setChatOpen(true)}>
            AI 助手
          </button>
        </div>

        <div className="k8s-tabs" role="tablist" aria-label="工作负载类型">
          {WORKLOAD_TABS.map((item) => (
            <button
              key={item}
              type="button"
              role="tab"
              aria-selected={tab === item}
              className={tab === item ? "k8s-tab k8s-tab-active" : "k8s-tab"}
              onClick={() => {
                setTab(item);
                setQueries((current) => ({ ...current, [item]: { ...current[item], page: 1 } }));
              }}
            >
              {tabLabel(item)}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-end gap-2">
          <label>
            <span className="label">名称</span>
            <input
              className="input"
              placeholder="名称"
              value={query.draftKeyword}
              onChange={(e) => updateCurrentQuery({ draftKeyword: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  updateCurrentQuery((current) => ({ ...current, keyword: current.draftKeyword, page: 1 }));
                }
              }}
            />
          </label>
          <button
            type="button"
            className="btn"
            disabled={loading}
            onClick={() => updateCurrentQuery((current) => ({ ...current, keyword: current.draftKeyword, page: 1 }))}
          >
            查询
          </button>
          <button
            type="button"
            className="btn-secondary"
            disabled={loading}
            onClick={() => updateCurrentQuery({ draftKeyword: "", keyword: "", page: 1 })}
          >
            重置
          </button>
          {tab === "Pod" ? null : (
            <button type="button" className="btn-secondary" disabled={!canManage} onClick={() => openCreate(`YAML ${tab}`)}>
              YAML
            </button>
          )}
          <button type="button" className="btn" disabled={!canManage} onClick={() => openCreate(`新增 ${tab}`)}>
            新增
          </button>
        </div>

        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>{tab === "Pod" ? <PodHead /> : <ControllerHead tab={tab} />}</thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={tab === "Pod" ? podColSpan() : controllerColSpan(tab)}>
                    <div className="empty-state">加载中…</div>
                  </td>
                </tr>
              ) : null}
              {!loading &&
                items.map((item, index) =>
                  tab === "Pod" ? (
                    <PodRow
                      key={rowKey(item, index)}
                      clusterId={clusterId}
                      pod={item as PodObject}
                      canManage={canManage}
                      metric={podMetrics[podKey(item as PodObject)]}
                      selected={selected}
                      setSelected={setSelected}
                      onDetail={openPodDetail}
                      onEdit={openEdit}
                      onDeleted={load}
                      onError={setError}
                    />
                  ) : (
                    <ControllerRow
                      key={rowKey(item, index)}
                      item={item}
                      tab={tab}
                      canManage={canManage}
                      selected={selected}
                      setSelected={setSelected}
                      onDetail={openDetail}
                      onYaml={openYaml}
                      onEdit={openEdit}
                      onDelete={deleteItem}
                    />
                  ),
                )}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={tab === "Pod" ? podColSpan() : controllerColSpan(tab)}>
                    <div className="empty-state">暂无数据</div>
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <span className="text-muted">共 {total} 条</span>
          <div className="flex items-center gap-2">
            <select
              className="input w-auto"
              value={query.pageSize}
              onChange={(e) => updateCurrentQuery({ pageSize: Number(e.target.value), page: 1 })}
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size}条/页
                </option>
              ))}
            </select>
            <button
              type="button"
              className="btn-secondary text-sm"
              disabled={query.page <= 1 || loading}
              onClick={() => updateCurrentQuery((current) => ({ ...current, page: Math.max(1, current.page - 1) }))}
            >
              上一页
            </button>
            <span>
              {query.page} / {totalPages}
            </span>
            <button
              type="button"
              className="btn-secondary text-sm"
              disabled={query.page >= totalPages || loading}
              onClick={() => updateCurrentQuery((current) => ({ ...current, page: current.page + 1 }))}
            >
              下一页
            </button>
          </div>
        </div>
      </section>

      <Modal open={Boolean(detail)} title={`详情 ${detail?.metadata?.name ?? ""}`} size="xl" onClose={() => setDetail(null)}>
        {detail ? <pre className="k8s-yaml-panel font-mono text-xs whitespace-pre-wrap">{JSON.stringify(detail, null, 2)}</pre> : null}
      </Modal>
      <Modal open={Boolean(podDetail)} title="详情" size="xl" onClose={closePodDetail}>
        {podDetail?.loading || !podDetail?.pod ? (
          <div className="empty-state">Pod 详情加载中…</div>
        ) : (
          <PodDetailView
            pod={podDetail.pod}
            events={podDetail.events}
            eventsError={podDetail.eventsError}
            eventsLoading={podDetail.eventsLoading}
          />
        )}
      </Modal>
      <Modal open={Boolean(yamlTarget)} title={`YAML ${yamlTarget?.value.metadata?.name ?? ""}`} size="xl" onClose={() => setYamlTarget(null)}>
        {yamlTarget ? (
          <YamlEditor
            title={`${yamlTarget.def.kind} ${qualifiedName(yamlTarget.value)}`}
            value={yamlTarget.value}
            canSave={canManage}
            saving={saving}
            onSave={saveExisting}
            onClose={() => setYamlTarget(null)}
          />
        ) : null}
      </Modal>
      <Modal open={Boolean(editTarget)} title={`编辑 ${editTarget?.value.metadata?.name ?? ""}`} size="xl" onClose={() => setEditTarget(null)}>
        {editTarget ? (
          <YamlEditor
            title={`${editTarget.def.kind} ${qualifiedName(editTarget.value)}`}
            value={editTarget.value}
            canSave={canManage}
            saving={saving}
            onSave={saveExisting}
            onClose={() => setEditTarget(null)}
          />
        ) : null}
      </Modal>
      <Modal open={Boolean(createTarget)} title={createTarget?.title ?? "新增"} size="xl" onClose={() => setCreateTarget(null)}>
        {createTarget ? (
          <YamlEditor
            title={createTarget.title}
            description={namespace ? `将在命名空间 ${namespace} 中创建。` : "All Namespaces 下请在 metadata.namespace 中指定命名空间。"}
            value={createTarget.value}
            canSave={canManage}
            saving={saving}
            onSave={createWorkload}
            onClose={() => setCreateTarget(null)}
          />
        ) : null}
      </Modal>
      <Modal nested open={cloudOpen} title="CloudShell" size="xl" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal nested open={chatOpen} title="AI 助手" size="xl" onClose={() => setChatOpen(false)}>
        <ChatPage clusterId={clusterId} canManage={canManage} />
      </Modal>
    </div>
  );
}

function resetPages(queries: Record<WorkloadTab, QueryState>): Record<WorkloadTab, QueryState> {
  return Object.fromEntries(Object.entries(queries).map(([key, value]) => [key, { ...value, page: 1 }])) as Record<WorkloadTab, QueryState>;
}

function tabLabel(tab: WorkloadTab): string {
  if (tab === "Pod") return "Pods";
  return `${tab}s`;
}

function controllerColSpan(tab: WorkloadTab): number {
  if (tab === "DaemonSet") return 8;
  if (tab === "CronJob") return 8;
  if (tab === "Job") return 7;
  return 6;
}

function podColSpan(): number {
  return 15;
}

function qualifiedName(item: K8sObject): string {
  return item.metadata?.namespace ? `${item.metadata.namespace}/${item.metadata.name ?? ""}` : item.metadata?.name ?? "";
}

function ControllerHead({ tab }: { tab: WorkloadTab }) {
  return (
    <tr>
      <th>
        <span className="sr-only">选择</span>
      </th>
      <th>名称</th>
      <th>命名空间</th>
      {tab === "Deployment" || tab === "StatefulSet" ? <th>容器组数量</th> : null}
      {tab === "DaemonSet" ? (
        <>
          <th>准备就绪</th>
          <th>当前调度</th>
          <th>期望调度</th>
        </>
      ) : null}
      {tab === "Job" ? (
        <>
          <th>状态</th>
          <th>耗时</th>
        </>
      ) : null}
      {tab === "CronJob" ? (
        <>
          <th>调度</th>
          <th>最后调度时间</th>
          <th>暂停</th>
        </>
      ) : null}
      <th>存活时间</th>
      <th>操作</th>
    </tr>
  );
}

function ControllerRow({
  item,
  tab,
  canManage,
  selected,
  setSelected,
  onDetail,
  onYaml,
  onEdit,
  onDelete,
}: {
  item: K8sObject;
  tab: WorkloadTab;
  canManage: boolean;
  selected: string[];
  setSelected: Dispatch<SetStateAction<string[]>>;
  onDetail: (item: K8sObject) => Promise<void>;
  onYaml: (item: K8sObject) => Promise<void>;
  onEdit: (item: K8sObject) => Promise<void>;
  onDelete: (item: K8sObject) => Promise<void>;
}) {
  const key = qualifiedName(item);
  return (
    <tr>
      <td>
        <input
          type="checkbox"
          checked={selected.includes(key)}
          onChange={(e) => setSelected((current) => (e.target.checked ? [...current, key] : current.filter((value) => value !== key)))}
        />
      </td>
      <td>
        <button type="button" className="k8s-action-link font-semibold" onClick={() => void onDetail(item)}>
          {item.metadata?.name ?? "-"}
        </button>
        <LabelsHint labels={item.metadata?.labels} />
      </td>
      <td>{item.metadata?.namespace ?? "-"}</td>
      <ControllerSpecificCells item={item} tab={tab} />
      <td>{formatNodeAge(item.metadata?.creationTimestamp)}</td>
      <td className="whitespace-nowrap">
        <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void onYaml(item)}>
          YAML
        </button>
        <span className="text-muted"> / </span>
        <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void onEdit(item)}>
          编辑
        </button>
        <span className="text-muted"> / </span>
        <button type="button" className="k8s-action-link is-danger" disabled={!canManage} onClick={() => void onDelete(item)}>
          删除
        </button>
      </td>
    </tr>
  );
}

function ControllerSpecificCells({ item, tab }: { item: K8sObject; tab: WorkloadTab }) {
  if (tab === "Deployment" || tab === "StatefulSet") {
    const badge = replicaBadge(Number(item.status?.readyReplicas ?? 0), Number(item.spec?.replicas ?? 0));
    return (
      <td>
        <span className={badge.tone === "ok" ? "k8s-badge-ok" : "k8s-badge-bad"}>{badge.text}</span>
      </td>
    );
  }
  if (tab === "DaemonSet") {
    const counts = daemonSetCounts(item);
    return (
      <>
        <td>{counts.ready}</td>
        <td>{counts.current}</td>
        <td>{counts.desired}</td>
      </>
    );
  }
  if (tab === "Job") {
    const status = jobStatus(item);
    return (
      <>
        <td>
          <div className="k8s-workload-badges">
            <span className={status.active > 0 ? "k8s-badge-ok" : "badge badge-neutral"}>active {status.active}</span>
            <span className={status.succeeded > 0 ? "k8s-badge-ok" : "badge badge-neutral"}>succeeded {status.succeeded}</span>
            <span className={status.failed > 0 ? "k8s-badge-bad" : "badge badge-neutral"}>failed {status.failed}</span>
          </div>
        </td>
        <td>{status.duration}</td>
      </>
    );
  }
  if (tab === "CronJob") {
    const view = cronJobView(item);
    return (
      <>
        <td className="font-mono text-xs">{view.schedule}</td>
        <td>{view.lastSchedule}</td>
        <td>
          <span className={view.suspend ? "k8s-badge-bad" : "k8s-badge-ok"}>{view.suspend ? "是" : "否"}</span>
        </td>
      </>
    );
  }
  return null;
}

function PodHead() {
  return (
    <tr>
      <th>
        <span className="sr-only">选择</span>
      </th>
      <th>名称</th>
      <th>CPU 申请</th>
      <th>CPU 限制</th>
      <th>内存申请</th>
      <th>内存限制</th>
      <th>容器</th>
      <th>CPU/内存/TCP</th>
      <th>状态</th>
      <th>重启次数</th>
      <th>命名空间</th>
      <th>Pod IP</th>
      <th>调度节点</th>
      <th>存活时间</th>
      <th>操作</th>
    </tr>
  );
}

function PodRow({
  clusterId,
  pod,
  canManage,
  metric,
  selected,
  setSelected,
  onDetail,
  onEdit,
  onDeleted,
  onError,
}: {
  clusterId: string;
  pod: PodObject;
  canManage: boolean;
  metric?: string;
  selected: string[];
  setSelected: Dispatch<SetStateAction<string[]>>;
  onDetail: (pod: PodObject) => Promise<void>;
  onEdit: (item: K8sObject) => Promise<void>;
  onDeleted: () => Promise<void>;
  onError: (error: ApiErrorInfo | null) => void;
}) {
  const key = podKey(pod);
  const badge = podPhaseBadge(pod.status?.phase);
  const resources = podResourceSummary(pod);
  const totals = sumPodResources([pod]);
  const readiness = containerReadinessDots(pod);
  return (
    <tr>
      <td>
        <input
          type="checkbox"
          checked={selected.includes(key)}
          onChange={(e) => setSelected((current) => (e.target.checked ? [...current, key] : current.filter((value) => value !== key)))}
        />
      </td>
      <td>
        <span className="inline-flex items-center gap-1.5">
          <svg className="h-3.5 w-3.5 text-sky-500" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
            <path d="M8 1 2 4v5c0 3.1 2.2 5.7 6 6 3.8-.3 6-2.9 6-6V4L8 1Zm0 2.2 3.8 1.9V9c0 2-1.3 3.6-3.8 3.9C5.5 12.6 4.2 11 4.2 9V5.1L8 3.2Z" />
          </svg>
          <button type="button" className="k8s-action-link font-semibold" onClick={() => void onDetail(pod)}>
            {pod.metadata?.name ?? "-"}
          </button>
          <LabelsHint labels={pod.metadata?.labels} />
        </span>
      </td>
      <td className={totals.cpuLim > 0 && totals.cpuReq > totals.cpuLim ? "text-red-600" : undefined} title={resources.cpu}>
        {formatCores(totals.cpuReq)} 核
      </td>
      <td title={resources.cpu}>{formatCores(totals.cpuLim)} 核</td>
      <td className={totals.memLim > 0 && totals.memReq > totals.memLim ? "text-red-600" : undefined} title={resources.memory}>
        {formatGi(totals.memReq)} Gi
      </td>
      <td title={resources.memory}>{formatGi(totals.memLim)} Gi</td>
      <td>
        <div className="flex flex-wrap gap-1">
          {readiness.map((dot) => (
            <span
              key={dot.name}
              title={dot.title}
              className={
                dot.tone === "ready"
                  ? "k8s-container-dot is-ready"
                  : dot.tone === "bad"
                    ? "k8s-container-dot is-bad"
                    : "k8s-container-dot"
              }
            />
          ))}
          {readiness.length === 0 ? "-" : null}
        </div>
      </td>
      <td className="font-mono text-xs whitespace-nowrap">{metric ?? "-"}</td>
      <td>
        <span className={badge.tone === "ok" ? "k8s-badge-ok" : badge.tone === "bad" ? "k8s-badge-bad" : "badge badge-neutral"}>
          {badge.text}
        </span>
      </td>
      <td>{podRestartCount(pod)}</td>
      <td>{pod.metadata?.namespace ?? "-"}</td>
      <td className="font-mono text-xs">{pod.status?.podIP ?? "-"}</td>
      <td>{pod.spec?.nodeName ?? "-"}</td>
      <td>{formatNodeAge(pod.metadata?.creationTimestamp)}</td>
      <td className="whitespace-nowrap">
        <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void onEdit(pod)}>
          编辑
        </button>
        <span className="text-muted"> / </span>
        <WorkloadPodActions
          clusterId={clusterId}
          pod={pod}
          canManage={canManage}
          onOpenDetail={(target) => void onDetail(target)}
          onDeleted={onDeleted}
          onError={onError}
        />
      </td>
    </tr>
  );
}

function podKey(pod: PodObject): string {
  return `${pod.metadata?.namespace ?? "_"}/${pod.metadata?.name ?? ""}`;
}

function LabelsHint({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {});
  if (entries.length === 0) return null;
  return (
    <span className="k8s-label-hint" title={entries.map(([key, value]) => `${key}: ${value}`).join("\n")}>
      ▸
    </span>
  );
}
