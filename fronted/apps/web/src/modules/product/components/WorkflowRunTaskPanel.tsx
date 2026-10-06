"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { delivery, product, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { BuildVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVersionDetailVM } from "@sparkops/contracts/workspace";
import { BuildLogStream } from "@/modules/delivery/components/BuildLogStream";
import { DeploymentLogStream } from "@/modules/delivery/components/DeploymentLogStream";
import { isActiveDeploymentRun } from "@/modules/workspace/components/project/deployment-console-utils";
import { formatImageRef } from "./deploy-job-spec";
import type { PipelineTask } from "./workflow-run-canvas-utils";
import { taskStatusTone } from "./workflow-run-canvas-utils";
import { runStatusLabel } from "./workflow-run-history-utils";
import { WorkflowRunStatusIcon } from "./WorkflowRunStatusIcon";

interface WorkflowRunTaskPanelProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  runId: string;
  task: PipelineTask | null;
  onClose: () => void;
  onRefresh: () => void;
}

function versionImageSummary(images: ProjectVersionDetailVM["images"]): string {
  if (!images.length) return "—";
  return images.map((item) => `${item.image}${item.tag ? `:${item.tag}` : ""}`.trim()).join(" · ");
}

export function WorkflowRunTaskPanel({
  workspaceId,
  productId,
  workflowId,
  runId,
  task,
  onClose,
  onRefresh,
}: WorkflowRunTaskPanelProps) {
  const [build, setBuild] = useState<BuildVM | null>(null);
  const [deploy, setDeploy] = useState<DeploymentRunVM | null>(null);
  const [version, setVersion] = useState<ProjectVersionDetailVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [rejectMessage, setRejectMessage] = useState("");
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!task) return;
    setLoading(true);
    setError(null);
    setBuild(null);
    setDeploy(null);
    setVersion(null);
    try {
      if (task.kind === "build" && task.refId) {
        setBuild(await delivery.getBuild(task.refId, workspaceId));
      } else if (task.kind === "deploy" && task.refId) {
        setDeploy(await delivery.getDeployment(task.refId, workspaceId));
      } else if (task.kind === "version" && task.refId && task.projectId) {
        setVersion(await workspace.getProjectVersion(workspaceId, task.projectId, task.refId));
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载任务详情失败");
    } finally {
      setLoading(false);
    }
  }, [task, workspaceId]);

  const refreshTaskDetail = useCallback(async () => {
    if (!task) return;
    try {
      if (task.kind === "build" && task.refId) {
        setBuild(await delivery.getBuild(task.refId, workspaceId));
      } else if (task.kind === "deploy" && task.refId) {
        setDeploy(await delivery.getDeployment(task.refId, workspaceId));
      } else if (task.kind === "version" && task.refId && task.projectId) {
        setVersion(await workspace.getProjectVersion(workspaceId, task.projectId, task.refId));
      }
    } catch {
      // keep existing detail visible while logs are shown
    }
  }, [task, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!task) {
    return (
      <div className="wf-run-live-panel is-empty">
        <p className="text-sm text-muted">点击上方流水线中的任务，查看构建日志、版本快照与部署详情。</p>
      </div>
    );
  }

  const title = `${task.jobTypeLabel} · ${task.projectLabel}`;
  const activeBuild = build?.status === "running";
  const activeDeploy = deploy ? isActiveDeploymentRun(deploy) : task.status === "running";
  const deployStatus = deploy?.status ?? task.status;
  const deployError = deploy?.error_message?.trim() || task.errorMessage;
  const versionImageRef =
    version && version.images.length > 0
      ? versionImageSummary(version.images)
      : formatImageRef(task.imageRepository, task.imageTag);
  const imageRef = task.kind === "version" ? versionImageRef : formatImageRef(
    deploy?.image_repository ?? task.imageRepository,
    deploy?.image_tag ?? task.imageTag,
  );

  return (
    <section className="wf-run-live-panel">
      <header className="wf-run-live-panel-head">
        <div className="wf-run-live-panel-title">
          <WorkflowRunStatusIcon status={task.status} size={20} />
          <div>
            <h3>{title}</h3>
            <p className="wf-run-live-panel-sub">
              {runStatusLabel(task.status)}
              {task.jobName ? ` · ${task.jobName}` : ""}
            </p>
          </div>
        </div>
        <div className="wf-run-live-panel-actions">
          {task.kind === "build" && task.refId ? (
            <Link href={`/delivery/builds/${task.refId}`} className="btn btn-ghost text-sm">
              打开构建页
            </Link>
          ) : null}
          {task.kind === "version" && task.refId ? (
            <Link href={`/products/${productId}?tab=versions`} className="btn btn-ghost text-sm">
              打开版本管理
            </Link>
          ) : null}
          {task.kind === "deploy" && task.refId ? (
            <Link href={`/delivery/deployments/${task.refId}`} className="btn btn-ghost text-sm">
              打开部署页
            </Link>
          ) : null}
          <button type="button" className="wf-run-live-panel-close" onClick={onClose} aria-label="关闭详情">
            ×
          </button>
        </div>
      </header>

      {error ? <p className="text-sm text-red-600 px-4 pt-3">{error}</p> : null}
      {task.errorMessage && task.errorMessage !== error ? (
        <p className="text-sm text-red-600 px-4 pt-3">{task.errorMessage}</p>
      ) : null}

      <div className="wf-run-live-panel-meta">
        {task.kind === "version" && task.versionTag ? (
          <div className="wf-run-live-meta-item">
            <span className="wf-run-live-meta-label">版本号</span>
            <span className="mono">{task.versionTag}</span>
          </div>
        ) : null}
        {task.branch ? (
          <div className="wf-run-live-meta-item">
            <span className="wf-run-live-meta-label">代码信息</span>
            <span>
              {task.gitRefType === "tag" ? "Tag" : "Branch"} · {task.branch}
            </span>
          </div>
        ) : null}
        {task.kind === "version" && version ? (
          <div className="wf-run-live-meta-item">
            <span className="wf-run-live-meta-label">Git 引用</span>
            <span>
              {version.git_ref_type} / {version.git_ref_value}
            </span>
          </div>
        ) : null}
        {imageRef !== "—" ? (
          <div className="wf-run-live-meta-item">
            <span className="wf-run-live-meta-label">镜像</span>
            <span className="wf-run-image-ref mono" title={imageRef}>
              {imageRef}
            </span>
          </div>
        ) : null}
        {task.environmentId ? (
          <div className="wf-run-live-meta-item">
            <span className="wf-run-live-meta-label">环境</span>
            <span>{task.environmentId}</span>
          </div>
        ) : null}
        {task.kind === "deploy" && deploy ? (
          <>
            <div className="wf-run-live-meta-item">
              <span className="wf-run-live-meta-label">部署状态</span>
              <span className={taskStatusTone(deployStatus)}>{runStatusLabel(deployStatus)}</span>
            </div>
            {deploy.git_ref_value ? (
              <div className="wf-run-live-meta-item">
                <span className="wf-run-live-meta-label">代码信息</span>
                <span>
                  {deploy.git_ref_type === "tag" ? "Tag" : "Branch"} · {deploy.git_ref_value}
                </span>
              </div>
            ) : null}
          </>
        ) : null}
      </div>

      {loading ? <div className="wf-run-live-panel-loading text-sm text-muted">加载任务详情…</div> : null}

      {task.kind === "build" && task.refId ? (
        <div className="wf-run-live-panel-log">
          <BuildLogStream
            buildId={task.refId}
            initialLog={build?.console_log}
            active={activeBuild}
            terminal
            onDone={refreshTaskDetail}
          />
        </div>
      ) : null}

      {task.kind === "version" ? (
        <div className="wf-run-live-panel-version">
          {!loading && !error && version ? (
            <>
              <p className="wf-run-live-panel-note text-sm text-muted">
                已快照 Service / Deployment YAML，并绑定上游构建镜像。
              </p>
              {version.manifests.map((manifest) => (
                <details key={manifest.kind} open={manifest.kind === "Deployment"}>
                  <summary className="text-sm font-medium">{manifest.kind}</summary>
                  <pre className="product-version-yaml">{manifest.content}</pre>
                </details>
              ))}
            </>
          ) : null}
          {!loading && !error && !version && task.status === "success" ? (
            <p className="wf-run-live-panel-note text-sm text-muted">版本已创建，暂无快照详情。</p>
          ) : null}
          {!loading && task.kind === "version" && task.status !== "running" && !task.refId ? (
            <p className="wf-run-live-panel-note text-sm text-muted">该版本任务未产生版本记录。</p>
          ) : null}
        </div>
      ) : null}

      {task.kind === "deploy" && task.refId ? (
        <div className="wf-run-live-panel-log">
          <DeploymentLogStream
            runId={task.refId}
            active={activeDeploy}
            terminal
            onDone={refreshTaskDetail}
          />
          {!activeDeploy && deployError ? (
            <pre className="wf-run-live-panel-log-text">{deployError}</pre>
          ) : null}
          {!activeDeploy && !deployError && deployStatus === "succeeded" ? (
            <p className="wf-run-live-panel-note text-sm text-muted">部署已完成。</p>
          ) : null}
        </div>
      ) : null}

      {task.kind === "approval" && task.status === "running" ? (
        <div className="wf-run-live-panel-approval">
          <div className="wf-run-live-panel-note text-sm text-muted">
            该任务等待人工审批。
            {task.requiredApprovals && task.requiredApprovals > 1
              ? ` 已通过 ${task.approvedCount ?? 0}/${task.requiredApprovals}。`
              : null}
            {task.timeoutMinutes ? ` 超时 ${task.timeoutMinutes} 分钟。` : null}
          </div>
          <div className="pwf-run-actions">
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                void product
                  .approveWorkflowJob(workspaceId, productId, workflowId, runId, task.jobName)
                  .then(() => onRefresh())
                  .catch((err) => setError(err instanceof BffError ? err.message : "审批失败"))
                  .finally(() => setBusy(false));
              }}
            >
              通过
            </button>
            <input
              className="input"
              placeholder="驳回原因"
              value={rejectMessage}
              onChange={(e) => setRejectMessage(e.target.value)}
            />
            <button
              type="button"
              className="btn btn-secondary text-sm"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                void product
                  .rejectWorkflowJob(workspaceId, productId, workflowId, runId, task.jobName, {
                    message: rejectMessage,
                  })
                  .then(() => {
                    setRejectMessage("");
                    onRefresh();
                  })
                  .catch((err) => setError(err instanceof BffError ? err.message : "驳回失败"))
                  .finally(() => setBusy(false));
              }}
            >
              驳回
            </button>
          </div>
        </div>
      ) : null}
    </section>
  );
}
