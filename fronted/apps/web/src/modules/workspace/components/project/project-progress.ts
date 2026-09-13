import type { DeploymentRunVM, DeploymentStatus } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { hasSelectedProjectTemplate } from "./project-utils";

const ACTIVE_DEPLOYMENT_STATUSES = new Set<DeploymentStatus>([
  "queued",
  "preparing",
  "building",
  "deploying",
]);

export interface ProjectDeploymentHint {
  latest: DeploymentRunVM | null;
  total?: number | null;
}

/** Derive display label from latest deployment run, if any. */
export function deploymentStatusLabel(latest: DeploymentRunVM | null | undefined): string | null {
  if (!latest) return null;
  if (latest.status === "succeeded") return "已部署";
  if (ACTIVE_DEPLOYMENT_STATUSES.has(latest.status)) return "部署中";
  if (latest.status === "failed") return "部署失败";
  if (latest.status === "aborted") return "已中止";
  return null;
}

export type ProgressStatus = "done" | "active" | "pending";

export interface ProjectProgressStep {
  key: "created" | "configured" | "submitted" | "approved";
  title: string;
  subtitle: string;
  status: ProgressStatus;
}

export type ProjectPrimaryAction = "configure" | "submit" | "review" | "pending" | "deploy";

export function isProjectConfigComplete(
  project: Pick<ProjectVM, "git_url" | "git_ref_value" | "dockerfile_path" | "template_id" | "resources">,
) {
  return Boolean(
    project.git_url?.trim() &&
      project.git_ref_value?.trim() &&
      (project.dockerfile_path?.trim() || "./Dockerfile") &&
      (project.template_id?.trim() || hasSelectedProjectTemplate(project.resources)),
  );
}

export function computeProjectProgress(
  project: ProjectVM,
  deployment?: ProjectDeploymentHint | null,
): {
  steps: ProjectProgressStep[];
  configComplete: boolean;
  statusLabel: string;
} {
  const configComplete = isProjectConfigComplete(project);
  const reviewStatus = project.review_status ?? "none";
  const submitted = reviewStatus === "pending" || reviewStatus === "approved";
  const approved = reviewStatus === "approved";
  const deployedLabel = approved ? deploymentStatusLabel(deployment?.latest) : null;
  const steps: ProjectProgressStep[] = [
    { key: "created", title: "创建项目", subtitle: "基本信息", status: "done" },
    {
      key: "configured",
      title: "配置完成",
      subtitle: "项目设置",
      status: configComplete ? "done" : "active",
    },
    {
      key: "submitted",
      title: "提交审核",
      subtitle:
        reviewStatus === "rejected"
          ? "重新提交"
          : reviewStatus === "pending"
            ? "等待审核"
            : submitted
              ? "已提交"
              : "等待提交",
      status: submitted ? "done" : configComplete ? "active" : "pending",
    },
    {
      key: "approved",
      title: "审核完成",
      subtitle: approved
        ? (deployedLabel ?? "待部署")
        : reviewStatus === "pending"
          ? "等待审核"
          : "未开始",
      status: approved ? "done" : reviewStatus === "pending" ? "active" : "pending",
    },
  ];
  return {
    steps,
    configComplete,
    statusLabel: !configComplete
      ? "配置中"
      : reviewStatus === "pending"
        ? "审核中"
        : reviewStatus === "approved"
          ? (deployedLabel ?? "待部署")
          : reviewStatus === "rejected"
            ? "审核驳回"
            : "待提交审核",
  };
}

export function projectPrimaryAction(project: ProjectVM): ProjectPrimaryAction {
  if (project.review_status === "approved") return "deploy";
  if (!isProjectConfigComplete(project)) return "configure";
  if (project.can_review) return "review";
  if (project.can_submit_review) return "submit";
  if (project.review_status === "pending") return "pending";
  return "configure";
}

/** 审核中锁定配置；其余状态可打开配置向导修改。 */
export function canEditProjectConfig(project: Pick<ProjectVM, "review_status">) {
  return (project.review_status ?? "none") !== "pending";
}

export function shouldShowEditConfigAction(project: ProjectVM) {
  return canEditProjectConfig(project) && isProjectConfigComplete(project);
}

export function projectConfigActionLabel(project: ProjectVM) {
  return isProjectConfigComplete(project) ? "修改配置" : "继续配置";
}
