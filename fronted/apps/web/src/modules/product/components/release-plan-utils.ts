export const RELEASE_PLAN_STATUS_LABELS: Record<string, string> = {
  planning: "规划中",
  wait_for_approval: "待审批",
  wait_for_finish_planning_external_check: "等待规划外部校验",
  wait_for_execute_external_check: "等待执行前外部校验",
  wait_for_all_done_external_check: "等待完成外部校验",
  finish_planning_external_check_failed: "规划外部校验失败",
  execute_external_check_failed: "执行前外部校验失败",
  all_done_external_check_failed: "完成外部校验失败",
  executing: "执行中",
  success: "已完成",
  denied: "已驳回",
  cancel: "已取消",
};

export function releasePlanStatusLabel(status: string): string {
  return RELEASE_PLAN_STATUS_LABELS[status] ?? status;
}

export function defaultReleaseWindow() {
  const start = new Date();
  const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000);
  return { start: start.toISOString(), end: end.toISOString() };
}

export function toDatetimeLocalValue(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fromDatetimeLocalValue(value: string): string {
  return new Date(value).toISOString();
}

export function statusTone(status: string): string {
  if (status === "success" || status === "done") return "text-green-600";
  if (status === "failed" || status === "denied") return "text-red-600";
  if (status === "running" || status === "executing") return "text-blue-600";
  return "text-muted-foreground";
}

export function releasePlanStatusBadgeClass(status: string): string {
  switch (status) {
    case "success":
      return "badge-success";
    case "executing":
      return "badge-primary";
    case "wait_for_approval":
      return "badge-warning";
    case "denied":
    case "cancel":
      return "badge-danger";
    default:
      return "badge-neutral";
  }
}

export function formatReleaseWindow(start: string, end: string): string {
  const fmt = (iso: string) =>
    new Date(iso).toLocaleString("zh-CN", {
      year: "numeric",
      month: "numeric",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  return `${fmt(start)} — ${fmt(end)}`;
}

export function releasePlanStatusCalendarClass(status: string): string {
  switch (status) {
    case "success":
      return "release-plan-cal-event-success";
    case "executing":
      return "release-plan-cal-event-executing";
    case "wait_for_approval":
      return "release-plan-cal-event-approval";
    case "denied":
    case "cancel":
      return "release-plan-cal-event-cancelled";
    default:
      return "release-plan-cal-event-planning";
  }
}

export const RELEASE_PLAN_AUDIT_ACTION_LABELS: Record<string, string> = {
  create: "创建计划",
  update: "更新计划",
  add_job: "添加发布项",
  update_job: "更新发布项",
  delete_job: "删除发布项",
  reorder_jobs: "调整发布项顺序",
  finish_planning: "完成规划",
  approve: "审批通过",
  reject: "驳回计划",
  approve_by_token: "外部审批通过",
  reject_by_token: "外部审批驳回",
  execute: "开始执行",
  job_started: "启动工作流",
  job_done: "完成检查项",
  job_failed: "工作流失败",
  job_skip: "跳过发布项",
  job_retry: "重试工作流",
  cancel: "取消计划",
  reopen_planning: "重新编辑",
  copy: "复制计划",
  save_template: "存为模板",
  create_from_template: "从模板创建",
  add_jobs_from_work_items: "从工作项生成检查项",
  delete: "删除计划",
};

export function releasePlanCanDelete(status: string): boolean {
  return [
    "planning",
    "denied",
    "cancel",
    "success",
    "finish_planning_external_check_failed",
    "execute_external_check_failed",
    "all_done_external_check_failed",
  ].includes(status);
}

export function releasePlanAuditActionLabel(action: string): string {
  return RELEASE_PLAN_AUDIT_ACTION_LABELS[action] ?? action;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function shortId(id: string): string {
  return id.length > 8 ? `${id.slice(0, 8)}…` : id;
}

export function formatAuditDetail(detail: unknown): string | null {
  const record = asRecord(detail);
  if (!record) return null;

  const parts: string[] = [];
  if (typeof record.name === "string" && record.name.trim()) {
    parts.push(`名称：${record.name.trim()}`);
  }
  if (typeof record.reason === "string" && record.reason.trim()) {
    parts.push(`原因：${record.reason.trim()}`);
  }
  if (typeof record.remark === "string" && record.remark.trim()) {
    parts.push(`备注：${record.remark.trim()}`);
  }
  if (typeof record.note === "string" && record.note.trim()) {
    parts.push(`说明：${record.note.trim()}`);
  }
  if (typeof record.job_id === "string") {
    parts.push(`发布项：${shortId(record.job_id)}`);
  }
  if (typeof record.workflow_run_id === "string") {
    parts.push(`Run：${shortId(record.workflow_run_id)}`);
  }
  if (typeof record.error_message === "string" && record.error_message.trim()) {
    parts.push(`错误：${record.error_message.trim()}`);
  }
  if (typeof record.status === "string") {
    parts.push(`状态：${releasePlanStatusLabel(record.status)}`);
  }
  if (typeof record.count === "number") {
    parts.push(`数量：${record.count}`);
  }
  if (typeof record.source_plan_id === "string") {
    parts.push(`来源计划：${shortId(record.source_plan_id)}`);
  }
  if (typeof record.template_id === "string") {
    parts.push(`模板：${shortId(record.template_id)}`);
  }
  if (typeof record.sprint_id === "string") {
    parts.push(`迭代：${shortId(record.sprint_id)}`);
  }

  return parts.length > 0 ? parts.join(" · ") : null;
}
