import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { deploymentRunStatusLabel, deploymentStatusTone } from "./deployment-console-utils";

export interface EnvironmentClusterRef {
  id: string;
  cluster_name: string;
  is_build_env?: boolean;
  priority_id?: number;
  available?: boolean;
}

export function deployableClusters(clusters: EnvironmentClusterRef[]) {
  return deployTargetClusters(clusters).filter((cluster) => cluster.available !== false);
}

export interface EnvironmentDeployRow {
  clusterId: string;
  clusterName: string;
  latest: DeploymentRunVM | null;
}

export function isProjectInDeployPhase(
  project: Pick<ProjectVM, "review_status">,
): boolean {
  return (project.review_status ?? "none") === "approved";
}

export function sortEnvironmentClusters(clusters: EnvironmentClusterRef[]) {
  return [...clusters].sort(
    (left, right) =>
      (left.priority_id ?? Number.MAX_SAFE_INTEGER) -
      (right.priority_id ?? Number.MAX_SAFE_INTEGER),
  );
}

export function deployTargetClusters(clusters: EnvironmentClusterRef[]) {
  return sortEnvironmentClusters(clusters).filter((cluster) => !cluster.is_build_env);
}

export function latestRunByCluster(runs: DeploymentRunVM[]) {
  const map = new Map<string, DeploymentRunVM>();
  for (const run of runs) {
    if (!run.cluster_id || map.has(run.cluster_id)) continue;
    map.set(run.cluster_id, run);
  }
  return map;
}

export function buildEnvironmentDeployRows(
  clusters: EnvironmentClusterRef[],
  runs: DeploymentRunVM[],
): EnvironmentDeployRow[] {
  const latestByCluster = latestRunByCluster(runs);
  const deployClusters = deployTargetClusters(clusters);

  if (deployClusters.length > 0) {
    return deployClusters.map((cluster) => ({
      clusterId: cluster.id,
      clusterName: cluster.cluster_name,
      latest: latestByCluster.get(cluster.id) ?? null,
    }));
  }

  return sortEnvironmentClusters(
    [...latestByCluster.keys()].map((clusterId) => ({
      id: clusterId,
      cluster_name: clusterId,
    })),
  ).map((cluster) => ({
    clusterId: cluster.id,
    clusterName: cluster.cluster_name,
    latest: latestByCluster.get(cluster.id) ?? null,
  }));
}

export function environmentVersionLabel(run: DeploymentRunVM | null) {
  if (!run) return "—";
  const tag = run.image_tag?.trim();
  const ref = run.git_ref_value?.trim();
  if (tag && ref) return tag === ref ? tag : `${ref} · ${tag}`;
  if (tag) return tag;
  if (ref) return ref;
  return run.id.slice(0, 8);
}

export function environmentStatusLabel(run: DeploymentRunVM | null) {
  if (!run) return "未部署";
  return deploymentRunStatusLabel(run.status);
}

export function environmentStatusTone(run: DeploymentRunVM | null) {
  if (!run) return "neutral";
  return deploymentStatusTone(run.status);
}
