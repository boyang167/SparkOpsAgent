"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { workflow } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkflowRunVM } from "@sparkops/contracts/workflow";
import {
  formatWorkflowTime,
  statusBadgeClass,
  summarizeWorkflowRun,
} from "../utils/workflowRun";

interface WorkflowRunPanelProps {
  projectId: string;
  workspaceId: string;
  workflowId: string;
  workflowStatus: string;
  runs: WorkflowRunVM[];
}

export function WorkflowRunPanel({
  projectId,
  workspaceId,
  workflowId,
  workflowStatus,
  runs,
}: WorkflowRunPanelProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onRun() {
    if (workflowStatus !== "active") {
      setError("请先激活 Workflow 后再运行");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await workflow.startWorkflowRun(workflowId, workspaceId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "运行失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">运行记录</h2>
          <p className="text-muted mt-1">
            同步执行 definition 中的 steps（支持 log、noop）
          </p>
        </div>
        <button className="btn" type="button" onClick={onRun} disabled={loading || workflowStatus !== "active"}>
          {loading ? "运行中..." : "运行 Workflow"}
        </button>
      </div>

      <pre className="text-xs text-muted bg-[var(--surface)] p-3 rounded-lg overflow-x-auto">{`{
  "steps": [
    { "type": "log", "message": "Hello workflow" },
    { "type": "noop" }
  ]
}`}</pre>

      {error ? <p className="error">{error}</p> : null}

      {runs.length === 0 ? (
        <div className="empty-state">暂无运行记录</div>
      ) : (
        <div className="stack-md">
          {runs.map((run) => {
            const summary = summarizeWorkflowRun(run);
            return (
              <Link
                key={run.id}
                href={`/projects/${projectId}/workflows/${workflowId}/runs/${run.id}`}
                className="card block hover:border-[var(--border-strong)] transition-colors"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-medium text-sm">{formatWorkflowTime(run.started_at)}</p>
                    <p className="text-xs text-muted mt-1 truncate">
                      {summary.stepCount > 0
                        ? `${summary.stepCount} 步 · ${summary.preview}`
                        : run.error || run.id}
                    </p>
                  </div>
                  <span className={`badge shrink-0 ${statusBadgeClass(run.status)}`}>{run.status}</span>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </section>
  );
}
