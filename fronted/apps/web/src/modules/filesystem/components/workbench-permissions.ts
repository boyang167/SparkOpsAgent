const SUPER_ADMIN_PERMISSIONS = ["file:read", "storage:read", "file:admin"];

export function resolveWorkbenchPermissions(
  workspacePermissions: string[] | undefined,
  isSuperAdmin: boolean,
): string[] {
  return [
    ...new Set([
      ...(workspacePermissions ?? []),
      ...(isSuperAdmin ? SUPER_ADMIN_PERMISSIONS : []),
    ]),
  ];
}

export function hasFilesystemPageAccess(permissions: string[]): boolean {
  return permissions.includes("file:read");
}
