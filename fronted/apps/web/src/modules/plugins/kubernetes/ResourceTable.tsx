"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sResourceTabs,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "./api";
import { CATALOG, proxyPath, type ResourceDef } from "./catalog";
import { humanizeResourceLoadError, isCrdUnavailableError } from "./k8s-api-errors";
import { formatNodeAge } from "./node-view";
import { resourceCollectionUrl } from "./resource-url";
import { YamlEditor } from "./YamlEditor";

type ResourceTableProps = {
  clusterId: string;
  resources: ResourceDef[];
  canManage: boolean;
};

type K8sList = {
  items?: K8sObject[];
  total?: number;
};

type K8sObject = {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
  };
  [key: string]: unknown;
};

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");
const PAGE_SIZE = 20;

export function ResourceTable({ clusterId, resources, canManage }: ResourceTableProps) {
  const [resourceId, setResourceId] = useState(resources[0]?.id ?? "");
  const selected = resources.find((item) => item.id === resourceId) ?? resources[0];
  const [namespace, setNamespace] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<K8sObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [detail, setDetail] = useState<K8sObject | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const humanizedError = useMemo(
    () => (error && selected ? humanizeResourceLoadError(error, selected.kind, selected.group) : null),
    [error, selected],
  );
  const resourceUnavailable = selected ? isCrdUnavailableError(error, selected.kind, selected.group) : false;
  const hasActiveFilters = keyword.trim().length > 0 || namespace.length > 0;
  const resourceKinds = useMemo(() => resources.map((item) => item.kind), [resources]);
  const showResourceTabs = resources.length > 1;
  const showFilters = !resourceUnavailable;

  useEffect(() => {
    setResourceId(resources[0]?.id ?? "");
    setNamespace("");
    setKeyword("");
    setPage(1);
  }, [resources]);

  const loadNamespaces = useCallback(async () => {
    if (!clusterId || !namespaceDef) return;
    try {
      const url = resourceCollectionUrl(clusterId, namespaceDef, { page: 1, pageSize: 500 }) ?? proxyPath(clusterId, namespaceDef);
      const data = await k8sFetch<K8sList>(url);
      setNamespaces((data.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[]);
    } catch {
      setNamespaces([]);
    }
  }, [clusterId]);

  const load = useCallback(async () => {
    if (!clusterId || !selected) return;
    const url = resourceCollectionUrl(clusterId, selected, { namespace, page, pageSize: PAGE_SIZE, keyword });
    if (!url) {
      setItems([]);
      setTotal(0);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await k8sFetch<K8sList>(url);
      setItems(data.items ?? []);
      setTotal(data.total ?? data.items?.length ?? 0);
    } catch (err) {
      setItems([]);
      setTotal(0);
      setError(errorInfo(err, `加载 ${selected.kind} 失败`));
    } finally {
      setLoading(false);
    }
  }, [clusterId, keyword, namespace, page, selected]);

  useEffect(() => {
    void loadNamespaces();
  }, [loadNamespaces]);

  useEffect(() => {
    void load();
  }, [load]);

  async function openDetail(item: K8sObject) {
    if (!selected) return;
    const name = item.metadata?.name;
    const ns = item.metadata?.namespace;
    if (!name) return;
    setError(null);
    try {
      setDetail(await k8sFetch<K8sObject>(proxyPath(clusterId, selected, ns, name)));
    } catch (err) {
      setError(errorInfo(err, "加载资源详情失败"));
    }
  }

  async function saveDetail(next: unknown) {
    if (!selected || !detail?.metadata?.name) return;
    setSaving(true);
    setError(null);
    try {
      const updated = await k8sFetch<K8sObject>(
        proxyPath(clusterId, selected, detail.metadata.namespace, detail.metadata.name),
        { method: "PUT", body: JSON.stringify(next) },
      );
      setDetail(updated);
      await load();
    } catch (err) {
      setError(errorInfo(err, "保存资源失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function deleteItem(item: K8sObject) {
    if (!selected || !item.metadata?.name) return;
    const ok = await ask({
      title: "删除资源",
      description: `确认删除 ${selected.kind} ${item.metadata.namespace ? `${item.metadata.namespace}/` : ""}${item.metadata.name}？此操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await k8sFetch<void>(proxyPath(clusterId, selected, item.metadata.namespace, item.metadata.name), { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除资源失败"));
    }
  }

  if (!selected) {
    return (
      <K8sPanelCard>
        <K8sEmptyState variant="missing-def" title="此页面暂无资源定义" />
      </K8sPanelCard>
    );
  }

  if (!clusterId) {
    return (
      <K8sPanelCard>
        <K8sEmptyState variant="no-cluster" />
      </K8sPanelCard>
    );
  }

  function selectResourceKind(kind: string) {
    const next = resources.find((item) => item.kind === kind);
    if (!next) return;
    setResourceId(next.id);
    setNamespace("");
    setKeyword("");
    setPage(1);
    setError(null);
  }

  return (
    <K8sPanelCard className="k8s-resource-table-panel">
      {!showResourceTabs ? (
        <div className="k8s-resource-table-kind-label">
          <span className="text-sm text-muted">{selected.kind}</span>
        </div>
      ) : null}

      <div className="k8s-clusters-toolbar k8s-resource-table-toolbar">
        <div className="k8s-clusters-toolbar-filters">
          {showFilters && selected.namespaced ? (
            <label className="k8s-resource-table-namespace k8s-workloads-namespace">
              <span className="sr-only">命名空间</span>
              <select
                className="input"
                value={namespace}
                aria-label="命名空间"
                onChange={(e) => {
                  setNamespace(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">请选择命名空间</option>
                {namespaces.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {showFilters ? (
            <label className="projects-search-wrap">
              <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M9 4a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z" stroke="currentColor" strokeWidth="1.5" />
                <path d="M13 13l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
              <input
                className="projects-search-input"
                value={keyword}
                placeholder={`搜索${selected.kind}`}
                aria-label="搜索资源"
                onChange={(e) => {
                  setKeyword(e.target.value);
                  setPage(1);
                }}
              />
            </label>
          ) : null}
          {showFilters && hasActiveFilters ? (
            <button
              type="button"
              className="btn-secondary projects-reset-btn"
              onClick={() => {
                setNamespace("");
                setKeyword("");
                setPage(1);
              }}
            >
              重置
            </button>
          ) : null}
        </div>
        <div className="k8s-toolbar-actions">
          {showFilters && !loading && total > 0 ? (
            <span className="k8s-clusters-count" role="status">
              共 {total} 个{selected.kind}
            </span>
          ) : null}
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

      {showResourceTabs ? (
        <K8sResourceTabs
          tabs={resourceKinds}
          active={selected.kind}
          ariaLabel="资源类型"
          onChange={selectResourceKind}
        />
      ) : null}

      {selected.namespaced && !namespace && !resourceUnavailable ? (
        <K8sEmptyState
          compact
          title="请选择命名空间"
          description={`选择命名空间后即可加载 ${selected.kind} 列表。`}
        />
      ) : null}

      {resourceUnavailable && humanizedError ? (
        <div className="k8s-resource-unavailable">
          <K8sEmptyState title={humanizedError.message} description={humanizedError.hint} />
        </div>
      ) : null}

      {!resourceUnavailable && error ? <ApiErrorBanner error={error} /> : null}

      {selected.namespaced && !namespace ? null : resourceUnavailable ? null : loading ? (
        <K8sLoadingState compact label={`加载 ${selected.kind} 中…`} />
      ) : (
        <>
          <div className="table-wrap k8s-resource-table">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  {selected.columns.map((column) => (
                    <th key={column.key}>{column.title}</th>
                  ))}
                  <th className="w-28">操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, idx) => (
                  <tr key={`${item.metadata?.namespace ?? "_"}-${item.metadata?.name ?? idx}`}>
                    {selected.columns.map((column) => (
                      <td
                        key={column.key}
                        className={
                          column.key === "metadata.name"
                            ? "font-medium"
                            : column.key === "metadata.namespace" || column.key.endsWith("creationTimestamp")
                              ? "text-muted"
                              : undefined
                        }
                      >
                        {formatCell(valueAtPath(item, column.key), column.key)}
                      </td>
                    ))}
                    <td>
                      <div className="k8s-row-actions">
                        <button type="button" className="btn-ghost text-sm" onClick={() => void openDetail(item)}>
                          YAML
                        </button>
                        {canManage ? (
                          <button type="button" className="btn-ghost text-sm is-danger" onClick={() => void deleteItem(item)}>
                            删除
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
                {items.length === 0 ? (
                  <tr>
                    <td colSpan={selected.columns.length + 1}>
                      <K8sEmptyState
                        variant="empty"
                        compact
                        title={`暂无 ${selected.kind}`}
                        description={
                          hasActiveFilters
                            ? "没有符合筛选条件的资源，可调整命名空间或搜索词。"
                            : `当前范围下没有 ${selected.kind} 资源。`
                        }
                      />
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
          <K8sPagination
            page={page}
            pageSize={PAGE_SIZE}
            total={total}
            pageSizes={[PAGE_SIZE]}
            loading={loading}
            onPageChange={setPage}
            onPageSizeChange={() => undefined}
          />
        </>
      )}

      <Modal
        open={Boolean(detail)}
        title={detail?.metadata?.name ? `YAML · ${detail.metadata.name}` : "资源详情"}
        description={
          detail ? `${selected.kind}${detail.metadata?.namespace ? ` · ${detail.metadata.namespace}` : ""}` : undefined
        }
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setDetail(null)}
      >
        {detail ? (
          <YamlEditor
            value={detail}
            canSave={canManage}
            saving={saving}
            onSave={saveDetail}
            onClose={() => setDetail(null)}
          />
        ) : null}
      </Modal>
      {confirmDialog}
    </K8sPanelCard>
  );
}

function valueAtPath(item: K8sObject, path: string): unknown {
  return path.split(".").reduce<unknown>((current, key) => {
    if (current && typeof current === "object" && key in current) {
      return (current as Record<string, unknown>)[key];
    }
    return undefined;
  }, item);
}

function formatCell(value: unknown, key: string): string {
  if (value === undefined || value === null || value === "") return "-";
  if (key.endsWith("creationTimestamp") && typeof value === "string") return formatNodeAge(value);
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  const json = JSON.stringify(value);
  return json.length > 48 ? `${json.slice(0, 45)}…` : json;
}
