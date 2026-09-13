"use client";

import { useEffect, useMemo, useState } from "react";
import type { DeploymentOverviewVM } from "@sparkops/contracts/cluster";
import type {
  DeploymentRunListVM,
  DeploymentRunVM,
  RollbackDiffFieldVM,
  RollbackPreviewVM,
} from "@sparkops/contracts/delivery";
import { BffError } from "@sparkops/contracts";
import { delivery } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import {
  findDeploymentRun,
  succeededDeployments,
} from "@/modules/workspace/components/project/project-deployment-state";
import { formatDateTime } from "@/modules/workspace/components/project/project-utils";

interface RuntimeRollbackModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  deploymentPage: DeploymentRunListVM | null;
  deployment?: DeploymentOverviewVM | null;
  onClose: () => void;
  onRollbackStarted: (run: DeploymentRunVM) => void;
}

function gitRefTypeLabel(type?: string) {
  return type === "tag" ? "标签" : "分支";
}

function formatRollbackOption(run: DeploymentRunVM) {
  const ref = run.git_ref_value?.trim() || "-";
  const tag = run.image_tag?.trim();
  const typeLabel = gitRefTypeLabel(run.git_ref_type);
  const time = formatDateTime(run.finished_at || run.created_at);
  const parts = [`${ref}（${typeLabel}）`];
  if (tag && tag !== ref) parts.push(`镜像 ${tag}`);
  parts.push(time);
  return parts.join(" · ");
}

function formatDiffValue(label: string, value: string) {
  if (label === "Git 类型") return gitRefTypeLabel(value);
  if (label === "集群" && value.length > 16) {
    return `${value.slice(0, 8)}…${value.slice(-4)}`;
  }
  return value || "-";
}

function buildRuntimeSnapshot(deployment?: DeploymentOverviewVM | null) {
  if (!deployment) return [];
  return [
    { label: "副本", value: String(deployment.replicas ?? "-") },
    { label: "CPU", value: `${deployment.cpu_request || "-"} ~ ${deployment.cpu_limit || "-"}` },
    {
      label: "内存",
      value: `${deployment.memory_request || "-"} ~ ${deployment.memory_limit || "-"}`,
    },
  ];
}

function VersionCard({
  title,
  run,
  tone = "current",
}: {
  title: string;
  run: DeploymentRunVM | null | undefined;
  tone?: "current" | "target";
}) {
  if (!run) {
    return (
      <div className={`runtime-rollback-version-card is-${tone}`}>
        <span className="runtime-rollback-version-label">{title}</span>
        <p className="runtime-rollback-version-empty">-</p>
      </div>
    );
  }
  const ref = run.git_ref_value?.trim() || "-";
  const tag = run.image_tag?.trim() || ref;
  return (
    <div className={`runtime-rollback-version-card is-${tone}`}>
      <span className="runtime-rollback-version-label">{title}</span>
      <strong className="runtime-rollback-version-tag">{tag}</strong>
      <span className="runtime-rollback-version-meta">
        {ref} · {gitRefTypeLabel(run.git_ref_type)}
      </span>
      <span className="runtime-rollback-version-time">
        {formatDateTime(run.finished_at || run.created_at)}
      </span>
    </div>
  );
}

function ChangeRow({ row }: { row: RollbackDiffFieldVM }) {
  return (
    <div className={`runtime-rollback-change-row${row.changed ? " is-changed" : ""}`}>
      <span className="runtime-rollback-change-label">{row.label}</span>
      <div className="runtime-rollback-change-values">
        <span className="runtime-rollback-change-from" title={row.current}>
          {formatDiffValue(row.label, row.current)}
        </span>
        <span className="runtime-rollback-change-arrow" aria-hidden>
          →
        </span>
        <span className="runtime-rollback-change-to" title={row.target}>
          {formatDiffValue(row.label, row.target)}
        </span>
      </div>
    </div>
  );
}

export function RuntimeRollbackModal({
  open,
  projectId,
  workspaceId,
  deploymentPage,
  deployment,
  onClose,
  onRollbackStarted,
}: RuntimeRollbackModalProps) {
  const succeeded = useMemo(() => succeededDeployments(deploymentPage), [deploymentPage]);
  const currentRun = succeeded[0] ?? null;
  const rollbackTargets = useMemo(
    () => succeeded.filter((item) => item.id !== currentRun?.id),
    [currentRun?.id, succeeded],
  );
  const [targetRunId, setTargetRunId] = useState<string>("");
  const [preview, setPreview] = useState<RollbackPreviewVM | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTargetRunId(rollbackTargets[0]?.id ?? "");
    setPreview(null);
    setError(null);
  }, [open, rollbackTargets]);

  useEffect(() => {
    if (!open || !targetRunId) {
      setPreview(null);
      return;
    }
    let cancelled = false;
    setLoadingPreview(true);
    setError(null);
    void delivery
      .getRollbackPreview(projectId, workspaceId, targetRunId)
      .then((next) => {
        if (!cancelled) setPreview(next);
      })
      .catch((err) => {
        if (!cancelled) {
          setPreview(null);
          setError(err instanceof BffError ? err.message : "加载回滚预览失败");
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingPreview(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, projectId, workspaceId, targetRunId]);

  const selectedTarget =
    preview?.target_run ?? findDeploymentRun(deploymentPage, targetRunId);
  const runtimeSnapshot = buildRuntimeSnapshot(deployment);
  const changedDiff = (preview?.diff ?? []).filter((row) => row.changed);
  const unchangedDiff = (preview?.diff ?? []).filter((row) => !row.changed);

  async function handleSubmit() {
    if (!targetRunId) return;
    setSubmitting(true);
    setError(null);
    try {
      const run = await delivery.rollbackDeployment(projectId, workspaceId, {
        target_run_id: targetRunId,
      });
      onRollbackStarted(run);
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "发起回滚失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="服务回滚"
      size="lg"
      panelClassName="runtime-rollback-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary project-service-danger"
            onClick={() => void handleSubmit()}
            disabled={submitting || loadingPreview || !preview?.can_rollback || !targetRunId}
          >
            {submitting ? "回滚中…" : "确认回滚"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}

      <p className="runtime-rollback-hint">
        跳过 CI，使用历史镜像重新部署；副本与资源配置不变，失败不会自动再次回滚。
        {selectedTarget?.cross_cluster
          ? " 该版本为跨集群发布，回滚会同步到其余授权集群。"
          : null}
      </p>

      <section className="runtime-rollback-section">
        <label className="runtime-rollback-label" htmlFor="rollback-target">
          回滚目标版本
        </label>
        <select
          id="rollback-target"
          className="runtime-rollback-select"
          value={targetRunId}
          onChange={(event) => setTargetRunId(event.target.value)}
          disabled={rollbackTargets.length === 0}
        >
          {rollbackTargets.map((item) => (
            <option key={item.id} value={item.id}>
              {formatRollbackOption(item)}
            </option>
          ))}
        </select>
      </section>

      {loadingPreview ? (
        <p className="runtime-rollback-loading text-muted text-sm">预览加载中…</p>
      ) : null}

      {!loadingPreview && preview && !preview.can_rollback ? (
        <p className="runtime-config-modal-error">{preview.reason || "当前不可回滚"}</p>
      ) : null}

      {!loadingPreview && preview?.can_rollback ? (
        <>
          <div className="runtime-rollback-compare">
            <VersionCard title="当前版本" run={preview.current_run ?? currentRun} tone="current" />
            <span className="runtime-rollback-compare-arrow" aria-hidden>
              →
            </span>
            <VersionCard title="回滚目标" run={selectedTarget} tone="target" />
          </div>

          {changedDiff.length > 0 ? (
            <section className="runtime-rollback-section">
              <h4 className="runtime-rollback-section-title">将发生变更</h4>
              <div className="runtime-rollback-changes">
                {changedDiff.map((row) => (
                  <ChangeRow key={row.label} row={row} />
                ))}
              </div>
            </section>
          ) : null}

          {(unchangedDiff.length > 0 || runtimeSnapshot.length > 0) && (
            <section className="runtime-rollback-section">
              <h4 className="runtime-rollback-section-title">保持不变</h4>
              <div className="runtime-rollback-unchanged">
                {unchangedDiff.map((row) => (
                  <span key={row.label} className="project-tag project-tag-gray" title={row.current}>
                    {row.label} {formatDiffValue(row.label, row.current)}
                  </span>
                ))}
                {runtimeSnapshot.map((item) => (
                  <span key={item.label} className="project-tag project-tag-gray">
                    {item.label} {item.value}
                  </span>
                ))}
              </div>
            </section>
          )}
        </>
      ) : null}
    </Modal>
  );
}
