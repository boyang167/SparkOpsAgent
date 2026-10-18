import type { EnvironmentProjectStatusVM } from "@sparkops/contracts/product";
import { formatSyncResourceKinds } from "./environment-sync-resource-kinds";

export const SYNC_OPTION_IMAGE_LABEL = "容器镜像";
export const SYNC_OPTION_IMAGE_HINT = "同步运行中的镜像仓库与版本号";
export const SYNC_OPTION_YAML_LABEL = "部署配置";
export const SYNC_OPTION_YAML_HINT = "同步 Deployment、Service 等 K8s 资源配置";

export interface ProjectSyncContentLine {
  label: string;
  value: string;
  muted?: boolean;
}

function projectStatus(
  projects: EnvironmentProjectStatusVM[] | undefined,
  projectId: string,
): EnvironmentProjectStatusVM | undefined {
  return projects?.find((item) => item.project_id === projectId);
}

export function formatEnvironmentImage(project?: EnvironmentProjectStatusVM): string {
  const full = project?.current_image?.trim();
  if (full) return full;
  const tag = project?.current_image_tag?.trim();
  return tag || "未记录";
}

function formatDeployedAt(at: string | undefined): string | null {
  if (!at?.trim()) return null;
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function buildProjectSyncContentLines(input: {
  projectId: string;
  sourceProjects?: EnvironmentProjectStatusVM[];
  targetProjects?: EnvironmentProjectStatusVM[];
  syncImage: boolean;
  syncYaml: boolean;
  syncResourceKinds?: string[];
  firstSync: boolean;
}): ProjectSyncContentLine[] {
  const source = projectStatus(input.sourceProjects, input.projectId);
  const target = projectStatus(input.targetProjects, input.projectId);
  const lines: ProjectSyncContentLine[] = [];

  if (input.syncImage) {
    const sourceImage = formatEnvironmentImage(source);
    const targetImage = input.firstSync ? "未部署" : formatEnvironmentImage(target);
    lines.push({
      label: "容器镜像",
      value: `${sourceImage} → ${targetImage}`,
    });
  }

  if (input.syncYaml) {
    lines.push({
      label: "部署配置",
      value: formatSyncResourceKinds(input.syncResourceKinds ?? []),
    });
  }

  const deployedAt = formatDeployedAt(source?.last_deployed_at);
  if (deployedAt) {
    lines.push({
      label: "源环境最近部署",
      value: deployedAt,
      muted: true,
    });
  }

  if (lines.length === 0) {
    lines.push({
      label: "同步内容",
      value: "请至少选择一项同步内容",
      muted: true,
    });
  }

  return lines;
}
