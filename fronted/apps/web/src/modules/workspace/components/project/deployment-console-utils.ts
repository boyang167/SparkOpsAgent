import type { DeploymentRunVM } from "@sparkops/contracts/delivery";

const ACTIVE_STATUSES = new Set(["queued", "preparing", "building", "deploying"]);

export function isActiveDeploymentRun(run: DeploymentRunVM): boolean {
  return ACTIVE_STATUSES.has(run.status);
}

export function deploymentRunStatusLabel(status: string) {
  const map: Record<string, string> = {
    queued: "排队中",
    preparing: "准备中",
    building: "构建中",
    deploying: "部署中",
    succeeded: "成功",
    failed: "失败",
    aborted: "已中止",
  };
  return map[status] ?? status;
}

export function deploymentRunTitle(run: DeploymentRunVM) {
  const ref = run.git_ref_value?.trim();
  const tag = run.image_tag?.trim();
  const prefix =
    run.run_type === "rollback"
      ? "回滚 · "
      : run.run_type === "sync"
        ? "同步 · "
        : run.run_type === "undeploy"
          ? "下线 · "
          : "";
  if (ref && tag) return `${prefix}${ref} · ${tag}`;
  if (ref) return `${prefix}${ref}`;
  if (tag) return `${prefix}${tag}`;
  return `${prefix}${run.id.slice(0, 8)}`;
}

export function deploymentRunKindLabel(run: DeploymentRunVM) {
  if (run.run_type === "undeploy") return "下线";
  if (run.run_type === "rollback") return "回滚部署";
  if (run.run_type === "sync") return "跨集群同步";
  if (run.cross_cluster) return "跨集群发布";
  return "部署";
}

export function deploymentRollbackSourceTitle(
  run: DeploymentRunVM,
  runs: DeploymentRunVM[],
) {
  if (!run.rollback_of_run_id) return null;
  const source = runs.find((item) => item.id === run.rollback_of_run_id);
  if (!source) return run.rollback_of_run_id.slice(0, 8);
  return deploymentRunTitle(source);
}

export function deploymentStatusTone(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed") return "failed";
  if (status === "aborted") return "aborted";
  if (ACTIVE_STATUSES.has(status)) return "running";
  return "neutral";
}
