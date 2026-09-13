import type { DeploymentOptionsVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { deployableClusters } from "./project-environment-status";

export interface DeploymentGateResult {
  allowed: boolean;
  missing: string[];
}

export interface DeploymentEntryAction {
  disabled: boolean;
  label: "发布" | "发布检查中…" | "查看部署进度" | "部署" | "部署检查中…";
  reason: string | null;
}

const ACTIVE_STATUSES = new Set(["queued", "preparing", "building", "deploying"]);

export function deploymentGate(
  project: ProjectVM,
  secrets: SecretVM[],
  options: DeploymentOptionsVM,
  activeRun: DeploymentRunVM | null,
): DeploymentGateResult {
  const missing = [...options.missing];
  const secretIds = new Set(secrets.map((item) => item.id));

  if (project.review_status !== "approved") missing.push("项目审核通过");
  if (activeRun && ACTIVE_STATUSES.has(activeRun.status)) missing.push("已有部署正在进行");
  if (!deployableClusters(options.clusters).length) missing.push("未授权集群");
  if (!project.secret_id || !secretIds.has(project.secret_id)) missing.push("Git 密钥");
  if (!project.jenkins_secret_id || !secretIds.has(project.jenkins_secret_id)) {
    missing.push("Jenkins 密钥");
  }
  if (
    !project.resources?.registry_secret_id ||
    !secretIds.has(project.resources.registry_secret_id)
  ) {
    missing.push("镜像仓库密钥");
  }
  if (
    project.resources?.enable_static_scan &&
    (!project.resources.sonarqube_secret_id ||
      !secretIds.has(project.resources.sonarqube_secret_id))
  ) {
    missing.push("Sonarqube 密钥");
  }

  const uniqueMissing = [...new Set(missing)];
  return { allowed: uniqueMissing.length === 0, missing: uniqueMissing };
}

export function deploymentEntryAction(
  project: ProjectVM,
  options: DeploymentOptionsVM | null,
  activeRun: DeploymentRunVM | null,
  loading: boolean,
  loadError: string | null,
): DeploymentEntryAction {
  if (loading) {
    return { disabled: true, label: "发布检查中…", reason: "正在检查发布条件" };
  }
  if (activeRun) {
    return { disabled: false, label: "查看部署进度", reason: null };
  }

  const missing: string[] = [];
  if (project.review_status !== "approved") missing.push("项目审核通过");
  if (loadError) missing.push(loadError);
  if (options) {
    missing.push(...options.missing);
    if (deployableClusters(options.clusters).length === 0) missing.push("未授权集群");
  } else if (!loadError) {
    missing.push("部署条件不可用");
  }
  const reason = [...new Set(missing)].join("；");
  return { disabled: Boolean(reason), label: "发布", reason: reason || null };
}

/** 打开发布向导：只需审核通过。Jenkins / 集群门禁在向导「先构建」里再拦。 */
export function releaseEntryAction(
  project: ProjectVM,
  activeRun: DeploymentRunVM | null,
  loading: boolean,
  loadError: string | null,
): DeploymentEntryAction {
  if (loading) {
    return { disabled: true, label: "发布检查中…", reason: "正在检查发布条件" };
  }
  if (activeRun) {
    return { disabled: false, label: "查看部署进度", reason: null };
  }
  if (project.review_status !== "approved") {
    return { disabled: true, label: "发布", reason: "项目审核通过" };
  }
  if (loadError) {
    return { disabled: true, label: "发布", reason: loadError };
  }
  return { disabled: false, label: "发布", reason: null };
}
