"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import type { ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { RichTextField } from "./RichTextField";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import { releasePlanStatusBadgeClass, releasePlanStatusLabel } from "./release-plan-utils";

interface ReleasePlanJobEditorProps {
  workspaceId: string;
  productId: string;
  plan: ReleasePlanVM;
  workflows: ProductWorkflowVM[];
  canEdit: boolean;
  busy?: boolean;
  onMove: (jobId: string, direction: "up" | "down") => void | Promise<void>;
  onDelete: (jobId: string) => void | Promise<void>;
  onUpdate: (jobId: string, patch: { name?: string; text_content?: string; text_format?: string; owner_id?: string; workflow_ref?: { workflow_id: string; parameters?: Record<string, string> } }) => void | Promise<void>;
  onAddText: () => void | Promise<void>;
  onAddWorkflow: (workflowId: string) => void | Promise<void>;
  renderJobActions?: (job: ReleasePlanJobVM) => ReactNode;
}

function jobTypeLabel(type: string) {
  return type === "text" ? "检查项" : "工作流";
}

export function ReleasePlanJobEditor({
  workspaceId,
  productId,
  plan,
  workflows,
  canEdit,
  busy,
  onMove,
  onDelete,
  onUpdate,
  onAddText,
  onAddWorkflow,
  renderJobActions,
}: ReleasePlanJobEditorProps) {
  const [selectedWorkflowId, setSelectedWorkflowId] = useState(workflows[0]?.id ?? "");
  const jobs = [...(plan.jobs ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));

  return (
    <div className="release-plan-jobs">
      {canEdit ? (
        <div className="release-plan-jobs-toolbar">
          <div className="release-plan-jobs-toolbar-primary">
            <button type="button" className="btn btn-secondary text-xs" disabled={busy} onClick={() => void onAddText()}>
              添加检查项
            </button>
            {workflows.length > 0 ? (
              <div className="release-plan-jobs-workflow-add">
                <select className="input text-xs" value={selectedWorkflowId} onChange={(e) => setSelectedWorkflowId(e.target.value)} aria-label="选择工作流">
                  {workflows.map((wf) => (
                    <option key={wf.id} value={wf.id}>
                      {wf.name}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  className="btn btn-secondary text-xs shrink-0"
                  disabled={busy || !selectedWorkflowId}
                  onClick={() => void onAddWorkflow(selectedWorkflowId)}
                >
                  添加工作流项
                </button>
              </div>
            ) : (
              <span className="text-xs text-muted-foreground">请先在「工作流」Tab 创建工作流</span>
            )}
          </div>
          <span className="text-xs text-muted-foreground">{jobs.length} 项</span>
        </div>
      ) : null}

      {jobs.length === 0 ? (
        <div className="empty-state release-plan-jobs-empty">
          <p>{canEdit ? "还没有发布项，从上方添加检查项或工作流" : "暂无发布项"}</p>
          {canEdit ? (
            <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onAddText()}>
              添加第一个检查项
            </button>
          ) : null}
        </div>
      ) : (
        <ul className="release-plan-jobs-list">
          {jobs.map((job, index) => (
            <li key={job.id} className="release-plan-job-card">
              <div className="release-plan-job-card-head">
                <div className="release-plan-job-card-title-row">
                  <span className={`badge badge-neutral release-plan-job-type`}>{jobTypeLabel(job.type)}</span>
                  <span className={`badge ${releasePlanStatusBadgeClass(job.status)}`}>{releasePlanStatusLabel(job.status)}</span>
                </div>
                <div className="release-plan-job-card-actions">
                  {canEdit ? (
                    <>
                      <button
                        type="button"
                        className="btn-ghost text-xs px-1.5"
                        disabled={busy || index === 0}
                        aria-label="上移"
                        onClick={() => void onMove(job.id, "up")}
                      >
                        ↑
                      </button>
                      <button
                        type="button"
                        className="btn-ghost text-xs px-1.5"
                        disabled={busy || index === jobs.length - 1}
                        aria-label="下移"
                        onClick={() => void onMove(job.id, "down")}
                      >
                        ↓
                      </button>
                      <button type="button" className="btn-ghost text-xs text-red-600 px-1.5" disabled={busy} onClick={() => void onDelete(job.id)}>
                        删除
                      </button>
                    </>
                  ) : null}
                  {renderJobActions?.(job)}
                </div>
              </div>

              <div className="release-plan-job-card-body">
                {canEdit ? (
                  <input
                    className="input text-sm font-medium"
                    defaultValue={job.name}
                    onBlur={(e) => {
                      const v = e.target.value.trim();
                      if (v && v !== job.name) void onUpdate(job.id, { name: v });
                    }}
                  />
                ) : (
                  <h4 className="release-plan-job-name">{job.name}</h4>
                )}

                {job.type === "text" ? (
                  canEdit ? (
                    <RichTextField
                      className="rich-text-field-compact"
                      value={job.text_content ?? ""}
                      onChange={(html) => {
                        if (html !== (job.text_content ?? "")) void onUpdate(job.id, { text_content: html, text_format: "html" });
                      }}
                      placeholder="检查项说明"
                    />
                  ) : job.text_content ? (
                    <div className="release-plan-job-content prose prose-sm max-w-none text-sm" dangerouslySetInnerHTML={{ __html: job.text_content }} />
                  ) : null
                ) : null}

                {job.type === "workflow" && job.workflow_ref?.workflow_id ? (
                  <p className="text-xs text-muted-foreground">关联工作流 · {job.workflow_ref.workflow_id.slice(0, 8)}…</p>
                ) : null}

                {job.execution_ref?.workflow_run_id && job.workflow_ref?.workflow_id ? (
                  <Link
                    href={`/products/${productId}/workflows/${job.workflow_ref.workflow_id}/runs/${job.execution_ref.workflow_run_id}`}
                    className="text-xs text-primary hover:underline font-mono w-fit"
                  >
                    查看 Run {job.execution_ref.workflow_run_id.slice(0, 8)}…
                  </Link>
                ) : null}

                {job.execution_ref?.error_message ? (
                  <InlineAlert tone="error">{job.execution_ref.error_message}</InlineAlert>
                ) : null}

                {canEdit || job.owner_id ? (
                  <label className="release-plan-job-owner">
                    <span className="release-plan-job-owner-label">执行人</span>
                    {canEdit ? (
                      <WorkItemAssigneeField
                        workspaceId={workspaceId}
                        assigneeId={job.owner_id}
                        onSelect={(id) => {
                          if (id) void onUpdate(job.id, { owner_id: id });
                        }}
                      />
                    ) : (
                      <span className="text-sm text-muted-foreground">{job.owner_id ?? "—"}</span>
                    )}
                  </label>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
