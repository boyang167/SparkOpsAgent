"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { parseYamlEditorDocument } from "../yaml-document";
import type { KubernetesCluster } from "../cluster-types";

type Tab = "kubeconfig" | "users" | "roles";

type K8sList<T> = { items?: T[] };

type Binding = {
  metadata?: { name?: string };
  roleRef?: { kind?: string; name?: string };
  subjects?: { kind?: string; name?: string; namespace?: string }[];
};

type Role = {
  metadata?: { name?: string };
  rules?: { apiGroups?: string[]; resources?: string[]; verbs?: string[] }[];
};

function proxyURL(clusterId: string, path: string) {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/${path}`;
}

function clusterRolesURL(clusterId: string, name?: string) {
  const base = proxyURL(clusterId, "apis/rbac.authorization.k8s.io/v1/clusterroles");
  return name ? `${base}/${encodeURIComponent(name)}` : base;
}

function clusterRoleBindingsURL(clusterId: string, name?: string) {
  const base = proxyURL(clusterId, "apis/rbac.authorization.k8s.io/v1/clusterrolebindings");
  return name ? `${base}/${encodeURIComponent(name)}` : base;
}

const DEFAULT_ROLE_YAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sparkops-view
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
`;

export function ClusterAuthorizeDialog({
  cluster,
  canManage,
}: {
  cluster: KubernetesCluster;
  canManage: boolean;
}) {
  const [tab, setTab] = useState<Tab>("kubeconfig");
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [kubeconfig, setKubeconfig] = useState("");
  const [bindings, setBindings] = useState<Binding[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [saName, setSaName] = useState("");
  const [saNamespace, setSaNamespace] = useState("default");
  const [roleName, setRoleName] = useState("view");
  const [roleYaml, setRoleYaml] = useState(DEFAULT_ROLE_YAML);

  const loadKubeconfig = useCallback(async () => {
    const data = await k8sFetch<{ kubeconfig?: string }>(
      `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(cluster.id)}/kubeconfig`,
    );
    setKubeconfig(data.kubeconfig ?? "");
  }, [cluster.id]);

  const loadUsers = useCallback(async () => {
    const [bindingList, nsList] = await Promise.all([
      k8sFetch<K8sList<Binding>>(clusterRoleBindingsURL(cluster.id)),
      k8sFetch<K8sList<{ metadata?: { name?: string } }>>(proxyURL(cluster.id, "api/v1/namespaces")),
    ]);
    setBindings(bindingList.items ?? []);
    setNamespaces((nsList.items ?? []).map((item) => item.metadata?.name).filter(Boolean) as string[]);
  }, [cluster.id]);

  const loadRoles = useCallback(async () => {
    const data = await k8sFetch<K8sList<Role>>(clusterRolesURL(cluster.id));
    setRoles(data.items ?? []);
  }, [cluster.id]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setError(null);
      try {
        if (tab === "kubeconfig") await loadKubeconfig();
        if (tab === "users") await loadUsers();
        if (tab === "roles") await loadRoles();
      } catch (err) {
        if (!cancelled) setError(errorInfo(err, "加载授权信息失败"));
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [tab, loadKubeconfig, loadUsers, loadRoles]);

  async function copyKubeconfig() {
    if (!kubeconfig) return;
    await navigator.clipboard.writeText(kubeconfig);
  }

  function downloadKubeconfig() {
    const blob = new Blob([kubeconfig], { type: "text/yaml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${cluster.name || cluster.id}.kubeconfig`;
    link.click();
    URL.revokeObjectURL(url);
  }

  async function createUser() {
    if (!canManage || !saName.trim() || !saNamespace.trim() || !roleName.trim()) {
      setError({ code: "INVALID_REQUEST", message: "请填写 ServiceAccount 名称、命名空间和 ClusterRole" });
      return;
    }
    setError(null);
    const name = saName.trim();
    const ns = saNamespace.trim();
    const bindingName = `sparkops-${ns}-${name}`.slice(0, 63);
    try {
      await k8sFetch(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(cluster.id)}/proxy/api/v1/namespaces/${encodeURIComponent(ns)}/serviceaccounts`,
        {
          method: "POST",
          body: JSON.stringify({
            apiVersion: "v1",
            kind: "ServiceAccount",
            metadata: { name, namespace: ns },
          }),
        },
      ).catch(() => undefined);
      await k8sFetch(clusterRoleBindingsURL(cluster.id), {
        method: "POST",
        body: JSON.stringify({
          apiVersion: "rbac.authorization.k8s.io/v1",
          kind: "ClusterRoleBinding",
          metadata: { name: bindingName },
          subjects: [{ kind: "ServiceAccount", name, namespace: ns }],
          roleRef: { apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: roleName.trim() },
        }),
      });
      setSaName("");
      await loadUsers();
    } catch (err) {
      setError(errorInfo(err, "创建用户绑定失败"));
    }
  }

  async function deleteBinding(name: string) {
    if (!canManage) return;
    if (!window.confirm(`确定删除 ClusterRoleBinding「${name}」？`)) return;
    try {
      await k8sFetch(clusterRoleBindingsURL(cluster.id, name), { method: "DELETE" });
      await loadUsers();
    } catch (err) {
      setError(errorInfo(err, "删除绑定失败"));
    }
  }

  async function applyRole() {
    if (!canManage) return;
    try {
      const doc = parseYamlEditorDocument(roleYaml);
      const name = String((doc.metadata as { name?: string } | undefined)?.name ?? "").trim();
      if (!name) {
        setError({ code: "INVALID_REQUEST", message: "ClusterRole YAML 必须包含 metadata.name" });
        return;
      }
      const body = {
        ...doc,
        apiVersion: "rbac.authorization.k8s.io/v1",
        kind: "ClusterRole",
        metadata: { ...(typeof doc.metadata === "object" && doc.metadata ? doc.metadata : {}), name },
      };
      try {
        await k8sFetch(clusterRolesURL(cluster.id), { method: "POST", body: JSON.stringify(body) });
      } catch {
        await k8sFetch(clusterRolesURL(cluster.id, name), { method: "PUT", body: JSON.stringify(body) });
      }
      await loadRoles();
    } catch (err) {
      setError(errorInfo(err, "保存角色失败"));
    }
  }

  async function deleteRole(name: string) {
    if (!canManage) return;
    if (!window.confirm(`确定删除 ClusterRole「${name}」？`)) return;
    try {
      await k8sFetch(clusterRolesURL(cluster.id, name), { method: "DELETE" });
      await loadRoles();
    } catch (err) {
      setError(errorInfo(err, "删除角色失败"));
    }
  }

  return (
    <div className="stack-md">
      <div className="k8s-tabs">
        {(
          [
            ["kubeconfig", "连接信息"],
            ["users", "用户"],
            ["roles", "角色"],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            className={tab === id ? "k8s-tab k8s-tab-active" : "k8s-tab"}
            onClick={() => setTab(id)}
          >
            {label}
          </button>
        ))}
      </div>
      <ApiErrorBanner error={error} />
      {tab === "kubeconfig" ? (
        <div className="stack-md">
          <p className="text-sm text-muted">导出当前工作区接入该集群的平台凭证。吊销请使用「删除」撤销工作区授权。</p>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn-secondary" disabled={!kubeconfig} onClick={() => void copyKubeconfig()}>
              复制
            </button>
            <button type="button" className="btn-secondary" disabled={!kubeconfig} onClick={downloadKubeconfig}>
              下载
            </button>
          </div>
          <textarea className="input font-mono text-xs" rows={16} readOnly value={kubeconfig} />
        </div>
      ) : null}
      {tab === "users" ? (
        <div className="stack-md">
          <div className="grid gap-3 md:grid-cols-4">
            <label>
              <span className="label">ServiceAccount</span>
              <input className="input" value={saName} onChange={(e) => setSaName(e.target.value)} />
            </label>
            <label>
              <span className="label">命名空间</span>
              <select className="input" value={saNamespace} onChange={(e) => setSaNamespace(e.target.value)}>
                {(namespaces.length ? namespaces : ["default"]).map((ns) => (
                  <option key={ns} value={ns}>
                    {ns}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span className="label">ClusterRole</span>
              <input className="input" value={roleName} onChange={(e) => setRoleName(e.target.value)} />
            </label>
            <div className="flex items-end">
              <button type="button" className="btn" disabled={!canManage} onClick={() => void createUser()}>
                添加
              </button>
            </div>
          </div>
          <div className="table-wrap">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>Binding</th>
                  <th>主体</th>
                  <th>角色</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {bindings.map((item) => (
                  <tr key={item.metadata?.name}>
                    <td>{item.metadata?.name}</td>
                    <td>
                      {(item.subjects ?? [])
                        .map((subject) => `${subject.kind}/${subject.namespace ?? ""}/${subject.name}`)
                        .join(", ") || "-"}
                    </td>
                    <td>{item.roleRef?.name || "-"}</td>
                    <td>
                      <button
                        type="button"
                        className="k8s-action-link"
                        disabled={!canManage}
                        onClick={() => item.metadata?.name && void deleteBinding(item.metadata.name)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
                {bindings.length === 0 ? (
                  <tr>
                    <td colSpan={4}>
                      <div className="empty-state">暂无 ClusterRoleBinding</div>
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}
      {tab === "roles" ? (
        <div className="stack-md">
          <textarea className="input font-mono text-xs" rows={10} value={roleYaml} onChange={(e) => setRoleYaml(e.target.value)} />
          <div>
            <button type="button" className="btn" disabled={!canManage} onClick={() => void applyRole()}>
              创建 / 更新
            </button>
          </div>
          <div className="table-wrap">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>规则数</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {roles.map((item) => (
                  <tr key={item.metadata?.name}>
                    <td>{item.metadata?.name}</td>
                    <td>{item.rules?.length ?? 0}</td>
                    <td>
                      <button
                        type="button"
                        className="k8s-action-link"
                        disabled={!canManage}
                        onClick={() => item.metadata?.name && void deleteRole(item.metadata.name)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}
    </div>
  );
}
