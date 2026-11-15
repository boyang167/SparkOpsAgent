import type { StorageBindingScopeType, StorageBindingVM } from "@sparkops/contracts/filesystem";
import type { OrganizationVM, OrgMemberVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import type { ProjectVM } from "@sparkops/contracts/workspace";

export const STORAGE_BINDING_SCOPE_TYPES: StorageBindingScopeType[] = [
  "ORGANIZATION",
  "WORKSPACE",
  "PROJECT",
  "USER",
];

export const STORAGE_BINDING_SCOPE_LABELS: Record<StorageBindingScopeType, string> = {
  ORGANIZATION: "租户（Organization）",
  WORKSPACE: "工作区",
  PROJECT: "项目",
  USER: "用户",
};

export type ScopeTargetCatalog = {
  organizations: Array<Pick<OrganizationVM, "id" | "name" | "slug">>;
  workspaces: Array<Pick<WorkspaceVM, "id" | "org_id" | "name" | "slug">>;
  projects: Array<Pick<ProjectVM, "id" | "name" | "display_name">>;
  orgMembers: Array<Pick<OrgMemberVM, "member_id" | "email" | "display_name">>;
};

export type ScopeTargetOption = {
  id: string;
  label: string;
  detail?: string;
};

function catalogOrgMembers(catalog: ScopeTargetCatalog) {
  return catalog.orgMembers ?? [];
}

export function workspaceOrganizationId(
  workspaces: Array<Pick<WorkspaceVM, "id" | "org_id">>,
  workspaceId: string,
) {
  return workspaces.find((workspace) => workspace.id === workspaceId)?.org_id ?? "";
}

export function defaultScopeIdForType(
  scopeType: StorageBindingScopeType,
  workspaceId: string,
  catalog: ScopeTargetCatalog,
): string {
  switch (scopeType) {
    case "WORKSPACE":
      return workspaceId;
    case "ORGANIZATION":
      return workspaceOrganizationId(catalog.workspaces, workspaceId);
    case "PROJECT":
      return catalog.projects[0]?.id ?? "";
    case "USER":
      return catalogOrgMembers(catalog)[0]?.member_id ?? "";
    default:
      return "";
  }
}

export function scopeTargetOptions(
  scopeType: StorageBindingScopeType,
  catalog: ScopeTargetCatalog,
): ScopeTargetOption[] {
  switch (scopeType) {
    case "ORGANIZATION":
      return catalog.organizations.map((org) => ({
        id: org.id,
        label: org.name,
        detail: org.slug,
      }));
    case "WORKSPACE":
      return catalog.workspaces.map((workspace) => ({
        id: workspace.id,
        label: workspace.name,
        detail: workspace.slug,
      }));
    case "PROJECT":
      return catalog.projects.map((project) => ({
        id: project.id,
        label: project.display_name?.trim() || project.name,
        detail: project.name,
      }));
    case "USER":
      return catalogOrgMembers(catalog).map((member) => ({
        id: member.member_id,
        label: member.display_name?.trim() || member.email,
        detail: member.email,
      }));
    default:
      return [];
  }
}

export function formatScopeTargetLabel(
  scopeType: StorageBindingScopeType,
  scopeId: string,
  catalog: ScopeTargetCatalog,
): string {
  const trimmed = scopeId.trim();
  if (!trimmed) {
    return "—";
  }
  const options = scopeTargetOptions(scopeType, catalog);
  const match = options.find((option) => option.id === trimmed);
  if (match) {
    return match.detail ? `${match.label}（${match.detail}）` : match.label;
  }
  const fallback = trimmed.slice(0, 8);
  switch (scopeType) {
    case "ORGANIZATION":
      return `租户 · ${fallback}`;
    case "WORKSPACE":
      return `工作区 · ${fallback}`;
    case "PROJECT":
      return `项目 · ${fallback}`;
    case "USER":
      return `用户 · ${fallback}`;
    default:
      return fallback;
  }
}

export function formatBindingScopeRow(binding: StorageBindingVM, catalog: ScopeTargetCatalog) {
  return {
    scopeLabel: STORAGE_BINDING_SCOPE_LABELS[binding.scope_type],
    targetLabel: formatScopeTargetLabel(binding.scope_type, binding.scope_id, catalog),
  };
}

export type BindingMountStatusTone = "mounted" | "degraded";

export type BindingDisplayRow = {
  mountPoint: string;
  namespace: string;
  poolName: string;
  priority: number;
  mountType: string;
  accessMode: string;
  status: { label: string; tone: BindingMountStatusTone };
};

function organizationSlug(
  organizationId: string,
  catalog: ScopeTargetCatalog,
): string {
  const org = catalog.organizations.find((item) => item.id === organizationId);
  return org?.slug?.trim() || organizationId.slice(0, 8);
}

export function formatBindingMountPoint(
  binding: StorageBindingVM,
  catalog: ScopeTargetCatalog,
): string {
  switch (binding.scope_type) {
    case "WORKSPACE": {
      const workspace = catalog.workspaces.find((item) => item.id === binding.scope_id);
      const slug = workspace?.slug?.trim() || binding.scope_id.slice(0, 8);
      return `/workspace/${slug}`;
    }
    case "ORGANIZATION": {
      const org = catalog.organizations.find((item) => item.id === binding.scope_id);
      const slug = org?.slug?.trim() || binding.scope_id.slice(0, 8);
      return `/org/${slug}`;
    }
    case "PROJECT": {
      const project = catalog.projects.find((item) => item.id === binding.scope_id);
      const segment = project?.name?.trim() || binding.scope_id.slice(0, 8);
      return `/project/${segment}`;
    }
    case "USER":
      return `/user/${binding.scope_id.slice(0, 8)}`;
    default:
      return "—";
  }
}

export function formatBindingNamespace(
  binding: StorageBindingVM,
  catalog: ScopeTargetCatalog,
): string {
  const orgPart = organizationSlug(binding.organization_id, catalog);
  switch (binding.scope_type) {
    case "ORGANIZATION":
      return orgPart;
    case "WORKSPACE": {
      const workspace = catalog.workspaces.find((item) => item.id === binding.scope_id);
      const slug = workspace?.slug?.trim() || "workspace";
      return `${orgPart}/${slug}`;
    }
    case "PROJECT": {
      const project = catalog.projects.find((item) => item.id === binding.scope_id);
      const segment = project?.name?.trim() || "project";
      return `${orgPart}/${segment}`;
    }
    case "USER":
      return `${orgPart}/user`;
    default:
      return orgPart;
  }
}

export function bindingMountType(scopeType: StorageBindingVM["scope_type"]): string {
  switch (scopeType) {
    case "ORGANIZATION":
      return "Gateway Mount";
    case "USER":
      return "Direct Mount";
    default:
      return "Virtual Mount";
  }
}

export function bindingAccessMode(): string {
  return "Read / Write";
}

export function bindingMountStatus(poolStatus?: string): { label: string; tone: BindingMountStatusTone } {
  const normalized = poolStatus?.trim().toUpperCase();
  if (normalized === "DISABLED") {
    return { label: "Degraded", tone: "degraded" };
  }
  return { label: "Mounted", tone: "mounted" };
}

export function buildBindingDisplayRow(
  binding: StorageBindingVM,
  catalog: ScopeTargetCatalog,
  poolName: string,
  poolStatus?: string,
): BindingDisplayRow {
  return {
    mountPoint: formatBindingMountPoint(binding, catalog),
    namespace: formatBindingNamespace(binding, catalog),
    poolName,
    priority: binding.priority,
    mountType: bindingMountType(binding.scope_type),
    accessMode: bindingAccessMode(),
    status: bindingMountStatus(poolStatus),
  };
}

export function validateStorageBindingForm(input: {
  pool_id: string;
  scope_id: string;
  scope_type: StorageBindingScopeType;
}): string | null {
  if (!input.pool_id.trim()) {
    return "请选择存储池";
  }
  if (!input.scope_id.trim()) {
    if (input.scope_type === "USER") {
      return "请选择用户";
    }
    return "请选择绑定目标";
  }
  return null;
}