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

const WORKFLOW_PARAM_LABELS: Record<string, string> = {
  _environment_overrides: "部署环境",
  _project_ids: "关联项目",
  _branch_overrides: "分支覆盖",
  _ref_type_overrides: "引用类型",
  version: "版本号",
  branch: "分支",
};

export interface VersionDiffDisplayField {
  label: string;
  value: string;
  mono?: boolean;
}

export interface VersionDiffDisplaySection {
  title?: string;
  fields: VersionDiffDisplayField[];
}

export function releasePlanVersionVerbLabel(verb: string): string {
  return VERSION_VERB_LABELS[verb] ?? releasePlanAuditActionLabel(verb);
}

export function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value != null && typeof value === "object" && !Array.isArray(value);
}

export function isJobSnapshot(value: unknown): value is Record<string, unknown> {
  if (!isPlainObject(value)) return false;
  return typeof value.name === "string" || typeof value.type === "string";
}

export function isWorkflowRef(value: unknown): value is Record<string, unknown> {
  if (!isPlainObject(value)) return false;
  return "workflow_id" in value || "mode" in value || "workflow_snapshot" in value;
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
    if (value === "reference") return "引用工作流";
    if (value === "snapshot") return "锁定快照";
    if (value === "已填写") return value;
    if (value === "未填写") return value;
    if (value === "已修改") return value;
    return value.trim() || "—";
  }
  if (typeof value === "number") return String(value);
  if (Array.isArray(value)) {
    if (value.every((item) => typeof item === "string")) {
      return value.length > 0 ? value.join("、") : "—";
    }
    return summarizeCollection(value);
  }
  if (isPlainObject(value)) {
    if (isJobSnapshot(value)) return summarizeJobSnapshot(value);
    if (isWorkflowRef(value)) return summarizeWorkflowRef(value);
    return summarizeObject(value);
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
    if (shouldUseStructuredDiff(change.before, change.after)) {
      return null;
    }
    return `${formatDiffValue(change.before)} → ${formatDiffValue(change.after)}`;
  }
  return null;
}

export function shouldUseStructuredDiff(before: unknown, after: unknown): boolean {
  return isJobSnapshot(before ?? after) || isWorkflowRef(before ?? after);
}

export function describeJobSnapshot(
  snapshot: unknown,
  memberName?: (memberId?: string | null) => string,
): VersionDiffDisplaySection[] {
  if (!isJobSnapshot(snapshot)) return [];
  const sections: VersionDiffDisplaySection[] = [
    {
      fields: [
        field("名称", String(snapshot.name ?? "—")),
        field("类型", formatDiffValue(snapshot.type)),
        field("排序", formatDiffValue(snapshot.sort_order)),
      ],
    },
  ];
  if (typeof snapshot.owner_id === "string" && snapshot.owner_id) {
    sections[0].fields.push(
      field("执行人", memberName ? memberName(snapshot.owner_id) : snapshot.owner_id, Boolean(memberName)),
    );
  }
  if (typeof snapshot.text_content === "string" && snapshot.text_content.trim()) {
    sections[0].fields.push(field("检查项内容", snapshot.text_content.trim()));
  }
  if (snapshot.workflow_ref != null) {
    sections.push({
      title: "工作流配置",
      fields: flattenWorkflowRefFields(snapshot.workflow_ref),
    });
  }
  return sections;
}

export function describeWorkflowRefDiff(
  before: unknown,
  after: unknown,
): VersionDiffDisplayField[] {
  const beforeFields = workflowRefFieldMap(before);
  const afterFields = workflowRefFieldMap(after);
  const keys = [...new Set([...Object.keys(beforeFields), ...Object.keys(afterFields)])].sort();
  const rows: VersionDiffDisplayField[] = [];
  for (const key of keys) {
    const b = beforeFields[key] ?? "—";
    const a = afterFields[key] ?? "—";
    if (b === a) continue;
    rows.push(field(key, `${b} → ${a}`));
  }
  return rows;
}

function workflowRefFieldMap(value: unknown): Record<string, string> {
  if (!isWorkflowRef(value)) return {};
  const out: Record<string, string> = {};
  if (value.mode != null) out["绑定方式"] = formatDiffValue(value.mode);
  if (typeof value.workflow_id === "string" && value.workflow_id) {
    out["工作流 ID"] = value.workflow_id;
  }
  const params = value.parameters;
  if (isPlainObject(params)) {
    for (const [key, raw] of Object.entries(params)) {
      if (key.startsWith("_environment_sync_preview:")) continue;
      out[workflowParamLabel(key)] = formatWorkflowParamValue(key, raw);
    }
  }
  if (value.workflow_snapshot != null) {
    out["工作流快照"] = summarizeWorkflowSnapshot(value.workflow_snapshot);
  }
  return out;
}

function flattenWorkflowRefFields(value: unknown): VersionDiffDisplayField[] {
  const map = workflowRefFieldMap(value);
  return Object.entries(map).map(([label, val]) => field(label, val, label === "工作流 ID"));
}

function field(label: string, value: string, mono = false): VersionDiffDisplayField {
  return { label, value: value || "—", mono };
}

function summarizeJobSnapshot(snapshot: Record<string, unknown>): string {
  const type = formatDiffValue(snapshot.type);
  const name = typeof snapshot.name === "string" ? snapshot.name : "发布项";
  if (snapshot.workflow_ref != null) {
    return `${name}（${type}）`;
  }
  return `${name}（${type}）`;
}

function summarizeWorkflowRef(ref: Record<string, unknown>): string {
  const parts: string[] = [];
  if (ref.mode != null) parts.push(formatDiffValue(ref.mode));
  if (typeof ref.workflow_id === "string" && ref.workflow_id) {
    parts.push(ref.workflow_id);
  }
  if (ref.workflow_snapshot != null) {
    parts.push(summarizeWorkflowSnapshot(ref.workflow_snapshot));
  }
  return parts.length > 0 ? parts.join(" · ") : "—";
}

function summarizeWorkflowSnapshot(snapshot: unknown): string {
  if (snapshot == null) return "—";
  if (isPlainObject(snapshot)) {
    const stages = Array.isArray(snapshot.stages) ? snapshot.stages : [];
    const jobCount = stages.reduce((count, stage) => {
      if (!isPlainObject(stage)) return count;
      const jobs = Array.isArray(stage.jobs) ? stage.jobs.length : 0;
      return count + jobs;
    }, 0);
    const name = typeof snapshot.name === "string" && snapshot.name.trim() ? snapshot.name.trim() : "工作流";
    if (stages.length > 0) {
      return `${name} · ${stages.length} 个阶段 · ${jobCount} 个任务`;
    }
    return `${name} · 已锁定快照`;
  }
  return "已锁定快照";
}

function summarizeCollection(value: unknown[]): string {
  if (value.length === 0) return "—";
  if (value.every((item) => typeof item === "string" || typeof item === "number")) {
    return value.map(String).join("、");
  }
  return `${value.length} 项`;
}

function summarizeObject(value: Record<string, unknown>): string {
  const keys = Object.keys(value);
  if (keys.length === 0) return "—";
  return `${keys.length} 个字段`;
}

function workflowParamLabel(key: string): string {
  return WORKFLOW_PARAM_LABELS[key] ?? key;
}

function formatWorkflowParamValue(key: string, raw: unknown): string {
  if (raw == null || raw === "") return "—";
  if (key === "_project_ids") {
    if (typeof raw === "string") {
      try {
        const parsed = JSON.parse(raw) as unknown;
        if (Array.isArray(parsed)) return parsed.map(String).join("、") || "—";
      } catch {
        return raw;
      }
    }
    if (Array.isArray(raw)) return raw.map(String).join("、") || "—";
  }
  if (key === "_environment_overrides" && typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw) as unknown;
      if (isPlainObject(parsed)) {
        return Object.entries(parsed)
          .map(([envKey, envValue]) => `${envKey}: ${String(envValue)}`)
          .join("；") || "—";
      }
    } catch {
      return raw;
    }
  }
  if (typeof raw === "string") return raw;
  if (typeof raw === "number" || typeof raw === "boolean") return String(raw);
  if (Array.isArray(raw)) return raw.map(String).join("、") || "—";
  return summarizeObject(raw as Record<string, unknown>);
}
