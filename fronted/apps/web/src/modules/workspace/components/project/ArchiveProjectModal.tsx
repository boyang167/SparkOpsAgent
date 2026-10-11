"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery, workspace } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { archiveSteps, type ArchiveStep } from "./archive-progress";
import { ProductBindingNotice } from "./ProductBindingNotice";
import {
  buildEnvironmentDeployRows,
  onlineEnvironmentRows,
  type EnvironmentDeployRow,
} from "./project-environment-status";
import { projectDisplayName } from "./project-utils";

interface ArchiveProjectModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM | null;
  onClose: () => void;
  onArchived: () => void;
}

type Phase = "loading" | "confirm" | "progress" | "failed" | "done";

const STEP_LABEL: Record<ArchiveStep["state"], string> = {
  waiting: "等待",
  running: "下线中",
  done: "已下线",
  failed: "失败",
};

const STEP_TONE: Record<ArchiveStep["state"], string> = {
  waiting: "neutral",
  running: "running",
  done: "success",
  failed: "failed",
};

function pendingClusters(rows: EnvironmentDeployRow[]) {
  return rows.map((row) => ({ clusterId: row.clusterId, clusterName: row.clusterName }));
}

export function ArchiveProjectModal({
  open,
  workspaceId,
  project,
  onClose,
  onArchived,
}: ArchiveProjectModalProps) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [online, setOnline] = useState<EnvironmentDeployRow[]>([]);
  const [confirmName, setConfirmName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [steps, setSteps] = useState<ArchiveStep[]>([]);
  const startedAtRef = useRef("");

  useEffect(() => {
    if (!open || !project) return;
    let cancelled = false;
    setPhase("loading");
    setConfirmName("");
    setError(null);
    setSteps([]);
    void Promise.all([
      delivery.listDeployments(project.id, workspaceId),
      delivery.getDeploymentOptions(project.id, workspaceId).catch(() => ({ clusters: [] })),
    ])
      .then(([page, options]) => {
        if (cancelled) return;
        const clusters = options.clusters.map((item) => ({
          id: item.id,
          cluster_name: item.cluster_name,
          is_build_env: item.is_build_env,
          priority_id: item.priority_id,
        }));
        setOnline(onlineEnvironmentRows(buildEnvironmentDeployRows(clusters, page.items)));
        setPhase("confirm");
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof BffError ? err.message : "加载环境信息失败");
        setOnline([]);
        setPhase("confirm");
      });
    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId]);

  const poll = useCallback(async () => {
    if (!project) return;
    let status: string | null = null;
    try {
      status = (await workspace.getProject(workspaceId, project.id)).status;
    } catch (err) {
      if (err instanceof BffError && err.code === "WORKSPACE_APP_PROJECT_NOT_FOUND") {
        status = "archived";
      }
    }
    const page = await delivery.listDeployments(project.id, workspaceId).catch(() => null);
    const runs: DeploymentRunVM[] = page?.items ?? [];
    const next = archiveSteps(pendingClusters(online), runs, startedAtRef.current);
    setSteps(next);
    if (status === "archived") {
      setPhase("done");
      onArchived();
    } else if (status === "active") {
      setPhase("failed");
      setError(
        next.find((step) => step.state === "failed")?.error ??
          "归档已中止，请查看部署历史中的下线记录",
      );
    }
  }, [online, onArchived, project, workspaceId]);

  useEffect(() => {
    if (phase !== "progress") return;
    const timer = window.setInterval(() => void poll(), 3000);
    return () => window.clearInterval(timer);
  }, [phase, poll]);

  if (!project) return null;
  const hasOnline = online.length > 0;
  const boundProductId = project.product_id || null;
  const canSubmit = phase === "confirm" && confirmName === project.name && !submitting && !boundProductId;

  async function submit() {
    if (!project || !canSubmit) return;
    setSubmitting(true);
    setError(null);
    startedAtRef.current = new Date(Date.now() - 5000).toISOString();
    try {
      const result = await workspace.archiveProjectWithTeardown(workspaceId, project.id, {
        confirm_name: confirmName,
        teardown: hasOnline,
      });
      if (result.status === "archived") {
        setPhase("done");
        onArchived();
        return;
      }
      setSteps(archiveSteps(pendingClusters(online), [], startedAtRef.current));
      setPhase("progress");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "归档失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  const footer =
    phase === "confirm" || phase === "loading" ? (
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
          {submitting ? "提交中…" : hasOnline ? "下线全部环境并归档" : "归档"}
        </button>
      </div>
    ) : (
      <div className="modal-footer-actions">
        <button type="button" className="btn-secondary" onClick={onClose}>
          {phase === "progress" ? "后台继续，关闭" : "关闭"}
        </button>
      </div>
    );

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!submitting) onClose();
      }}
      title={`归档项目「${projectDisplayName(project)}」`}
      size="md"
      footer={footer}
    >
      <div className="stack-md">
        {phase === "loading" ? (
          <p className="text-sm text-muted" role="status">检查在线环境…</p>
        ) : null}
        {phase === "confirm" && boundProductId ? (
          <ProductBindingNotice workspaceId={workspaceId} productId={boundProductId} action="归档" />
        ) : null}
        {phase === "confirm" && !boundProductId ? (
          <>
            {hasOnline ? (
              <div className="stack-sm">
                <p className="text-sm">将依次下线以下环境，完成后禁用 Jenkins 流水线并归档：</p>
                <ul className="undeploy-resource-list">
                  {online.map((row) => (
                    <li key={row.clusterId}>{row.clusterName}</li>
                  ))}
                </ul>
              </div>
            ) : (
              <p className="text-sm">项目没有在线环境。归档将禁用 Jenkins 流水线，项目移出列表。</p>
            )}
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
        ) : null}
        {phase === "progress" || phase === "failed" || phase === "done" ? (
          steps.length > 0 ? (
            <ul className="project-env-list" role="status">
              {steps.map((step) => (
                <li key={step.clusterId} className={`project-env-item is-${STEP_TONE[step.state]}`}>
                  <span className="project-env-chip">{step.clusterName}</span>
                  <span />
                  <span className={`project-status-badge is-${STEP_TONE[step.state]}`}>
                    <span className="project-status-badge-dot" aria-hidden />
                    {STEP_LABEL[step.state]}
                  </span>
                </li>
              ))}
            </ul>
          ) : null
        ) : null}
        {phase === "done" ? <InlineAlert tone="success">项目已归档。</InlineAlert> : null}
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </div>
    </Modal>
  );
}
