import {
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
} from "@sparkops/contracts/product";
import type { TemplateVM } from "@sparkops/contracts/product";

export type SortField = "created_at" | "name" | "status";
export type SortOrder = "asc" | "desc";
export type TableDensity = "compact" | "comfortable";

export function labelOf<T extends { value: string; label: string }>(
  options: readonly T[],
  value?: string,
) {
  return options.find((o) => o.value === value)?.label ?? value ?? "-";
}

export function formatDateTime(value?: string) {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString("zh-CN");
}

export function statusBadgeClass(status?: string) {
  if (status === "active") return "badge badge-success";
  if (status === "inactive") return "badge badge-danger";
  if (status === "draft") return "badge badge-warning";
  return "badge badge-neutral";
}

export function templateIsEditable(template: TemplateVM) {
  return template.editable ?? !template.shared;
}

export function templateVisibilityLabel(template: TemplateVM) {
  if (template.shared) {
    return template.workspace_name ? `组织共享 · ${template.workspace_name}` : "组织共享";
  }
  return template.is_public ? "公开（本工作区）" : "私有";
}

export function truncateContent(content?: string, max = 100) {
  if (!content) return "暂无描述";
  if (content.length <= max) return content;
  return `${content.slice(0, max)}...`;
}

export function filterTemplates(
  items: TemplateVM[],
  filters: {
    keyword: string;
    kind: string;
    language: string;
    status: string;
  },
) {
  const kw = filters.keyword.trim().toLowerCase();
  return items.filter((item) => {
    if (filters.kind && item.kind !== filters.kind) return false;
    if (filters.language && item.language !== filters.language) return false;
    if (filters.status && item.status !== filters.status) return false;
    if (!kw) return true;
    return [item.name, item.detail, item.kind, item.language].some((v) =>
      v?.toLowerCase().includes(kw),
    );
  });
}

export function sortTemplates(
  items: TemplateVM[],
  sortField: SortField,
  sortOrder: SortOrder,
) {
  const sorted = [...items];
  sorted.sort((a, b) => {
    let cmp = 0;
    if (sortField === "name") {
      cmp = a.name.localeCompare(b.name, "zh-CN");
    } else if (sortField === "status") {
      cmp = (a.status ?? "").localeCompare(b.status ?? "", "zh-CN");
    } else {
      const ta = a.created_at ? new Date(a.created_at).getTime() : 0;
      const tb = b.created_at ? new Date(b.created_at).getTime() : 0;
      cmp = ta - tb;
    }
    return sortOrder === "asc" ? cmp : -cmp;
  });
  return sorted;
}

export { TEMPLATE_KIND_OPTIONS, TEMPLATE_LANGUAGE_OPTIONS, TEMPLATE_STATUS_OPTIONS };
