export interface NavLeafConfig {
  href: string;
  label: string;
}

export interface NavGroupConfig {
  id: string;
  label: string;
  items: NavLeafConfig[];
  /** showRbac | showSuperAdmin | always */
  visibility?: "rbac" | "superAdmin" | "always";
}

export const PLATFORM_NAV_TREE: NavGroupConfig[] = [
  {
    id: "delivery",
    label: "交付运维",
    items: [
      { href: "/delivery/builds", label: "构建" },
      { href: "/runtime/pods", label: "Pod 运维" },
      { href: "/cronjobs", label: "定时任务" },
      { href: "/publish/products", label: "产品清单" },
      { href: "/publish/plans", label: "迭代计划" },
      { href: "/publish/releases", label: "应用发布" },
      { href: "/templates", label: "模板" },
    ],
  },
  {
    id: "infra",
    label: "基础设施",
    items: [
      { href: "/config/configmaps", label: "配置字典" },
      { href: "/config/secrets", label: "秘钥管理" },
      { href: "/config/storage", label: "持久化存储" },
      { href: "/config/consul", label: "Consul KV" },
      { href: "/config/webhooks", label: "Webhooks" },
      { href: "/security/ingress", label: "Ingress" },
      { href: "/security/virtualservices", label: "VirtualService" },
      { href: "/discovery/services", label: "服务发现" },
      { href: "/cluster/list", label: "集群列表" },
      { href: "/cluster/sync", label: "资源同步" },
      { href: "/cluster/config", label: "密钥配置" },
    ],
  },
  {
    id: "resource",
    label: "资源中心",
    items: [
      { href: "/market/dockerfiles", label: "市场" },
      { href: "/knowledge", label: "知识库" },
    ],
  },
  {
    id: "permission",
    label: "权限管理",
    visibility: "rbac",
    items: [
      { href: "/permission/roles", label: "角色管理" },
      { href: "/permission/members", label: "成员绑定" },
      { href: "/permission/menus", label: "菜单管理" },
      { href: "/permission/apis", label: "API 管理" },
      { href: "/permission/data", label: "数据权限" },
      { href: "/permission/api-tokens", label: "API Token" },
    ],
  },
  {
    id: "monitor",
    label: "运维监控",
    items: [
      { href: "/monitor/operation", label: "操作日志" },
      { href: "/monitor/log-viewer", label: "日志查看器" },
      { href: "/monitor/server", label: "服务器状态" },
      { href: "/monitor/overview", label: "监控概览" },
      { href: "/monitor/statistics", label: "部署统计" },
      { href: "/monitor/org-audit", label: "组织审计" },
      { href: "/monitor/platform-audit", label: "平台审计" },
    ],
  },
  {
    id: "platform",
    label: "平台治理",
    visibility: "superAdmin",
    items: [
      { href: "/settings/platform/members", label: "平台用户" },
      { href: "/settings/platform/orgs", label: "平台组织" },
      { href: "/tools", label: "工具箱" },
      { href: "/proclaims", label: "公告" },
    ],
  },
  {
    id: "settings",
    label: "设置",
    items: [
      { href: "/settings/tenant", label: "组织与工作区" },
      { href: "/notifications", label: "站内信" },
      { href: "/settings/profile", label: "个人设置" },
    ],
  },
];

export function filterNavGroups(options: { showRbac: boolean; showSuperAdmin: boolean }) {
  return PLATFORM_NAV_TREE.filter((group) => {
    if (group.visibility === "rbac") return options.showRbac;
    if (group.visibility === "superAdmin") return options.showSuperAdmin;
    if (group.id === "monitor") {
      return group.items.filter(
        (item) => item.href !== "/monitor/platform-audit" || options.showSuperAdmin,
      ).length > 0;
    }
    if (group.id === "infra") {
      return group.items.filter(
        (item) =>
          !["/cluster/list", "/cluster/sync", "/cluster/config"].includes(item.href) ||
          options.showSuperAdmin,
      ).length > 0;
    }
    return true;
  }).map((group) => {
    if (group.id === "monitor") {
      return {
        ...group,
        items: group.items.filter(
          (item) => item.href !== "/monitor/platform-audit" || options.showSuperAdmin,
        ),
      };
    }
    if (group.id === "infra") {
      return {
        ...group,
        items: group.items.filter(
          (item) =>
            !["/cluster/list", "/cluster/sync", "/cluster/config"].includes(item.href) ||
            options.showSuperAdmin,
        ),
      };
    }
    return group;
  });
}
