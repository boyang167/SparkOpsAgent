import type { EnvironmentSyncJobSpec, EnvironmentSyncProjectSpec } from "./environment-sync-job-spec";
import { DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS, normalizeSyncResourceKinds } from "./environment-sync-resource-kinds";

export interface EnvironmentSyncJobValidationErrors {
  sourceEnvironmentId?: string;
  targetEnvironmentId?: string;
  projects?: string;
}

export function validateEnvironmentSyncJob(
  spec: Pick<EnvironmentSyncJobSpec, "sourceEnvironmentId" | "targetEnvironmentId" | "projects">,
): EnvironmentSyncJobValidationErrors {
  const errors: EnvironmentSyncJobValidationErrors = {};
  const sourceId = spec.sourceEnvironmentId.trim();
  const targetId = spec.targetEnvironmentId.trim();

  if (!sourceId) {
    errors.sourceEnvironmentId = "请选择源环境";
  }
  if (!targetId) {
    errors.targetEnvironmentId = "请选择目标环境";
  }
  if (sourceId && targetId && sourceId === targetId) {
    errors.sourceEnvironmentId = "源环境和目标环境不能相同";
  }
  if (spec.projects.length === 0) {
    errors.projects = "请至少选择一个项目";
  }

  return errors;
}

export function isEnvironmentSyncJobValid(errors: EnvironmentSyncJobValidationErrors): boolean {
  return Object.keys(errors).length === 0;
}

export function normalizeEnvironmentSyncProjects(
  projects: EnvironmentSyncProjectSpec[],
  targetProjectIds: Set<string>,
): EnvironmentSyncProjectSpec[] {
  return projects.map((project) => {
    const firstSync = !targetProjectIds.has(project.projectId);
    if (!firstSync) {
      return {
        ...project,
        syncResourceKinds: normalizeSyncResourceKinds(
          project.syncResourceKinds,
          project.syncYaml,
          false,
        ),
      };
    }
    return {
      ...project,
      syncImage: true,
      syncYaml: true,
      syncResourceKinds: [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS],
    };
  });
}

export function isEnvironmentSyncProjectContentValid(project: EnvironmentSyncProjectSpec): boolean {
  if (!project.syncImage && !project.syncYaml) return false;
  if (project.syncYaml && project.syncResourceKinds.length === 0) return false;
  return true;
}
