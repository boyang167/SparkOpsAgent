const BASE_PERMISSIONS = ["file:read"];
const SUPER_ADMIN_PERMISSIONS = ["storage:read", "file:admin"];

export function resolveWorkbenchPermissions(
  workspacePermissions: string[] | undefined,
  isSuperAdmin: boolean,
): string[] {
  return [
    ...new Set([
      ...BASE_PERMISSIONS,
      ...(workspacePermissions ?? []),
      ...(isSuperAdmin ? SUPER_ADMIN_PERMISSIONS : []),
    ]),
  ];
}
