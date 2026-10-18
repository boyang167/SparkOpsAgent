import {
  DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS,
  normalizeSyncResourceKinds,
  type EnvironmentSyncResourceKind,
} from "./environment-sync-resource-kinds";
import type { EnvironmentSyncResourceOverrides } from "./environment-sync-resource-overrides";

export interface EnvironmentSyncProjectSpec {
  projectId: string;
  syncImage: boolean;
  syncYaml: boolean;
  syncResourceKinds: EnvironmentSyncResourceKind[];
  resourceOverrides: EnvironmentSyncResourceOverrides;
}

function parseResourceOverrides(raw: unknown): EnvironmentSyncResourceOverrides {
  if (!raw || typeof raw !== "object") return {};
  const out: EnvironmentSyncResourceOverrides = {};
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof key === "string" && typeof value === "string" && value.trim()) {
      out[key] = value;
    }
  }
  return out;
}

export interface EnvironmentSyncJobSpec {
  sourceEnvironmentId: string;
  targetEnvironmentId: string;
  projects: EnvironmentSyncProjectSpec[];
}

export function parseEnvironmentSyncJobSpec(raw?: Record<string, unknown>): EnvironmentSyncJobSpec {
  const projects = Array.isArray(raw?.projects) ? raw.projects : [];
  return {
    sourceEnvironmentId: typeof raw?.source_environment_id === "string" ? raw.source_environment_id : "",
    targetEnvironmentId: typeof raw?.target_environment_id === "string" ? raw.target_environment_id : "",
    projects: projects.flatMap((value) => {
      if (!value || typeof value !== "object") return [];
      const item = value as Record<string, unknown>;
      const projectId = typeof item.project_id === "string" ? item.project_id.trim() : "";
      const syncYaml = item.sync_yaml === true;
      const rawKinds = Array.isArray(item.sync_resource_kinds)
        ? item.sync_resource_kinds.filter((value): value is string => typeof value === "string")
        : undefined;
      return projectId ? [{
        projectId,
        syncImage: item.sync_image !== false,
        syncYaml,
        syncResourceKinds: normalizeSyncResourceKinds(rawKinds, syncYaml, false),
        resourceOverrides: parseResourceOverrides(item.resource_overrides),
      }] : [];
    }),
  };
}

export function environmentSyncProjectToApiRecord(project: EnvironmentSyncProjectSpec): Record<string, unknown> {
  const record: Record<string, unknown> = {
    project_id: project.projectId,
    sync_image: project.syncImage,
    sync_yaml: project.syncYaml,
    sync_resource_kinds: project.syncYaml ? project.syncResourceKinds : [],
  };
  if (Object.keys(project.resourceOverrides).length > 0) {
    record.resource_overrides = project.resourceOverrides;
  }
  return record;
}

export function environmentSyncJobSpecToRecord(spec: EnvironmentSyncJobSpec): Record<string, unknown> {
  return {
    source_environment_id: spec.sourceEnvironmentId,
    target_environment_id: spec.targetEnvironmentId,
    projects: spec.projects.map(environmentSyncProjectToApiRecord),
  };
}
