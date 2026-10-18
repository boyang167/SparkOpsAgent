import type { EnvironmentSyncProjectSpec } from "./environment-sync-job-spec";
import { DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS } from "./environment-sync-resource-kinds";
import type { RunFormEnvironmentSyncJob } from "./workflow-run-form";

export interface EnvironmentSyncResourceDiff {
  kind: string;
  name: string;
  action: "create" | "update" | "unchanged" | "excluded";
  before?: string;
  after?: string;
  note?: string;
}

export interface EnvironmentSyncPreviewProject {
  project_id: string;
  first_sync: boolean;
  sync_image: boolean;
  sync_yaml: boolean;
  source_image?: string;
  target_image?: string;
  diffs: EnvironmentSyncResourceDiff[];
}

export interface EnvironmentSyncPreviewVM {
  preview_id: string;
  source_environment_id: string;
  target_environment_id: string;
  projects: EnvironmentSyncPreviewProject[];
  warnings?: string[];
}

export interface EnvironmentSyncRunJobState {
  sourceEnvironmentId: string;
  targetEnvironmentId: string;
  projects: EnvironmentSyncProjectSpec[];
  previewId: string;
  pendingPreview: EnvironmentSyncPreviewVM | null;
}

export function createEnvironmentSyncRunState(job: RunFormEnvironmentSyncJob): EnvironmentSyncRunJobState {
  return {
    sourceEnvironmentId: job.sourceEnvironmentId,
    targetEnvironmentId: job.targetEnvironmentId,
    projects: job.projects.map((project) => ({
      ...project,
      resourceOverrides: { ...project.resourceOverrides },
    })),
    previewId: "",
    pendingPreview: null,
  };
}

function invalidatePreview(state: EnvironmentSyncRunJobState): EnvironmentSyncRunJobState {
  return { ...state, previewId: "", pendingPreview: null };
}

export function updateEnvironmentSyncSource(
  state: EnvironmentSyncRunJobState,
  sourceEnvironmentId: string,
): EnvironmentSyncRunJobState {
  if (state.sourceEnvironmentId === sourceEnvironmentId) return state;
  return invalidatePreview({ ...state, sourceEnvironmentId });
}

export function updateEnvironmentSyncTarget(
  state: EnvironmentSyncRunJobState,
  targetEnvironmentId: string,
): EnvironmentSyncRunJobState {
  if (state.targetEnvironmentId === targetEnvironmentId) return state;
  return invalidatePreview({ ...state, targetEnvironmentId });
}

export function updateEnvironmentSyncProjects(
  state: EnvironmentSyncRunJobState,
  projects: EnvironmentSyncProjectSpec[],
): EnvironmentSyncRunJobState {
  return invalidatePreview({ ...state, projects: projects.map((project) => ({ ...project })) });
}

export function updateEnvironmentSyncProject(
  state: EnvironmentSyncRunJobState,
  projectId: string,
  patch: Partial<
    Pick<EnvironmentSyncProjectSpec, "syncImage" | "syncYaml" | "syncResourceKinds" | "resourceOverrides">
  >,
): EnvironmentSyncRunJobState {
  return invalidatePreview({
    ...state,
    projects: state.projects.map((project) => {
      if (project.projectId !== projectId) return project;
      const next = { ...project, ...patch };
      if (patch.syncYaml === true && next.syncResourceKinds.length === 0) {
        next.syncResourceKinds = [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS];
      }
      if (patch.syncYaml === false) {
        next.syncResourceKinds = [];
      }
      return next;
    }),
  });
}

export function toggleEnvironmentSyncProject(
  state: EnvironmentSyncRunJobState,
  projectId: string,
  selected: boolean,
): EnvironmentSyncRunJobState {
  if (selected) {
    if (state.projects.some((project) => project.projectId === projectId)) return state;
    return invalidatePreview({
      ...state,
      projects: [{ projectId, syncImage: true, syncYaml: false, syncResourceKinds: [], resourceOverrides: {} }],
    });
  }
  return invalidatePreview({
    ...state,
    projects: state.projects.filter((project) => project.projectId !== projectId),
  });
}

export function setEnvironmentSyncPendingPreview(
  state: EnvironmentSyncRunJobState,
  preview: EnvironmentSyncPreviewVM,
): EnvironmentSyncRunJobState {
  return { ...state, pendingPreview: preview, previewId: "" };
}

export function confirmEnvironmentSyncPreview(
  state: EnvironmentSyncRunJobState,
  preview: EnvironmentSyncPreviewVM,
): EnvironmentSyncRunJobState {
  return { ...state, pendingPreview: preview, previewId: preview.preview_id };
}

export function clearEnvironmentSyncPreview(state: EnvironmentSyncRunJobState): EnvironmentSyncRunJobState {
  return invalidatePreview(state);
}

export function environmentSyncPreviewIdsFromStates(
  jobs: RunFormEnvironmentSyncJob[],
  states: Record<string, EnvironmentSyncRunJobState>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const job of jobs) {
    const previewId = states[job.jobName]?.previewId.trim() ?? "";
    if (previewId) out[job.jobName] = previewId;
  }
  return out;
}

export function allEnvironmentSyncPreviewsConfirmed(
  jobs: RunFormEnvironmentSyncJob[],
  states: Record<string, EnvironmentSyncRunJobState>,
): boolean {
  return jobs.every((job) => Boolean(states[job.jobName]?.previewId.trim()));
}

export function groupDiffsByAction(diffs: EnvironmentSyncResourceDiff[]) {
  return {
    create: diffs.filter((item) => item.action === "create"),
    update: diffs.filter((item) => item.action === "update"),
    unchanged: diffs.filter((item) => item.action === "unchanged"),
    excluded: diffs.filter((item) => item.action === "excluded"),
  };
}
