"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { K8sEmptyState, K8sLoadingState, useK8sConfirm } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import type { KubernetesCluster } from "../cluster-types";
import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "../yaml-document";
import { formatRoleAge, isBuiltinRole } from "./cluster-authorize-roles";

type Scope = "cluster" | "namespace";
type EditorMode = "create" | "edit" | "view";

type K8sList<T> = { items?: T[] };

type RoleObject = {
  apiVersion?: string;
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    resourceVersion?: string;
    uid?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
    managedFields?: unknown;
  };
  rules?: unknown[];
  aggregationRule?: unknown;
  status?: unknown;
};

const DEFAULT_CLUSTER_ROLE_YAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sparkops-view
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
`;

function defaultNamespaceRoleYaml(namespace: string) {
  return `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sparkops-ns-view
  namespace: ${namespace}
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
`;
}

function proxyURL(clusterId: string, path: string) {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/${path}`;
}

function clusterRolesURL(clusterId: string, name?: string) {
  const base = proxyURL(clusterId, "apis/rbac.authorization.k8s.io/v1/clusterroles");
  return name ? `${base}/${encodeURIComponent(name)}` : base;
}

function namespacedRolesURL(clusterId: string, namespace: string, name?: string) {
  const base = proxyURL(
    clusterId,
    `apis/rbac.authorization.k8s.io/v1/namespaces/${encodeURIComponent(namespace)}/roles`,
  );
  return name ? `${base}/${encodeURIComponent(name)}` : base;
}

function editorObject(item: RoleObject): Record<string, unknown> {
  const metadata = { ...(item.metadata ?? {}) };
  delete metadata.managedFields;
  return {
    apiVersion: item.apiVersion ?? "rbac.authorization.k8s.io/v1",
    kind: item.kind,
    metadata,
    rules: item.rules ?? [],
    ...(item.aggregationRule ? { aggregationRule: item.aggregationRule } : {}),
  };
}

function SearchIcon() {
  return (
    <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z" stroke="currentColor" strokeWidth="1.5" />
      <path d="M13.5 13.5 17 17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function ClusterAuthorizeRoles({
  cluster,
  canManage,
}: {
  cluster: KubernetesCluster;
  canManage: boolean;
}) {
  const [scope, setScope] = useState<Scope>("cluster");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [formError, setFormError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [namespace, setNamespace] = useState("default");
  const [roles, setRoles] = useState<RoleObject[]>([]);
  const [editor, setEditor] = useState<{ mode: EditorMode; yaml: string; name: string } | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const loadNamespaces = useCallback(async () => {
    const data = await k8sFetch<K8sList<{ metadata?: { name?: string } }>>(proxyURL(cluster.id, "api/v1/namespaces"));
    const names = (data.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[];
    setNamespaces(names);
    setNamespace((current) => (names.includes(current) ? current : names[0] ?? "default"));
  }, [cluster.id]);

  const loadRoles = useCallback(async () => {
    const data =
      scope === "cluster"
        ? await k8sFetch<K8sList<RoleObject>>(clusterRolesURL(cluster.id))
        : await k8sFetch<K8sList<RoleObject>>(namespacedRolesURL(cluster.id, namespace));
    setRoles(data.items ?? []);
  }, [cluster.id, namespace, scope]);

  const reload = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      if (scope === "namespace") await loadNamespaces();
      await loadRoles();
    } catch (err) {
      setError(errorInfo(err, "加载角色失败"));
    } finally {
      setLoading(false);
    }
  }, [loadNamespaces, loadRoles, scope]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return roles;
    return roles.filter((item) => (item.metadata?.name ?? "").toLowerCase().includes(q));
  }, [query, roles]);

  function openCreate() {
    setFormError(null);
    setEditor({
      mode: "create",
      name: "",
      yaml: scope === "cluster" ? DEFAULT_CLUSTER_ROLE_YAML : defaultNamespaceRoleYaml(namespace),
    });
  }

  async function openRole(item: RoleObject, mode: "edit" | "view") {
    const name = item.metadata?.name;
    if (!name) return;
    setFormError(null);
    setError(null);
    try {
      const fetched =
        scope === "cluster"
          ? await k8sFetch<RoleObject>(clusterRolesURL(cluster.id, name))
          : await k8sFetch<RoleObject>(namespacedRolesURL(cluster.id, namespace, name));
      setEditor({
        mode,
        name,
        yaml: stringifyYamlEditorDocument(editorObject({ ...fetched, kind: scope === "cluster" ? "ClusterRole" : "Role" })),
      });
    } catch (err) {
      setError(errorInfo(err, "加载角色详情失败"));
    }
  }

  async function saveEditor() {
    if (!editor || editor.mode === "view" || !canManage || saving) return;
    setFormError(null);
    let doc: Record<string, unknown>;
    try {
      doc = parseYamlEditorDocument(editor.yaml);
    } catch (err) {
      setFormError({
        code: "INVALID_REQUEST",
        message: err instanceof Error ? err.message : "YAML 无法解析",
      });
      return;
    }
    const metadata =
      typeof doc.metadata === "object" && doc.metadata ? { ...(doc.metadata as Record<string, unknown>) } : {};
    const name = String(metadata.name ?? editor.name).trim();
    if (!name) {
      setFormError({ code: "INVALID_REQUEST", message: "请填写角色名称" });
      return;
    }
    if (scope === "namespace") metadata.namespace = namespace;
    const kind = scope === "cluster" ? "ClusterRole" : "Role";
    const body = {
      ...doc,
      apiVersion: "rbac.authorization.k8s.io/v1",
      kind,
      metadata: { ...metadata, name },
    };
    const collectionURL = scope === "cluster" ? clusterRolesURL(cluster.id) : namespacedRolesURL(cluster.id, namespace);
    const itemURL =
      scope === "cluster" ? clusterRolesURL(cluster.id, name) : namespacedRolesURL(cluster.id, namespace, name);
    setSaving(true);
    try {
      if (editor.mode === "edit") {
        await k8sFetch(itemURL, { method: "PUT", body: JSON.stringify(body) });
      } else {
        try {
          await k8sFetch(collectionURL, { method: "POST", body: JSON.stringify(body) });
        } catch {
          await k8sFetch(itemURL, { method: "PUT", body: JSON.stringify(body) });
        }
      }
      setEditor(null);
      await loadRoles();
    } catch (err) {
      setFormError(errorInfo(err, "保存角色失败"));
    } finally {
      setSaving(false);
    }
  }

  async function deleteRole(item: RoleObject) {
    const name = item.metadata?.name;
    if (!canManage || !name || isBuiltinRole(item, scope)) return;
    const kindLabel = scope === "cluster" ? "ClusterRole" : "Role";
    const ok = await ask({
      title: `删除${kindLabel}`,
      description: `确定删除 ${kindLabel}「${name}」？`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      const url =
        scope === "cluster" ? clusterRolesURL(cluster.id, name) : namespacedRolesURL(cluster.id, namespace, name);
      await k8sFetch(url, { method: "DELETE" });
      await loadRoles();
    } catch (err) {
      setError(errorInfo(err, "删除角色失败"));
    }
  }

  const kindLabel = scope === "cluster" ? "集群角色" : "命名空间角色";

  return (
    <div className="stack-sm">
      <ApiErrorBanner error={error} />
      <div className="k8s-dialog-toolbar">
        <div className="k8s-dialog-toolbar-filters">
          <div className="k8s-segmented" role="tablist" aria-label="角色范围">
            <button
              type="button"
              role="tab"
              aria-selected={scope === "cluster"}
              className={`k8s-segmented-btn${scope === "cluster" ? " is-active" : ""}`}
              onClick={() => {
                setQuery("");
                setScope("cluster");
              }}
            >
              集群
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={scope === "namespace"}
              className={`k8s-segmented-btn${scope === "namespace" ? " is-active" : ""}`}
              onClick={() => {
                setQuery("");
                setScope("namespace");
              }}
            >
              命名空间
            </button>
          </div>
          {scope === "namespace" ? (
            <select
              className="input w-auto text-sm"
              value={namespace}
              aria-label="命名空间"
              onChange={(e) => setNamespace(e.target.value)}
            >
              {(namespaces.length ? namespaces : [namespace]).map((ns) => (
                <option key={ns} value={ns}>
                  {ns}
                </option>
              ))}
            </select>
          ) : null}
          <label className="projects-search-wrap k8s-dialog-search">
            <SearchIcon />
            <input
              className="projects-search-input"
              type="search"
              placeholder="搜索"
              aria-label="搜索角色"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
        </div>
        <div className="k8s-dialog-toolbar-actions">
          <span className="text-xs text-muted hidden sm:inline">{loading ? "…" : `${filtered.length} 项`}</span>
          <button type="button" className="btn-secondary text-sm" disabled={loading} aria-label="刷新" onClick={() => void reload()}>
            ↻
          </button>
          {canManage ? (
            <button type="button" className="btn text-sm" onClick={openCreate}>
              添加
            </button>
          ) : null}
        </div>
      </div>
      <div className="table-wrap k8s-dialog-table">
        <table className="data-table text-sm">
          <colgroup>
            <col />
            <col style={{ width: "4.5rem" }} />
            <col style={{ width: "5.5rem" }} />
            <col style={{ width: "9rem" }} />
          </colgroup>
          <thead>
            <tr>
              <th>名称</th>
              <th className="col-compact">规则</th>
              <th className="col-compact">时长</th>
              <th className="col-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={4}>
                  <K8sLoadingState compact />
                </td>
              </tr>
            ) : null}
            {!loading &&
              filtered.map((item) => {
                const name = item.metadata?.name ?? "-";
                const builtin = isBuiltinRole(item, scope);
                return (
                  <tr key={`${item.metadata?.namespace ?? "cluster"}/${name}`}>
                    <td className="min-w-0">
                      <button type="button" className="k8s-cluster-name-link text-sm" onClick={() => void openRole(item, "view")}>
                        {name}
                      </button>
                      {builtin ? <span className="k8s-meta-tag">内置</span> : null}
                    </td>
                    <td className="col-compact">{item.rules?.length ?? 0}</td>
                    <td className="col-compact">{formatRoleAge(item.metadata?.creationTimestamp)}</td>
                    <td className="col-actions">
                      <div className="k8s-row-actions">
                        <button type="button" className="btn-ghost text-sm" onClick={() => void openRole(item, "view")}>
                          查看
                        </button>
                        <button
                          type="button"
                          className="btn-ghost text-sm"
                          disabled={!canManage || builtin}
                          onClick={() => void openRole(item, "edit")}
                        >
                          编辑
                        </button>
                        <button
                          type="button"
                          className="btn-ghost text-sm"
                          disabled={!canManage || builtin}
                          onClick={() => void deleteRole(item)}
                        >
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            {!loading && filtered.length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <K8sEmptyState variant="empty" title={`暂无${kindLabel}`} compact />
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <Modal
        nested
        open={Boolean(editor)}
        size="lg"
        panelClassName="k8s-cluster-form-modal"
        title={
          editor?.mode === "create" ? `添加${kindLabel}` : editor?.mode === "edit" ? `编辑 ${editor.name}` : `查看 ${editor?.name ?? ""}`
        }
        description={scope === "namespace" ? namespace : undefined}
        onClose={() => {
          if (!saving) setEditor(null);
        }}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" disabled={saving} onClick={() => setEditor(null)}>
              {editor?.mode === "view" ? "关闭" : "取消"}
            </button>
            {editor?.mode !== "view" ? (
              <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void saveEditor()}>
                {saving ? "保存中…" : "保存"}
              </button>
            ) : null}
          </div>
        }
      >
        <div className="stack-md">
          <ApiErrorBanner error={formError} />
          <textarea
            className="input font-mono text-xs k8s-kubeconfig-area"
            rows={16}
            readOnly={editor?.mode === "view"}
            value={editor?.yaml ?? ""}
            spellCheck={false}
            onChange={(e) => setEditor((current) => (current ? { ...current, yaml: e.target.value } : current))}
          />
        </div>
      </Modal>
      {confirmDialog}
    </div>
  );
}
