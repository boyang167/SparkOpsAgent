import type { WorkbenchTab } from "./workbench-state";

export type FilesystemPlaneId = "data" | "control" | "governance";

export type FilesystemModuleId =
  | "workspace"
  | "transfers"
  | "search"
  | "providers"
  | "pools"
  | "bindings"
  | "placement"
  | "health"
  | "access"
  | "approvals"
  | "quota"
  | "audit"
  | "lifecycle"
  | "agent";

export type FilesystemPlane = {
  id: FilesystemPlaneId;
  label: string;
  eyebrow: string;
  defaultModule: FilesystemModuleId;
};

export type FilesystemModule = {
  id: FilesystemModuleId;
  plane: FilesystemPlaneId;
  label: string;
  href: string;
  permission?: string;
  organizationPermission?: string;
  workbenchTab?: WorkbenchTab;
  placeholder?: boolean;
};

export const FILESYSTEM_DEFAULT_ENTRY = "/files/data/workspace";

export const FILESYSTEM_PLANES: FilesystemPlane[] = [
  { id: "data", label: "数据面", eyebrow: "DATA PLANE", defaultModule: "workspace" },
  { id: "control", label: "控制面", eyebrow: "CONTROL PLANE", defaultModule: "providers" },
  { id: "governance", label: "治理面", eyebrow: "GOVERNANCE PLANE", defaultModule: "audit" },
];

export const FILESYSTEM_MODULES: FilesystemModule[] = [
  {
    id: "workspace",
    plane: "data",
    label: "文件空间",
    href: "/files/data/workspace",
    permission: "file:read",
    workbenchTab: "files",
  },
  {
    id: "transfers",
    plane: "data",
    label: "传输中心",
    href: "/files/data/transfers",
    permission: "file:read",
    workbenchTab: "transfers",
  },
  {
    id: "search",
    plane: "data",
    label: "统一检索",
    href: "/files/data/search",
    permission: "file:read",
    placeholder: true,
  },
  {
    id: "providers",
    plane: "control",
    label: "存储 Provider",
    href: "/files/control/providers",
    permission: "storage:read",
    workbenchTab: "governance",
  },
  {
    id: "pools",
    plane: "control",
    label: "Storage Pools",
    href: "/files/control/pools",
    permission: "storage:read",
    workbenchTab: "governance",
  },
  {
    id: "bindings",
    plane: "control",
    label: "挂载管理",
    href: "/files/control/bindings",
    permission: "storage:read",
    workbenchTab: "governance",
  },
  {
    id: "placement",
    plane: "control",
    label: "放置策略",
    href: "/files/control/placement",
    permission: "storage:read",
    placeholder: true,
  },
  {
    id: "health",
    plane: "control",
    label: "副本与健康",
    href: "/files/control/health",
    permission: "storage:read",
    placeholder: true,
  },
  {
    id: "access",
    plane: "governance",
    label: "访问管理",
    href: "/files/governance/access",
    permission: "file:read",
    placeholder: true,
  },
  {
    id: "approvals",
    plane: "governance",
    label: "共享审批",
    href: "/files/governance/approvals",
    organizationPermission: "org:file:admin",
    workbenchTab: "approvals",
  },
  {
    id: "quota",
    plane: "governance",
    label: "配额管理",
    href: "/files/governance/quota",
    organizationPermission: "org:file:admin",
    placeholder: true,
  },
  {
    id: "audit",
    plane: "governance",
    label: "审计中心",
    href: "/files/governance/audit",
    workbenchTab: "audit",
  },
  {
    id: "lifecycle",
    plane: "governance",
    label: "生命周期策略",
    href: "/files/governance/lifecycle",
    permission: "storage:read",
    workbenchTab: "governance",
  },
  {
    id: "agent",
    plane: "governance",
    label: "Agent 能力授权",
    href: "/files/governance/agent",
    permission: "file:admin",
    placeholder: true,
  },
];

export function filesystemPlaneHref(plane: FilesystemPlaneId): string {
  const config = FILESYSTEM_PLANES.find((item) => item.id === plane);
  const moduleId = config?.defaultModule ?? "workspace";
  const module = FILESYSTEM_MODULES.find((item) => item.id === moduleId);
  return module?.href ?? FILESYSTEM_DEFAULT_ENTRY;
}

export function resolvePlaneFromPathname(pathname: string): FilesystemPlaneId {
  if (pathname.startsWith("/files/control")) return "control";
  if (pathname.startsWith("/files/governance")) return "governance";
  return "data";
}

export function resolveModuleFromPathname(pathname: string): FilesystemModule | undefined {
  return FILESYSTEM_MODULES.find((item) => item.href === pathname);
}

export function modulesForPlane(
  plane: FilesystemPlaneId,
  permissions: string[],
): FilesystemModule[] {
  return FILESYSTEM_MODULES.filter((item) => {
    if (item.plane !== plane) return false;
    if (item.placeholder) return true;
    if (item.organizationPermission) {
      return permissions.includes(item.organizationPermission);
    }
    if (item.permission) {
      return permissions.includes(item.permission);
    }
    if (item.workbenchTab === "audit") {
      return permissions.includes("org:file:admin") || permissions.includes("file:admin");
    }
    return true;
  });
}

export function migrateLegacyFilesPath(
  pathname: string,
  search: URLSearchParams,
): string | null {
  if (pathname !== "/files") return null;
  const tab = search.get("tab");
  const base = new URLSearchParams();
  for (const key of ["view", "space", "source", "parent", "node", "q", "cursor"]) {
    const value = search.get(key);
    if (value) base.set(key, value);
  }
  const suffix = base.toString();
  const query = suffix ? `?${suffix}` : "";
  switch (tab) {
    case "transfers":
      return `/files/data/transfers${query}`;
    case "governance":
      return `/files/control/providers${query}`;
    case "approvals":
      return `/files/governance/approvals${query}`;
    case "audit":
      return `/files/governance/audit${query}`;
    default:
      return `/files/data/workspace${query}`;
  }
}