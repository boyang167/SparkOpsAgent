"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { K8sEmptyState, K8sLoadingState, useK8sConfirm } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { useShellContext } from "@/components/layout/shell-context";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import type { KubernetesCluster } from "../cluster-types";

type K8sList<T> = { items?: T[] };

type NamespaceRole = { namespace: string; roles: string[] };

type ClusterUserGrant = {
  member_id: string;
  email: string;
  display_name: string;
  cluster_roles: string[];
  namespace_roles: NamespaceRole[];
};

type GrantList = { items?: ClusterUserGrant[] };

type WorkspaceMember = {
  member_id: string;
  email: string;
  display_name: string;
};

const COMMON_ROLES = ["view", "edit", "admin", "cluster-admin"] as const;

function usersURL(clusterId: string, memberId?: string) {
  const base = `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/users`;
  return memberId ? `${base}/${encodeURIComponent(memberId)}` : base;
}

function proxyURL(clusterId: string, path: string) {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/${path}`;
}

function toggleValue(list: string[], value: string) {
  return list.includes(value) ? list.filter((item) => item !== value) : [...list, value];
}

function RoleChipPicker({
  roles,
  selected,
  onChange,
  disabled,
}: {
  roles: string[];
  selected: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState("");
  const catalog = roles.length ? roles : [...COMMON_ROLES];
  const common = COMMON_ROLES.filter((name) => catalog.includes(name));
  const filtered = catalog.filter((name) => name.toLowerCase().includes(query.trim().toLowerCase()));

  return (
    <div className="stack-sm">
      <div className="flex flex-wrap gap-1.5">
        {common.map((name) => (
          <button
            key={name}
            type="button"
            className={selected.includes(name) ? "k8s-role-chip k8s-role-chip-active" : "k8s-role-chip"}
            disabled={disabled}
            onClick={() => onChange(toggleValue(selected, name))}
          >
            {name}
          </button>
        ))}
      </div>
      <input
        className="input"
        type="search"
        placeholder="搜索完整 ClusterRole…"
        value={query}
        disabled={disabled}
        onChange={(e) => setQuery(e.target.value)}
      />
      <div className="k8s-role-list">
        {filtered.length === 0 ? <p className="text-sm text-muted">无匹配角色</p> : null}
        {filtered.map((name) => (
          <label key={name} className="flex items-center gap-2 py-1 text-sm">
            <input
              type="checkbox"
              checked={selected.includes(name)}
              disabled={disabled}
              onChange={() => onChange(toggleValue(selected, name))}
            />
            <span className="font-mono text-xs">{name}</span>
          </label>
        ))}
      </div>
      {selected.length > 0 ? (
        <div className="k8s-role-badges">
          {selected.map((name) => (
            <span key={name} className="badge badge-primary">
              {name}
              {!disabled ? (
                <button
                  type="button"
                  className="ml-1 border-0 bg-transparent p-0 text-xs leading-none"
                  aria-label={`移除 ${name}`}
                  onClick={() => onChange(selected.filter((item) => item !== name))}
                >
                  ×
                </button>
              ) : null}
            </span>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted">尚未选择角色</p>
      )}
    </div>
  );
}

function GrantRoleSummary({ grant }: { grant: ClusterUserGrant }) {
  const nsLabels = (grant.namespace_roles ?? []).flatMap((item) =>
    (item.roles ?? []).map((role) => `${item.namespace}:${role}`),
  );
  return (
    <>
      <td>
        <div className="k8s-role-badges">
          {(grant.cluster_roles ?? []).length === 0 ? <span className="text-muted">-</span> : null}
          {(grant.cluster_roles ?? []).map((role) => (
            <span key={role} className="badge badge-primary">
              {role}
            </span>
          ))}
        </div>
      </td>
      <td>
        <div className="k8s-role-badges">
          {nsLabels.length === 0 ? <span className="text-muted">-</span> : null}
          {nsLabels.map((label) => (
            <span key={label} className="badge badge-neutral">
              {label}
            </span>
          ))}
        </div>
      </td>
    </>
  );
}

export function ClusterAuthorizeUsers({
  cluster,
  canManage,
}: {
  cluster: KubernetesCluster;
  canManage: boolean;
}) {
  const { workspaceId } = useShellContext();
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [formError, setFormError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [grants, setGrants] = useState<ClusterUserGrant[]>([]);
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [roles, setRoles] = useState<string[]>([]);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [memberQuery, setMemberQuery] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<ClusterUserGrant | null>(null);
  const [pickedMembers, setPickedMembers] = useState<string[]>([]);
  const [clusterRoles, setClusterRoles] = useState<string[]>([]);
  const [namespaceRoles, setNamespaceRoles] = useState<NamespaceRole[]>([]);
  const [credential, setCredential] = useState<{ email: string; kubeconfig: string; expires_at: string } | null>(null);
  const [issuing, setIssuing] = useState(false);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const load = useCallback(async () => {
    const [grantList, memberList, roleList, nsList] = await Promise.all([
      k8sFetch<GrantList>(usersURL(cluster.id)),
      workspaceId ? governance.listMembers(workspaceId) : Promise.resolve([]),
      k8sFetch<K8sList<{ metadata?: { name?: string } }>>(
        proxyURL(cluster.id, "apis/rbac.authorization.k8s.io/v1/clusterroles"),
      ),
      k8sFetch<K8sList<{ metadata?: { name?: string } }>>(proxyURL(cluster.id, "api/v1/namespaces")),
    ]);
    setGrants(grantList.items ?? []);
    setMembers(memberList);
    setRoles((roleList.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[]);
    setNamespaces((nsList.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[]);
  }, [cluster.id, workspaceId]);

  useEffect(() => {
    let cancelled = false;
    async function run() {
      setError(null);
      setLoading(true);
      try {
        await load();
      } catch (err) {
        if (!cancelled) setError(errorInfo(err, "加载用户授权失败"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void run();
    return () => {
      cancelled = true;
    };
  }, [load]);

  const grantedIds = useMemo(() => new Set(grants.map((item) => item.member_id)), [grants]);
  const addableMembers = useMemo(() => {
    const q = memberQuery.trim().toLowerCase();
    return members
      .filter((item) => !grantedIds.has(item.member_id))
      .filter((item) => {
        if (!q) return true;
        return item.email.toLowerCase().includes(q) || item.display_name.toLowerCase().includes(q);
      });
  }, [members, grantedIds, memberQuery]);

  function openCreate() {
    setEditing(null);
    setPickedMembers([]);
    setMemberQuery("");
    setClusterRoles([]);
    setNamespaceRoles([]);
    setFormError(null);
    setShowForm(true);
  }

  function openEdit(grant: ClusterUserGrant) {
    setEditing(grant);
    setPickedMembers([grant.member_id]);
    setMemberQuery("");
    setClusterRoles(grant.cluster_roles ?? []);
    setNamespaceRoles(grant.namespace_roles?.length ? grant.namespace_roles : []);
    setFormError(null);
    setShowForm(true);
  }

  function closeForm() {
    if (saving) return;
    setShowForm(false);
    setFormError(null);
  }

  async function save() {
    if (!canManage || saving) return;
    if (!editing && pickedMembers.length === 0) {
      setFormError({ code: "INVALID_REQUEST", message: "请选择至少一个工作区成员" });
      return;
    }
    const cleanedNamespaceRoles = namespaceRoles.filter((item) => item.namespace && item.roles.length > 0);
    if (clusterRoles.length === 0 && cleanedNamespaceRoles.length === 0) {
      setFormError({ code: "INVALID_REQUEST", message: "请至少选择一个集群角色或命名空间角色" });
      return;
    }
    setFormError(null);
    setSaving(true);
    const payload = { cluster_roles: clusterRoles, namespace_roles: cleanedNamespaceRoles };
    try {
      if (editing) {
        await k8sFetch(usersURL(cluster.id, editing.member_id), { method: "PUT", body: JSON.stringify(payload) });
      } else {
        await k8sFetch(usersURL(cluster.id), {
          method: "POST",
          body: JSON.stringify({ member_ids: pickedMembers, ...payload }),
        });
      }
      setShowForm(false);
      setSelected([]);
      await load();
    } catch (err) {
      setFormError(errorInfo(err, "保存用户授权失败"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(ids: string[]) {
    if (!canManage || ids.length === 0) return;
    const ok = await ask({
      title: "删除用户授权",
      description: `确定删除选中的 ${ids.length} 个用户授权？删除后将吊销其集群访问。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      for (const id of ids) {
        await k8sFetch(usersURL(cluster.id, id), { method: "DELETE" });
      }
      setSelected([]);
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除用户授权失败"));
    }
  }

  async function issueKubeconfig(grant: ClusterUserGrant) {
    if (!canManage || issuing) return;
    setError(null);
    setIssuing(true);
    try {
      const data = await k8sFetch<{ kubeconfig?: string; expires_at?: string }>(
        `${usersURL(cluster.id, grant.member_id)}/kubeconfig`,
        { method: "POST" },
      );
      setCredential({
        email: grant.email || grant.display_name || grant.member_id,
        kubeconfig: data.kubeconfig ?? "",
        expires_at: data.expires_at ?? "",
      });
    } catch (err) {
      setError(errorInfo(err, "签发凭据失败"));
    } finally {
      setIssuing(false);
    }
  }

  function updateNamespaceRow(index: number, patch: Partial<NamespaceRole>) {
    setNamespaceRoles((current) => current.map((item, i) => (i === index ? { ...item, ...patch } : item)));
  }

  return (
    <div className="stack-md">
      <ApiErrorBanner error={error} />
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn" disabled={!canManage} onClick={openCreate}>
          添加
        </button>
        <button
          type="button"
          className="btn-secondary"
          disabled={!canManage || selected.length === 0}
          onClick={() => void remove(selected)}
        >
          删除
        </button>
        <button
          type="button"
          className="btn-secondary"
          disabled={loading}
          onClick={() => {
            setError(null);
            setLoading(true);
            void load()
              .catch((err) => setError(errorInfo(err, "刷新失败")))
              .finally(() => setLoading(false));
          }}
        >
          刷新
        </button>
      </div>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  checked={grants.length > 0 && selected.length === grants.length}
                  onChange={(e) => setSelected(e.target.checked ? grants.map((item) => item.member_id) : [])}
                />
              </th>
              <th>英文名称</th>
              <th>名称</th>
              <th>集群角色</th>
              <th>命名空间</th>
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
              grants.map((item) => (
                <tr key={item.member_id}>
                  <td>
                    <input
                      type="checkbox"
                      checked={selected.includes(item.member_id)}
                      onChange={(e) =>
                        setSelected((current) =>
                          e.target.checked
                            ? [...current, item.member_id]
                            : current.filter((id) => id !== item.member_id),
                        )
                      }
                    />
                  </td>
                  <td>{item.email || "-"}</td>
                  <td>{item.display_name || "-"}</td>
                  <GrantRoleSummary grant={item} />
                  <td className="whitespace-nowrap">
                    <button
                      type="button"
                      className="k8s-action-link"
                      disabled={!canManage || issuing}
                      onClick={() => void issueKubeconfig(item)}
                    >
                      凭据
                    </button>
                    <span className="text-muted"> / </span>
                    <button
                      type="button"
                      className="k8s-action-link"
                      disabled={!canManage}
                      onClick={() => openEdit(item)}
                    >
                      变更
                    </button>
                    <span className="text-muted"> / </span>
                    <button
                      type="button"
                      className="k8s-action-link"
                      disabled={!canManage}
                      onClick={() => void remove([item.member_id])}
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            {!loading && grants.length === 0 ? (
              <tr>
                <td colSpan={6}>
                  <K8sEmptyState variant="empty" title="暂无已授权成员" description="点击「添加」为工作区成员授予集群访问。" compact />
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <Modal
        nested
        open={showForm}
        size="lg"
        title={editing ? "变更用户授权" : "添加用户"}
        description="集群角色覆盖全集群；命名空间角色仅在所选命名空间生效（RoleRef 仍为 ClusterRole）。"
        onClose={closeForm}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" disabled={saving} onClick={closeForm}>
              取消
            </button>
            <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void save()}>
              {saving ? "保存中…" : "确定"}
            </button>
          </div>
        }
      >
        <div className="stack-md">
          <ApiErrorBanner error={formError} />
          {editing ? (
            <div>
              <span className="label">成员</span>
              <p className="text-sm">
                {editing.email} / {editing.display_name}
              </p>
            </div>
          ) : (
            <div className="stack-sm">
              <span className="label">工作区成员</span>
              <input
                className="input"
                type="search"
                placeholder="按邮箱或名称搜索…"
                value={memberQuery}
                onChange={(e) => setMemberQuery(e.target.value)}
              />
              <div className="k8s-role-list">
                {addableMembers.length === 0 ? (
                  <p className="text-sm text-muted">没有可添加的工作区成员</p>
                ) : null}
                {addableMembers.map((item) => (
                  <label key={item.member_id} className="flex items-center gap-2 py-1 text-sm">
                    <input
                      type="checkbox"
                      checked={pickedMembers.includes(item.member_id)}
                      onChange={() => setPickedMembers((current) => toggleValue(current, item.member_id))}
                    />
                    <span>
                      {item.email}
                      <span className="text-muted"> / {item.display_name}</span>
                    </span>
                  </label>
                ))}
              </div>
              {pickedMembers.length > 0 ? (
                <p className="text-xs text-muted">已选 {pickedMembers.length} 人</p>
              ) : null}
            </div>
          )}

          <div className="stack-sm">
            <span className="label">集群角色</span>
            <RoleChipPicker roles={roles} selected={clusterRoles} onChange={setClusterRoles} disabled={!canManage} />
          </div>

          <div className="stack-sm">
            <div className="flex items-center justify-between gap-2">
              <span className="label">命名空间角色</span>
              <button
                type="button"
                className="k8s-action-link"
                disabled={!canManage}
                onClick={() =>
                  setNamespaceRoles((current) => [
                    ...current,
                    { namespace: namespaces[0] ?? "default", roles: [] },
                  ])
                }
              >
                添加一组
              </button>
            </div>
            {namespaceRoles.length === 0 ? (
              <p className="text-xs text-muted">可选。仅需命名空间内权限时再添加。</p>
            ) : null}
            {namespaceRoles.map((row, index) => (
              <div key={`${row.namespace}-${index}`} className="stack-sm rounded border p-3">
                <div className="flex flex-wrap items-end gap-2">
                  <label className="min-w-[10rem] flex-1">
                    <span className="label">命名空间</span>
                    <select
                      className="input"
                      value={row.namespace}
                      disabled={!canManage}
                      onChange={(e) => updateNamespaceRow(index, { namespace: e.target.value })}
                    >
                      {(namespaces.length ? namespaces : ["default"]).map((ns) => (
                        <option key={ns} value={ns}>
                          {ns}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    type="button"
                    className="k8s-action-link mb-2"
                    disabled={!canManage}
                    onClick={() => setNamespaceRoles((current) => current.filter((_, i) => i !== index))}
                  >
                    移除
                  </button>
                </div>
                <RoleChipPicker
                  roles={roles}
                  selected={row.roles}
                  disabled={!canManage}
                  onChange={(next) => updateNamespaceRow(index, { roles: next })}
                />
              </div>
            ))}
          </div>
        </div>
      </Modal>

      <Modal
        nested
        open={Boolean(credential)}
        size="lg"
        title={credential ? `用户凭据 · ${credential.email}` : "用户凭据"}
        description={
          credential
            ? `ServiceAccount Token，有效期至 ${credential.expires_at || "24 小时后"}。Token 不会入库，关闭后需重新签发。`
            : undefined
        }
        onClose={() => setCredential(null)}
        footer={
          <div className="modal-footer-actions">
            <button
              type="button"
              className="btn-secondary"
              disabled={!credential?.kubeconfig}
              onClick={() => credential && void navigator.clipboard.writeText(credential.kubeconfig)}
            >
              复制
            </button>
            <button
              type="button"
              className="btn-secondary"
              disabled={!credential?.kubeconfig}
              onClick={() => {
                if (!credential?.kubeconfig) return;
                const blob = new Blob([credential.kubeconfig], { type: "text/yaml;charset=utf-8" });
                const url = URL.createObjectURL(blob);
                const link = document.createElement("a");
                link.href = url;
                link.download = `${credential.email}.kubeconfig`;
                link.click();
                URL.revokeObjectURL(url);
              }}
            >
              下载
            </button>
            <button type="button" className="btn" onClick={() => setCredential(null)}>
              关闭
            </button>
          </div>
        }
      >
        <textarea className="input font-mono text-xs" rows={14} readOnly value={credential?.kubeconfig ?? ""} />
      </Modal>
      {confirmDialog}
    </div>
  );
}
