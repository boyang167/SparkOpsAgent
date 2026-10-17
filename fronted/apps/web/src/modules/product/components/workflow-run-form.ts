import type { ProductProjectVM, WorkflowJobVM, WorkflowStageVM } from "@sparkops/contracts/product";
import { parseBuildJobSpec } from "./build-job-spec";
import { parseCreateVersionJobSpec, runtimeVersionTag } from "./create-version-job-spec";
import { parseDeployJobSpec } from "./deploy-job-spec";
import { jobTypeLabel } from "./workflow-editor-utils";

export interface RunFormBuildRow {
  key: string;
  jobName: string;
  projectId: string;
  projectName: string;
  repoLabel: string;
  gitUrl: string;
  secretId: string;
  branch: string;
  refType: "branch" | "tag" | "pr";
}

export interface ProjectGitInfo {
  id: string;
  name?: string;
  display_name?: string | null;
  git_url?: string;
  secret_id?: string;
}

export interface RunFormDeployRow {
  projectId: string;
  projectName: string;
  buildKey: string;
  buildJobName: string;
}

export interface RunFormDeployJob {
  jobName: string;
  environmentId: string;
  rows: RunFormDeployRow[];
}

export interface RunFormOtherJob {
  jobName: string;
  typeLabel: string;
}

export interface RunFormVersionJob {
  jobName: string;
  versionTag: string;
  /** deploy job that inline-creates version from build */
  fromDeploy?: boolean;
}

export interface RunFormStage {
  id: string;
  name: string;
  buildRows: RunFormBuildRow[];
  versionJobs: RunFormVersionJob[];
  deployJobs: RunFormDeployJob[];
  otherJobs: RunFormOtherJob[];
}

export interface WorkflowRunFormModel {
  stages: RunFormStage[];
  pills: Array<{ id: string; name: string }>;
}

function projectLabel(projects: Array<ProductProjectVM | ProjectGitInfo>, projectId: string): string {
  const item = projects.find((project) => project.id === projectId);
  return item?.display_name?.trim() || item?.name || projectId;
}

function projectGit(projects: ProjectGitInfo[], projectId: string): ProjectGitInfo | undefined {
  return projects.find((project) => project.id === projectId);
}

export function repoLabelFromGitUrl(gitUrl: string, fallback: string): string {
  const trimmed = gitUrl.trim().replace(/\.git$/i, "");
  if (!trimmed) return fallback;
  const parts = trimmed.split(/[/:]/).filter((part) => part && part !== "git");
  if (parts.length >= 2) return `${parts[parts.length - 2]}/${parts[parts.length - 1]}`;
  return fallback;
}

function buildRowsForJob(
  job: WorkflowJobVM,
  products: ProductProjectVM[],
  gitProjects: ProjectGitInfo[],
): RunFormBuildRow[] {
  const spec = parseBuildJobSpec(job.spec);
  if (spec.project_builds.length === 0) {
    const rawBranch = typeof job.spec?.branch === "string" ? job.spec.branch : "";
    return [
      {
        key: job.name,
        jobName: job.name,
        projectId: "",
        projectName: job.name,
        repoLabel: "—",
        gitUrl: "",
        secretId: "",
        branch: rawBranch || "main",
        refType: "branch",
      },
    ];
  }
  return spec.project_builds.map((row, index) => {
    const bound = projectGit(gitProjects, row.project_id);
    const gitUrl = row.git_url.trim() || bound?.git_url?.trim() || "";
    const secretId = row.secret_id.trim() || bound?.secret_id?.trim() || "";
    const projectName =
      projectLabel([...gitProjects, ...products], row.project_id) ||
      row.build_name.trim() ||
      row.project_id;
    return {
      key: `${job.name}:${row.project_id || index}`,
      jobName: job.name,
      projectId: row.project_id,
      projectName,
      repoLabel: repoLabelFromGitUrl(gitUrl, projectName),
      gitUrl,
      secretId,
      branch: row.branch || "main",
      refType: row.git_ref_type === "tag" ? "tag" : "branch",
    };
  });
}

function findUpstreamJob(stages: WorkflowStageVM[], jobName: string): WorkflowJobVM | undefined {
  if (!jobName.trim()) return undefined;
  return stages.flatMap((stage) => stage.jobs).find((item) => item.name === jobName);
}

function deployVersionNeedsRunTag(stages: WorkflowStageVM[], job: WorkflowJobVM): boolean {
  const parsed = parseDeployJobSpec(job.spec, job.type);
  if (parsed.artifactSource !== "version") return false;
  if (parsed.sourceKind === "build") return true;
  if (parsed.sourceKind === "create-version") return false;
  const upstream = findUpstreamJob(stages, parsed.jobName);
  return upstream?.type === "sparkops-build";
}

function deployRows(
  job: WorkflowJobVM,
  stages: WorkflowStageVM[],
  products: ProductProjectVM[],
  gitProjects: ProjectGitInfo[],
): RunFormDeployRow[] {
  const deploy = parseDeployJobSpec(job.spec);
  const source = stages
    .flatMap((stage) => stage.jobs)
    .find((item) => item.type === "sparkops-build" && item.name === deploy.jobName);
  if (source) {
    return buildRowsForJob(source, products, gitProjects).map((row) => ({
      projectId: row.projectId,
      projectName: row.projectName,
      buildKey: row.key,
      buildJobName: source.name,
    }));
  }
  const own = parseBuildJobSpec(job.spec);
  if (own.project_builds.length > 0) {
    return own.project_builds.map((row, index) => ({
      projectId: row.project_id,
      projectName:
        projectLabel([...gitProjects, ...products], row.project_id) ||
        row.build_name.trim() ||
        row.project_id,
      buildKey: `${job.name}:${row.project_id || index}`,
      buildJobName: job.name,
    }));
  }
  return [{ projectId: "", projectName: job.name, buildKey: job.name, buildJobName: job.name }];
}

export function defaultRunBranch(stages: WorkflowStageVM[] | undefined): string {
  for (const stage of stages ?? []) {
    for (const job of stage.jobs) {
      if (job.type !== "sparkops-build") continue;
      const spec = parseBuildJobSpec(job.spec);
      const fromRow = spec.project_builds.find((row) => row.branch.trim())?.branch;
      if (fromRow) return fromRow;
      const raw = typeof job.spec?.branch === "string" ? job.spec.branch.trim() : "";
      if (raw) return raw;
    }
  }
  return "main";
}

export function defaultDeployEnvironmentId(stages: WorkflowStageVM[] | undefined): string {
  for (const stage of stages ?? []) {
    for (const job of stage.jobs) {
      if (job.type !== "sparkops-deploy" && job.type !== "sparkops-deploy-version") continue;
      const env = parseDeployJobSpec(job.spec, job.type).environmentId.trim();
      if (env) return env;
    }
  }
  return "";
}

export function buildWorkflowRunForm(
  stages: WorkflowStageVM[] | undefined,
  products: ProductProjectVM[] = [],
  gitProjects: ProjectGitInfo[] = [],
): WorkflowRunFormModel {
  const list = stages ?? [];
  const mapped = list
    .map((stage, index) => {
      const buildRows: RunFormBuildRow[] = [];
      const versionJobs: RunFormVersionJob[] = [];
      const deployJobs: RunFormDeployJob[] = [];
      const otherJobs: RunFormOtherJob[] = [];
      for (const job of stage.jobs) {
        if (job.type === "sparkops-build") {
          buildRows.push(...buildRowsForJob(job, products, gitProjects));
          continue;
        }
        if (job.type === "sparkops-create-version") {
          versionJobs.push({
            jobName: job.name,
            versionTag: runtimeVersionTag(parseCreateVersionJobSpec(job.spec).versionTag),
          });
          continue;
        }
        if (job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version") {
          const parsed = parseDeployJobSpec(job.spec, job.type);
          if (deployVersionNeedsRunTag(list, job)) {
            versionJobs.push({
              jobName: job.name,
              versionTag: "",
              fromDeploy: true,
            });
          }
          deployJobs.push({
            jobName: job.name,
            environmentId: parsed.environmentId,
            rows: parsed.artifactSource === "version" ? [] : deployRows(job, list, products, gitProjects),
          });
          continue;
        }
        otherJobs.push({ jobName: job.name, typeLabel: jobTypeLabel(job.type) });
      }
      const id = `wf-run-stage-${index}`;
      const name = stage.name.trim() || `阶段${index + 1}`;
      return { id, name, buildRows, versionJobs, deployJobs, otherJobs };
    })
    .filter(
      (stage) =>
        stage.buildRows.length +
          stage.versionJobs.length +
          stage.deployJobs.length +
          stage.otherJobs.length >
        0,
    );

  return {
    stages: mapped,
    pills: mapped.map((stage) => ({ id: stage.id, name: stage.name })),
  };
}
