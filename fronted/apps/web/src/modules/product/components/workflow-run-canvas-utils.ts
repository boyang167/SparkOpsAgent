import type { ProductWorkflowRunVM } from "@sparkops/contracts/product";
import { jobTypeLabel } from "./workflow-editor-utils";

export type PipelineTaskKind = "build" | "deploy" | "version" | "approval";

export interface PipelineTask {
  id: string;
  stageIndex: number;
  jobName: string;
  kind: PipelineTaskKind;
  projectId?: string;
  projectLabel: string;
  jobTypeLabel: string;
  status: string;
  refId?: string;
  versionTag?: string;
  branch?: string;
  gitRefType?: string;
  imageTag?: string;
  imageRepository?: string;
  environmentId?: string;
  errorMessage?: string;
  approvalStatus?: string;
  approvedCount?: number;
  requiredApprovals?: number;
  timeoutMinutes?: number;
}

export interface PipelineStageView {
  index: number;
  name: string;
  status: string;
  tasks: PipelineTask[];
}

export function projectDisplayName(projectId: string, names: Record<string, string>): string {
  const label = names[projectId]?.trim();
  return label || projectId;
}

export function buildPipelineStages(
  run: ProductWorkflowRunVM,
  projectNames: Record<string, string>,
): PipelineStageView[] {
  return run.stage_runs.map((stage, stageIndex) => {
    const tasks: PipelineTask[] = [];
    for (const job of stage.jobs) {
      const typeLabel = jobTypeLabel(job.type);
      if (job.type === "sparkops-build" && job.builds?.length) {
        for (const build of job.builds) {
          tasks.push({
            id: `${stageIndex}:${job.name}:build:${build.project_id}`,
            stageIndex,
            jobName: job.name,
            kind: "build",
            projectId: build.project_id,
            projectLabel: projectDisplayName(build.project_id, projectNames),
            jobTypeLabel: typeLabel,
            status: build.status,
            refId: build.build_id,
            branch: build.branch,
            gitRefType: build.git_ref_type,
            imageTag: build.image_tag,
            imageRepository: build.image_repository,
            errorMessage: build.error_message,
          });
        }
        continue;
      }
      if (job.type === "sparkops-create-version" && job.versions?.length) {
        for (const version of job.versions) {
          tasks.push({
            id: `${stageIndex}:${job.name}:version:${version.project_id}`,
            stageIndex,
            jobName: job.name,
            kind: "version",
            projectId: version.project_id,
            projectLabel: `${projectDisplayName(version.project_id, projectNames)} · ${version.version_tag}`,
            jobTypeLabel: typeLabel,
            status: version.status,
            refId: version.version_id,
            versionTag: version.version_tag,
            errorMessage: version.error_message,
          });
        }
        continue;
      }
      if (
        (job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version") &&
        job.deploys?.length
      ) {
        for (const deploy of job.deploys) {
          tasks.push({
            id: `${stageIndex}:${job.name}:deploy:${deploy.project_id}`,
            stageIndex,
            jobName: job.name,
            kind: "deploy",
            projectId: deploy.project_id,
            projectLabel: projectDisplayName(deploy.project_id, projectNames),
            jobTypeLabel: typeLabel,
            status: deploy.status,
            refId: deploy.deployment_run_id || deploy.version_id,
            imageTag: deploy.image_tag,
            imageRepository: deploy.image_repository,
            environmentId: deploy.environment_id,
            errorMessage: deploy.error_message,
          });
        }
        continue;
      }
      tasks.push({
        id: `${stageIndex}:${job.name}:${job.type}`,
        stageIndex,
        jobName: job.name,
        kind: job.type === "sparkops-approval" ? "approval" : "build",
        projectLabel: job.name,
        jobTypeLabel: typeLabel,
        status: job.status,
        errorMessage: job.error_message,
        approvalStatus: job.approval?.status,
        approvedCount: job.approval?.approved_by?.length ?? 0,
        requiredApprovals: job.approval?.required_approvals ?? 1,
        timeoutMinutes: job.approval?.timeout_minutes,
      });
    }
    return { index: stageIndex, name: stage.name, status: stage.status, tasks };
  });
}

export function pickDefaultTask(stages: PipelineStageView[]): PipelineTask | null {
  for (const stage of stages) {
    const running = stage.tasks.find((task) => task.status === "running");
    if (running) return running;
  }
  for (const stage of stages) {
    const pending = stage.tasks.find((task) => task.status === "pending");
    if (pending) return pending;
  }
  for (let i = stages.length - 1; i >= 0; i--) {
    const stage = stages[i];
    if (!stage?.tasks.length) continue;
    return stage.tasks[stage.tasks.length - 1] ?? null;
  }
  return null;
}

export function formatElapsedSeconds(totalSeconds: number): string {
  if (totalSeconds < 0 || !Number.isFinite(totalSeconds)) return "0 秒";
  if (totalSeconds < 60) return `${totalSeconds} 秒`;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) return seconds > 0 ? `${minutes} 分 ${seconds} 秒` : `${minutes} 分`;
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  return restMinutes > 0 ? `${hours} 小时 ${restMinutes} 分` : `${hours} 小时`;
}

export function runElapsedSeconds(run: ProductWorkflowRunVM, nowMs = Date.now()): number {
  const start = run.started_at ?? run.created_at;
  if (!start) return 0;
  const startMs = Date.parse(start);
  if (Number.isNaN(startMs)) return 0;
  const endMs = run.completed_at ? Date.parse(run.completed_at) : nowMs;
  if (Number.isNaN(endMs)) return 0;
  return Math.max(0, Math.round((endMs - startMs) / 1000));
}

export function taskStatusTone(status: string): string {
  switch (status) {
    case "success":
    case "succeeded":
    case "approved":
      return "is-success";
    case "failed":
    case "rejected":
      return "is-failed";
    case "running":
      return "is-running";
    case "paused":
      return "is-paused";
    default:
      return "is-pending";
  }
}

export function isRunActive(status: string): boolean {
  return status === "running" || status === "pending" || status === "paused";
}
