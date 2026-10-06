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
          notify: [],
        },
      };
    case "sparkops-create-version":
      return {
        ...base,
        name: `create-version-${index}`,
        spec: { version_tag: "{{version}}", source: "fromjob", job_name: "", project_ids: [] },
      };
    case "sparkops-deploy":
      return {
        ...base,
        name: `deploy-${index}`,
        spec: {
          artifact_source: "build",
          environment_id: "",
          source: "fromjob",
          job_name: "",
          deploy_contents: ["image"],
          status_check: true,
        },
      };
    case "sparkops-approval":
      return {
        ...base,
        name: `approval-${index}`,
        spec: {
          provider: "sparkops",
          approver_ids: [],
          required_approvals: 1,
          timeout_minutes: 60,
          strategy: { timeout_minutes: 60, on_timeout: "fail" },
          notify: [],
        },
      };
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

export function upstreamBuildJobs(stages: EditorStage[], stageIndex: number, jobIndex: number): EditorJob[] {
  const out: EditorJob[] = [];
  for (let s = 0; s < stages.length && s <= stageIndex; s++) {
    const jobs = stages[s]?.jobs ?? [];
    const limit = s === stageIndex ? jobIndex : jobs.length;
    for (let j = 0; j < limit; j++) {
      const job = jobs[j];
      if (job?.type === "sparkops-build" && job.name.trim()) out.push(job);
    }
  }
  return out;
}

export function upstreamCreateVersionJobs(stages: EditorStage[], stageIndex: number, jobIndex: number): EditorJob[] {
  const out: EditorJob[] = [];
  for (let s = 0; s < stages.length && s <= stageIndex; s++) {
    const jobs = stages[s]?.jobs ?? [];
    const limit = s === stageIndex ? jobIndex : jobs.length;
    for (let j = 0; j < limit; j++) {
      const job = jobs[j];
      if (job?.type === "sparkops-create-version" && job.name.trim()) out.push(job);
    }
  }
  return out;
}

export function upstreamVersionSourceJobs(stages: EditorStage[], stageIndex: number, jobIndex: number): EditorJob[] {
  const out: EditorJob[] = [];
  for (let s = 0; s < stages.length && s <= stageIndex; s++) {
    const jobs = stages[s]?.jobs ?? [];
    const limit = s === stageIndex ? jobIndex : jobs.length;
    for (let j = 0; j < limit; j++) {
      const job = jobs[j];
      if (
        (job?.type === "sparkops-build" || job?.type === "sparkops-create-version") &&
        job.name.trim()
      ) {
        out.push(job);
      }
    }
  }
  return out;
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

export function pickDefaultUpstreamBuildJob(stages: EditorStage[], stageIndex: number, jobIndex: number): string {
  const upstream = upstreamBuildJobs(stages, stageIndex, jobIndex);
  return upstream[upstream.length - 1]?.name.trim() ?? "";
}

export function pickDefaultDeployUpstreamJob(
  stages: EditorStage[],
  stageIndex: number,
  jobIndex: number,
  jobType: string,
  spec?: Record<string, unknown>,
): string {
  const artifactSource =
    jobType === "sparkops-deploy-version" || spec?.artifact_source === "version" ? "version" : "build";
  if (artifactSource === "version") {
    const versionJobs = upstreamCreateVersionJobs(stages, stageIndex, jobIndex);
    if (versionJobs.length > 0) {
      return versionJobs[versionJobs.length - 1].name.trim();
    }
  }
  return pickDefaultUpstreamBuildJob(stages, stageIndex, jobIndex);
}

export function prepareNewJob(type: string, stages: EditorStage[], stageIndex: number, jobIndex: number): EditorJob {
  const job = defaultJob(type, nextJobIndex(stages));
  const spec = { ...(job.spec ?? {}) };

  if (type === "sparkops-create-version") {
    const upstream = pickDefaultUpstreamBuildJob(stages, stageIndex, jobIndex);
    if (upstream) spec.job_name = upstream;
  }

  if (type === "sparkops-deploy" || type === "sparkops-deploy-version") {
    const upstream = pickDefaultDeployUpstreamJob(stages, stageIndex, jobIndex, type, spec);
    if (upstream) spec.job_name = upstream;
  }

  return { ...job, spec };
}
