"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sResourceTabs,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath, type ResourceDef } from "../catalog";
import { formatNodeAge } from "../node-view";
import {
  NETWORK_TABS,
  emptyNetworkTemplate,
  formatEndpointSubsets,
  formatExternalIPs,
  formatIngressRules,
  formatLoadBalancerIngress,
  formatSelector,
  formatServiceExternalEndpoints,
  formatServiceInternalEndpoints,
  networkTabLabel,
  networkTableColSpan,
  type NetworkTab,
} from "../network-view";
import { resourceCollectionUrl } from "../resource-url";
import { clampPageToTotal, validateCreateIdentity, validateExistingIdentity } from "../workload-view";
import { YamlEditor } from "../YamlEditor";

type K8sObject = {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
  subsets?: unknown[];
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

type NetworkTarget = {
  def: ResourceDef;
  value: K8sObject;
};

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");
const initialQuery = (): QueryState => ({
  draftKeyword: "",
  keyword: "",
  page: 1,
  pageSize: 10,
});

const initialQueries = (): Record<NetworkTab, QueryState> =>
  Object.fromEntries(NETWORK_TABS.map((item) => [item, initialQuery()])) as Record<NetworkTab, QueryState>;

export function NetworksPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const mountedRef = useRef(false);
  const namespaceRequestSeq = useRef(0);
  const listRequestSeq = useRef(0);
  const objectRequestSeq = useRef(0);
  const activeObjectRequest = useRef<AbortController | null>(null);

  const [namespace, setNamespace] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [tab, setTab] = useState<NetworkTab>("Service");
  const [queries, setQueries] = useState<Record<NetworkTab, QueryState>>(() => initialQueries());
  const [items, setItems] = useState<K8sObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [detail, setDetail] = useState<K8sObject | null>(null);
  const [yamlTarget, setYamlTarget] = useState<NetworkTarget | null>(null);
  const [editTarget, setEditTarget] = useState<NetworkTarget | null>(null);
  const [createTarget, setCreateTarget] = useState<{ title: string; def: ResourceDef; value: K8sObject } | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const defByKind = useMemo(
    () => new Map(CATALOG.filter((item) => NETWORK_TABS.includes(item.kind as NetworkTab)).map((item) => [item.kind, item])),
    [],
  );
  const selectedDef = defByKind.get(tab);
  const query = queries[tab];
  const colSpan = networkTableColSpan(tab);

  const abortObjectRequest = useCallback(() => {
    activeObjectRequest.current?.abort();
    activeObjectRequest.current = null;
    objectRequestSeq.current += 1;
  }, []);

  const closeModals = useCallback(() => {
    setDetail(null);
    setYamlTarget(null);
    setEditTarget(null);
    setCreateTarget(null);
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      abortObjectRequest();
    };
  }, [abortObjectRequest]);

  const updateCurrentQuery = useCallback(
    (patch: Partial<QueryState> | ((current: QueryState) => QueryState)) => {
      setQueries((current) => ({
        ...current,
        [tab]: typeof patch === "function" ? patch(current[tab]) : { ...current[tab], ...patch },
      }));
    },
    [tab],
  );

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
        setQueries((current) => ({ ...current, [tab]: { ...current[tab], page: nextPage } }));
        return;
      }
      setItems(nextItems);
      setTotal(nextTotal);
    } catch (err) {
      if (!isCurrent()) return;
      setItems([]);
      setTotal(0);
      setError(errorInfo(err, `加载 ${networkTabLabel(tab)} 失败`));
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
    closeModals();
    abortObjectRequest();
  }, [abortObjectRequest, closeModals, clusterId]);

  useEffect(() => {
    setSelected([]);
    closeModals();
    abortObjectRequest();
  }, [abortObjectRequest, closeModals, namespace, tab]);

  useEffect(() => {
    setSelected([]);
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  function rowKey(item: K8sObject, index: number): string {
    return `${item.metadata?.namespace ?? "_"}-${item.metadata?.name ?? index}`;
  }

  function qualifiedName(item: K8sObject): string {
    return item.metadata?.namespace ? `${item.metadata.namespace}/${item.metadata.name}` : (item.metadata?.name ?? "");
  }

  async function fetchLatest(item: K8sObject, def: ResourceDef, signal: AbortSignal): Promise<K8sObject | null> {
    if (!item.metadata?.name) return null;
    return k8sFetch<K8sObject>(proxyPath(clusterId, def, item.metadata.namespace, item.metadata.name), { signal });
  }

  async function openLatest(item: K8sObject, onLoaded: (latest: K8sObject, def: ResourceDef) => void, fallbackMessage: string) {
    const requestDef = defByKind.get(typeof item.kind === "string" ? item.kind : tab) ?? selectedDef;
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
      if (latest && isCurrent()) onLoaded(latest, requestDef);
    } catch (err) {
      if (!controller.signal.aborted && isCurrent()) setError(errorInfo(err, fallbackMessage));
    } finally {
      if (activeObjectRequest.current === controller) activeObjectRequest.current = null;
    }
  }

  async function openDetail(item: K8sObject) {
    await openLatest(item, (latest) => setDetail(latest), "加载详情失败");
  }

  async function openYaml(item: K8sObject) {
    await openLatest(item, (latest, def) => setYamlTarget({ def, value: latest }), "加载 YAML 失败");
  }

  async function openEdit(item: K8sObject) {
    await openLatest(item, (latest, def) => setEditTarget({ def, value: latest }), "加载资源失败");
  }

  async function saveExisting(next: unknown) {
    const target = yamlTarget ?? editTarget;
    if (!target?.value.metadata?.name) return;
    const identityError = validateExistingIdentity(target.value, next as Record<string, unknown>);
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    setSaving(true);
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, target.def, target.value.metadata.namespace, target.value.metadata.name), {
        method: "PUT",
        body: JSON.stringify(next),
      });
      setYamlTarget(null);
      setEditTarget(null);
      await load();
    } catch (err) {
      setError(errorInfo(err, "保存资源失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function createResource(next: unknown) {
    const requestDef = createTarget?.def;
    if (!requestDef) return;
    const obj = next as K8sObject;
    const identityError = validateCreateIdentity(requestDef, obj as Record<string, unknown>);
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    const ns = obj.metadata?.namespace?.trim() || namespace;
    if (requestDef.namespaced && !ns) {
      const message = "All Namespaces 下新增资源时，请在 metadata.namespace 中指定命名空间。";
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
      setError(errorInfo(err, "新增资源失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function deleteItem(item: K8sObject) {
    const requestDef = defByKind.get(typeof item.kind === "string" ? item.kind : tab) ?? selectedDef;
    if (!canManage || !requestDef || !item.metadata?.name) return;
    const label = item.metadata.namespace ? `${item.metadata.namespace}/${item.metadata.name}` : item.metadata.name;
    const ok = await ask({
      title: "删除资源",
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
      setError(errorInfo(err, "删除资源失败"));
    }
  }

  function openCreate(title: string) {
    if (!selectedDef) return;
    setCreateTarget({
      title,
      def: selectedDef,
      value: emptyNetworkTemplate(tab, namespace) as K8sObject,
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

  return (
    <div className="stack-md">
      <K8sPanelCard>
        <ApiErrorBanner error={error} />

        <div className="k8s-toolbar-row">
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
        </div>

        <K8sResourceTabs
          tabs={NETWORK_TABS}
          active={tab}
          ariaLabel="网络资源类型"
          label={networkTabLabel}
          onChange={(item) => {
            setTab(item);
            setQueries((current) => ({ ...current, [item]: { ...current[item], page: 1 } }));
          }}
        />

        <div className="k8s-toolbar-row">
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
          {tab === "Endpoints" ? null : (
            <button type="button" className="btn-secondary" disabled={!canManage} onClick={() => openCreate(`YAML ${tab}`)}>
              YAML
            </button>
          )}
          <button type="button" className="btn" disabled={!canManage} onClick={() => openCreate(`新增 ${networkTabLabel(tab)}`)}>
            新增
          </button>
        </div>

        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <NetworkTableHead tab={tab} />
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={colSpan}>
                    <K8sLoadingState compact />
                  </td>
                </tr>
              ) : null}
              {!loading &&
                items.map((item, index) => (
                  <NetworkTableRow
                    key={rowKey(item, index)}
                    tab={tab}
                    item={item}
                    canManage={canManage}
                    selected={selected}
                    setSelected={setSelected}
                    onDetail={openDetail}
                    onYaml={openYaml}
                    onEdit={openEdit}
                    onDelete={deleteItem}
                  />
                ))}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={colSpan}>
                    <K8sEmptyState variant="empty" compact />
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

      <Modal open={Boolean(detail)} title={`详情 ${detail?.metadata?.name ?? ""}`} size="xl" onClose={() => setDetail(null)}>
        {detail ? <pre className="k8s-yaml-panel font-mono text-xs whitespace-pre-wrap">{JSON.stringify(detail, null, 2)}</pre> : null}
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
            onSave={createResource}
            onClose={() => setCreateTarget(null)}
          />
        ) : null}
      </Modal>
      {confirmDialog}
    </div>
  );
}

function resetPages(queries: Record<NetworkTab, QueryState>): Record<NetworkTab, QueryState> {
  return Object.fromEntries(Object.entries(queries).map(([key, value]) => [key, { ...value, page: 1 }])) as Record<
    NetworkTab,
    QueryState
  >;
}

function NetworkTableHead({ tab }: { tab: NetworkTab }) {
  return (
    <tr>
      <th className="w-10">
        <span className="sr-only">选择</span>
      </th>
      <th>名称</th>
      <th>命名空间</th>
      {tab === "Service" ? (
        <>
          <th>类型</th>
          <th>ClusterIP</th>
          <th>externalIPs</th>
          <th>内部端点</th>
          <th>外部端点</th>
          <th>选择器</th>
        </>
      ) : null}
      {tab === "Ingress" ? (
        <>
          <th>端点</th>
          <th>规则</th>
        </>
      ) : null}
      {tab === "Endpoints" ? <th>Endpoints</th> : null}
      <th>存活时间</th>
      <th>操作</th>
    </tr>
  );
}

function NetworkTableRow({
  tab,
  item,
  canManage,
  selected,
  setSelected,
  onDetail,
  onYaml,
  onEdit,
  onDelete,
}: {
  tab: NetworkTab;
  item: K8sObject;
  canManage: boolean;
  selected: string[];
  setSelected: Dispatch<SetStateAction<string[]>>;
  onDetail: (item: K8sObject) => Promise<void>;
  onYaml: (item: K8sObject) => Promise<void>;
  onEdit: (item: K8sObject) => Promise<void>;
  onDelete: (item: K8sObject) => Promise<void>;
}) {
  const key = item.metadata?.namespace ? `${item.metadata.namespace}/${item.metadata.name}` : (item.metadata?.name ?? "");
  const name = item.metadata?.name ?? "-";
  const spec = item.spec ?? {};
  const status = item.status ?? {};

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
          {name}
        </button>
        <LabelsHint labels={item.metadata?.labels} />
      </td>
      <td>{item.metadata?.namespace ?? "-"}</td>
      {tab === "Service" ? <ServiceCells spec={spec} status={status} name={name} /> : null}
      {tab === "Ingress" ? <IngressCells spec={spec} status={status} /> : null}
      {tab === "Endpoints" ? (
        <td>
          <LineList lines={formatEndpointSubsets(item.subsets as Parameters<typeof formatEndpointSubsets>[0])} />
        </td>
      ) : null}
      <td>{formatNodeAge(item.metadata?.creationTimestamp)}</td>
      <td className="whitespace-nowrap">
        {tab === "Endpoints" ? null : (
          <>
            <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void onYaml(item)}>
              YAML
            </button>
            <span className="text-muted"> / </span>
          </>
        )}
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

function ServiceCells({
  spec,
  status,
  name,
}: {
  spec: Record<string, unknown>;
  status: Record<string, unknown>;
  name: string;
}) {
  const ports = spec.ports as Parameters<typeof formatServiceInternalEndpoints>[1];
  const loadBalancer = (status.loadBalancer as { ingress?: Parameters<typeof formatServiceExternalEndpoints>[1] })?.ingress;
  const internalLines = formatServiceInternalEndpoints(name, ports);
  const externalLines = formatServiceExternalEndpoints(ports, loadBalancer);
  const selectors = formatSelector(spec.selector as Record<string, string> | undefined);

  return (
    <>
      <td>{String(spec.type ?? "-")}</td>
      <td>{String(spec.clusterIP ?? "-")}</td>
      <td>{formatExternalIPs(spec.externalIPs as string[] | undefined)}</td>
      <td>
        <LineList lines={internalLines} />
      </td>
      <td>
        <LineList lines={externalLines} fallback="-" />
      </td>
      <td>
        <SelectorChips items={selectors} />
      </td>
    </>
  );
}

function IngressCells({ spec, status }: { spec: Record<string, unknown>; status: Record<string, unknown> }) {
  const loadBalancer = (status.loadBalancer as { ingress?: Parameters<typeof formatLoadBalancerIngress>[0] })?.ingress;
  const endpointLines = formatLoadBalancerIngress(loadBalancer);
  const ruleLines = formatIngressRules(spec.rules as Parameters<typeof formatIngressRules>[0]);

  return (
    <>
      <td>
        <LineList lines={endpointLines} />
      </td>
      <td>
        <LineList lines={ruleLines} />
      </td>
    </>
  );
}

function LineList({ lines, fallback = "-" }: { lines: string[]; fallback?: string }) {
  if (lines.length === 0) return <>{fallback}</>;
  return (
    <div className="k8s-endpoint-lines">
      {lines.map((line) => (
        <div key={line}>{line}</div>
      ))}
    </div>
  );
}

function SelectorChips({ items }: { items: string[] }) {
  if (items.length === 0) return <>-</>;
  return (
    <div className="k8s-selector-list">
      {items.map((item) => (
        <span key={item} className="k8s-selector-chip">
          {item}
        </span>
      ))}
    </div>
  );
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
