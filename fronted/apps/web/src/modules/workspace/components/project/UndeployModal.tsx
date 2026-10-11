"use client";

import { useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { onlineEnvironmentRows, type EnvironmentDeployRow } from "./project-environment-status";

interface UndeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  rows: EnvironmentDeployRow[];
  initialClusterId?: string;
  onClose: () => void;
  onStarted: (run: DeploymentRunVM) => void;
}

export function UndeployModal({
  open,
  workspaceId,
  project,
  rows,
  initialClusterId,
  onClose,
  onStarted,
}: UndeployModalProps) {
  const onlineRows = useMemo(() => onlineEnvironmentRows(rows), [rows]);
  const [clusterId, setClusterId] = useState("");
  const [deleteConfig, setDeleteConfig] = useState(false);
  const [deleteStorage, setDeleteStorage] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setClusterId(initialClusterId ?? onlineRows[0]?.clusterId ?? "");
    setDeleteConfig(false);
    setDeleteStorage(false);
    setConfirmName("");
    setError(null);
  }, [open, initialClusterId, onlineRows]);

  const selected = onlineRows.find((row) => row.clusterId === clusterId) ?? null;
  const namespace = selected?.latest?.deploy_namespace?.trim() || project.namespace || "—";
  const canSubmit = Boolean(selected) && confirmName === project.name && !submitting;

  async function submit() {
    if (!selected || !canSubmit) return;
    setSubmitting(true);
    setError(null);
    try {
      const run = await delivery.undeploy(project.id, workspaceId, {
        cluster_id: selected.clusterId,
        confirm_name: confirmName,
        delete_config: deleteConfig,
        delete_storage: deleteStorage,
      });
      onStarted(run);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "下线失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!submitting) onClose();
      }}
      title={selected ? `下线 ${project.name} · ${selected.clusterName}` : `下线 ${project.name}`}
      description="删除该环境中的工作负载，项目配置与历史记录保留，可随时重新发布。"
      size="md"
      footer={
        <>
          <button type="button" className="btn-secondary" disabled={submitting} onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn-danger-solid"
            disabled={!canSubmit}
            onClick={() => void submit()}
          >
            {submitting ? "下线中…" : "下线"}
          </button>
        </>
      }
    >
      <div className="stack-md">
        {onlineRows.length === 0 ? (
          <InlineAlert tone="info">当前没有在线环境。</InlineAlert>
        ) : (
          <>
            {onlineRows.length > 1 ? (
              <label className="stack-sm">
                <span className="text-sm font-medium">环境</span>
                <select
                  className="input"
                  value={clusterId}
                  onChange={(e) => setClusterId(e.target.value)}
                >
                  {onlineRows.map((row) => (
                    <option key={row.clusterId} value={row.clusterId}>
                      {row.clusterName}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <div className="stack-sm">
              <span className="text-sm font-medium">将删除以下资源</span>
              <ul className="undeploy-resource-list">
                <li>Deployment/{project.name}</li>
                <li>Service/{project.name}</li>
                <li>Ingress（app={project.name}）</li>
                <li className="text-muted">命名空间：{namespace}</li>
              </ul>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={deleteConfig}
                onChange={(e) => setDeleteConfig(e.target.checked)}
              />
              同时删除配置（ConfigMap / Secret）
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={deleteStorage}
                onChange={(e) => setDeleteStorage(e.target.checked)}
              />
              同时删除存储卷（PVC）
            </label>
            {deleteStorage ? <p className="undeploy-danger-hint">PVC 删除后数据不可恢复。</p> : null}
            <label className="stack-sm">
              <span className="text-sm">
                输入项目名 <strong>{project.name}</strong> 以确认
              </span>
              <input
                className="input"
                value={confirmName}
                onChange={(e) => setConfirmName(e.target.value)}
                autoComplete="off"
                aria-label="确认项目名"
              />
            </label>
          </>
        )}
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </div>
    </Modal>
  );
}
