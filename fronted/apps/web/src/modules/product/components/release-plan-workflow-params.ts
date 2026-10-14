import type { EnvironmentVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import { defaultDeployEnvironmentId } from "./workflow-run-form";

const ENV_OVERRIDES_KEY = "_environment_overrides";
const PROJECT_IDS_KEY = "_project_ids";

export async function resolveWorkflowDetails(
  workflow: ProductWorkflowVM,
  loadWorkflow: (workflowId: string) => Promise<ProductWorkflowVM>,
): Promise<ProductWorkflowVM> {
  if (workflow.stages !== undefined) return workflow;
  return loadWorkflow(workflow.id);
}

export function workflowProjectIds(workflow: ProductWorkflowVM | null | undefined): string[] {
  const seen = new Set<string>();
  for (const stage of workflow?.stages ?? []) {
    for (const job of stage.jobs ?? []) {
      const spec = job.spec as Record<string, unknown> | undefined;
      const builds = spec?.project_builds;
      if (Array.isArray(builds)) {
        for (const raw of builds) {
          if (!raw || typeof raw !== "object") continue;
          const projectId = (raw as { project_id?: unknown }).project_id;
          if (typeof projectId === "string" && projectId.trim()) seen.add(projectId.trim());
        }
      }
      const projectIds = spec?.project_ids;
      if (Array.isArray(projectIds)) {
        for (const projectId of projectIds) {
          if (typeof projectId === "string" && projectId.trim()) seen.add(projectId.trim());
        }
      }
    }
  }
  return [...seen];
}

export function deployJobNames(workflow: ProductWorkflowVM | null | undefined): string[] {
  const names: string[] = [];
  for (const stage of workflow?.stages ?? []) {
    for (const job of stage.jobs ?? []) {
      if (job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version") {
        names.push(job.name);
      }
    }
  }
  return names;
}

export function workflowHasBuildJobs(workflow: ProductWorkflowVM | null | undefined): boolean {
  for (const stage of workflow?.stages ?? []) {
    for (const job of stage.jobs ?? []) {
      if (job.type === "sparkops-build") return true;
    }
  }
  return false;
}

export function defaultBranchForWorkflow(workflow: ProductWorkflowVM | null | undefined): string {
  for (const stage of workflow?.stages ?? []) {
    for (const job of stage.jobs ?? []) {
      if (job.type !== "sparkops-build") continue;
      const spec = job.spec as Record<string, unknown> | undefined;
      const branch = typeof spec?.branch === "string" ? spec.branch.trim() : "";
      if (branch) return branch;
      const builds = spec?.project_builds;
      if (Array.isArray(builds)) {
        for (const row of builds) {
          if (!row || typeof row !== "object") continue;
          const value = (row as { branch?: string }).branch?.trim();
          if (value) return value;
        }
      }
    }
  }
  return "main";
}

export function buildEnvironmentOverrides(
  workflow: ProductWorkflowVM | null | undefined,
  environmentId: string,
): Record<string, string> {
  const env = environmentId.trim();
  if (!env) return {};
  const overrides: Record<string, string> = {};
  for (const name of deployJobNames(workflow)) {
    overrides[name] = env;
  }
  return overrides;
}

export function buildWorkflowJobParameters(
  workflow: ProductWorkflowVM | null | undefined,
  environmentId?: string,
  branch?: string,
  projectIds?: string[],
): Record<string, string> {
  const params: Record<string, string> = {};
  const env = environmentId?.trim() ?? "";
  const ref = branch?.trim() ?? "";
  if (env) {
    const overrides = buildEnvironmentOverrides(workflow, env);
    params.environment_id = env;
    if (Object.keys(overrides).length > 0) {
      params[ENV_OVERRIDES_KEY] = JSON.stringify(overrides);
    }
  }
  if (ref) params.branch = ref;
  if (projectIds) params[PROJECT_IDS_KEY] = JSON.stringify(projectIds);
  return params;
}

export function parseWorkflowJobEnvironmentId(parameters?: Record<string, string>): string {
  return parameters?.environment_id?.trim() ?? "";
}

export function parseWorkflowJobBranch(parameters?: Record<string, string>): string {
  return parameters?.branch?.trim() ?? "";
}

export function parseWorkflowJobProjectIds(parameters?: Record<string, string>): string[] {
  const raw = parameters?.[PROJECT_IDS_KEY];
  if (!raw) return [];
  try {
    const values: unknown = JSON.parse(raw);
    return Array.isArray(values)
      ? values.filter((value): value is string => typeof value === "string" && Boolean(value.trim())).map((value) => value.trim())
      : [];
  } catch {
    return [];
  }
}

export function defaultEnvironmentIdForWorkflow(
  workflow: ProductWorkflowVM | null | undefined,
  environments: EnvironmentVM[],
): string {
  const fromWorkflow = defaultDeployEnvironmentId(workflow?.stages);
  if (fromWorkflow && environments.some((env) => env.id === fromWorkflow)) {
    return fromWorkflow;
  }
  return environments[0]?.id ?? "";
}

export function environmentLabel(environments: EnvironmentVM[], environmentId?: string): string {
  const id = environmentId?.trim();
  if (!id) return "—";
  const match = environments.find((env) => env.id === id);
  if (!match) return id.length > 8 ? `${id.slice(0, 8)}…` : id;
  return match.name;
}

export interface WorkflowJobParamsPatch {
  environmentId?: string;
  branch?: string | null;
  projectIds?: string[];
}

export function mergeWorkflowJobParameters(
  current: Record<string, string> | undefined,
  workflow: ProductWorkflowVM | null | undefined,
  patch: WorkflowJobParamsPatch,
): Record<string, string> {
  const merged = { ...(current ?? {}) };
  const envId = patch.environmentId ?? merged.environment_id ?? "";
  const branch =
    patch.branch !== undefined ? (patch.branch?.trim() ?? "") : (merged.branch?.trim() ?? "");

  if (patch.environmentId !== undefined) {
    const next = buildWorkflowJobParameters(workflow, envId, branch);
    if (next.environment_id) merged.environment_id = next.environment_id;
    else delete merged.environment_id;
    if (next[ENV_OVERRIDES_KEY]) merged[ENV_OVERRIDES_KEY] = next[ENV_OVERRIDES_KEY];
    else delete merged[ENV_OVERRIDES_KEY];
  }

  if (patch.branch !== undefined) {
    if (branch) merged.branch = branch;
    else delete merged.branch;
  }
  if (patch.projectIds !== undefined) {
    merged[PROJECT_IDS_KEY] = JSON.stringify(patch.projectIds);
  }

  return merged;
}
