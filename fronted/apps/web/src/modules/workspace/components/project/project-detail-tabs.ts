export type ProjectDetailTab =
  | "overview"
  | "config"
  | "build-logs"
  | "storage"
  | "webhooks"
  | "monitor";

export const PROJECT_DETAIL_TABS: { id: ProjectDetailTab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "config", label: "配置" },
  { id: "build-logs", label: "Build日志" },
  { id: "storage", label: "持久化存储" },
  { id: "webhooks", label: "Webhooks" },
  { id: "monitor", label: "监控" },
];

export function parseProjectDetailTab(value: string | null | undefined): ProjectDetailTab {
  if (value && PROJECT_DETAIL_TABS.some((tab) => tab.id === value)) {
    return value as ProjectDetailTab;
  }
  return "overview";
}
