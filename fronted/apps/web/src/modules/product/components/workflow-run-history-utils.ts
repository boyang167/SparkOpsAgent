import type { ProductWorkflowRunSummaryVM } from "@sparkops/contracts/product";

export function runStatusLabel(status: string): string {
  switch (status) {
    case "success":
      return "成功";
    case "failed":
      return "失败";
    case "running":
      return "运行中";
    case "pending":
      return "等待中";
    case "paused":
      return "等待手动";
    case "cancelled":
      return "已取消";
    case "succeeded":
      return "成功";
    case "aborted":
      return "已中止";
    case "skipped":
      return "已跳过";
    default:
      return status;
  }
}

export function runStatusTone(status: string): string {
  switch (status) {
    case "success":
    case "succeeded":
      return "is-success";
    case "failed":
      return "is-failed";
    case "running":
    case "pending":
      return "is-running";
    case "paused":
      return "is-paused";
    default:
      return "is-neutral";
  }
}

export interface MemberLookup {
  email: string;
  display_name?: string;
}

export function memberUsername(member: MemberLookup | undefined, memberId?: string): string {
  if (member?.email) {
    return member.email.includes("@") ? member.email.split("@")[0]! : member.email;
  }
  if (memberId) return memberId.slice(0, 8);
  return "—";
}

export function executorLabel(
  run: ProductWorkflowRunSummaryVM,
  members: Record<string, MemberLookup>,
): string {
  if (run.executor_id) {
    return memberUsername(members[run.executor_id], run.executor_id);
  }
  switch (run.trigger_source) {
    case "git":
      return "Git";
    case "webhook":
      return "Webhook";
    case "cron":
      return "定时任务";
    default:
      return "—";
  }
}

export function formatRunTime(at?: string): string {
  if (!at) return "—";
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return "—";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${month}-${day} ${hour}:${minute}`;
}

export function projectLabel(projectId: string, names: Record<string, string>): string {
  return names[projectId]?.trim() || `项目 ${projectId.slice(0, 8)}`;
}

export interface ServiceGroup {
  id: string;
  label: string;
  known: boolean;
  kinds: Array<"构建" | "部署">;
}

export function serviceGroups(
  buildIds: string[] | undefined,
  deployIds: string[] | undefined,
  names: Record<string, string>,
): ServiceGroup[] {
  const groups = new Map<string, ServiceGroup>();
  const add = (id: string, kind: "构建" | "部署") => {
    const group = groups.get(id) ?? {
      id,
      label: projectLabel(id, names),
      known: Boolean(names[id]?.trim()),
      kinds: [],
    };
    if (!group.kinds.includes(kind)) group.kinds.push(kind);
    groups.set(id, group);
  };
  for (const id of buildIds ?? []) add(id, "构建");
  for (const id of deployIds ?? []) add(id, "部署");
  return [...groups.values()];
}

export function branchSummary(run: ProductWorkflowRunSummaryVM): string {
  const branches = run.branches ?? [];
  if (branches.length === 0) return "—";
  const unique = [...new Set(branches.map((item) => item.branch.trim()).filter(Boolean))];
  return unique.join(" / ");
}
