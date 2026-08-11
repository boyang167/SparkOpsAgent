import type { CreateProjectCommand } from "@sparkops/contracts/workspace";
import {
  languageColor,
  languageLabel,
  PROJECT_LANGUAGE_OPTIONS,
  PROJECT_STATUS_OPTIONS,
} from "@sparkops/contracts/workspace";

export { formatDateTime } from "@/modules/workspace/components/template/template-utils";
export { languageColor, languageLabel, PROJECT_LANGUAGE_OPTIONS, PROJECT_STATUS_OPTIONS };

export function labelOf<T extends { value: string; label: string }>(
  options: readonly T[],
  value?: string,
) {
  return options.find((o) => o.value === value)?.label ?? value ?? "-";
}

export function statusBadgeClass(status?: string) {
  return status === "active" ? "project-tag project-tag-green" : "project-tag project-tag-gray";
}

export const EMPTY_PROJECT_FORM: CreateProjectCommand = {
  name: "",
  display_name: "",
  description: "",
  namespace: "",
  language: "go",
  git_url: "",
  secret_id: "",
  git_ref_type: "branch",
  git_ref_value: "main",
  dockerfile_path: "./Dockerfile",
  template_id: "",
  resources: {
    cpu_request: "500m",
    cpu_limit: "1000m",
    memory_request: "512Mi",
    memory_limit: "1Gi",
    replicas: 1,
    ports: [],
    command: "",
    args: "",
    project_config: "",
  },
};

export function projectDisplayName(project: { display_name?: string; name: string }) {
  return project.display_name?.trim() || project.name;
}

export function validateProjectName(name: string) {
  return /^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$/.test(name);
}
