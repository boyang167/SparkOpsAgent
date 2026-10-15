import type {
  ReleasePlanJobVM,
  ReleasePlanVM,
  WorkflowRuntimeInput,
  WorkflowRuntimeInputRef,
} from "@sparkops/contracts/product";

export const MASKED_PARAMETER_VALUE = "******";

const RESERVED_JOB_KEYS = new Set(["branch", "environment_id"]);

export function canOverrideInJob(key: string): boolean {
  return Boolean(key) && !key.startsWith("_") && !RESERVED_JOB_KEYS.has(key);
}

export function formatInputRef(ref: WorkflowRuntimeInputRef): string {
  return `阶段${ref.stage_index + 1} · ${ref.job_name}`;
}

function pendingWorkflowJobs(jobs: ReleasePlanJobVM[] | undefined): ReleasePlanJobVM[] {
  return (jobs ?? []).filter((job) => job.type === "workflow" && job.status !== "skipped");
}

export interface PlanInputRow {
  key: string;
  label: string;
  secret: boolean;
  usedBy: string[];
}

export function planInputRows(jobs: ReleasePlanJobVM[]): PlanInputRow[] {
  const rows = new Map<string, PlanInputRow>();
  for (const job of pendingWorkflowJobs(jobs)) {
    for (const input of job.runtime_inputs ?? []) {
      const row = rows.get(input.key) ?? { key: input.key, label: input.label || input.key, secret: false, usedBy: [] };
      row.secret = row.secret || Boolean(input.secret);
      const places = input.refs?.length ? input.refs.map(formatInputRef) : ["运行时变量"];
      for (const place of places) {
        const label = `${job.name} · ${place}`;
        if (!row.usedBy.includes(label)) row.usedBy.push(label);
      }
      rows.set(input.key, row);
    }
  }
  return [...rows.values()];
}

export type JobInputSource = "job" | "plan" | "missing";

export interface JobInputState {
  key: string;
  label: string;
  secret: boolean;
  source: JobInputSource;
  value: string;
  overridable: boolean;
}

export function jobInputStates(
  job: ReleasePlanJobVM,
  planParams: Record<string, string> | undefined,
): JobInputState[] {
  const params = job.workflow_ref?.parameters ?? {};
  return (job.runtime_inputs ?? []).map((input) => {
    const overridable = canOverrideInJob(input.key);
    const own = overridable ? (params[input.key] ?? "").trim() : "";
    const inherited = (planParams?.[input.key] ?? "").trim();
    const source: JobInputSource = own ? "job" : inherited ? "plan" : "missing";
    return {
      key: input.key,
      label: input.label || input.key,
      secret: Boolean(input.secret),
      source,
      value: own || inherited,
      overridable,
    };
  });
}

export interface JobMissingInputs {
  jobId: string;
  jobName: string;
  keys: string[];
}

export function planMissingInputs(plan: Pick<ReleasePlanVM, "jobs" | "parameters">): JobMissingInputs[] {
  const out: JobMissingInputs[] = [];
  for (const job of pendingWorkflowJobs(plan.jobs)) {
    const keys = jobInputStates(job, plan.parameters)
      .filter((state) => state.source === "missing")
      .map((state) => state.key);
    if (keys.length > 0) out.push({ jobId: job.id, jobName: job.name, keys });
  }
  return out;
}

export function formatMissingInputs(items: JobMissingInputs[]): string {
  return `请先填写运行参数：${items.map((item) => `「${item.jobName}」${item.keys.join("、")}`).join("；")}`;
}

/** 运行弹窗额外需要填写的输入：未声明，且不只出现在版本号字段里（版本号有专门输入框）。 */
export function runModalExtraInputs(inputs: WorkflowRuntimeInput[] | undefined): WorkflowRuntimeInput[] {
  return (inputs ?? []).filter(
    (input) =>
      !input.declared && !(input.refs?.length && input.refs.every((item) => item.field === "version_tag")),
  );
}
