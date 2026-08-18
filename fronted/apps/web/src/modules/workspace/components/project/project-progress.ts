import type { ProjectVM } from "@sparkops/contracts/workspace";

export type ProgressStatus = "done" | "active" | "pending";

export interface ProjectProgressStep {
  key: "created" | "configured" | "submitted" | "approved";
  title: string;
  subtitle: string;
  status: ProgressStatus;
}

export function isProjectConfigComplete(
  project: Pick<ProjectVM, "git_url" | "git_ref_value" | "dockerfile_path" | "template_id">,
) {
  return Boolean(
    project.git_url?.trim() &&
      project.git_ref_value?.trim() &&
      (project.dockerfile_path?.trim() || "./Dockerfile") &&
      project.template_id?.trim(),
  );
}

export function computeProjectProgress(project: ProjectVM): {
  steps: ProjectProgressStep[];
  configComplete: boolean;
  statusLabel: string;
} {
  const configComplete = isProjectConfigComplete(project);
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
      subtitle: "等待审核",
      status: configComplete ? "active" : "pending",
    },
    { key: "approved", title: "审核完成", subtitle: "审核通过", status: "pending" },
  ];
  return {
    steps,
    configComplete,
    statusLabel: "创建完成",
  };
}
