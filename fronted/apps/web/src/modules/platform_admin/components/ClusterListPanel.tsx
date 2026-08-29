"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { platform, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import {
  isCurrentWorkspaceRequest,
  selectWorkspaceAuthorization,
  type WorkspaceRequestIdentity,
} from "./cluster-authorization-state";

interface ClusterListPanelProps {
  variant?: "platform" | "default";
}

export function ClusterListPanel({ variant = "default" }: ClusterListPanelProps) {
  const isPlatform = variant === "platform";
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [clusterName, setClusterName] = useState("");
  const [server, setServer] = useState("");
  const [workspaceId, setWorkspaceId] = useState("");
  const [workspaceClusters, setWorkspaceClusters] = useState<AuthorizedClusterVM[]>([]);
  const [workspaceLoading, setWorkspaceLoading] = useState(false);
  const [workspaceError, setWorkspaceError] = useState<string | null>(null);
  const [workspaces, setWorkspaces] = useState<WorkspaceVM[]>([]);
  const [workspacesLoading, setWorkspacesLoading] = useState(true);
  const mountedRef = useRef(true);
  const workspaceIdentityRef = useRef<WorkspaceRequestIdentity>({
    workspaceId: "",
    generation: 0,
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const nextItems = await platform.listClusters();
      if (mountedRef.current) setItems(nextItems);
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
      }
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    void load();
    void tenant
      .listWorkspaces()
      .then((nextWorkspaces) => {
        if (!mountedRef.current) return;
        setWorkspaces(nextWorkspaces);
        if (nextWorkspaces[0]) selectWorkspace(nextWorkspaces[0].id);
      })
      .catch((err) => {
        if (mountedRef.current) {
          setWorkspaceError(err instanceof BffError ? err.message : "加载工作空间失败");
        }
      })
      .finally(() => {
        if (mountedRef.current) setWorkspacesLoading(false);
      });
    return () => {
      mountedRef.current = false;
    };
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    try {
      await platform.createCluster({ cluster_name: clusterName, server });
      if (!mountedRef.current) return;
      setOpen(false);
      setClusterName("");
      setServer("");
      await load();
      notifyEntitlementsUpdated();
    } catch (err) {
      if (mountedRef.current) setError(err instanceof BffError ? err.message : "创建失败");
    }
  }

  function currentWorkspaceRequest(request: WorkspaceRequestIdentity) {
    return (
      mountedRef.current &&
      isCurrentWorkspaceRequest(workspaceIdentityRef.current, request)
    );
  }

  async function loadWorkspaceClusters(request: WorkspaceRequestIdentity) {
    try {
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "加载工作空间授权失败");
      }
    } finally {
      if (currentWorkspaceRequest(request)) setWorkspaceLoading(false);
    }
  }

  function selectWorkspace(nextWorkspaceId: string) {
    const next = selectWorkspaceAuthorization(
      {
        workspaceId,
        generation: workspaceIdentityRef.current.generation,
        items: workspaceClusters,
        error: workspaceError,
      },
      nextWorkspaceId,
    );
    workspaceIdentityRef.current = {
      workspaceId: next.workspaceId,
      generation: next.generation,
    };
    setWorkspaceId(next.workspaceId);
    setWorkspaceClusters(next.items);
    setWorkspaceError(next.error);
    setWorkspaceLoading(Boolean(next.loading));
    if (next.workspaceId) {
      void loadWorkspaceClusters({
        workspaceId: next.workspaceId,
        generation: next.generation,
      });
    }
  }

  async function grantCluster(clusterId: string) {
    const request = { ...workspaceIdentityRef.current };
    if (!request.workspaceId) return;
    setWorkspaceLoading(true);
    setWorkspaceError(null);
    try {
      await platform.grantWorkspaceCluster(request.workspaceId, clusterId);
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "授权失败");
      }
    } finally {
      if (currentWorkspaceRequest(request)) setWorkspaceLoading(false);
    }
  }

  async function revokeCluster(clusterId: string) {
    const request = { ...workspaceIdentityRef.current };
    if (!request.workspaceId) return;
    setWorkspaceLoading(true);
    setWorkspaceError(null);
    try {
      await platform.revokeWorkspaceCluster(request.workspaceId, clusterId);
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "撤销授权失败");
      }
    } finally {
      if (currentWorkspaceRequest(request)) setWorkspaceLoading(false);
    }
  }

  const registerBtn = (
    <button type="button" className="btn platform-add-btn" onClick={() => setOpen(true)}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <path d="M12 5v14M5 12h14" strokeLinecap="round" />
      </svg>
      注册集群
    </button>
  );

  const body = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>加载中…</div>
      ) : items.length === 0 ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>
          暂无集群，点击「注册集群」添加
        </div>
      ) : (
        <div className={isPlatform ? "platform-table-wrap" : "table-wrap"}>
          <table className={isPlatform ? "platform-table" : "data-table text-sm"}>
            <thead>
              <tr>
                <th>名称</th>
                <th>API Server</th>
                <th>构建环境</th>
                <th>优先级</th>
                {isPlatform ? <th>工作空间授权</th> : null}
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={String(row.id)}>
                  <td>{String(row.cluster_name ?? row.name ?? "-")}</td>
                  <td className="font-mono text-xs">{String(row.server ?? row.api_server ?? "-")}</td>
                  <td>{row.is_build_env ? "是" : "否"}</td>
                  <td>{String(row.priority_id ?? "-")}</td>
                  {isPlatform ? (
                    <td>
                      <button
                        type="button"
                        className="btn-secondary"
                        disabled={workspaceLoading || !workspaceId}
                        onClick={() => void grantCluster(String(row.id))}
                      >
                        授权给该工作空间
                      </button>
                    </td>
                  ) : null}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {!loading && items.length > 0 && isPlatform ? (
        <div className="platform-table-footer">
          <span className="text-sm text-muted">共 {items.length} 个集群</span>
        </div>
      ) : null}
    </>
  );

  return (
    <div className={isPlatform ? "platform-orgs-page" : "stack-md"}>
      {isPlatform ? (
        <section className="platform-table-card platform-orgs-card">
          <PlatformPanelHead
            title="集群管理"
            description="多集群接入与 Kubeconfig 注册。"
            action={registerBtn}
          />
          <div className="platform-orgs-card-body">{body}</div>
          <div className="platform-orgs-card-body stack-md">
            <h3>授权工作空间</h3>
            <label className="stack-sm">
              <span>工作空间</span>
              <select
                className="input"
                value={workspaceId}
                onChange={(event) => selectWorkspace(event.target.value)}
                disabled={workspacesLoading}
              >
                <option value="">
                  {workspacesLoading ? "加载工作空间中…" : "请选择工作空间"}
                </option>
                {workspaces.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name} ({item.id})
                  </option>
                ))}
              </select>
            </label>
            {workspaceError ? <InlineAlert tone="error">{workspaceError}</InlineAlert> : null}
            {workspaceLoading ? <p className="text-muted">加载授权中…</p> : null}
            {workspaceClusters.length > 0 ? (
              <ul className="stack-sm">
                {workspaceClusters.map((cluster) => (
                  <li key={cluster.id} className="flex items-center justify-between gap-2">
                    <span>{cluster.cluster_name}</span>
                    <button
                      type="button"
                      className="btn-secondary"
                      disabled={workspaceLoading}
                      onClick={() => void revokeCluster(cluster.id)}
                    >
                      撤销授权
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-muted">
                {workspaceId ? "该工作空间暂无集群授权。" : "请选择工作空间。"}
              </p>
            )}
          </div>
        </section>
      ) : (
        <>
          <div className="flex justify-end">{registerBtn}</div>
          {body}
        </>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="注册 K8s 集群">
        <form onSubmit={onCreate} className="stack-md">
          <input className="input" placeholder="集群名称" value={clusterName} onChange={(e) => setClusterName(e.target.value)} required />
          <input className="input" placeholder="API Server URL" value={server} onChange={(e) => setServer(e.target.value)} />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>取消</button>
            <button type="submit" className="btn">保存</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
