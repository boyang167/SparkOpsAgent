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

function SearchIcon() {
  return (
    <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z" stroke="currentColor" strokeWidth="1.5" />
      <path d="M13.5 13.5 17 17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

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
  const q = query.trim().toLowerCase();
  const filtered = q ? catalog.filter((name) => name.toLowerCase().includes(q)) : [];

  return (
    <div className="k8s-grant-form-section">
      {selected.length > 0 ? (
        <div className="k8s-selected-tags">
          {selected.map((name) => (
            <span key={name} className="k8s-selected-tag">
              {name}
              {!disabled ? (
                <button
                  type="button"
                  className="k8s-selected-tag-remove"
                  aria-label={`移除 ${name}`}
                  onClick={() => onChange(selected.filter((item) => item !== name))}
                >
                  ×
                </button>
              ) : null}
            </span>
          ))}
        </div>
      ) : null}
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
      <label className="projects-search-wrap">
        <SearchIcon />
        <input
          className="projects-search-input"
          type="search"
          placeholder="搜索角色"
          value={query}
          disabled={disabled}
          onChange={(e) => setQuery(e.target.value)}
        />
      </label>
      {q ? (
        <div className="k8s-picker-list">
          {filtered.length === 0 ? <p className="text-xs text-muted px-1 py-2">无匹配</p> : null}
          {filtered.map((name) => (
            <label key={name} className="k8s-picker-row">
              <input
                type="checkbox"
                checked={selected.includes(name)}
                disabled={disabled}
                onChange={() => onChange(toggleValue(selected, name))}
              />
              <span className="k8s-picker-row-label font-mono text-xs truncate" title={name}>
                {name}
              </span>
            </label>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function GrantRoleSummary({ grant }: { grant: ClusterUserGrant }) {
  const nsLabels = (grant.namespace_roles ?? []).flatMap((item) =>
    (item.roles ?? []).map((role) => `${item.namespace}:${role}`),
  );
  const clusterText = (grant.cluster_roles ?? []).join("、") || "—";
  const nsText = nsLabels.join("、") || "—";
  return (
    <>
      <td className="text-xs text-muted font-mono truncate max-w-[10rem]" title={clusterText}>{clusterText}</td>
      <td className="text-xs text-muted font-mono truncate max-w-[10rem]" title={nsText}>{nsText}</td>
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
    <div className="stack-sm">
      <ApiErrorBanner error={error} />
      <div className="k8s-dialog-toolbar">
        <span className="text-xs text-muted">{loading ? "…" : `${grants.length} 人`}</span>
        <div className="k8s-dialog-toolbar-actions">
          {canManage && selected.length > 0 ? (
            <button type="button" className="btn-secondary text-sm" onClick={() => void remove(selected)}>
              删除 ({selected.length})
            </button>
          ) : null}
          <button
            type="button"
            className="btn-secondary text-sm"
            disabled={loading}
            aria-label="刷新"
            onClick={() => {
              setError(null);
              setLoading(true);
              void load()
                .catch((err) => setError(errorInfo(err, "刷新失败")))
                .finally(() => setLoading(false));
            }}
          >
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
            <col style={{ width: "2.25rem" }} />
            <col />
            <col style={{ width: "28%" }} />
            <col style={{ width: "28%" }} />
            <col style={{ width: "8.5rem" }} />
          </colgroup>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  checked={grants.length > 0 && selected.length === grants.length}
                  onChange={(e) => setSelected(e.target.checked ? grants.map((item) => item.member_id) : [])}
                  aria-label="全选"
                />
              </th>
              <th>成员</th>
              <th>集群角色</th>
              <th>命名空间</th>
              <th className="col-actions">操作</th>
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
                      aria-label={`选择 ${item.display_name || item.email}`}
                    />
                  </td>
                  <td className="min-w-0">
                    <div className="k8s-member-cell">
                      <span className="k8s-member-name truncate">{item.display_name || item.email || "—"}</span>
                      {item.display_name && item.email ? (
                        <span className="text-xs text-muted truncate">{item.email}</span>
                      ) : null}
                    </div>
                  </td>
                  <GrantRoleSummary grant={item} />
                  <td className="col-actions">
                    <div className="k8s-row-actions">
                      <button
                        type="button"
                        className="btn-ghost text-sm"
                        disabled={!canManage || issuing}
                        onClick={() => void issueKubeconfig(item)}
                      >
                        凭据
                      </button>
                      <button type="button" className="btn-ghost text-sm" disabled={!canManage} onClick={() => openEdit(item)}>
                        变更
                      </button>
                      <button
                        type="button"
                        className="btn-ghost text-sm"
                        disabled={!canManage}
                        onClick={() => void remove([item.member_id])}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            {!loading && grants.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <K8sEmptyState variant="empty" title="暂无成员" compact />
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
        panelClassName="k8s-cluster-form-modal"
        title={editing ? "变更授权" : "添加成员"}
        onClose={closeForm}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" disabled={saving} onClick={closeForm}>
              取消
            </button>
            <button type="button" className="btn" disabled={!canManage || saving} onClick={() => void save()}>
              {saving ? "保存中…" : "保存"}
            </button>
          </div>
        }
      >
        <div className="k8s-cluster-form">
          <ApiErrorBanner error={formError} />
          {editing ? (
            <section className="k8s-cluster-form-section">
              <header className="k8s-cluster-form-section-head">
                <h3 className="k8s-cluster-form-section-title">成员</h3>
              </header>
              <div className="k8s-member-cell px-0.5">
                <span className="k8s-member-name">{editing.display_name || editing.email}</span>
                {editing.display_name && editing.email ? (
                  <span className="text-xs text-muted">{editing.email}</span>
                ) : null}
              </div>
            </section>
          ) : (
            <section className="k8s-cluster-form-section">
              <header className="k8s-cluster-form-section-head">
                <h3 className="k8s-cluster-form-section-title">成员</h3>
                {pickedMembers.length > 0 ? (
                  <span className="k8s-cluster-form-section-hint">已选 {pickedMembers.length}</span>
                ) : null}
              </header>
              <div className="k8s-cluster-form-section-body stack-sm">
                <label className="projects-search-wrap">
                  <SearchIcon />
                  <input
                    className="projects-search-input"
                    type="search"
                    placeholder="搜索"
                    value={memberQuery}
                    onChange={(e) => setMemberQuery(e.target.value)}
                  />
                </label>
                <div className="k8s-picker-list">
                  {addableMembers.length === 0 ? (
                    <p className="text-xs text-muted px-1 py-2">无可添加成员</p>
                  ) : null}
                  {addableMembers.map((item) => (
                    <label key={item.member_id} className="k8s-picker-row">
                      <input
                        type="checkbox"
                        checked={pickedMembers.includes(item.member_id)}
                        onChange={() => setPickedMembers((current) => toggleValue(current, item.member_id))}
                      />
                      <span className="k8s-picker-row-label">
                        {item.display_name || item.email}
                        {item.display_name && item.email ? (
                          <span className="k8s-picker-row-meta">{item.email}</span>
                        ) : null}
                      </span>
                    </label>
                  ))}
                </div>
              </div>
            </section>
          )}

          <section className="k8s-cluster-form-section">
            <header className="k8s-cluster-form-section-head">
              <h3 className="k8s-cluster-form-section-title">集群角色</h3>
            </header>
            <div className="k8s-cluster-form-section-body">
              <RoleChipPicker roles={roles} selected={clusterRoles} onChange={setClusterRoles} disabled={!canManage} />
            </div>
          </section>

          <section className="k8s-cluster-form-section">
            <header className="k8s-cluster-form-section-head k8s-cluster-form-section-head-row">
              <h3 className="k8s-cluster-form-section-title">命名空间角色</h3>
              <button
                type="button"
                className="btn-ghost text-sm shrink-0"
                disabled={!canManage}
                onClick={() =>
                  setNamespaceRoles((current) => [
                    ...current,
                    { namespace: namespaces[0] ?? "default", roles: [] },
                  ])
                }
              >
                添加
              </button>
            </header>
            {namespaceRoles.length > 0 ? (
              <div className="k8s-cluster-form-section-body stack-sm">
                {namespaceRoles.map((row, index) => (
                  <div key={`${row.namespace}-${index}`} className="stack-sm k8s-form-subsection">
                    <div className="flex flex-wrap items-center gap-2">
                      <select
                        className="input text-sm flex-1 min-w-[8rem]"
                        value={row.namespace}
                        disabled={!canManage}
                        aria-label="命名空间"
                        onChange={(e) => updateNamespaceRow(index, { namespace: e.target.value })}
                      >
                        {(namespaces.length ? namespaces : ["default"]).map((ns) => (
                          <option key={ns} value={ns}>
                            {ns}
                          </option>
                        ))}
                      </select>
                      <button
                        type="button"
                        className="btn-ghost text-sm"
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
            ) : null}
          </section>
        </div>
      </Modal>

      <Modal
        nested
        open={Boolean(credential)}
        size="lg"
        title={credential ? `凭据 · ${credential.email}` : "凭据"}
        description={credential?.expires_at ? `有效期至 ${credential.expires_at}` : undefined}
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
