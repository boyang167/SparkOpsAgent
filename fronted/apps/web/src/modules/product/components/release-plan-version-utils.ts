import type { ReleasePlanVersionDiffChangeVM } from "@sparkops/contracts/product";
import { releasePlanAuditActionLabel, releasePlanStatusLabel } from "./release-plan-utils";

const VERSION_VERB_LABELS: Record<string, string> = {
  create: "创建计划",
  update: "更新计划",
  add_job: "添加发布项",
  update_job: "更新发布项",
  delete_job: "删除发布项",
  reorder_jobs: "调整顺序",
  finish_planning: "完成规划",
  reopen_planning: "重新编辑",
  add_jobs_from_work_items: "从工作项生成",
  copy: "复制计划",
  create_from_template: "从模板创建",
  backfill: "历史回填",
};

export function releasePlanVersionVerbLabel(verb: string): string {
  return VERSION_VERB_LABELS[verb] ?? releasePlanAuditActionLabel(verb);
}

export function formatDiffValue(value: unknown): string {
  if (value == null) return "—";
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value === "string") {
    if (value === "planning") return releasePlanStatusLabel(value);
    if (value === "wait_for_approval") return releasePlanStatusLabel(value);
    if (value === "executing") return releasePlanStatusLabel(value);
    if (value === "success") return releasePlanStatusLabel(value);
    if (value === "denied") return releasePlanStatusLabel(value);
    if (value === "cancel") return releasePlanStatusLabel(value);
    if (value === "text") return "检查项";
    if (value === "workflow") return "工作流";
    if (value === "internal") return "平台内审批";
    if (value === "feishu") return "飞书";
    if (value === "dingtalk") return "钉钉";
    return value.trim() || "—";
  }
  if (typeof value === "number") return String(value);
  if (Array.isArray(value)) {
    if (value.every((item) => typeof item === "string")) {
      return value.length > 0 ? value.join("、") : "—";
    }
    return JSON.stringify(value, null, 2);
  }
  if (typeof value === "object") {
    return JSON.stringify(value, null, 2);
  }
  return String(value);
}

export function diffChangeSummary(change: ReleasePlanVersionDiffChangeVM): string | null {
  if (change.change_type === "order_changed") {
    return "发布项执行顺序发生变化";
  }
  if (change.change_type === "job_added" || change.change_type === "job_removed") {
    return null;
  }
  if (change.before !== undefined || change.after !== undefined) {
    return `${formatDiffValue(change.before)} → ${formatDiffValue(change.after)}`;
  }
  return null;
}
