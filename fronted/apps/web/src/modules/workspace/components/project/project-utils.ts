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
    service_template_id: "",
    deployment_template_id: "",
    jenkinsfile_template_id: "",
    registry_type: "Harbor",
    registry_secret_id: "",
    sonarqube_secret_id: "",
    enable_static_scan: false,
  },
};

export function projectDisplayName(project: { display_name?: string; name: string }) {
  return project.display_name?.trim() || project.name;
}

export const PROJECT_NAME_RULE_HINT =
  "仅支持英文小写字母数字 - 名称不能纯数字,不能有大写,不能有'_'";

function projectNamePatternValid(name: string) {
  return /^[a-z][a-z0-9-]*[a-z0-9]$/.test(name);
}

export function getProjectNameError(name: string): string | null {
  const value = name.trim();
  if (!value) return "请填写项目英文名";
  if (value.length < 2) return "项目英文名至少 2 个字符";
  if (/[A-Z]/.test(value)) return "项目英文名不能包含大写字母";
  if (/_/.test(value)) return "项目英文名不能包含下划线 '_'";
  if (/^\d+$/.test(value)) return "项目英文名不能为纯数字";
  if (/[^a-z0-9-]/.test(value)) return "项目英文名仅支持小写字母、数字和连字符(-)";
  if (!/^[a-z]/.test(value)) return "项目英文名须以小写字母开头";
  if (!/[a-z0-9]$/.test(value)) return "项目英文名须以字母或数字结尾";
  if (!projectNamePatternValid(value)) return PROJECT_NAME_RULE_HINT;
  return null;
}

export function validateProjectName(name: string) {
  return getProjectNameError(name) === null;
}

export interface ProjectConfigEntry {
  key: string;
  value: string;
}

export function parseProjectConfig(raw?: string): ProjectConfigEntry[] {
  const text = raw?.trim();
  if (!text) return [{ key: "", value: "" }];

  try {
    const parsed = JSON.parse(text) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const entries = Object.entries(parsed as Record<string, unknown>).map(([key, value]) => ({
        key,
        value: value == null ? "" : String(value),
      }));
      return entries.length > 0 ? entries : [{ key: "", value: "" }];
    }
  } catch {
    // KEY=VALUE or YAML-style KEY: VALUE
  }

  const entries = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"))
    .map((line) => {
      const eq = line.indexOf("=");
      const colon = line.indexOf(":");
      const sep = eq >= 0 && (colon < 0 || eq < colon) ? eq : colon;
      if (sep < 0) return { key: line, value: "" };
      return { key: line.slice(0, sep).trim(), value: line.slice(sep + 1).trim() };
    })
    .filter((entry) => entry.key);

  return entries.length > 0 ? entries : [{ key: "", value: "" }];
}

export function serializeProjectConfig(entries: ProjectConfigEntry[]): string {
  const obj: Record<string, string> = {};
  for (const entry of entries) {
    const key = entry.key.trim();
    if (!key) continue;
    obj[key] = entry.value;
  }
  return Object.keys(obj).length > 0 ? JSON.stringify(obj) : "";
}

export function resolvePrimaryTemplateId(resources?: {
  service_template_id?: string;
  deployment_template_id?: string;
  jenkinsfile_template_id?: string;
}) {
  return (
    resources?.deployment_template_id?.trim() ||
    resources?.service_template_id?.trim() ||
    resources?.jenkinsfile_template_id?.trim() ||
    ""
  );
}

export function hasSelectedProjectTemplate(resources?: {
  service_template_id?: string;
  deployment_template_id?: string;
  jenkinsfile_template_id?: string;
}) {
  return Boolean(resolvePrimaryTemplateId(resources));
}
