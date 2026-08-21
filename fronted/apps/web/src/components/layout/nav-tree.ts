import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { filterNavGroupsByEntitlements, isPathEntitled } from "@/components/layout/entitlements";

export interface NavLeafConfig {
  href: string;
  label: string;
  /** 所需功能模块，未设置则始终可见 */
  entitlement?: "agent" | "knowledge" | "marketplace" | "delivery" | "k8s" | "observability" | "mcp";
}

export interface NavGroupConfig {
  id: string;
  label: string;
  items: NavLeafConfig[];
  /** 基础设施等分组默认折叠 */
  defaultCollapsed?: boolean;
}

export type ConsoleKind = "business" | "admin" | "platform";

/** 业务控制台：应用中心 + 运行保障 */
export const BUSINESS_NAV_TREE: NavGroupConfig[] = [
  {
    id: "application",
    label: "应用中心",
    items: [
      { href: "/projects", label: "项目管理", entitlement: "agent" },
      { href: "/knowledge", label: "知识库", entitlement: "knowledge" },
      { href: "/market/dockerfiles", label: "能力目录", entitlement: "marketplace" },
    ],
  },
  {
    id: "ops-delivery",
    label: "持续交付",
    items: [
      { href: "/delivery/builds", label: "构建管理", entitlement: "delivery" },
      { href: "/publish/products", label: "产品管理", entitlement: "delivery" },
      { href: "/publish/plans", label: "版本计划", entitlement: "delivery" },
      { href: "/publish/releases", label: "发布管理", entitlement: "delivery" },
    ],
  },
  {
    id: "ops-runtime",
    label: "运行管理",
    items: [
      { href: "/runtime/pods", label: "运行实例", entitlement: "k8s" },
      { href: "/cronjobs", label: "定时任务", entitlement: "k8s" },
    ],
  },
  {
    id: "ops-observability",
    label: "可观测性",
    items: [
      { href: "/monitor/overview", label: "监控概览", entitlement: "observability" },
      { href: "/monitor/log-viewer", label: "日志查询", entitlement: "observability" },
      { href: "/monitor/statistics", label: "部署统计", entitlement: "observability" },
    ],
  },
  {
    id: "ops-infra",
    label: "基础设施",
    defaultCollapsed: true,
    items: [
      { href: "/config/configmaps", label: "配置管理", entitlement: "k8s" },
      { href: "/config/secrets", label: "密钥管理", entitlement: "k8s" },
      { href: "/config/storage", label: "存储管理", entitlement: "k8s" },
      { href: "/discovery/services", label: "服务发现", entitlement: "k8s" },
      { href: "/security/ingress", label: "网络入口", entitlement: "k8s" },
      { href: "/security/virtualservices", label: "流量策略", entitlement: "k8s" },
      { href: "/config/webhooks", label: "外部集成", entitlement: "mcp" },
      { href: "/config/consul", label: "配置中心", entitlement: "k8s" },
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
  {
    id: "tools",
    label: "运维工具",
    items: [{ href: "/admin/tools", label: "运维工具" }],
  },
];

/** 平台运营中心 */
export const PLATFORM_OPS_NAV_TREE: NavGroupConfig[] = [
  {
    id: "platform-ops",
    label: "平台运营",
    items: [
      { href: "/platform/accounts", label: "平台账号" },
      { href: "/platform/organizations", label: "平台组织" },
      { href: "/platform/clusters", label: "集群管理" },
      { href: "/platform/clusters/sync", label: "资源同步" },
      { href: "/platform/audit", label: "平台审计" },
      { href: "/platform/settings", label: "平台配置" },
    ],
  },
];

/** @deprecated 兼容旧引用，等同于业务控制台 */
export const PLATFORM_NAV_TREE = BUSINESS_NAV_TREE;

export function resolveConsoleKind(pathname: string): ConsoleKind {
  if (pathname.startsWith("/admin")) return "admin";
  if (pathname.startsWith("/platform")) return "platform";
  return "business";
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
  "/admin/operation-logs": "操作日志",
  "/admin/org-audit": "组织审计",
  "/admin/server-status": "服务器状态",
  "/admin/tools": "运维工具",
};
