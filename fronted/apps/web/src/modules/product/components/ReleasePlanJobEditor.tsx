"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import type { ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { RichTextField } from "./RichTextField";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import { releasePlanStatusLabel, statusTone } from "./release-plan-utils";

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
    <div className="stack-md">
      {canEdit ? (
        <div className="flex flex-wrap gap-2">
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onAddText()}>
            添加检查项
          </button>
          {workflows.length > 0 ? (
            <>
              <select className="input text-xs" value={selectedWorkflowId} onChange={(e) => setSelectedWorkflowId(e.target.value)}>
                {workflows.map((wf) => (
                  <option key={wf.id} value={wf.id}>
                    {wf.name}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn-secondary text-xs"
                disabled={busy || !selectedWorkflowId}
                onClick={() => void onAddWorkflow(selectedWorkflowId)}
              >
                添加工作流项
              </button>
            </>
          ) : (
            <span className="text-xs text-muted-foreground">请先在「工作流」Tab 创建工作流</span>
          )}
        </div>
      ) : null}

      <ul className="stack-xs text-sm">
        {jobs.map((job, index) => (
          <li key={job.id} className="border rounded p-3 stack-sm">
            <div className="flex justify-between gap-2 items-start">
              <div className="stack-xs flex-1">
                {canEdit ? (
                  <input
                    className="input text-sm"
                    defaultValue={job.name}
                    onBlur={(e) => {
                      const v = e.target.value.trim();
                      if (v && v !== job.name) void onUpdate(job.id, { name: v });
                    }}
                  />
                ) : (
                  <span className="font-medium">{job.name}</span>
                )}
                <span className="text-xs text-muted-foreground">
                  {job.type === "text" ? "检查项" : "工作流"} · {releasePlanStatusLabel(job.status)}
                </span>
              </div>
              <div className="flex gap-1 items-center">
                {canEdit ? (
                  <>
                    <button type="button" className="btn-secondary text-xs px-2" disabled={busy || index === 0} onClick={() => void onMove(job.id, "up")}>
                      ↑
                    </button>
                    <button
                      type="button"
                      className="btn-secondary text-xs px-2"
                      disabled={busy || index === jobs.length - 1}
                      onClick={() => void onMove(job.id, "down")}
                    >
                      ↓
                    </button>
                    <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onDelete(job.id)}>
                      删除
                    </button>
                  </>
                ) : null}
                {renderJobActions?.(job)}
              </div>
            </div>

            {job.type === "text" ? (
              canEdit ? (
                <RichTextField
                  value={job.text_content ?? ""}
                  onChange={(html) => {
                    if (html !== (job.text_content ?? "")) void onUpdate(job.id, { text_content: html, text_format: "html" });
                  }}
                  placeholder="检查项说明"
                />
              ) : job.text_content ? (
                <div className="prose prose-sm max-w-none text-xs dark:prose-invert" dangerouslySetInnerHTML={{ __html: job.text_content }} />
              ) : null
            ) : null}

            {canEdit ? (
              <div className="text-xs">
                <span className="text-muted-foreground mr-2">执行人</span>
                <WorkItemAssigneeField
                  workspaceId={workspaceId}
                  assigneeId={job.owner_id}
                  onSelect={(id) => {
                    if (id) void onUpdate(job.id, { owner_id: id });
                  }}
                />
              </div>
            ) : null}

            {job.type === "workflow" && job.workflow_ref?.workflow_id ? (
              <span className="text-xs text-muted-foreground">工作流 {job.workflow_ref.workflow_id.slice(0, 8)}…</span>
            ) : null}

            {job.execution_ref?.workflow_run_id && job.workflow_ref?.workflow_id ? (
              <Link
                href={`/products/${productId}/workflows/${job.workflow_ref.workflow_id}/runs/${job.execution_ref.workflow_run_id}`}
                className="text-xs text-primary hover:underline font-mono"
              >
                查看 Run {job.execution_ref.workflow_run_id.slice(0, 8)}…
              </Link>
            ) : null}

            {job.execution_ref?.error_message ? (
              <span className={`text-xs ${statusTone("failed")}`}>{job.execution_ref.error_message}</span>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
