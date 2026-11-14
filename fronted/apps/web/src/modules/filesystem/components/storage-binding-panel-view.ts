import type { StorageBindingScopeType, StorageBindingVM } from "@sparkops/contracts/filesystem";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
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
};

export type ScopeTargetOption = {
  id: string;
  label: string;
  detail?: string;
};

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
      return "";
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
      return [];
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
      return "请填写用户 ID";
    }
    return "请选择绑定目标";
  }
  return null;
}