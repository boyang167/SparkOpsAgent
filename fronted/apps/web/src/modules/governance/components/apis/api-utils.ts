import type { RegisteredAPI } from "@sparkops/contracts/governance";

export const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;

export const METHOD_OPTIONS = [
  { value: "POST", label: "创建", badge: "badge-success" },
  { value: "GET", label: "查看", badge: "badge-primary" },
  { value: "PUT", label: "更新", badge: "badge-warning" },
  { value: "PATCH", label: "部分更新", badge: "badge-warning" },
  { value: "DELETE", label: "删除", badge: "!bg-red-100 !text-red-700" },
] as const;

export const PAGE_SIZES = [10, 20, 50, 100] as const;

export interface ApiSearchFilters {
  path: string;
  api_group: string;
  method: string;
  description: string;
}

export const EMPTY_API_FILTERS: ApiSearchFilters = {
  path: "",
  api_group: "",
  method: "",
  description: "",
};

export function normalizeRegisteredAPI(raw: RegisteredAPI & Record<string, unknown>): RegisteredAPI {
  return {
    id: String(raw.id ?? raw.ID ?? ""),
    path: String(raw.path ?? raw.Path ?? ""),
    method: String(raw.method ?? raw.Method ?? "GET").toUpperCase(),
    api_group: String(raw.api_group ?? raw.APIGroup ?? "default"),
    description: String(raw.description ?? raw.Description ?? ""),
  };
}

export function normalizeApiSyncDiff(raw: {
  new?: unknown[] | null;
  delete?: unknown[] | null;
  ignore?: unknown[] | null;
}) {
  const mapNewRow = (item: unknown) =>
    normalizeRegisteredAPI({
      ...(item as RegisteredAPI & Record<string, unknown>),
      api_group:
        (item as RegisteredAPI & Record<string, unknown>).api_group ??
        (item as Record<string, unknown>).APIGroup ??
        "default",
      description:
        (item as RegisteredAPI & Record<string, unknown>).description ??
        (item as Record<string, unknown>).Description ??
        "",
    } as RegisteredAPI & Record<string, unknown>);

  return {
    new: (raw.new ?? []).map(mapNewRow),
    delete: (raw.delete ?? []).map((item) =>
      normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>),
    ),
    ignore: (raw.ignore ?? []).map((item) =>
      normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>),
    ),
  };
}

export function methodBadgeClass(method: string) {
  const target = METHOD_OPTIONS.find((item) => item.value === method.toUpperCase());
  return target?.badge ?? "badge-neutral";
}

export function methodLabel(method: string) {
  const target = METHOD_OPTIONS.find((item) => item.value === method.toUpperCase());
  return target?.label ?? method;
}

export function computeApiStats(apis: RegisteredAPI[]) {
  const groups = new Set<string>();
  const methods: Record<string, number> = {};
  for (const api of apis) {
    if (api.api_group) groups.add(api.api_group);
    const m = api.method.toUpperCase();
    methods[m] = (methods[m] ?? 0) + 1;
  }
  const topMethod = Object.entries(methods).sort((a, b) => b[1] - a[1])[0];
  return {
    total: apis.length,
    groups: groups.size,
    topMethod: topMethod ? `${topMethod[0]} (${topMethod[1]})` : "-",
    getCount: methods.GET ?? 0,
  };
}

export function filterApis(apis: RegisteredAPI[], filters: ApiSearchFilters) {
  const pathQ = filters.path.trim().toLowerCase();
  const groupQ = filters.api_group.trim().toLowerCase();
  const descQ = filters.description.trim().toLowerCase();
  const methodQ = filters.method.trim().toUpperCase();

  return apis.filter((api) => {
    if (pathQ && !api.path.toLowerCase().includes(pathQ)) return false;
    if (groupQ && !api.api_group.toLowerCase().includes(groupQ)) return false;
    if (descQ && !api.description.toLowerCase().includes(descQ)) return false;
    if (methodQ && api.method.toUpperCase() !== methodQ) return false;
    return true;
  });
}

export function collectApiGroups(apis: RegisteredAPI[]) {
  const groups = new Set<string>();
  for (const api of apis) {
    groups.add(api.api_group || "default");
  }
  return Array.from(groups).sort((a, b) => a.localeCompare(b, "zh-CN"));
}

export type ApiSortKey = "path" | "api_group" | "method" | "description";
export type ApiSortOrder = "asc" | "desc";

export function sortApis(apis: RegisteredAPI[], key: ApiSortKey, order: ApiSortOrder) {
  const sorted = [...apis].sort((a, b) => {
    const av = (a[key] ?? "").toLowerCase();
    const bv = (b[key] ?? "").toLowerCase();
    if (av < bv) return -1;
    if (av > bv) return 1;
    return 0;
  });
  return order === "desc" ? sorted.reverse() : sorted;
}
