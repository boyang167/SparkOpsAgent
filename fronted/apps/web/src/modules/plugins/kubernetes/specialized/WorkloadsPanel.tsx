"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sResourceTabs,
  K8sStatusBadge,
  useK8sConfirm,
  type K8sStatusTone,
} from "@/components/k8s";
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
const POD_METRIC_CONCURRENCY = 6;

const initialQuery = (): QueryState => ({
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
  const { ask, dialog: confirmDialog } = useK8sConfirm();

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
    const ok = await ask({
      title: "删除工作负载",
      description: `确定要删除 ${requestDef.kind} ${label} 吗？此操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
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
      <K8sPanelCard>
        <K8sEmptyState variant="no-cluster" />
      </K8sPanelCard>
    );
  }

  if (!selectedDef) {
    return (
      <K8sPanelCard>
        <K8sEmptyState variant="missing-def" kind={tab} />
      </K8sPanelCard>
    );
  }

  const hasActiveFilters = query.keyword.trim().length > 0 || namespace.length > 0;

  return (
    <div className="stack-md">
      <K8sPanelCard className="k8s-workloads-panel">
        <div className="k8s-clusters-toolbar k8s-workloads-toolbar">
          <div className="k8s-clusters-toolbar-filters">
            <label className="k8s-workloads-namespace">
              <span className="sr-only">命名空间</span>
              <select
                className="input"
                value={namespace}
                aria-label="命名空间"
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
            <label className="projects-search-wrap">
              <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M9 4a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z" stroke="currentColor" strokeWidth="1.5" />
                <path d="M13 13l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
              <input
                className="projects-search-input"
                value={query.keyword}
                placeholder={`搜索${tabLabel(tab)}`}
                aria-label="搜索名称"
                onChange={(e) => updateCurrentQuery({ keyword: e.target.value, page: 1 })}
              />
            </label>
            {hasActiveFilters ? (
              <button
                type="button"
                className="btn-secondary projects-reset-btn"
                onClick={() => {
                  setNamespace("");
                  updateCurrentQuery({ keyword: "", page: 1 });
                }}
              >
                重置
              </button>
            ) : null}
          </div>
          <div className="k8s-toolbar-actions">
            {!loading && total > 0 ? (
              <span className="k8s-clusters-count" role="status">
                共 {total} 个{tabLabel(tab)}
              </span>
            ) : null}
            <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={() => setCloudOpen(true)}>
              CloudShell
            </button>
            <button type="button" className="btn-secondary text-sm" onClick={() => setChatOpen(true)}>
              AI 助手
            </button>
            {tab === "Pod" ? null : (
              <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={() => openCreate(`YAML ${tab}`)}>
                YAML 创建
              </button>
            )}
            <button type="button" className="btn text-sm" disabled={!canManage} onClick={() => openCreate(`新增 ${tab}`)}>
              新增
            </button>
            <button
              type="button"
              className="btn-secondary k8s-clusters-refresh-btn"
              disabled={loading}
              aria-label="刷新列表"
              onClick={() => void load()}
            >
              ↻
            </button>
          </div>
        </div>

        <K8sResourceTabs
          tabs={WORKLOAD_TABS}
          active={tab}
          ariaLabel="工作负载类型"
          label={tabLabel}
          onChange={(item) => {
            setTab(item);
            setQueries((current) => ({ ...current, [item]: { ...current[item], page: 1 } }));
          }}
        />

        <ApiErrorBanner error={error} />

        <div className="table-wrap k8s-workloads-table">
          <table className="data-table text-sm">
            <thead>{tab === "Pod" ? <PodHead /> : <ControllerHead tab={tab} />}</thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={tab === "Pod" ? podColSpan() : controllerColSpan(tab)}>
                    <K8sLoadingState compact />
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
                    <K8sEmptyState
                      variant="empty"
                      compact
                      title={`暂无${tabLabel(tab)}`}
                      description={hasActiveFilters ? "没有符合筛选条件的资源，可调整命名空间或搜索词。" : "当前命名空间下没有该类型的工作负载。"}
                    />
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <K8sPagination
          page={query.page}
          pageSize={query.pageSize}
          total={total}
          loading={loading}
          onPageChange={(page) => updateCurrentQuery({ page })}
          onPageSizeChange={(pageSize) => updateCurrentQuery({ pageSize, page: 1 })}
        />
      </K8sPanelCard>

      <Modal
        open={Boolean(detail)}
        title={detail?.metadata?.name ?? "详情"}
        description={detail ? `${tab} · ${detail.metadata?.namespace ?? "-"}` : undefined}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setDetail(null)}
      >
        {detail ? (
          <YamlEditor value={detail} canSave={false} onSave={async () => undefined} onClose={() => setDetail(null)} cancelLabel="关闭" />
        ) : null}
      </Modal>
      <Modal
        open={Boolean(podDetail)}
        title={podDetail?.pod?.metadata?.name ?? "Pod 详情"}
        description={
          podDetail?.pod
            ? `${podDetail.pod.metadata?.namespace ?? "-"} · ${podDetail.pod.status?.phase ?? "-"}`
            : undefined
        }
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
      <Modal
        open={Boolean(yamlTarget)}
        title={yamlTarget?.value.metadata?.name ?? "YAML"}
        description={yamlTarget ? `${yamlTarget.def.kind} · ${qualifiedName(yamlTarget.value)}` : undefined}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setYamlTarget(null)}
      >
        {yamlTarget ? (
          <YamlEditor
            value={yamlTarget.value}
            canSave={canManage}
            saving={saving}
            onSave={saveExisting}
            onClose={() => setYamlTarget(null)}
          />
        ) : null}
      </Modal>
      <Modal
        open={Boolean(editTarget)}
        title={editTarget?.value.metadata?.name ?? "编辑"}
        description={editTarget ? `${editTarget.def.kind} · ${qualifiedName(editTarget.value)}` : undefined}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setEditTarget(null)}
      >
        {editTarget ? (
          <YamlEditor
            value={editTarget.value}
            canSave={canManage}
            saving={saving}
            onSave={saveExisting}
            onClose={() => setEditTarget(null)}
          />
        ) : null}
      </Modal>
      <Modal
        open={Boolean(createTarget)}
        title={createTarget?.title ?? "新增"}
        description={createTarget ? createTarget.def.kind : undefined}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setCreateTarget(null)}
      >
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
      <Modal nested open={cloudOpen} title="CloudShell" size="xl" panelClassName="k8s-pod-submodal" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      <Modal nested open={chatOpen} title="AI 助手" size="xl" panelClassName="k8s-pod-submodal" onClose={() => setChatOpen(false)}>
        <ChatPage clusterId={clusterId} canManage={canManage} />
      </Modal>
      {confirmDialog}
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
      <td>
        <div className="k8s-row-actions">
          <button type="button" className="btn-ghost text-sm" disabled={!canManage} onClick={() => void onYaml(item)}>
            YAML
          </button>
          <button type="button" className="btn-ghost text-sm" disabled={!canManage} onClick={() => void onEdit(item)}>
            编辑
          </button>
          <button type="button" className="btn-ghost text-sm is-danger" disabled={!canManage} onClick={() => void onDelete(item)}>
            删除
          </button>
        </div>
      </td>
    </tr>
  );
}

function ControllerSpecificCells({ item, tab }: { item: K8sObject; tab: WorkloadTab }) {
  if (tab === "Deployment" || tab === "StatefulSet") {
    const badge = replicaBadge(Number(item.status?.readyReplicas ?? 0), Number(item.spec?.replicas ?? 0));
    return (
      <td>
        <K8sStatusBadge tone={workloadBadgeTone(badge.tone)} dot>
          {badge.text}
        </K8sStatusBadge>
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
            <K8sStatusBadge tone={status.active > 0 ? "info" : "neutral"}>active {status.active}</K8sStatusBadge>
            <K8sStatusBadge tone={status.succeeded > 0 ? "success" : "neutral"}>succeeded {status.succeeded}</K8sStatusBadge>
            <K8sStatusBadge tone={status.failed > 0 ? "danger" : "neutral"}>failed {status.failed}</K8sStatusBadge>
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
          <K8sStatusBadge tone={view.suspend ? "warning" : "success"} dot>
            {view.suspend ? "已暂停" : "运行中"}
          </K8sStatusBadge>
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
          <button type="button" className="k8s-action-link font-semibold" onClick={() => void onDetail(pod)}>
            {pod.metadata?.name ?? "-"}
          </button>
          <LabelsHint labels={pod.metadata?.labels} />
        </span>
      </td>
      <td className={totals.cpuLim > 0 && totals.cpuReq > totals.cpuLim ? "k8s-overcommit" : undefined} title={resources.cpu}>
        {formatCores(totals.cpuReq)} 核
      </td>
      <td title={resources.cpu}>{formatCores(totals.cpuLim)} 核</td>
      <td className={totals.memLim > 0 && totals.memReq > totals.memLim ? "k8s-overcommit" : undefined} title={resources.memory}>
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
        <K8sStatusBadge tone={workloadBadgeTone(badge.tone)} dot>
          {badge.text}
        </K8sStatusBadge>
      </td>
      <td>{podRestartCount(pod)}</td>
      <td>{pod.metadata?.namespace ?? "-"}</td>
      <td className="font-mono text-xs">{pod.status?.podIP ?? "-"}</td>
      <td>{pod.spec?.nodeName ?? "-"}</td>
      <td>{formatNodeAge(pod.metadata?.creationTimestamp)}</td>
      <td>
        <div className="k8s-row-actions">
          <button type="button" className="btn-ghost text-sm" disabled={!canManage} onClick={() => void onEdit(pod)}>
            编辑
          </button>
          <WorkloadPodActions
            clusterId={clusterId}
            pod={pod}
            canManage={canManage}
            trigger="ghost"
            onOpenDetail={(target) => void onDetail(target)}
            onDeleted={onDeleted}
            onError={onError}
          />
        </div>
      </td>
    </tr>
  );
}

function podKey(pod: PodObject): string {
  return `${pod.metadata?.namespace ?? "_"}/${pod.metadata?.name ?? ""}`;
}

function workloadBadgeTone(tone: "ok" | "bad" | "muted"): K8sStatusTone {
  if (tone === "ok") return "success";
  if (tone === "bad") return "danger";
  return "neutral";
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
