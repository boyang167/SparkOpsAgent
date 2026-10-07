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

const METHOD_ACTION_LABELS: Record<string, string> = {
  GET: "查询",
  POST: "创建",
  PUT: "更新",
  PATCH: "部分更新",
  DELETE: "删除",
  ANY: "代理",
};

const SEGMENT_LABELS: Record<string, string> = {
  members: "成员",
  roles: "角色",
  menus: "菜单",
  apis: "API",
  projects: "项目",
  products: "产品",
  builds: "构建",
  runtime: "运行时",
  deployment: "部署",
  orgs: "组织",
  workspaces: "工作区",
  notifications: "通知",
  auth: "认证",
  plugins: "插件",
  "member-candidates": "成员候选",
  "api-tokens": "API Token",
  "data-resources": "数据资源",
  "menu-buttons": "菜单按钮",
  "api-policies": "API 策略",
  "data-policies": "数据策略",
  entitlements: "权益",
  "release-plans": "发布计划",
  versions: "版本",
  "sprint-process": "迭代流程",
  "sprint-processes": "迭代流程",
  "sprint-process-templates": "迭代模板",
};

const PLUGIN_LABELS: Record<string, string> = {
  kubernetes: "Kubernetes",
};

function humanizeSegment(segment: string) {
  if (!segment || segment.startsWith(":")) return "";
  return SEGMENT_LABELS[segment] ?? segment.replace(/-/g, " ");
}

function methodActionLabel(method: string) {
  return METHOD_ACTION_LABELS[method.toUpperCase()] ?? method.toUpperCase();
}

/** Infer api_group + description from REST path (used during API sync preview). */
export function inferApiMeta(path: string, method: string): { api_group: string; description: string } {
  const normalized = path.split("?")[0]?.replace(/\/+$/, "") ?? "";
  const segments = normalized.split("/").filter(Boolean);
  if (segments[0] !== "api" || segments[1] !== "v1") {
    return { api_group: "default", description: methodActionLabel(method) };
  }

  const rest = segments.slice(2);
  const action = methodActionLabel(method);

  if (rest[0] === "plugins" && rest[1]) {
    const pluginId = rest[1];
    const tail = rest.slice(2).filter((s) => !s.startsWith(":"))[0];
    const pluginLabel = PLUGIN_LABELS[pluginId] ?? humanizeSegment(pluginId);
    return {
      api_group: `plugin:${pluginId}`,
      description: tail ? `${pluginLabel} · ${humanizeSegment(tail)} · ${action}` : `${pluginLabel} · ${action}`,
    };
  }

  if (rest[0] === "workspaces" && rest.length >= 3) {
    const resource = rest[2]!;
    const tail = rest.slice(3).filter((s) => !s.startsWith(":"))[0];
    const resourceLabel = humanizeSegment(resource);
    return {
      api_group: `governance:${resource}`,
      description: tail
        ? `${resourceLabel} · ${humanizeSegment(tail)} · ${action}`
        : `${resourceLabel} · ${action}`,
    };
  }

  if (rest[0] === "projects" && rest.length >= 3) {
    const tail = rest.slice(2).filter((s) => !s.startsWith(":"));
    if (tail.length >= 2) {
      return {
        api_group: `${tail[0]}:${tail[1]}`,
        description: `${humanizeSegment(tail[0])} · ${humanizeSegment(tail[1])} · ${action}`,
      };
    }
    if (tail.length === 1) {
      return {
        api_group: `project:${tail[0]}`,
        description: `${humanizeSegment(tail[0]!)} · ${action}`,
      };
    }
  }

  if (rest[0] === "products" && rest.length >= 3) {
    const resource = rest[2]!;
    const tail = rest.slice(3).filter((s) => !s.startsWith(":"))[0];
    return {
      api_group: `product:${resource}`,
      description: tail
        ? `产品 · ${humanizeSegment(resource)} · ${humanizeSegment(tail)} · ${action}`
        : `产品 · ${humanizeSegment(resource)} · ${action}`,
    };
  }

  if (rest[0] === "orgs" && rest.length >= 3) {
    const resource = rest[2]!;
    return {
      api_group: `tenant:${resource}`,
      description: `组织 · ${humanizeSegment(resource)} · ${action}`,
    };
  }

  if (rest[0]) {
    const module = rest[0];
    const tail = rest.slice(1).filter((s) => !s.startsWith(":"))[0];
    return {
      api_group: module,
      description: tail
        ? `${humanizeSegment(module)} · ${humanizeSegment(tail)} · ${action}`
        : `${humanizeSegment(module)} · ${action}`,
    };
  }

  return { api_group: "default", description: action };
}

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
  const mapNewRow = (item: unknown) => {
    const raw = item as RegisteredAPI & Record<string, unknown>;
    const path = String(raw.path ?? raw.Path ?? "");
    const method = String(raw.method ?? raw.Method ?? "GET").toUpperCase();
    const existingGroup = String(raw.api_group ?? raw.APIGroup ?? "").trim();
    const existingDesc = String(raw.description ?? raw.Description ?? "").trim();
    const inferred = inferApiMeta(path, method);
    return normalizeRegisteredAPI({
      ...raw,
      api_group: existingGroup || inferred.api_group,
      description: existingDesc || inferred.description,
    } as RegisteredAPI & Record<string, unknown>);
  };

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
