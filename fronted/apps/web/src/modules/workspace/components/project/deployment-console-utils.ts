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
  if (ref && tag) return `${ref} · ${tag}`;
  if (ref) return ref;
  if (tag) return tag;
  return run.id.slice(0, 8);
}

export function deploymentStatusTone(status: string) {
  if (status === "succeeded") return "success";
  if (status === "failed") return "failed";
  if (status === "aborted") return "aborted";
  if (ACTIVE_STATUSES.has(status)) return "running";
  return "neutral";
}
