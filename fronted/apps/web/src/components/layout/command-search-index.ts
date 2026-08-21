import { buildNavSearchItems, type NavSearchItem } from "./nav-search";

export type CommandSearchKind =
  | "recent"
  | "action"
  | "menu"
  | "project"
  | "agent"
  | "workflow"
  | "knowledge"
  | "build"
  | "product"
  | "cluster";

export interface CommandSearchItem extends NavSearchItem {
  kind: CommandSearchKind;
  keywords?: string;
  score?: number;
}

export function buildQuickActions(options: {
  canManageAdmin: boolean;
  isSuperAdmin: boolean;
}): CommandSearchItem[] {
  const items: CommandSearchItem[] = [
    {
      href: "/projects/create",
      label: "新建项目",
      group: "快捷操作",
      kind: "action",
      keywords: "create project 创建",
    },
    {
      href: "/settings/profile",
      label: "个人资料",
      group: "快捷操作",
      kind: "action",
    },
    {
      href: "/notifications",
      label: "消息中心",
      group: "快捷操作",
      kind: "action",
    },
  ];

  if (options.canManageAdmin) {
    items.unshift({
      href: "/admin/members",
      label: "打开管理控制台",
      group: "快捷操作",
      kind: "action",
      keywords: "admin 管理 IAM",
    });
  }

  if (options.isSuperAdmin) {
    items.unshift({
      href: "/platform/accounts",
      label: "打开平台运营中心",
      group: "快捷操作",
      kind: "action",
      keywords: "platform super admin 平台",
    });
  }

  return items;
}

export function buildStaticCommandItems(options: {
  canManageAdmin: boolean;
  isSuperAdmin: boolean;
  entitlements?: import("@sparkops/contracts/platform").WorkspaceEntitlementsVM;
}): CommandSearchItem[] {
  const menuItems = buildNavSearchItems({
    showAdmin: options.canManageAdmin,
    showPlatform: options.isSuperAdmin,
    entitlements: options.entitlements,
  }).map((item) => ({
    ...item,
    kind: "menu" as const,
  }));

  return [...buildQuickActions(options), ...menuItems];
}

function normalize(value: string) {
  return value.trim().toLowerCase();
}

export function scoreCommandItem(item: CommandSearchItem, keyword: string): number {
  const q = normalize(keyword);
  if (!q) {
    if (item.kind === "recent") return 120;
    if (item.kind === "action") return 100;
    if (item.kind === "project") return 80;
    if (item.kind === "agent" || item.kind === "workflow") return 70;
    if (item.kind === "build" || item.kind === "product") return 65;
    if (item.kind === "cluster") return 60;
    return 50;
  }

  const label = normalize(item.label);
  const href = normalize(item.href);
  const group = normalize(item.group);
  const keywords = normalize(item.keywords ?? "");

  if (label === q) return 1000;
  if (label.startsWith(q)) return 900;
  if (label.includes(q)) return 800;
  if (keywords.includes(q)) return 750;
  if (group.includes(q)) return 600;
  if (href.includes(q)) return 500;
  return 0;
}

export function rankCommandItems(items: CommandSearchItem[], keyword: string, limit = 20) {
  const q = normalize(keyword);
  const ranked = items
    .map((item) => ({ ...item, score: scoreCommandItem(item, q) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => (b.score ?? 0) - (a.score ?? 0) || a.label.localeCompare(b.label, "zh-CN"));

  const seen = new Set<string>();
  const unique: CommandSearchItem[] = [];
  for (const item of ranked) {
    const key = `${item.href}::${item.label}`;
    if (seen.has(key)) continue;
    seen.add(key);
    unique.push(item);
    if (unique.length >= limit) break;
  }
  return unique;
}

export function groupCommandResults(items: CommandSearchItem[]) {
  const order: CommandSearchKind[] = [
    "recent",
    "action",
    "project",
    "agent",
    "workflow",
    "knowledge",
    "build",
    "product",
    "cluster",
    "menu",
  ];
  const labels: Record<CommandSearchKind, string> = {
    recent: "最近访问",
    action: "快捷操作",
    project: "项目",
    agent: "智能体",
    workflow: "工作流",
    knowledge: "知识库",
    build: "构建",
    product: "产品",
    cluster: "集群",
    menu: "菜单与页面",
  };

  return order
    .map((kind) => ({
      kind,
      label: labels[kind],
      items: items.filter((item) => item.kind === kind),
    }))
    .filter((section) => section.items.length > 0);
}

export const COMMAND_KIND_LABELS: Record<CommandSearchKind, string> = {
  recent: "最近",
  action: "操作",
  menu: "菜单",
  project: "项目",
  agent: "Agent",
  workflow: "工作流",
  knowledge: "知识库",
  build: "构建",
  product: "产品",
  cluster: "集群",
};
