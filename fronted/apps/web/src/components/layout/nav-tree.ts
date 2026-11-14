import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { filterNavGroupsByEntitlements, isPathEntitled } from "@/components/layout/entitlements";

export interface NavLeafConfig {
  href: string;
  label: string;
  /** 同一聚合入口覆盖的其他页面路径，用于保持导航激活态 */
  activeHrefs?: string[];
  /** 所需功能模块，未设置则始终可见 */
  entitlement?: "agent" | "knowledge" | "delivery" | "filesystem" | "k8s" | "observability" | "mcp";
}

export interface NavGroupConfig {
  id: string;
  label: string;
  items: NavLeafConfig[];
  /** 设置等低频分组默认折叠 */
  defaultCollapsed?: boolean;
}

export type ConsoleKind = "business" | "admin" | "platform";

/** 业务控制台顶层入口，不归入任何分组 */
export const WORKPLACE_NAV_ITEM: NavLeafConfig = { href: "/dashboard/workplace", label: "工作台" };
export const KNOWLEDGE_NAV_ITEM: NavLeafConfig = {
  href: "/knowledge",
  label: "知识库",
  entitlement: "knowledge",
};
export const FILES_NAV_ITEM: NavLeafConfig = {
  href: "/files",
  label: "文件中心",
  entitlement: "filesystem",
};

/** 业务控制台顶层直达入口（知识库、文件等），按 entitlement 过滤 */
export const BUSINESS_DIRECT_NAV_ITEMS: NavLeafConfig[] = [
  KNOWLEDGE_NAV_ITEM,
  FILES_NAV_ITEM,
];

export function filterBusinessDirectNavItems(
  entitlements?: WorkspaceEntitlementsVM,
): NavLeafConfig[] {
  if (!entitlements) return BUSINESS_DIRECT_NAV_ITEMS;
  return BUSINESS_DIRECT_NAV_ITEMS.filter((item) => {
    if (!item.entitlement) return true;
    return entitlements[item.entitlement];
  });
}

/** 业务控制台按用户任务分组，知识库作为顶层直达入口。 */
export const BUSINESS_NAV_TREE: NavGroupConfig[] = [
  {
    id: "application",
    label: "产品交付",
    items: [
      { href: "/publish/products", label: "产品", entitlement: "delivery" },
      { href: "/projects", label: "项目", entitlement: "agent" },
      { href: "/publish/release-plans", label: "发布计划", entitlement: "delivery" },
      { href: "/products/workflows", label: "工作流", entitlement: "delivery" },
      { href: "/cronjobs", label: "定时构建", entitlement: "k8s" },
      { href: "/templates", label: "模板中心", entitlement: "agent" },
    ],
  },
  {
    id: "ops-runtime",
    label: "运行观测",
    items: [
      { href: "/runtime/pods", label: "运行实例", entitlement: "k8s" },
      { href: "/monitor/overview", label: "监控概览", entitlement: "observability" },
      { href: "/monitor/log-viewer", label: "日志查询", entitlement: "observability" },
    ],
  },
  {
    id: "ops-infra",
    label: "配置与集成",
    defaultCollapsed: true,
    items: [
      { href: "/config/secrets", label: "密钥管理", entitlement: "delivery" },
      { href: "/config/webhooks", label: "外部集成", entitlement: "mcp" },
      { href: "/config/mcp", label: "MCP 集成", entitlement: "mcp" },
    ],
  },
];

export interface AdminNavSection {
  id: string;
  label: string;
  items: NavLeafConfig[];
}

/** 管理控制台 IAM 导航 */
export const ADMIN_NAV_SECTIONS: AdminNavSection[] = [
  {
    id: "identity",
    label: "用户与访问",
    items: [
      { href: "/admin/members", label: "成员管理" },
      { href: "/admin/roles", label: "角色管理" },
      { href: "/admin/api-tokens", label: "访问密钥" },
    ],
  },
  {
    id: "policy",
    label: "权限策略",
    items: [
      { href: "/admin/menus", label: "菜单策略" },
      { href: "/admin/apis", label: "接口策略" },
      { href: "/admin/data-policies", label: "数据策略" },
    ],
  },
  {
    id: "tenant",
    label: "组织与空间",
    items: [
      { href: "/admin/tenant", label: "组织与工作区" },
      { href: "/admin/announcements", label: "公告管理" },
      { href: "/admin/plugins", label: "工作区插件" },
    ],
  },
  {
    id: "compliance",
    label: "安全与合规",
    items: [
      { href: "/admin/operation-logs", label: "操作日志" },
      { href: "/admin/org-audit", label: "组织审计" },
      { href: "/admin/server-status", label: "服务器状态" },
    ],
  },
];

/** 平台运营中心 */
export const PLATFORM_OPS_NAV_ITEMS: NavLeafConfig[] = [
  { href: "/platform/accounts", label: "平台账号" },
  { href: "/platform/organizations", label: "平台组织" },
  { href: "/platform/clusters", label: "集群管理" },
  { href: "/platform/plugins", label: "插件管理" },
  { href: "/platform/audit", label: "平台审计" },
  { href: "/platform/settings", label: "平台配置" },
];

export const PLATFORM_OPS_NAV_TREE: NavGroupConfig[] = [
  {
    id: "platform-ops",
    label: "平台运营",
    items: PLATFORM_OPS_NAV_ITEMS,
  },
];

/** @deprecated 兼容旧引用，等同于业务控制台 */
export const PLATFORM_NAV_TREE = BUSINESS_NAV_TREE;

export function resolveConsoleKind(pathname: string): ConsoleKind {
  if (pathname.startsWith("/admin")) return "admin";
  if (pathname.startsWith("/platform")) return "platform";
  return "business";
}

export const CONSOLE_LABELS: Record<ConsoleKind, string> = {
  business: "业务控制台",
  admin: "管理控制台",
  platform: "平台运营中心",
};

export interface ConsoleOption {
  kind: ConsoleKind;
  label: string;
  href: string;
}

export function buildConsoleOptions(
  isSuperAdmin: boolean,
  canManageAdmin: boolean,
): ConsoleOption[] {
  const options: ConsoleOption[] = [
    { kind: "business", label: CONSOLE_LABELS.business, href: "/dashboard/workplace" },
  ];
  if (canManageAdmin || isSuperAdmin) {
    options.push({ kind: "admin", label: CONSOLE_LABELS.admin, href: "/admin/members" });
  }
  if (isSuperAdmin) {
    options.push({ kind: "platform", label: CONSOLE_LABELS.platform, href: "/platform/accounts" });
  }
  return options;
}

export function filterBusinessNavGroups(options: {
  showSuperAdmin: boolean;
  entitlements?: WorkspaceEntitlementsVM;
}) {
  void options.showSuperAdmin;
  const ents = options.entitlements;
  if (!ents) return BUSINESS_NAV_TREE;

  return BUSINESS_NAV_TREE.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      if (!item.entitlement) return true;
      return ents[item.entitlement];
    }),
  })).filter((group) => group.items.length > 0);
}

export function filterNavTreeByEntitlements(
  groups: NavGroupConfig[],
  entitlements: WorkspaceEntitlementsVM,
) {
  return filterNavGroupsByEntitlements(groups, entitlements);
}

export function isNavPathEntitled(path: string, entitlements: WorkspaceEntitlementsVM) {
  return isPathEntitled(path, entitlements);
}

/** @deprecated */
export function filterNavGroups(options: { showRbac?: boolean; showSuperAdmin?: boolean }) {
  return filterBusinessNavGroups({ showSuperAdmin: options.showSuperAdmin ?? false });
}

export const ADMIN_PAGE_TITLES: Record<string, string> = {
  "/admin/members": "成员管理",
  "/admin/roles": "角色管理",
  "/admin/api-tokens": "访问密钥",
  "/admin/menus": "菜单策略",
  "/admin/apis": "接口策略",
  "/admin/data-policies": "数据策略",
  "/admin/tenant": "组织与工作区",
  "/admin/announcements": "公告管理",
  "/admin/plugins": "工作区插件",
  "/admin/operation-logs": "操作日志",
  "/admin/org-audit": "组织审计",
  "/admin/server-status": "服务器状态",
};

export const ADMIN_PAGE_DESCRIPTIONS: Record<string, string> = {
  "/admin/members": "邀请成员加入当前工作区，并分配 RBAC 角色。",
  "/admin/roles": "管理工作区角色树与权限绑定。",
  "/admin/api-tokens": "创建与管理 API 访问密钥。",
  "/admin/menus": "配置工作区菜单可见性与排序。",
  "/admin/apis": "配置接口访问策略与 RBAC 映射。",
  "/admin/data-policies": "定义数据资源隔离与访问范围。",
  "/admin/tenant": "管理组织与工作区结构，创建租户边界并切换协作上下文。",
  "/admin/plugins": "启用已部署到平台的插件，绑定当前工作区菜单与接口权限。",
  "/admin/operation-logs": "查看工作区内的操作审计记录。",
  "/admin/org-audit": "查看组织级合规与变更审计。",
  "/admin/server-status": "查看平台服务健康与运行状态。",
};

/** 管理控制台首页（IAM 概览） */
export const ADMIN_HOME_HREF = "/admin/members";
