"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { EnvironmentVM, ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MemberNameLabel } from "./MemberNameLabel";
import { RichTextField } from "./RichTextField";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import {
  defaultBranchForWorkflow,
  defaultEnvironmentIdForWorkflow,
  environmentLabel,
  parseWorkflowJobBranch,
  parseWorkflowJobEnvironmentId,
  workflowHasBuildJobs,
  type WorkflowJobParamsPatch,
} from "./release-plan-workflow-params";
import { releasePlanStatusBadgeClass, releasePlanStatusLabel } from "./release-plan-utils";

interface ReleasePlanJobEditorProps {
  workspaceId: string;
  productId: string;
  plan: ReleasePlanVM;
  workflows: ProductWorkflowVM[];
  environments: EnvironmentVM[];
  canEdit: boolean;
  busy?: boolean;
  onMove: (jobId: string, direction: "up" | "down") => void | Promise<void>;
  onReorder: (jobIds: string[]) => void | Promise<void>;
  onDelete: (jobId: string) => void | Promise<void>;
  onUpdate: (
    jobId: string,
    patch: {
      name?: string;
      text_content?: string;
      text_format?: string;
      owner_id?: string;
      workflow_ref?: { mode?: string; workflow_id: string; parameters?: Record<string, string> };
    },
  ) => void | Promise<void>;
  onAddText: () => void | Promise<void>;
  onAddWorkflow: (
    workflowId: string,
    options?: { environmentId?: string; branch?: string },
  ) => void | Promise<void>;
  onWorkflowParamsChange: (
    jobId: string,
    workflowId: string,
    patch: WorkflowJobParamsPatch,
  ) => void | Promise<void>;
  renderJobActions?: (job: ReleasePlanJobVM) => ReactNode;
  memberName?: (memberId?: string | null) => string;
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
  environments,
  canEdit,
  busy,
  onMove,
  onReorder,
  onDelete,
  onUpdate,
  onAddText,
  onAddWorkflow,
  onWorkflowParamsChange,
  renderJobActions,
  memberName,
}: ReleasePlanJobEditorProps) {
  const [selectedWorkflowId, setSelectedWorkflowId] = useState(workflows[0]?.id ?? "");
  const [selectedEnvironmentId, setSelectedEnvironmentId] = useState("");
  const [selectedBranch, setSelectedBranch] = useState("main");
  const [dragJobId, setDragJobId] = useState<string | null>(null);
  const [dropJobId, setDropJobId] = useState<string | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const prevJobCount = useRef(0);

  const selectedWorkflow = useMemo(
    () => workflows.find((item) => item.id === selectedWorkflowId) ?? null,
    [workflows, selectedWorkflowId],
  );

  const jobs = [...(plan.jobs ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
  const hasDeployJobs = (selectedWorkflow?.stages ?? []).some((stage) =>
    (stage.jobs ?? []).some(
      (job) => job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version",
    ),
  );
  const hasBuildJobs = workflowHasBuildJobs(selectedWorkflow);

  useEffect(() => {
    setSelectedWorkflowId(workflows[0]?.id ?? "");
  }, [workflows]);

  useEffect(() => {
    setSelectedEnvironmentId(defaultEnvironmentIdForWorkflow(selectedWorkflow, environments));
  }, [selectedWorkflow, environments]);

  useEffect(() => {
    setSelectedBranch(defaultBranchForWorkflow(selectedWorkflow));
  }, [selectedWorkflow]);

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
      const willExpand = !next.has(jobId);
      if (willExpand) next.add(jobId);
      else next.delete(jobId);
      if (willExpand) {
        requestAnimationFrame(() => {
          document
            .getElementById(`release-plan-job-${jobId}`)
            ?.scrollIntoView({ block: "nearest", behavior: "smooth" });
        });
      }
      return next;
    });
  }

  function expandAll() {
    setExpandedIds(new Set(jobs.map((job) => job.id)));
  }

  function collapseAll() {
    setExpandedIds(new Set());
  }

  function reorderByDrop(sourceId: string, targetId: string) {
    if (sourceId === targetId) return;
    const ids = jobs.map((job) => job.id);
    const from = ids.indexOf(sourceId);
    const to = ids.indexOf(targetId);
    if (from < 0 || to < 0) return;
    const next = [...ids];
    next.splice(from, 1);
    next.splice(to, 0, sourceId);
    void onReorder(next);
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

  function renderEnvironmentSelect(
    value: string,
    onChange: (environmentId: string) => void,
    ariaLabel: string,
    disabled?: boolean,
  ) {
    if (environments.length === 0) {
      return <span className="release-plan-jobs-toolbar-hint">请先在「环境」Tab 创建环境</span>;
    }
    return (
      <select
        className="input text-xs"
        value={value}
        disabled={disabled || busy}
        aria-label={ariaLabel}
        onChange={(e) => onChange(e.target.value)}
      >
        {environments.map((env) => (
          <option key={env.id} value={env.id}>
            {env.name}
          </option>
        ))}
      </select>
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
                {hasDeployJobs ? (
                  <>
                    <span className="release-plan-jobs-input-group-label">环境</span>
                    {renderEnvironmentSelect(
                      selectedEnvironmentId,
                      setSelectedEnvironmentId,
                      "选择部署环境",
                    )}
                  </>
                ) : null}
                {hasBuildJobs ? (
                  <>
                    <span className="release-plan-jobs-input-group-label">分支</span>
                    <input
                      className="input text-xs release-plan-jobs-branch-input"
                      value={selectedBranch}
                      disabled={busy}
                      aria-label="构建分支"
                      placeholder="main"
                      onChange={(e) => setSelectedBranch(e.target.value)}
                    />
                  </>
                ) : null}
                <button
                  type="button"
                  className="btn btn-primary text-xs release-plan-jobs-input-group-btn"
                  disabled={busy || !selectedWorkflowId || (hasDeployJobs && !selectedEnvironmentId)}
                  onClick={() =>
                    void onAddWorkflow(selectedWorkflowId, {
                      ...(hasDeployJobs ? { environmentId: selectedEnvironmentId } : {}),
                      ...(hasBuildJobs ? { branch: selectedBranch.trim() || "main" } : {}),
                    })
                  }
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
            const workflowId = job.workflow_ref?.workflow_id;
            const workflowLocked = job.workflow_ref?.mode === "snapshot";
            const canEditJob = canEdit && !workflowLocked;
            const wfLabel = workflowName(workflows, workflowId);
            const jobWorkflow = workflows.find((item) => item.id === workflowId) ?? null;
            const jobHasDeploy = (jobWorkflow?.stages ?? []).some((stage) =>
              (stage.jobs ?? []).some(
                (stageJob) =>
                  stageJob.type === "sparkops-deploy" || stageJob.type === "sparkops-deploy-version",
              ),
            );
            const jobEnvironmentId = parseWorkflowJobEnvironmentId(job.workflow_ref?.parameters);
            const jobBranch = parseWorkflowJobBranch(job.workflow_ref?.parameters);
            const jobHasBuild = workflowHasBuildJobs(jobWorkflow);

            const isDragging = dragJobId === job.id;
            const isDropTarget = dropJobId === job.id && dragJobId !== job.id;

            return (
              <li
                key={job.id}
                id={`release-plan-job-${job.id}`}
                className={`release-plan-job-card${expanded ? " is-expanded" : " is-collapsed"}${isDragging ? " is-dragging" : ""}${isDropTarget ? " is-drop-target" : ""}`}
                onDragOver={(event) => {
                  if (!canEdit || !dragJobId) return;
                  event.preventDefault();
                  setDropJobId(job.id);
                }}
                onDragLeave={() => {
                  if (dropJobId === job.id) setDropJobId(null);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  if (dragJobId) reorderByDrop(dragJobId, job.id);
                  setDragJobId(null);
                  setDropJobId(null);
                }}
              >
                <div className="release-plan-job-card-head">
                  {canEdit ? (
                    <span
                      className="release-plan-job-drag-handle"
                      draggable
                      aria-label="拖拽排序"
                      onDragStart={() => setDragJobId(job.id)}
                      onDragEnd={() => {
                        setDragJobId(null);
                        setDropJobId(null);
                      }}
                    >
                      ⋮⋮
                    </span>
                  ) : null}
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
                          {job.type === "workflow"
                            ? [
                                wfLabel,
                                jobEnvironmentId ? environmentLabel(environments, jobEnvironmentId) : null,
                                jobBranch ? `分支 ${jobBranch}` : null,
                              ]
                                .filter(Boolean)
                                .join(" · ")
                            : preview || "暂无说明"}
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

                    {job.type === "workflow" && workflowId ? (
                      <div className="release-plan-job-workflow-meta stack-xs">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-xs text-muted-foreground">关联工作流 · {wfLabel}</p>
                          {workflowLocked ? <span className="badge badge-neutral text-[10px]">工作流已锁定</span> : null}
                        </div>
                        {jobHasDeploy ? (
                          <label className="release-plan-job-env">
                            <span className="release-plan-job-owner-label">部署环境</span>
                            {canEditJob ? (
                              renderEnvironmentSelect(
                                jobEnvironmentId || defaultEnvironmentIdForWorkflow(jobWorkflow, environments),
                                (environmentId) => {
                                  if (!environmentId || environmentId === jobEnvironmentId) return;
                                  void onWorkflowParamsChange(job.id, workflowId, { environmentId });
                                },
                                "部署环境",
                              )
                            ) : (
                              <span className="text-sm text-muted-foreground">
                                {environmentLabel(environments, jobEnvironmentId)}
                              </span>
                            )}
                          </label>
                        ) : null}
                        {jobHasBuild ? (
                          <label className="release-plan-job-env">
                            <span className="release-plan-job-owner-label">构建分支</span>
                            {canEditJob ? (
                              <input
                                className="input text-xs"
                                defaultValue={jobBranch || defaultBranchForWorkflow(jobWorkflow)}
                                disabled={busy}
                                aria-label="构建分支"
                                placeholder="main"
                                onBlur={(e) => {
                                  const value = e.target.value.trim();
                                  const current = jobBranch || defaultBranchForWorkflow(jobWorkflow);
                                  if (value === current) return;
                                  void onWorkflowParamsChange(job.id, workflowId, {
                                    branch: value || null,
                                  });
                                }}
                              />
                            ) : (
                              <span className="text-sm text-muted-foreground">
                                {jobBranch || defaultBranchForWorkflow(jobWorkflow)}
                              </span>
                            )}
                          </label>
                        ) : null}
                      </div>
                    ) : null}

                    {job.execution_ref?.workflow_run_id && workflowId ? (
                      <Link
                        href={`/products/${productId}/workflows/${workflowId}/runs/${job.execution_ref.workflow_run_id}`}
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
                          <MemberNameLabel
                            workspaceId={workspaceId}
                            memberId={job.owner_id}
                            memberName={memberName}
                            className="text-sm text-muted-foreground"
                          />
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
