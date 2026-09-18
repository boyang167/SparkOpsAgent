"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductWorkflowRunVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { jobTypeLabel } from "./workflow-editor-utils";

interface ProductWorkflowRunDetailPanelProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  runId: string;
}

function statusLabel(status: string): string {
  if (status === "paused") return "等待手动执行";
  return status;
}

function statusTone(status: string): string {
  if (status === "success" || status === "approved") return "text-green-600";
  if (status === "failed" || status === "rejected") return "text-red-600";
  if (status === "running" || status === "pending") return "text-blue-600";
  return "text-muted-foreground";
}

export function ProductWorkflowRunDetailPanel({
  workspaceId,
  productId,
  workflowId,
  runId,
}: ProductWorkflowRunDetailPanelProps) {
  const [run, setRun] = useState<ProductWorkflowRunVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [rejectJobName, setRejectJobName] = useState<string | null>(null);
  const [rejectMessage, setRejectMessage] = useState("");

  const load = useCallback(async () => {
    try {
      setRun(await product.getProductWorkflowRun(workspaceId, productId, workflowId, runId));
      setError(null);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载执行详情失败");
    }
  }, [workspaceId, productId, workflowId, runId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!run || (run.status !== "running" && run.status !== "pending")) return;
    const timer = window.setInterval(() => {
      void load();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [run, load]);

  async function approve(jobName: string) {
    setBusy(true);
    try {
      await product.approveWorkflowJob(workspaceId, productId, workflowId, runId, jobName);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "审批失败");
    } finally {
      setBusy(false);
    }
  }

  async function reject(jobName: string) {
    setBusy(true);
    try {
      await product.rejectWorkflowJob(workspaceId, productId, workflowId, runId, jobName, {
        message: rejectMessage,
      });
      setRejectJobName(null);
      setRejectMessage("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "驳回失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="stack-lg">
      <PlatformPanelHead
        title="执行详情"
        description={run ? `状态 ${statusLabel(run.status)} · 当前阶段 #${run.current_stage + 1}` : "加载中…"}
        action={
          <Link href={`/products/workflows?productId=${productId}&workflowId=${workflowId}`} className="btn btn-ghost text-sm">
            返回工作流
          </Link>
        }
      />

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {!run ? (
        <div className="empty-state text-sm">加载执行记录…</div>
      ) : (
        <div className="pwf-run-detail stack-md">
          {run.error_message ? <InlineAlert tone="error">{run.error_message}</InlineAlert> : null}
          {run.status === "paused" ? (
            <div className="pwf-run-actions">
              <InlineAlert tone="info">当前阶段为手动执行，确认后才会进入该阶段任务。</InlineAlert>
              <button
                type="button"
                className="btn btn-primary text-sm"
                disabled={busy}
                onClick={() => {
                  setBusy(true);
                  void product
                    .continueProductWorkflowRun(workspaceId, productId, workflowId, runId)
                    .then(() => load())
                    .catch((err) => setError(err instanceof BffError ? err.message : "继续执行失败"))
                    .finally(() => setBusy(false));
                }}
              >
                继续执行
              </button>
            </div>
          ) : null}
          <div className="pwf-run-meta text-sm text-muted">
            <span>开始：{run.started_at ?? "-"}</span>
            <span>结束：{run.completed_at ?? "-"}</span>
            <span>创建：{run.created_at ?? "-"}</span>
          </div>
          {run.stage_runs.map((stage, si) => (
            <section key={`${stage.name}-${si}`} className="pwf-run-stage">
              <header className="pwf-run-stage-head">
                <h3>{stage.name}</h3>
                <span className={statusTone(stage.status)}>{stage.status}</span>
              </header>
              <div className="pwf-run-jobs">
                {stage.jobs.map((job) => (
                  <article key={job.name} className="pwf-run-job">
                    <div className="pwf-run-job-head">
                      <div>
                        <p className="text-xs text-muted">{jobTypeLabel(job.type)}</p>
                        <strong>{job.name}</strong>
                      </div>
                      <span className={statusTone(job.status)}>{job.status}</span>
                    </div>
                    {job.message ? <pre className="pwf-run-log">{job.message}</pre> : null}
                    {job.error_message ? <p className="text-sm text-red-600">{job.error_message}</p> : null}
                    {job.builds?.length ? (
                      <ul className="pwf-run-list">
                        {job.builds.map((build) => (
                          <li key={`${build.project_id}-${build.build_id ?? build.branch}`}>
                            项目 {build.project_id} · {build.branch} ·{" "}
                            <span className={statusTone(build.status)}>{build.status}</span>
                            {build.build_id ? (
                              <>
                                {" "}
                                <Link href={`/delivery/builds/${build.build_id}`} className="text-primary">
                                  查看构建
                                </Link>
                              </>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    ) : null}
                    {job.deploys?.length ? (
                      <ul className="pwf-run-list">
                        {job.deploys.map((deploy) => (
                          <li key={`${deploy.project_id}-${deploy.deployment_run_id ?? deploy.environment_id}`}>
                            环境 {deploy.environment_id} ·{" "}
                            <span className={statusTone(deploy.status)}>{deploy.status}</span>
                            {deploy.deployment_run_id ? (
                              <>
                                {" "}
                                <Link href={`/delivery/deployments/${deploy.deployment_run_id}`} className="text-primary">
                                  查看部署
                                </Link>
                              </>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    ) : null}
                    {job.approval?.status === "pending" ? (
                      <div className="pwf-run-actions">
                        <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void approve(job.name)}>
                          通过
                        </button>
                        {rejectJobName === job.name ? (
                          <div className="pwf-reject-box">
                            <input
                              className="input"
                              placeholder="驳回原因"
                              value={rejectMessage}
                              onChange={(e) => setRejectMessage(e.target.value)}
                            />
                            <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={() => void reject(job.name)}>
                              确认驳回
                            </button>
                          </div>
                        ) : (
                          <button type="button" className="btn btn-ghost text-sm" onClick={() => setRejectJobName(job.name)}>
                            驳回
                          </button>
                        )}
                      </div>
                    ) : null}
                  </article>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
