import type { ArtifactSource } from "../types";

export interface StoredReleaseRun {
  id: string;
  projectId: string;
  projectName: string;
  source: ArtifactSource;
  pipelineId: string;
  pipelineName: string;
  fleetIds: string[];
  fleetNames: string[];
  imageTag: string;
  imageRepository: string;
  gitRefType: string;
  gitRefValue: string;
  skippedBuildSteps: boolean;
  strategies: string[];
  deploymentRunId?: string;
  createdAt: string;
}

const KEY = "sparkops.release-runs";

function readAll(): StoredReleaseRun[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.sessionStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as StoredReleaseRun[]) : [];
  } catch {
    return [];
  }
}

export function saveReleaseRun(run: StoredReleaseRun) {
  const next = [run, ...readAll().filter((item) => item.id !== run.id)].slice(0, 20);
  window.sessionStorage.setItem(KEY, JSON.stringify(next));
}

export function getReleaseRun(id: string): StoredReleaseRun | undefined {
  return readAll().find((item) => item.id === id);
}
