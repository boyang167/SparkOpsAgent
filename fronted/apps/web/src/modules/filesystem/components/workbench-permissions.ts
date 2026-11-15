import type { FileNamespaceVM, NamespaceSummaryVM } from "@sparkops/contracts/filesystem";
import type { FileSpace } from "./workbench-state";

const SUPER_ADMIN_PERMISSIONS = ["file:read", "storage:read", "file:admin"];

/** 当前四空间浏览页是否允许新建/上传（依赖 RBAC + Namespace 根授权） */
export function canWriteGovernedFileSpace(input: {
  permissions: string[];
  fileSpace?: FileSpace;
  namespaceSummary?: NamespaceSummaryVM;
  namespace: Pick<FileNamespaceVM, "id">;
  workspaceId: string;
}): boolean {
  if (!input.permissions.includes("file:write")) return false;
  const { fileSpace, namespaceSummary, namespace, workspaceId } = input;
  if (!fileSpace || !namespaceSummary) return true;
  if (namespaceSummary.id !== namespace.id) return false;
  if (!namespaceSummary.capabilities.includes("WRITE")) return false;

  switch (fileSpace) {
    case "workspace":
      return (
        namespaceSummary.namespace_type === "WORKSPACE" &&
        namespaceSummary.workspace_id === workspaceId
      );
    case "my":
      return namespaceSummary.namespace_type === "USER";
    case "organization":
      return namespaceSummary.namespace_type === "ORGANIZATION";
    default:
      return false;
  }
}

export function resolveWorkbenchPermissions(
  workspacePermissions: string[] | undefined,
  isSuperAdmin: boolean,
  organizationPermissions: string[] | undefined = [],
): string[] {
  return [
    ...new Set([
      ...(workspacePermissions ?? []),
      ...(organizationPermissions ?? []),
      ...(isSuperAdmin ? SUPER_ADMIN_PERMISSIONS : []),
    ]),
  ];
}

export function hasFilesystemPageAccess(permissions: string[]): boolean {
  return permissions.includes("file:read");
}

export type FilesystemAuditMode = "organization" | "workspace";

export function resolveFilesystemAuditMode(
  permissions: string[],
): FilesystemAuditMode | undefined {
  if (permissions.includes("org:file:admin")) return "organization";
  if (permissions.includes("file:admin")) return "workspace";
  return undefined;
}
