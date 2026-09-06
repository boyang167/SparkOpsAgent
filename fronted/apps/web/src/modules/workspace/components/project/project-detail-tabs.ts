export type ProjectDetailTab =
  | "overview"
  | "yaml"
  | "jenkins"
  | "dockerfile"
  | "config"
  | "build-logs"
  | "storage"
  | "webhooks"
  | "monitor";

export const PROJECT_DETAIL_TABS_PUBLISHED: { id: ProjectDetailTab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "config", label: "配置" },
  { id: "build-logs", label: "Build日志" },
  { id: "storage", label: "持久化存储" },
  { id: "webhooks", label: "Webhooks" },
  { id: "monitor", label: "监控" },
];

export const PROJECT_DETAIL_TABS_UNPUBLISHED: { id: ProjectDetailTab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "yaml", label: "Yaml" },
  { id: "jenkins", label: "Jenkins" },
  { id: "dockerfile", label: "Dockerfile" },
];

/** @deprecated use PROJECT_DETAIL_TABS_PUBLISHED */
export const PROJECT_DETAIL_TABS = PROJECT_DETAIL_TABS_PUBLISHED;

export function projectDetailTabs(published: boolean) {
  return published ? PROJECT_DETAIL_TABS_PUBLISHED : PROJECT_DETAIL_TABS_UNPUBLISHED;
}

export function parseProjectDetailTab(
  value: string | null | undefined,
  published = true,
): ProjectDetailTab {
  const tabs = projectDetailTabs(published);
  if (value && tabs.some((tab) => tab.id === value)) {
    return value as ProjectDetailTab;
  }
  return "overview";
}
