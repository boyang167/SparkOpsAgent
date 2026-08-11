import { SECRET_SERVICE_TYPES, SECRET_STATUS_OPTIONS } from "@sparkops/contracts/secret";

export { formatDateTime } from "@/modules/workspace/components/template/template-utils";
export { creatorDisplayName } from "@/modules/workspace/components/template/TemplateCreator";

/** Ant Design / Element Plus light tag tones */
const SERVICE_TYPE_TAG: Record<string, string> = {
  GitLab: "secret-tag-blue",
  GitHub: "secret-tag-blue",
  Jenkins: "secret-tag-orange",
  Harbor: "secret-tag-green",
  Docker: "secret-tag-gray",
  Kubernetes: "secret-tag-purple",
  Nexus: "secret-tag-green",
  SonarQube: "secret-tag-orange",
  Prometheus: "secret-tag-red",
  Grafana: "secret-tag-purple",
  Other: "secret-tag-gray",
};

export type ConnectionStatus = "success" | "failed" | "testing";

export function serviceTypeBadgeClass(serviceType?: string) {
  const tone = SERVICE_TYPE_TAG[serviceType ?? ""] ?? "secret-tag-blue";
  return `secret-tag ${tone}`;
}

export function statusBadgeClass(status?: string) {
  return status === "active" ? "secret-tag secret-tag-green" : "secret-tag secret-tag-gray";
}

export function connectionBadgeClass(status?: ConnectionStatus) {
  if (status === "success") return "secret-tag secret-tag-green";
  if (status === "failed") return "secret-tag secret-tag-red";
  return "";
}

export function labelOf<T extends { value: string; label: string }>(
  options: readonly T[],
  value?: string,
) {
  return options.find((o) => o.value === value)?.label ?? value ?? "-";
}

export function formatSensitive(value?: string) {
  if (!value) return "-";
  if (value.length <= 8) return "******";
  return `${value.slice(0, 4)}******${value.slice(-4)}`;
}

export { SECRET_SERVICE_TYPES, SECRET_STATUS_OPTIONS };

export const PAGE_SIZES = [10, 20, 50, 100] as const;
