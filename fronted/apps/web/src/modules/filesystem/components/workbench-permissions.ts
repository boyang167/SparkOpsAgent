const SUPER_ADMIN_PERMISSIONS = ["file:read", "storage:read", "file:admin"];

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
