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
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath } from "../catalog";
import { formatNodeAge } from "../node-view";
import { resourceCollectionUrl } from "../resource-url";
import { clampPageToTotal, validateCreateIdentity, validateExistingIdentity } from "../workload-view";
import { emptyNamespaceTemplate, namespacePhaseBadge, type NamespaceObject } from "../namespace-view";
import { YamlEditor } from "../YamlEditor";
import { CloudTTYPage } from "./CloudTTYPage";

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");
export function NamespacesPanel({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const mountedRef = useRef(false);
  const listRequestSeq = useRef(0);
  const activeObjectRequest = useRef<AbortController | null>(null);
  const objectRequestSeq = useRef(0);

  const [draftKeyword, setDraftKeyword] = useState("");
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [items, setItems] = useState<NamespaceObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
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
    setSelected([]);
    setPage(1);
    setDraftKeyword("");
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

  function toggleSelected(name: string, checked: boolean) {
    setSelected((current) => (checked ? [...current, name] : current.filter((item) => item !== name)));
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
          <button type="button" className="btn" disabled={loading} onClick={() => { setKeyword(draftKeyword); setPage(1); }}>
            查询
          </button>
          <button
            type="button"
            className="btn-secondary"
            disabled={loading}
            onClick={() => {
              setDraftKeyword("");
              setKeyword("");
              setPage(1);
            }}
          >
            重置
          </button>
          <button type="button" className="btn" disabled={!canManage} onClick={() => setCloudOpen(true)}>
            CloudShell
          </button>
        </div>

        <div>
          <button type="button" className="btn" disabled={!canManage} onClick={() => setCreateOpen(true)}>
            + 新建
          </button>
        </div>

        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th className="w-10">
                  <input
                    type="checkbox"
                    aria-label="全选"
                    checked={items.length > 0 && selected.length === items.length}
                    onChange={(e) => setSelected(e.target.checked ? items.map((item) => item.metadata?.name ?? "").filter(Boolean) : [])}
                  />
                </th>
                <th>名称</th>
                <th>标签</th>
                <th>状态</th>
                <th>存活时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={6}>
                    <K8sLoadingState compact />
                  </td>
                </tr>
              ) : null}
              {!loading &&
                items.map((item) => {
                  const name = item.metadata?.name ?? "";
                  const phase = namespacePhaseBadge(item.status?.phase);
                  return (
                    <tr key={name}>
                      <td>
                        <input
                          type="checkbox"
                          checked={selected.includes(name)}
                          onChange={(e) => toggleSelected(name, e.target.checked)}
                        />
                      </td>
                      <td>{name || "-"}</td>
                      <td>
                        <NamespaceLabels labels={item.metadata?.labels} />
                      </td>
                      <td>
                        <span className={phase.tone === "ok" ? "k8s-badge-ok" : phase.tone === "bad" ? "k8s-badge-bad" : "text-muted"}>
                          {phase.text}
                        </span>
                      </td>
                      <td>{formatNodeAge(item.metadata?.creationTimestamp)}</td>
                      <td className="whitespace-nowrap">
                        <button type="button" className="k8s-action-link" disabled={!canManage} onClick={() => void openEdit(item)}>
                          编辑
                        </button>
                        <span className="text-muted"> / </span>
                        <button type="button" className="k8s-action-link is-danger" disabled={!canManage} onClick={() => void deleteItem(item)}>
                          删除
                        </button>
                      </td>
                    </tr>
                  );
                })}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <K8sEmptyState variant="empty" compact />
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

      <Modal open={cloudOpen} title="CloudShell" size="xl" onClose={() => setCloudOpen(false)}>
        <CloudTTYPage clusterId={clusterId} canManage={canManage} />
      </Modal>

      <Modal open={Boolean(editTarget)} title="编辑" size="xl" onClose={() => setEditTarget(null)}>
        {editTarget ? (
          <YamlEditor
            value={editTarget}
            canSave={canManage}
            saving={saving}
            confirmLabel="确定"
            cancelLabel="取消"
            onSave={saveExisting}
            onClose={() => setEditTarget(null)}
          />
        ) : null}
      </Modal>

      <Modal open={createOpen} title="创建" size="xl" onClose={() => setCreateOpen(false)}>
        <YamlEditor
          value={emptyNamespaceTemplate()}
          canSave={canManage}
          saving={saving}
          confirmLabel="确定"
          cancelLabel="取消"
          onSave={createNamespace}
          onClose={() => setCreateOpen(false)}
        />
      </Modal>
      {confirmDialog}
    </div>
  );
}

function NamespaceLabels({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {});
  if (entries.length === 0) return <>-</>;
  return (
    <div className="k8s-ns-label-list">
      {entries.map(([key, value]) => (
        <span key={key} className="k8s-ns-label-chip">
          {key}: {value}
        </span>
      ))}
    </div>
  );
}
