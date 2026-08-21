import {
  ADMIN_NAV_SECTIONS,
  ADMIN_PAGE_TITLES,
  BUSINESS_NAV_TREE,
  PLATFORM_OPS_NAV_TREE,
} from "@/components/layout/nav-tree";

export interface RouteLabelRule {
  prefix: string;
  label: string;
  group: string;
  exact?: boolean;
}

const STATIC_RULES: RouteLabelRule[] = [
  { prefix: "/dashboard/workplace", label: "工作台", group: "概览", exact: true },
  { prefix: "/dashboard/module-unavailable", label: "模块不可用", group: "概览", exact: true },
  { prefix: "/projects/create", label: "新建项目", group: "项目", exact: true },
  { prefix: "/settings/profile", label: "个人资料", group: "设置", exact: true },
  { prefix: "/settings/rbac", label: "权限说明", group: "设置", exact: true },
  { prefix: "/notifications", label: "消息中心", group: "概览", exact: true },
];

function rulesFromNavTree(): RouteLabelRule[] {
  const rules: RouteLabelRule[] = [];
  for (const group of BUSINESS_NAV_TREE) {
    for (const item of group.items) {
      rules.push({ prefix: item.href, label: item.label, group: group.label, exact: true });
    }
  }
  for (const group of PLATFORM_OPS_NAV_TREE) {
    for (const item of group.items) {
      rules.push({ prefix: item.href, label: item.label, group: group.label, exact: true });
    }
  }
  for (const section of ADMIN_NAV_SECTIONS) {
    for (const item of section.items) {
      rules.push({ prefix: item.href, label: item.label, group: section.label, exact: true });
    }
  }
  for (const [href, label] of Object.entries(ADMIN_PAGE_TITLES)) {
    rules.push({ prefix: href, label, group: "管理", exact: true });
  }
  return rules;
}

const DYNAMIC_RULES: RouteLabelRule[] = [
  { prefix: "/projects/", label: "项目详情", group: "项目" },
  { prefix: "/knowledge/", label: "知识库", group: "应用中心" },
  { prefix: "/delivery/builds/", label: "构建详情", group: "持续交付" },
  { prefix: "/market/", label: "能力目录", group: "应用中心" },
];

export const ROUTE_LABEL_RULES: RouteLabelRule[] = [
  ...STATIC_RULES,
  ...rulesFromNavTree(),
  ...DYNAMIC_RULES,
];

const SEGMENT_LABELS: Record<string, string> = {
  agents: "智能体",
  workflows: "工作流",
  builds: "构建",
  products: "产品",
  plans: "版本计划",
  releases: "发布",
  pods: "运行实例",
  cronjobs: "定时任务",
  configmaps: "配置管理",
  secrets: "密钥管理",
  clusters: "集群管理",
  sync: "资源同步",
};

function labelFromPathSegments(pathname: string): string | null {
  const parts = pathname.split("/").filter(Boolean);
  if (parts[0] === "projects" && parts.length >= 3) {
    const segment = parts[2];
    if (SEGMENT_LABELS[segment]) {
      return SEGMENT_LABELS[segment];
    }
  }
  if (parts[0] === "delivery" && parts[1] === "builds" && parts[2]) {
    return "构建详情";
  }
  const last = parts[parts.length - 1];
  return SEGMENT_LABELS[last] ?? null;
}

export function resolveRouteVisitMeta(pathname: string): { label: string; group: string } {
  for (const rule of ROUTE_LABEL_RULES) {
    const matched = rule.exact
      ? pathname === rule.prefix
      : pathname === rule.prefix || pathname.startsWith(`${rule.prefix}/`) || pathname.startsWith(rule.prefix);
    if (matched) {
      const segmentLabel = labelFromPathSegments(pathname);
      return {
        label: segmentLabel && !rule.exact ? segmentLabel : rule.label,
        group: rule.group,
      };
    }
  }

  const segmentLabel = labelFromPathSegments(pathname);
  if (segmentLabel) {
    return { label: segmentLabel, group: "页面" };
  }

  return { label: pathname, group: "页面" };
}
