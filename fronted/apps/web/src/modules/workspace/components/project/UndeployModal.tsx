"use client";

import { useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  environmentVersionLabel,
  onlineEnvironmentRows,
  type EnvironmentDeployRow,
} from "./project-environment-status";

interface UndeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  rows: EnvironmentDeployRow[];
  initialClusterId?: string;
  onClose: () => void;
  onStarted: (run: DeploymentRunVM) => void;
}

interface UndeployOptionProps {
  checked: boolean;
  danger?: boolean;
  title: string;
  description: string;
  onChange: (checked: boolean) => void;
}

function UndeployOption({ checked, danger, title, description, onChange }: UndeployOptionProps) {
  return (
    <label
      className={`undeploy-option${checked ? " is-checked" : ""}${checked && danger ? " is-danger" : ""}`}
    >
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="undeploy-option-copy">
        <span className="undeploy-option-title">{title}</span>
        <span className="undeploy-option-desc">{description}</span>
      </span>
    </label>
  );
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
  const version = selected ? environmentVersionLabel(selected.latest) : "—";
  const confirmed = confirmName === project.name;
  const canSubmit = Boolean(selected) && confirmed && !submitting;

  const resources = [
    { kind: "Deployment", name: project.name },
    { kind: "Service", name: project.name },
    { kind: "Ingress", name: `app=${project.name}` },
    ...(deleteConfig ? [{ kind: "ConfigMap · Secret", name: `app=${project.name}` }] : []),
    ...(deleteStorage ? [{ kind: "PVC", name: `app=${project.name}`, danger: true }] : []),
  ];

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
      title={selected ? `从 ${selected.clusterName} 下线服务` : "下线服务"}
      description="移除该环境中运行的工作负载。项目配置、构建记录与流水线保留，可随时重新发布。"
      size="md"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" disabled={submitting} onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn-danger-solid"
            disabled={!canSubmit}
            onClick={() => void submit()}
          >
            {submitting ? "下线中…" : "确认下线"}
          </button>
        </div>
      }
    >
      {onlineRows.length === 0 ? (
        <InlineAlert tone="info">当前没有在线环境。</InlineAlert>
      ) : (
        <div className="undeploy-modal">
          <section className="undeploy-summary" aria-label="环境信息">
            {onlineRows.length > 1 ? (
              <div className="undeploy-env-switch" role="radiogroup" aria-label="选择环境">
                {onlineRows.map((row) => (
                  <button
                    key={row.clusterId}
                    type="button"
                    role="radio"
                    aria-checked={row.clusterId === clusterId}
                    className={`undeploy-env-pill${row.clusterId === clusterId ? " is-active" : ""}`}
                    onClick={() => setClusterId(row.clusterId)}
                  >
                    {row.clusterName}
                  </button>
                ))}
              </div>
            ) : null}
            <dl className="undeploy-summary-grid">
              <div>
                <dt>项目</dt>
                <dd>{project.name}</dd>
              </div>
              <div>
                <dt>命名空间</dt>
                <dd>{namespace}</dd>
              </div>
              <div>
                <dt>当前版本</dt>
                <dd title={version}>{version}</dd>
              </div>
            </dl>
          </section>

          <section className="undeploy-section">
            <h4 className="undeploy-section-title">
              将删除的资源
              <span className="undeploy-section-count">{resources.length}</span>
            </h4>
            <ul className="undeploy-resources">
              {resources.map((item) => (
                <li key={item.kind} className={"danger" in item ? "is-danger" : undefined}>
                  <span className="undeploy-resource-kind">{item.kind}</span>
                  <span className="undeploy-resource-name">{item.name}</span>
                </li>
              ))}
            </ul>
          </section>

          <section className="undeploy-section">
            <h4 className="undeploy-section-title">额外清理</h4>
            <div className="undeploy-options">
              <UndeployOption
                checked={deleteConfig}
                title="删除配置"
                description="ConfigMap 与 Secret。默认保留，重新发布时直接复用。"
                onChange={setDeleteConfig}
              />
              <UndeployOption
                checked={deleteStorage}
                danger
                title="删除存储卷"
                description={
                  deleteStorage
                    ? "PVC 及其中的数据将被永久删除，无法恢复。"
                    : "PVC 及其数据。默认保留，避免数据丢失。"
                }
                onChange={setDeleteStorage}
              />
            </div>
          </section>

          <label className="undeploy-confirm">
            <span className="undeploy-confirm-label">
              请输入 <code>{project.name}</code> 确认下线
            </span>
            <input
              className={`input undeploy-confirm-input${confirmed ? " is-match" : ""}`}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void submit();
              }}
              placeholder={project.name}
              autoComplete="off"
              spellCheck={false}
              aria-label="确认项目名"
            />
          </label>

          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        </div>
      )}
    </Modal>
  );
}
