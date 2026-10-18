"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sStatusBadge,
  useK8sConfirm,
  k8sPhaseTone,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { formatNodeAge } from "../node-view";
import {
  NAMESPACE_LABEL_PREVIEW,
  namespaceDisplayLabels,
  namespacePhaseBadge,
  type NamespaceObject,
} from "../namespace-view";
import { resourceCollectionUrl } from "../resource-url";
import { clampPageToTotal, validateCreateIdentity, validateExistingIdentity } from "../workload-view";
import { YamlEditor } from "../YamlEditor";
import { CloudTTYPage } from "./CloudTTYPage";
import { NamespaceCreateForm } from "./NamespaceCreateForm";

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");

export function NamespacesPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const mountedRef = useRef(false);
  const listRequestSeq = useRef(0);
  const activeObjectRequest = useRef<AbortController | null>(null);
  const objectRequestSeq = useRef(0);

  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [items, setItems] = useState<NamespaceObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [editTarget, setEditTarget] = useState<NamespaceObject | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [cloudOpen, setCloudOpen] = useState(false);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      activeObjectRequest.current?.abort();
    };
  }, []);

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!clusterId || !namespaceDef) return;
    const url = resourceCollectionUrl(clusterId, namespaceDef, { page, pageSize, keyword });
    if (!url) return;
    const requestSeq = listRequestSeq.current + 1;
    listRequestSeq.current = requestSeq;
    const isCurrent = () => mountedRef.current && !signal?.aborted && requestSeq === listRequestSeq.current;
    setLoading(true);
    setError(null);
    try {
      const data = await k8sFetch<{ items?: NamespaceObject[]; total?: number }>(url, { signal });
      if (!isCurrent()) return;
      const nextTotal = data.total ?? data.items?.length ?? 0;
      const nextPage = clampPageToTotal(page, nextTotal, pageSize);
      if (nextPage !== page) {
        setPage(nextPage);
        return;
      }
      setItems(data.items ?? []);
      setTotal(nextTotal);
    } catch (err) {
      if (!isCurrent()) return;
      setItems([]);
      setTotal(0);
      setError(errorInfo(err, "加载命名空间失败"));
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [clusterId, keyword, page, pageSize]);

  useEffect(() => {
    setPage(1);
    setKeyword("");
  }, [clusterId]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function openEdit(item: NamespaceObject) {
    if (!namespaceDef || !item.metadata?.name) return;
    activeObjectRequest.current?.abort();
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
      const latest = await k8sFetch<NamespaceObject>(
        proxyPath(clusterId, namespaceDef, undefined, item.metadata.name),
        { signal: controller.signal },
      );
      if (isCurrent()) setEditTarget(latest);
    } catch (err) {
      if (!controller.signal.aborted && isCurrent()) setError(errorInfo(err, "加载命名空间失败"));
    } finally {
      if (activeObjectRequest.current === controller) activeObjectRequest.current = null;
    }
  }

  async function saveExisting(next: unknown) {
    if (!namespaceDef || !editTarget?.metadata?.name) return;
    const obj = next as NamespaceObject;
    const identityError = validateExistingIdentity(editTarget as Record<string, unknown>, obj as Record<string, unknown>);
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    setSaving(true);
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, namespaceDef, undefined, editTarget.metadata.name), {
        method: "PUT",
        body: JSON.stringify(next),
      });
      setEditTarget(null);
      await load();
    } catch (err) {
      setError(errorInfo(err, "保存命名空间失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function createNamespace(next: unknown) {
    if (!namespaceDef) return;
    const obj = next as NamespaceObject;
    const identityError = validateCreateIdentity(namespaceDef, obj as Record<string, unknown>);
    if (identityError) {
      setError({ message: identityError });
      throw new Error(identityError);
    }
    const name = obj.metadata?.name?.trim();
    if (!name) throw new Error("请在 metadata.name 中填写命名空间名称。");
    setSaving(true);
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, namespaceDef), { method: "POST", body: JSON.stringify(next) });
      setCreateOpen(false);
      await load();
    } catch (err) {
      setError(errorInfo(err, "创建命名空间失败"));
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function deleteItem(item: NamespaceObject) {
    if (!namespaceDef || !canManage || !item.metadata?.name) return;
    const ok = await ask({
      title: "删除命名空间",
      description: `确定要删除命名空间「${item.metadata.name}」吗？此操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await k8sFetch(proxyPath(clusterId, namespaceDef, undefined, item.metadata.name), { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除命名空间失败"));
    }
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
    <div className="stack-md">
      <K8sPanelCard className="k8s-namespaces-panel">
        <div className="k8s-clusters-toolbar">
          <div className="k8s-clusters-toolbar-filters">
            <label className="projects-search-wrap">
              <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M9 4a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z" stroke="currentColor" strokeWidth="1.5" />
                <path d="M13 13l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
              <input
                className="projects-search-input"
                value={keyword}
                placeholder="搜索命名空间"
                aria-label="搜索命名空间"
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
                共 {total} 个命名空间
              </span>
            ) : null}
            <button type="button" className="btn-secondary text-sm" disabled={!canManage} onClick={() => setCloudOpen(true)}>
              CloudShell
            </button>
            <button type="button" className="btn text-sm" disabled={!canManage} onClick={() => setCreateOpen(true)}>
              新建
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

        <ApiErrorBanner error={error} />

        <div className="table-wrap k8s-namespaces-table">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>标签</th>
                <th>状态</th>
                <th>存活时间</th>
                <th className="w-28">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={5}>
                    <K8sLoadingState compact />
                  </td>
                </tr>
              ) : null}
              {!loading &&
                items.map((item) => {
                  const name = item.metadata?.name ?? "";
                  const phase = namespacePhaseBadge(item.status?.phase);
                  const phaseText = phase.text === "-" ? "未知" : phase.text;
                  return (
                    <tr key={name}>
                      <td className="font-medium">{name || "-"}</td>
                      <td>
                        <NamespaceLabels labels={item.metadata?.labels} />
                      </td>
                      <td>
                        {phase.text === "-" ? (
                          <span className="text-muted">-</span>
                        ) : (
                          <K8sStatusBadge tone={k8sPhaseTone(phaseText)} dot>
                            {phaseText}
                          </K8sStatusBadge>
                        )}
                      </td>
                      <td className="text-muted">{formatNodeAge(item.metadata?.creationTimestamp)}</td>
                      <td>
                        <div className="k8s-row-actions">
                          <button type="button" className="btn-ghost text-sm" disabled={!canManage} onClick={() => void openEdit(item)}>
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
                  );
                })}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={5}>
                    <K8sEmptyState
                      variant="empty"
                      compact
                      title="暂无命名空间"
                      description={
                        hasActiveFilters ? "没有符合搜索条件的命名空间，可调整关键词。" : "当前集群下没有 Namespace 资源。"
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

      <Modal open={cloudOpen} title="CloudShell" size="xl" panelClassName="k8s-pod-submodal" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>

      <Modal
        open={Boolean(editTarget)}
        title={editTarget?.metadata?.name ? `编辑 · ${editTarget.metadata.name}` : "编辑命名空间"}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setEditTarget(null)}
      >
        {editTarget ? (
          <YamlEditor
            value={editTarget}
            canSave={canManage}
            saving={saving}
            confirmLabel="保存"
            cancelLabel="取消"
            onSave={saveExisting}
            onClose={() => setEditTarget(null)}
          />
        ) : null}
      </Modal>

      <Modal open={createOpen} title="新建命名空间" size="lg" panelClassName="k8s-pod-submodal" onClose={() => setCreateOpen(false)}>
        <NamespaceCreateForm
          open={createOpen}
          canSave={canManage}
          saving={saving}
          onSave={createNamespace}
          onClose={() => setCreateOpen(false)}
        />
      </Modal>
      {confirmDialog}
    </div>
  );
}

function NamespaceLabels({ labels }: { labels?: Record<string, string> }) {
  const entries = namespaceDisplayLabels(labels);
  if (entries.length === 0) return <span className="text-muted">-</span>;
  const preview = entries.slice(0, NAMESPACE_LABEL_PREVIEW);
  const overflow = entries.length - preview.length;
  return (
    <div className="k8s-ns-label-list" title={entries.map(({ key, value }) => `${key}: ${value}`).join("\n")}>
      {preview.map(({ key, value }) => (
        <span key={key} className="k8s-ns-label-chip" title={`${key}: ${value}`}>
          {key}: {value}
        </span>
      ))}
      {overflow > 0 ? <span className="k8s-ns-label-more">+{overflow}</span> : null}
    </div>
  );
}
