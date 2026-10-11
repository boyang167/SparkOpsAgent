import type { DeploymentRunVM } from "@sparkops/contracts/delivery";

export type ArchiveStepState = "waiting" | "running" | "done" | "failed";

export interface ArchiveStep {
  clusterId: string;
  clusterName: string;
  state: ArchiveStepState;
  error?: string;
}

export function archiveSteps(
  pending: { clusterId: string; clusterName: string }[],
  runs: DeploymentRunVM[],
  startedAt: string,
): ArchiveStep[] {
  const since = Date.parse(startedAt);
  return pending.map((item) => {
    const latest = runs.find(
      (run) =>
        run.cluster_id === item.clusterId &&
        run.run_type === "undeploy" &&
        Date.parse(run.created_at) >= since,
    );
    if (!latest) return { ...item, state: "waiting" };
    if (latest.status === "succeeded") return { ...item, state: "done" };
    if (latest.status === "failed" || latest.status === "aborted") {
      return { ...item, state: "failed", error: latest.error_message || undefined };
    }
    return { ...item, state: "running" };
  });
}
