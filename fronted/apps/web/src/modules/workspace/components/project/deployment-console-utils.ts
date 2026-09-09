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
  const prefix = run.run_type === "rollback" ? "回滚 · " : "";
  if (ref && tag) return `${prefix}${ref} · ${tag}`;
  if (ref) return `${prefix}${ref}`;
  if (tag) return `${prefix}${tag}`;
  return `${prefix}${run.id.slice(0, 8)}`;
}

export function deploymentRunKindLabel(run: DeploymentRunVM) {
  return run.run_type === "rollback" ? "回滚部署" : "部署";
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
