"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
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

function textPreview(html: string, maxLength = 72): string {
  const text = html
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (!text) return "";
  return text.length > maxLength ? `${text.slice(0, maxLength)}…` : text;
}

function workflowName(workflows: ProductWorkflowVM[], workflowId?: string): string {
  if (!workflowId) return "—";
  const match = workflows.find((item) => item.id === workflowId);
  return match?.name ?? `${workflowId.slice(0, 8)}…`;
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
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const prevJobCount = useRef(0);

  const jobs = [...(plan.jobs ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));

  useEffect(() => {
    setSelectedWorkflowId(workflows[0]?.id ?? "");
  }, [workflows]);

  useEffect(() => {
    const sorted = [...(plan.jobs ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
    const firstOpen =
      sorted.find((job) => job.status !== "done" && job.status !== "skipped") ?? sorted[0];
    setExpandedIds(firstOpen ? new Set([firstOpen.id]) : new Set());
    prevJobCount.current = sorted.length;
  }, [plan.id]);

  const jobCount = (plan.jobs ?? []).length;
  useEffect(() => {
    if (jobCount > prevJobCount.current && jobCount > 0) {
      const sorted = [...(plan.jobs ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
      const added = sorted[sorted.length - 1];
      if (added) {
        setExpandedIds((prev) => new Set([...prev, added.id]));
      }
    }
    prevJobCount.current = jobCount;
  }, [jobCount, plan.jobs]);

  useEffect(() => {
    return () => {
      for (const timer of debounceTimers.current.values()) {
        clearTimeout(timer);
      }
      debounceTimers.current.clear();
    };
  }, []);

  const flushTextUpdate = useCallback(
    (jobId: string, html: string, current: string) => {
      const timers = debounceTimers.current;
      const pending = timers.get(jobId);
      if (pending) {
        clearTimeout(pending);
        timers.delete(jobId);
      }
      if (html === current) return;
      void onUpdate(jobId, { text_content: html, text_format: "html" });
    },
    [onUpdate],
  );

  const scheduleTextUpdate = useCallback(
    (jobId: string, html: string, current: string) => {
      if (html === current) return;
      const timers = debounceTimers.current;
      const pending = timers.get(jobId);
      if (pending) clearTimeout(pending);
      timers.set(
        jobId,
        setTimeout(() => {
          timers.delete(jobId);
          void onUpdate(jobId, { text_content: html, text_format: "html" });
        }, 700),
      );
    },
    [onUpdate],
  );

  function toggleExpanded(jobId: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(jobId)) next.delete(jobId);
      else next.add(jobId);
      return next;
    });
  }

  function expandAll() {
    setExpandedIds(new Set(jobs.map((job) => job.id)));
  }

  function collapseAll() {
    setExpandedIds(new Set());
  }

  const expandedCount = expandedIds.size;
  const allExpanded = jobs.length > 0 && expandedCount === jobs.length;

  function renderListControls() {
    if (jobs.length <= 1) return null;
    return (
      <div className="segmented-control release-plan-jobs-view-toggle" role="group" aria-label="列表视图">
        <button
          type="button"
          className={`segmented-item${allExpanded ? " segmented-item-active" : ""}`}
          disabled={allExpanded}
          onClick={expandAll}
        >
          展开全部
        </button>
        <button
          type="button"
          className={`segmented-item${expandedCount === 0 ? " segmented-item-active" : ""}`}
          disabled={expandedCount === 0}
          onClick={collapseAll}
        >
          收起全部
        </button>
      </div>
    );
  }

  function renderToolbarSummary() {
    return (
      <div className="release-plan-jobs-toolbar-summary">
        <span className="release-plan-jobs-toolbar-title">发布项</span>
        <span className="release-plan-jobs-count">{jobs.length}</span>
        {renderListControls()}
      </div>
    );
  }

  return (
    <div className="release-plan-jobs">
      {canEdit ? (
        <div className="release-plan-jobs-toolbar">
          {renderToolbarSummary()}
          <div className="release-plan-jobs-toolbar-actions">
            <button
              type="button"
              className="btn btn-secondary text-xs release-plan-jobs-add-check"
              disabled={busy}
              onClick={() => void onAddText()}
            >
              <span className="release-plan-jobs-add-icon" aria-hidden>
                +
              </span>
              检查项
            </button>
            {workflows.length > 0 ? (
              <div className="release-plan-jobs-input-group">
                <span className="release-plan-jobs-input-group-label">工作流</span>
                <select
                  className="input text-xs"
                  value={selectedWorkflowId}
                  onChange={(e) => setSelectedWorkflowId(e.target.value)}
                  aria-label="选择工作流"
                >
                  {workflows.map((wf) => (
                    <option key={wf.id} value={wf.id}>
                      {wf.name}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  className="btn btn-primary text-xs release-plan-jobs-input-group-btn"
                  disabled={busy || !selectedWorkflowId}
                  onClick={() => void onAddWorkflow(selectedWorkflowId)}
                >
                  添加
                </button>
              </div>
            ) : (
              <span className="release-plan-jobs-toolbar-hint">请先在「工作流」Tab 创建工作流</span>
            )}
          </div>
        </div>
      ) : jobs.length > 0 ? (
        <div className="release-plan-jobs-toolbar release-plan-jobs-toolbar-readonly">{renderToolbarSummary()}</div>
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
          {jobs.map((job, index) => {
            const expanded = expandedIds.has(job.id);
            const preview = job.type === "text" ? textPreview(job.text_content ?? "") : "";
            const wfLabel = workflowName(workflows, job.workflow_ref?.workflow_id);

            return (
              <li
                key={job.id}
                className={`release-plan-job-card${expanded ? " is-expanded" : " is-collapsed"}`}
              >
                <div className="release-plan-job-card-head">
                  <button
                    type="button"
                    className="release-plan-job-card-toggle"
                    aria-expanded={expanded}
                    onClick={() => toggleExpanded(job.id)}
                  >
                    <span className="release-plan-job-index">{index + 1}</span>
                    <span className={`release-plan-job-chevron${expanded ? " is-open" : ""}`} aria-hidden />
                    <span className="release-plan-job-summary">
                      <span className="release-plan-job-summary-name">{job.name}</span>
                      {!expanded ? (
                        <span className="release-plan-job-summary-meta">
                          {job.type === "workflow" ? wfLabel : preview || "暂无说明"}
                        </span>
                      ) : null}
                    </span>
                    <span className="release-plan-job-card-badges">
                      <span className="badge badge-neutral release-plan-job-type">{jobTypeLabel(job.type)}</span>
                      <span className={`badge ${releasePlanStatusBadgeClass(job.status)}`}>
                        {releasePlanStatusLabel(job.status)}
                      </span>
                    </span>
                  </button>
                  <div
                    className="release-plan-job-card-actions"
                    onClick={(event) => event.stopPropagation()}
                    onKeyDown={(event) => event.stopPropagation()}
                  >
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
                        <button
                          type="button"
                          className="btn-ghost text-xs text-red-600 px-1.5"
                          disabled={busy}
                          onClick={() => void onDelete(job.id)}
                        >
                          删除
                        </button>
                      </>
                    ) : null}
                    {renderJobActions?.(job)}
                  </div>
                </div>

                {expanded ? (
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
                          onChange={(html) => scheduleTextUpdate(job.id, html, job.text_content ?? "")}
                          onBlur={(html) => flushTextUpdate(job.id, html, job.text_content ?? "")}
                          placeholder="检查项说明"
                        />
                      ) : job.text_content ? (
                        <div
                          className="release-plan-job-content prose prose-sm max-w-none text-sm"
                          dangerouslySetInnerHTML={{ __html: job.text_content }}
                        />
                      ) : null
                    ) : null}

                    {job.type === "workflow" && job.workflow_ref?.workflow_id ? (
                      <p className="text-xs text-muted">关联工作流 · {wfLabel}</p>
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
                          <span className="text-sm text-muted">{job.owner_id ?? "—"}</span>
                        )}
                      </label>
                    ) : null}
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
