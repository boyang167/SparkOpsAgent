"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { platform, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";
import { ApiErrorBanner } from "@/modules/plugins/kubernetes/ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "@/modules/plugins/kubernetes/api";
import {
  ClusterCreateForm,
  clusterSettingsPayload,
  EMPTY_CLUSTER_CREATE_FORM,
  type ClusterCreateFormValue,
} from "@/modules/plugins/kubernetes/ClusterCreateForm";
import type { KubernetesCluster } from "@/modules/plugins/kubernetes/cluster-types";
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
  const [createForm, setCreateForm] = useState<ClusterCreateFormValue>(EMPTY_CLUSTER_CREATE_FORM);
  const [createSaving, setCreateSaving] = useState(false);
  const [createError, setCreateError] = useState<ApiErrorInfo | null>(null);
  const [workspaceId, setWorkspaceId] = useState("");
  const [workspaceClusters, setWorkspaceClusters] = useState<AuthorizedClusterVM[]>([]);
  const [workspaceLoading, setWorkspaceLoading] = useState(false);
  const [workspaceError, setWorkspaceError] = useState<string | null>(null);
  const [workspaces, setWorkspaces] = useState<WorkspaceVM[]>([]);
  const [workspacesLoading, setWorkspacesLoading] = useState(true);
  const [actingClusterId, setActingClusterId] = useState<string | null>(null);
  const mountedRef = useRef(true);
  const workspaceIdentityRef = useRef<WorkspaceRequestIdentity>({
    workspaceId: "",
    generation: 0,
  });

  const authorizedIds = useMemo(
    () => new Set(workspaceClusters.map((cluster) => cluster.id)),
    [workspaceClusters],
  );

  const selectedWorkspace = useMemo(
    () => workspaces.find((item) => item.id === workspaceId),
    [workspaceId, workspaces],
  );

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

  function openJoinModal() {
    if (!workspaceId) {
      setWorkspaceError("请先选择工作空间");
      return;
    }
    setCreateForm(EMPTY_CLUSTER_CREATE_FORM);
    setCreateError(null);
    setOpen(true);
  }

  async function saveJoin() {
    const request = { ...workspaceIdentityRef.current };
    if (!request.workspaceId) {
      setCreateError({ code: "INVALID_REQUEST", message: "请先选择工作空间" });
      return;
    }
    if (!createForm.name.trim() || !createForm.kube_config.trim() || !createForm.api_address.trim()) {
      setCreateError({ code: "INVALID_REQUEST", message: "请填写名称、集群凭证和接口地址" });
      return;
    }
    setCreateSaving(true);
    setCreateError(null);
    try {
      const created = await platform.registerWorkspaceCluster(request.workspaceId, {
        cluster_name: createForm.name.trim(),
        api_address: createForm.api_address.trim(),
        server: createForm.api_address.trim(),
        kube_type: createForm.kube_type,
        kube_config: createForm.kube_config,
      });
      const payload = clusterSettingsPayload(createForm);
      if (Object.values(payload).some((value) => value.trim() !== "")) {
        await k8sFetch<KubernetesCluster>(
          `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(created.id)}/settings`,
          { method: "PUT", body: JSON.stringify(payload) },
        );
      }
      if (!mountedRef.current) return;
      setOpen(false);
      setCreateForm(EMPTY_CLUSTER_CREATE_FORM);
      await load();
      if (currentWorkspaceRequest(request)) {
        setWorkspaceLoading(true);
        await loadWorkspaceClusters(request);
      }
      notifyEntitlementsUpdated();
    } catch (err) {
      if (mountedRef.current) {
        setCreateError(
          err instanceof BffError
            ? { code: err.code || "REGISTER_FAILED", message: err.message }
            : errorInfo(err, "接入集群失败"),
        );
      }
    } finally {
      if (mountedRef.current) setCreateSaving(false);
    }
  }

  function currentWorkspaceRequest(request: WorkspaceRequestIdentity) {
    return mountedRef.current && isCurrentWorkspaceRequest(workspaceIdentityRef.current, request);
  }

  async function loadWorkspaceClusters(request: WorkspaceRequestIdentity) {
    try {
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "加载授权失败");
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
    setActingClusterId(clusterId);
    setWorkspaceError(null);
    try {
      await platform.grantWorkspaceCluster(request.workspaceId, clusterId);
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
      notifyEntitlementsUpdated();
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "授权失败");
      }
    } finally {
      if (currentWorkspaceRequest(request)) setActingClusterId(null);
    }
  }

  async function revokeCluster(clusterId: string) {
    const request = { ...workspaceIdentityRef.current };
    if (!request.workspaceId) return;
    setActingClusterId(clusterId);
    setWorkspaceError(null);
    try {
      await platform.revokeWorkspaceCluster(request.workspaceId, clusterId);
      const nextItems = await platform.listWorkspaceClusters(request.workspaceId);
      if (currentWorkspaceRequest(request)) setWorkspaceClusters(nextItems);
      notifyEntitlementsUpdated();
    } catch (err) {
      if (currentWorkspaceRequest(request)) {
        setWorkspaceError(err instanceof BffError ? err.message : "撤销失败");
      }
    } finally {
      if (currentWorkspaceRequest(request)) setActingClusterId(null);
    }
  }

  const joinBtn = (
    <button
      type="button"
      className="btn platform-add-btn"
      disabled={isPlatform && !workspaceId}
      onClick={openJoinModal}
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <path d="M12 5v14M5 12h14" strokeLinecap="round" />
      </svg>
      接入集群
    </button>
  );

  const platformToolbar = isPlatform ? (
    <div className="platform-cluster-toolbar">
      <label className="platform-cluster-workspace-field">
        <span className="label">工作空间</span>
        <select
          className="input"
          value={workspaceId}
          onChange={(event) => selectWorkspace(event.target.value)}
          disabled={workspacesLoading}
          aria-label="选择工作空间"
        >
          <option value="">{workspacesLoading ? "加载中…" : "请选择"}</option>
          {workspaces.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
      </label>
      <div className="platform-cluster-toolbar-meta">
        {workspaceId ? (
          <span className="text-xs text-muted">
            {workspaceLoading
              ? "同步中…"
              : `${authorizedIds.size} / ${items.length} 已授权`}
          </span>
        ) : (
          <span className="text-xs text-muted">选择工作空间后分配集群</span>
        )}
      </div>
    </div>
  ) : null;

  const body = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {isPlatform && workspaceError ? <InlineAlert tone="error">{workspaceError}</InlineAlert> : null}
      {loading ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>加载中…</div>
      ) : items.length === 0 ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>
          暂无集群，点击「接入集群」添加
        </div>
      ) : (
        <div className={isPlatform ? "platform-table-wrap k8s-dialog-table" : "table-wrap"}>
          <table className={isPlatform ? "platform-table" : "data-table text-sm"}>
            <thead>
              <tr>
                <th>名称</th>
                <th>API Server</th>
                {!isPlatform ? (
                  <>
                    <th>构建环境</th>
                    <th>优先级</th>
                  </>
                ) : (
                  <th className="col-compact">状态</th>
                )}
                {isPlatform ? <th className="col-actions">操作</th> : null}
              </tr>
            </thead>
            <tbody>
              {items.map((row) => {
                const clusterId = String(row.id);
                const authorized = authorizedIds.has(clusterId);
                const acting = actingClusterId === clusterId;
                return (
                  <tr key={clusterId}>
                    <td>{String(row.cluster_name ?? row.name ?? "-")}</td>
                    <td className="font-mono text-xs">{String(row.server ?? row.api_server ?? "-")}</td>
                    {!isPlatform ? (
                      <>
                        <td>{row.is_build_env ? "是" : "否"}</td>
                        <td>{String(row.priority_id ?? "-")}</td>
                      </>
                    ) : (
                      <td className="col-compact">
                        {!workspaceId ? (
                          <span className="text-muted text-xs">—</span>
                        ) : workspaceLoading ? (
                          <span className="text-muted text-xs">…</span>
                        ) : authorized ? (
                          <span className="k8s-meta-tag">已授权</span>
                        ) : (
                          <span className="text-muted text-xs">未授权</span>
                        )}
                      </td>
                    )}
                    {isPlatform ? (
                      <td className="col-actions">
                        {!workspaceId ? (
                          <span className="text-muted text-xs">—</span>
                        ) : authorized ? (
                          <button
                            type="button"
                            className="btn-ghost text-sm text-danger"
                            disabled={workspaceLoading || acting}
                            onClick={() => void revokeCluster(clusterId)}
                          >
                            {acting ? "处理中…" : "撤销"}
                          </button>
                        ) : (
                          <button
                            type="button"
                            className="btn-ghost text-sm"
                            disabled={workspaceLoading || acting}
                            onClick={() => void grantCluster(clusterId)}
                          >
                            {acting ? "处理中…" : "授权"}
                          </button>
                        )}
                      </td>
                    ) : null}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {!loading && items.length > 0 && isPlatform ? (
        <div className="platform-table-footer">
          <span className="text-sm text-muted">
            共 {items.length} 个集群
            {selectedWorkspace ? ` · ${selectedWorkspace.name}` : ""}
          </span>
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
            description="接入集群到平台，并授权当前工作空间。"
            action={joinBtn}
          />
          <div className="platform-orgs-card-body stack-sm">
            {platformToolbar}
            {body}
          </div>
        </section>
      ) : (
        <>
          <div className="flex justify-end">{joinBtn}</div>
          {body}
        </>
      )}
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="接入集群"
        size="xl"
        panelClassName="k8s-cluster-form-modal"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>
              取消
            </button>
            <button type="button" className="btn" disabled={createSaving} onClick={() => void saveJoin()}>
              {createSaving ? "保存中…" : "接入"}
            </button>
          </div>
        }
      >
        <ApiErrorBanner error={createError} />
        <ClusterCreateForm value={createForm} onChange={setCreateForm} />
      </Modal>
    </div>
  );
}
