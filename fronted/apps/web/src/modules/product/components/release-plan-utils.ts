export const RELEASE_PLAN_STATUS_LABELS: Record<string, string> = {
  planning: "规划中",
  wait_for_approval: "待审批",
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
