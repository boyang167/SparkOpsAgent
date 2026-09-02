"use client";

import { useCallback, useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "./api";
import { CATALOG, proxyPath, type ResourceDef } from "./catalog";
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
      const info = errorInfo(err, `加载 ${selected.kind} 失败`);
      if (info.status === 404 && selected.group === "gateway.networking.k8s.io") {
        info.message = `Gateway API 资源 ${selected.kind} 不可用：请确认集群已安装 Gateway API CRD，且控制器正常运行。`;
      }
      setError(info);
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

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

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
    const ok = window.confirm(`确认删除 ${selected.kind} ${item.metadata.namespace ? `${item.metadata.namespace}/` : ""}${item.metadata.name}？`);
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
    return <div className="empty-state">此页面暂无资源定义。</div>;
  }

  return (
    <section className="card stack-md p-4">
      <div className="flex flex-wrap items-end gap-3">
        <label className="min-w-52">
          <span className="label">资源类型</span>
          <select
            className="input"
            value={selected.id}
            onChange={(e) => {
              setResourceId(e.target.value);
              setNamespace("");
              setPage(1);
            }}
          >
            {resources.map((item) => (
              <option key={item.id} value={item.id}>
                {item.kind}
              </option>
            ))}
          </select>
        </label>
        {selected.namespaced ? (
          <label className="min-w-52">
            <span className="label">命名空间</span>
            <select
              className="input"
              value={namespace}
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
        <label className="min-w-64 flex-1">
          <span className="label">关键词</span>
          <input
            className="input"
            value={keyword}
            placeholder="名称、命名空间或 YAML/JSON 内容"
            onChange={(e) => {
              setKeyword(e.target.value);
              setPage(1);
            }}
          />
        </label>
        <button type="button" className="btn-secondary" disabled={loading || !clusterId} onClick={() => void load()}>
          {loading ? "加载中…" : "刷新"}
        </button>
      </div>

      <ApiErrorBanner error={error} />

      {!clusterId ? (
        <div className="empty-state">请先选择集群。</div>
      ) : selected.namespaced && !namespace ? (
        <div className="empty-state">请选择命名空间后加载 {selected.kind}。</div>
      ) : loading ? (
        <div className="empty-state">加载 {selected.kind} 中…</div>
      ) : (
        <>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  {selected.columns.map((column) => (
                    <th key={column.key}>{column.title}</th>
                  ))}
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, idx) => (
                  <tr key={`${item.metadata?.namespace ?? "_"}-${item.metadata?.name ?? idx}`}>
                    {selected.columns.map((column) => (
                      <td key={column.key}>{formatCell(valueAtPath(item, column.key))}</td>
                    ))}
                    <td>
                      <div className="flex flex-wrap gap-2">
                        <button type="button" className="btn-secondary text-xs" onClick={() => void openDetail(item)}>
                          详情/YAML
                        </button>
                        {canManage ? (
                          <button type="button" className="btn-secondary text-xs" onClick={() => void deleteItem(item)}>
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
              <button type="button" className="btn-secondary text-sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                上一页
              </button>
              <span>
                {page} / {totalPages}
              </span>
              <button type="button" className="btn-secondary text-sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                下一页
              </button>
            </div>
          </div>
        </>
      )}

      <Modal open={Boolean(detail)} title="资源详情" size="xl" onClose={() => setDetail(null)}>
        {detail ? (
          <YamlEditor
            title={`${selected.kind} ${detail.metadata?.namespace ? `${detail.metadata.namespace}/` : ""}${detail.metadata?.name ?? ""}`}
            value={detail}
            canSave={canManage}
            saving={saving}
            onSave={saveDetail}
            onClose={() => setDetail(null)}
          />
        ) : null}
      </Modal>
    </section>
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

function formatCell(value: unknown): string {
  if (value === undefined || value === null || value === "") return "-";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}
