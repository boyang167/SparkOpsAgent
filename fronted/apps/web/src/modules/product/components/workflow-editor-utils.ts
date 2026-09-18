"use client";

import type { WorkflowJobVM, WorkflowStageVM } from "@sparkops/contracts/product";
import { PRODUCT_WORKFLOW_JOB_TYPES } from "@sparkops/contracts/product";

export type EditorJob = WorkflowJobVM;
export type EditorStage = WorkflowStageVM;

export function emptyStage(index = 1): EditorStage {
  return { name: `阶段${index}`, parallel: false, exec_mode: "auto", jobs: [] };
}

export function stageExecModeLabel(mode?: string): string {
  return mode === "manual" ? "手动" : "自动";
}

export function defaultJob(type: string, index: number): EditorJob {
  const label = PRODUCT_WORKFLOW_JOB_TYPES.find((item) => item.type === type)?.label ?? type;
  const base: EditorJob = { name: `${type}-${index}`, type, spec: {} };
  switch (type) {
    case "sparkops-build":
      return {
        ...base,
        name: `build-${index}`,
        spec: {
          registry_id: "",
          branch: "main",
          project_builds: [],
          strategy: { timeout_minutes: 60, cache: false },
          notify: { on_success: false, on_failure: true, message: "" },
        },
      };
    case "sparkops-deploy":
      return {
        ...base,
        name: `deploy-${index}`,
        spec: { environment_id: "", source: "fromjob", job_name: "", deploy_contents: ["image"] },
      };
    case "sparkops-approval":
      return { ...base, name: `approval-${index}`, spec: { timeout_minutes: 60 } };
    case "freestyle":
      return { ...base, name: `freestyle-${index}`, spec: { script: "echo hello" } };
    case "notify":
      return { ...base, name: `notify-${index}`, spec: { channel: "in-app", message: "工作流通知" } };
    default:
      return { ...base, name: `${label}-${index}` };
  }
}

export function jobTypeLabel(type: string): string {
  return PRODUCT_WORKFLOW_JOB_TYPES.find((item) => item.type === type)?.label ?? type;
}

export function nextJobIndex(stages: EditorStage[]): number {
  let max = 0;
  for (const stage of stages) {
    for (const job of stage.jobs) {
      const match = /-(\d+)$/.exec(job.name);
      if (match) max = Math.max(max, Number(match[1]));
    }
  }
  return max + 1;
}
