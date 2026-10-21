import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ArtifactOption, ArtifactSource, PipelineStage, ReleaseDraft } from "../types";

export const BUILD_STEP_USES = new Set(["jenkins.build", "jenkins.prepare"]);

export function artifactsFromDeploymentRuns(
  runs: DeploymentRunVM[],
  imageRepository: string,
): ArtifactOption[] {
  const seen = new Set<string>();
  const artifacts: ArtifactOption[] = [];
  for (const run of runs) {
    if (run.status !== "succeeded" || !run.image_tag.trim()) continue;
    if (seen.has(run.image_tag)) continue;
    seen.add(run.image_tag);
    artifacts.push({
      image_tag: run.image_tag,
      image_repository: imageRepository,
      git_ref_type: run.git_ref_type,
      git_ref_value: run.git_ref_value,
      build_id: run.build_id,
      run_id: run.id,
    });
  }
  return artifacts;
}

export function defaultReleaseSource(artifactCount: number): ArtifactSource {
  return artifactCount > 0 ? "artifact" : "build";
}

/** Step 1 only decides artifact origin. Cluster / DeploymentRun belong to Fleet, not this step. */
export function sourceStepWarnings(_input: {
  source: ArtifactSource;
  missing: string[];
  clusterCount: number;
}): string[] {
  return [];
}

export function hasProjectJenkinsTemplate(project: {
  template_id?: string;
  resources?: {
    jenkinsfile_template_id?: string;
    deployment_template_id?: string;
    service_template_id?: string;
  };
}): boolean {
  return Boolean(
    project.resources?.jenkinsfile_template_id?.trim() ||
      project.template_id?.trim() ||
      project.resources?.deployment_template_id?.trim(),
  );
}

export function pipelineContainsBuild(stages: PipelineStage[]): boolean {
  return stages.some((stage) => stage.steps.some((step) => BUILD_STEP_USES.has(step.uses)));
}

export function imageCoordinate(repository: string, tag: string): string {
  const repo = repository.trim();
  const imageTag = tag.trim();
  if (!repo) return imageTag;
  return `${repo}:${imageTag}`;
}

export function validateReleaseDraft(
  draft: ReleaseDraft,
  ctx: { hasJenkinsTemplate: boolean; buildBlockedReasons: string[] },
): string[] {
  const errors: string[] = [];
  if (!draft.pipelineId.trim()) {
    errors.push("请选择发布 Pipeline");
  }
  if (draft.fleetIds.length === 0) {
    errors.push("请至少选择一个 Fleet");
  }
  if (draft.source === "artifact") {
    if (!draft.imageTag.trim()) {
      errors.push("请选择或填写镜像 Tag");
    }
  } else if (!draft.gitRefValue.trim()) {
    errors.push("请填写 Git 引用");
  }
  return errors;
}
