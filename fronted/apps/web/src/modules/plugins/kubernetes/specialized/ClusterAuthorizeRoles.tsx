"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
    if (!window.confirm(`确定删除 ${kindLabel}「${name}」？`)) return;
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
    <div className="stack-md">
      <div className="k8s-tabs">
        {(
          [
            ["cluster", "集群"],
            ["namespace", "命名空间"],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            className={scope === id ? "k8s-tab k8s-tab-active" : "k8s-tab"}
            onClick={() => {
              setQuery("");
              setScope(id);
            }}
          >
            {label}
          </button>
        ))}
      </div>
      <p className="text-sm text-muted">
        {scope === "cluster"
          ? "ClusterRole 在整个集群生效，授权用户时可绑定为集群角色或命名空间角色。"
          : "Role 仅在所选命名空间生效。授权用户页的「命名空间角色」绑定的是 ClusterRole，与此处 namespaced Role 不同。"}
      </p>
      <ApiErrorBanner error={error} />
      <div className="flex flex-wrap items-end gap-2">
        {scope === "namespace" ? (
          <label>
            <span className="label">命名空间</span>
            <select className="input" value={namespace} onChange={(e) => setNamespace(e.target.value)}>
              {(namespaces.length ? namespaces : [namespace]).map((ns) => (
                <option key={ns} value={ns}>
                  {ns}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <label className="min-w-[12rem] flex-1">
          <span className="label">搜索</span>
          <input
            className="input"
            type="search"
            placeholder="按名称过滤…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <button type="button" className="btn" disabled={!canManage} onClick={openCreate}>
          添加
        </button>
        <button type="button" className="btn-secondary" disabled={loading} onClick={() => void reload()}>
          刷新
        </button>
      </div>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>名称</th>
              <th>规则数</th>
              <th>存活时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={4}>
                  <div className="empty-state">加载中…</div>
                </td>
              </tr>
            ) : null}
            {!loading &&
              filtered.map((item) => {
                const name = item.metadata?.name ?? "-";
                const builtin = isBuiltinRole(item, scope);
                return (
                  <tr key={`${item.metadata?.namespace ?? "cluster"}/${name}`}>
                    <td>
                      <button type="button" className="k8s-action-link" onClick={() => void openRole(item, "view")}>
                        {name}
                      </button>
                      {builtin ? <span className="badge badge-neutral ml-2">内置</span> : null}
                    </td>
                    <td>{item.rules?.length ?? 0}</td>
                    <td>{formatRoleAge(item.metadata?.creationTimestamp)}</td>
                    <td className="whitespace-nowrap">
                      <button type="button" className="k8s-action-link" onClick={() => void openRole(item, "view")}>
                        查看
                      </button>
                      <span className="text-muted"> / </span>
                      <button
                        type="button"
                        className="k8s-action-link"
                        disabled={!canManage || builtin}
                        onClick={() => void openRole(item, "edit")}
                      >
                        编辑
                      </button>
                      <span className="text-muted"> / </span>
                      <button
                        type="button"
                        className="k8s-action-link"
                        disabled={!canManage || builtin}
                        onClick={() => void deleteRole(item)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                );
              })}
            {!loading && filtered.length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <div className="empty-state">暂无{kindLabel}</div>
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
        title={
          editor?.mode === "create" ? `添加${kindLabel}` : editor?.mode === "edit" ? `编辑 ${editor.name}` : `查看 ${editor?.name ?? ""}`
        }
        description={scope === "namespace" ? `命名空间：${namespace}` : "集群范围 ClusterRole"}
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
                {saving ? "保存中…" : "确定"}
              </button>
            ) : null}
          </div>
        }
      >
        <div className="stack-md">
          <ApiErrorBanner error={formError} />
          {editor?.mode === "create" ? (
            <p className="text-xs text-muted">名称以 YAML 中的 metadata.name 为准。</p>
          ) : null}
          <textarea
            className="input font-mono text-xs"
            rows={16}
            readOnly={editor?.mode === "view"}
            value={editor?.yaml ?? ""}
            spellCheck={false}
            onChange={(e) => setEditor((current) => (current ? { ...current, yaml: e.target.value } : current))}
          />
        </div>
      </Modal>
    </div>
  );
}
