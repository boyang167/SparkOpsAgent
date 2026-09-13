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
import {
  CONFIG_TABS,
  configTabLabel,
  configTableColSpan,
  configTabsForUser,
  emptyConfigTemplate,
  secretEditableType,
  type ConfigTab,
} from "../config-view";
import { formatNodeAge } from "../node-view";
import { resourceCollectionUrl } from "../resource-url";
import { clampPageToTotal, validateCreateIdentity, validateExistingIdentity } from "../workload-view";
import { YamlEditor } from "../YamlEditor";
import { SecretEditView } from "./SecretEditView";

type K8sObject = {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
  };
  type?: string;
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

type ConfigTarget = {
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

function initialQueries(tabs: ConfigTab[]): Record<ConfigTab, QueryState> {
  return Object.fromEntries(CONFIG_TABS.map((item) => [item, tabs.includes(item) ? initialQuery() : initialQuery()])) as Record<
    ConfigTab,
    QueryState
  >;
}

export function ConfigsPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const mountedRef = useRef(false);
  const namespaceRequestSeq = useRef(0);
  const listRequestSeq = useRef(0);
  const objectRequestSeq = useRef(0);
  const activeObjectRequest = useRef<AbortController | null>(null);

  const visibleTabs = useMemo(() => configTabsForUser(canManage), [canManage]);
  const defByKind = useMemo(
    () => new Map(CATALOG.filter((item) => CONFIG_TABS.includes(item.kind as ConfigTab)).map((item) => [item.kind, item])),
    [],
  );

  const [namespace, setNamespace] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [tab, setTab] = useState<ConfigTab>("ConfigMap");
  const [queries, setQueries] = useState<Record<ConfigTab, QueryState>>(() => initialQueries(visibleTabs));
  const [items, setItems] = useState<K8sObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [detail, setDetail] = useState<K8sObject | null>(null);
  const [yamlTarget, setYamlTarget] = useState<ConfigTarget | null>(null);
  const [editTarget, setEditTarget] = useState<ConfigTarget | null>(null);
  const [createTarget, setCreateTarget] = useState<{ title: string; def: ResourceDef; value: K8sObject } | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const selectedDef = defByKind.get(tab);
  const query = queries[tab];
  const colSpan = configTableColSpan(tab);

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

  useEffect(() => {
    if (!visibleTabs.includes(tab)) {
      setTab("ConfigMap");
    }
  }, [tab, visibleTabs]);

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
      setError(errorInfo(err, `加载 ${configTabLabel(tab)} 失败`));
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
    setQueries(initialQueries(visibleTabs));
    setSelected([]);
    closeModals();
    abortObjectRequest();
  }, [abortObjectRequest, closeModals, clusterId, visibleTabs]);

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

  async function openLatest(item: K8sObject, onLoaded: (latest: K8sObject, def: ResourceDef) => void, fallbackMessage: string) {
    const requestDef = defByKind.get(typeof item.kind === "string" ? item.kind : tab) ?? selectedDef;
    if (!requestDef || !item.metadata?.name) return;
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
      const latest = await k8sFetch<K8sObject>(
        proxyPath(clusterId, requestDef, item.metadata.namespace, item.metadata.name),
        { signal: controller.signal },
      );
      if (isCurrent()) onLoaded(latest, requestDef);
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
      value: emptyConfigTemplate(tab, namespace) as K8sObject,
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
          tabs={visibleTabs}
          active={tab}
          ariaLabel="配置资源类型"
          label={configTabLabel}
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
          <button type="button" className="btn-secondary" disabled={!canManage} onClick={() => openCreate(`YAML ${tab}`)}>
            YAML
          </button>
          <button type="button" className="btn" disabled={!canManage} onClick={() => openCreate(`新增 ${configTabLabel(tab)}`)}>
            新增
          </button>
        </div>

        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th className="w-10">
                  <span className="sr-only">选择</span>
                </th>
                <th>名称</th>
                <th>命名空间</th>
                {tab === "Secret" ? <th>类型</th> : null}
                <th>存活时间</th>
                <th>操作</th>
              </tr>
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
                  <ConfigRow
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
      <Modal open={Boolean(editTarget)} title="编辑" size="xl" onClose={() => setEditTarget(null)}>
        {editTarget?.def.kind === "Secret" && secretEditableType(editTarget.value.type) ? (
          <SecretEditView
            value={editTarget.value}
            saving={saving}
            onSave={saveExisting}
            onClose={() => setEditTarget(null)}
          />
        ) : editTarget ? (
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

function resetPages(queries: Record<ConfigTab, QueryState>): Record<ConfigTab, QueryState> {
  return Object.fromEntries(Object.entries(queries).map(([key, value]) => [key, { ...value, page: 1 }])) as Record<
    ConfigTab,
    QueryState
  >;
}

function ConfigRow({
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
  tab: ConfigTab;
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
  const editable = tab !== "Secret" || secretEditableType(item.type);

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
      {tab === "Secret" ? <td>{item.type ?? "-"}</td> : null}
      <td>{formatNodeAge(item.metadata?.creationTimestamp)}</td>
      <td className="whitespace-nowrap">
        <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void onYaml(item)}>
          YAML
        </button>
        <span className="text-muted"> / </span>
        <button type="button" className="k8s-action-link" disabled={!canManage || !editable} onClick={() => void onEdit(item)}>
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

function LabelsHint({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {});
  if (entries.length === 0) return null;
  return (
    <span className="k8s-label-hint" title={entries.map(([key, value]) => `${key}: ${value}`).join("\n")}>
      ▸
    </span>
  );
}
