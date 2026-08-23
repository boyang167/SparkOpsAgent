import type { CreateRoleCommand, RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { bffFetch } from "./client";

export async function listRoles(workspaceId: string) {
  const data = await bffFetch<{ items: RoleVM[] }>(`/workspaces/${workspaceId}/roles`, {
    workspaceId,
  });
  return data.items;
}

export function createRole(workspaceId: string, input: CreateRoleCommand) {
  return bffFetch<{ id: string; name: string; permissions: string[] }>(
    `/workspaces/${workspaceId}/roles`,
    {
      method: "POST",
      body: JSON.stringify(input),
      workspaceId,
    },
  );
}

export function updateRole(workspaceId: string, roleId: string, input: import("@sparkops/contracts/governance").UpdateRoleCommand) {
  return bffFetch<{ id: string; name: string; permissions: string[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}`,
    {
      method: "PATCH",
      body: JSON.stringify(input),
      workspaceId,
    },
  );
}

export function deleteRole(workspaceId: string, roleId: string) {
  return bffFetch<{ deleted: boolean }>(`/workspaces/${workspaceId}/roles/${roleId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export async function listMembers(workspaceId: string) {
  const data = await bffFetch<{ items: WorkspaceMemberVM[] }>(
    `/workspaces/${workspaceId}/members`,
    { workspaceId },
  );
  return data.items;
}

export async function searchMemberCandidates(workspaceId: string, q: string, limit = 10) {
  const params = new URLSearchParams({ q, limit: String(limit) });
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").MemberCandidateVM[] }>(
    `/workspaces/${workspaceId}/member-candidates?${params.toString()}`,
    { workspaceId },
  );
  return data.items;
}

export function inviteMember(workspaceId: string, email: string) {
  return bffFetch<{ member_id: string; email: string; display_name: string }>(
    `/workspaces/${workspaceId}/members`,
    {
      method: "POST",
      body: JSON.stringify({ email }),
      workspaceId,
    },
  );
}

export function assignRole(workspaceId: string, memberId: string, roleId: string) {
  return bffFetch<{ assigned: boolean }>(
    `/workspaces/${workspaceId}/members/${memberId}/roles`,
    {
      method: "POST",
      body: JSON.stringify({ role_id: roleId }),
      workspaceId,
    },
  );
}

export function removeRoleBinding(workspaceId: string, memberId: string, roleId: string) {
  return bffFetch<{ removed: boolean }>(
    `/workspaces/${workspaceId}/members/${memberId}/roles/${roleId}`,
    {
      method: "DELETE",
      workspaceId,
    },
  );
}

export async function listRoleTree(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").RoleTreeVM[] }>(
    `/workspaces/${workspaceId}/roles/tree`,
    { workspaceId },
  );
  return data.items;
}

export function copyRole(workspaceId: string, roleId: string, name?: string) {
  return bffFetch<{ id: string; name: string; permissions: string[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}/copy`,
    { method: "POST", body: JSON.stringify({ name }), workspaceId },
  );
}

export async function listWorkspaceApis(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").RegisteredAPI[] }>(
    `/workspaces/${workspaceId}/apis`,
    { workspaceId },
  );
  return data.items;
}

export async function listPlatformApis() {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").RegisteredAPI[] }>(
    "/platform/apis",
  );
  return data.items;
}

export async function previewApiSync(workspaceId: string) {
  const diff = await bffFetch<{ new: unknown[]; delete: unknown[]; ignore?: unknown[] }>(
    `/workspaces/${workspaceId}/apis/sync`,
    { workspaceId },
  );
  return {
    new: diff.new ?? [],
    delete: diff.delete ?? [],
    ignore: diff.ignore ?? [],
  };
}

export function confirmApiSync(workspaceId: string, input: { new: unknown[]; delete: unknown[] }) {
  return bffFetch<{ synced: boolean }>(`/workspaces/${workspaceId}/apis/sync`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function updateApiMeta(
  workspaceId: string,
  apiId: string,
  input: import("@sparkops/contracts/governance").UpdateAPIMetaCommand,
) {
  return bffFetch<{ updated: boolean }>(`/workspaces/${workspaceId}/apis/${apiId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function deleteApi(workspaceId: string, apiId: string) {
  return bffFetch<{ deleted: boolean }>(`/workspaces/${workspaceId}/apis/${apiId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function deleteApisByIds(
  workspaceId: string,
  input: import("@sparkops/contracts/governance").DeleteApisCommand,
) {
  return bffFetch<{ deleted: boolean; count: number }>(
    `/workspaces/${workspaceId}/apis/batch-delete`,
    {
      method: "POST",
      body: JSON.stringify(input),
      workspaceId,
    },
  );
}

export async function listApiTokens(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").APITokenVM[] }>(
    `/workspaces/${workspaceId}/api-tokens`,
    { workspaceId },
  );
  return data.items;
}

export function createApiToken(
  workspaceId: string,
  input: import("@sparkops/contracts/governance").CreateAPITokenCommand,
) {
  return bffFetch<{ id: string; token: string; token_prefix: string; expires_at?: string }>(
    `/workspaces/${workspaceId}/api-tokens`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function revokeApiToken(workspaceId: string, tokenId: string) {
  return bffFetch<{ revoked: boolean }>(`/workspaces/${workspaceId}/api-tokens/${tokenId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export async function listRoleAPIPolicies(workspaceId: string, roleId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").RoleAPIPolicy[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}/api-policies`,
    { workspaceId },
  );
  return data.items;
}

export function updateRoleAPIPolicies(
  workspaceId: string,
  roleId: string,
  items: import("@sparkops/contracts/governance").RoleAPIPolicy[],
) {
  return bffFetch<{ updated: boolean }>(
    `/workspaces/${workspaceId}/roles/${roleId}/api-policies`,
    { method: "PUT", body: JSON.stringify({ items }), workspaceId },
  );
}

export async function listDataResources(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").DataResourceVM[] }>(
    `/workspaces/${workspaceId}/data-resources`,
    { workspaceId },
  );
  return data.items;
}

export async function listRoleDataPolicies(workspaceId: string, roleId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").RoleDataPolicyVM[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}/data-policies`,
    { workspaceId },
  );
  return data.items;
}

export function updateRoleDataPolicies(
  workspaceId: string,
  roleId: string,
  items: import("@sparkops/contracts/governance").RoleDataPolicyVM[],
) {
  return bffFetch<{ updated: boolean }>(
    `/workspaces/${workspaceId}/roles/${roleId}/data-policies`,
    { method: "PUT", body: JSON.stringify({ items }), workspaceId },
  );
}

export async function myPermissions(workspaceId: string) {
  const data = await bffFetch<{ permissions: string[] }>(
    `/workspaces/${workspaceId}/me/permissions`,
    { workspaceId },
  );
  return data.permissions;
}

export async function listMenuTree(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").MenuTreeVM[] }>(
    `/workspaces/${workspaceId}/menus/tree`,
    { workspaceId },
  );
  return data.items;
}

export async function listNavMenus(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").MenuTreeVM[] }>(
    `/workspaces/${workspaceId}/me/menus`,
    { workspaceId },
  );
  return data.items;
}

export async function myMenuButtons(workspaceId: string, path: string) {
  const query = new URLSearchParams({ path });
  return bffFetch<import("@sparkops/contracts/governance").MyMenuButtonsVM>(
    `/workspaces/${workspaceId}/me/menu-buttons?${query.toString()}`,
    { workspaceId },
  );
}

export async function getMenu(workspaceId: string, menuId: string) {
  return bffFetch<import("@sparkops/contracts/governance").MenuVM>(
    `/workspaces/${workspaceId}/menus/${menuId}`,
    { workspaceId },
  );
}

export function createMenu(
  workspaceId: string,
  input: import("@sparkops/contracts/governance").UpsertMenuCommand,
) {
  return bffFetch<import("@sparkops/contracts/governance").MenuVM>(
    `/workspaces/${workspaceId}/menus`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateMenu(
  workspaceId: string,
  menuId: string,
  input: import("@sparkops/contracts/governance").UpsertMenuCommand,
) {
  return bffFetch<{ updated: true }>(`/workspaces/${workspaceId}/menus/${menuId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function deleteMenu(workspaceId: string, menuId: string) {
  return bffFetch<{ deleted: true }>(`/workspaces/${workspaceId}/menus/${menuId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export async function getMenuRoles(workspaceId: string, menuId: string) {
  const data = await bffFetch<{ role_ids: string[] }>(
    `/workspaces/${workspaceId}/menus/${menuId}/roles`,
    { workspaceId },
  );
  return data.role_ids;
}

export function setMenuRoles(
  workspaceId: string,
  menuId: string,
  input: import("@sparkops/contracts/governance").SetMenuRolesCommand,
) {
  return bffFetch<{ updated: true }>(`/workspaces/${workspaceId}/menus/${menuId}/roles`, {
    method: "PUT",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export async function getRoleMenus(workspaceId: string, roleId: string) {
  const data = await bffFetch<{ menu_ids: string[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}/menus`,
    { workspaceId },
  );
  return data.menu_ids;
}

export function setRoleMenus(
  workspaceId: string,
  roleId: string,
  input: import("@sparkops/contracts/governance").SetRoleMenusCommand,
) {
  return bffFetch<{ updated: true }>(`/workspaces/${workspaceId}/roles/${roleId}/menus`, {
    method: "PUT",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export async function listWorkspaceMenuButtons(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").WorkspaceMenuButtonVM[] }>(
    `/workspaces/${workspaceId}/menu-buttons`,
    { workspaceId },
  );
  return data.items;
}

export async function getRoleMenuButtons(workspaceId: string, roleId: string) {
  const data = await bffFetch<{ button_ids: string[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}/menu-buttons`,
    { workspaceId },
  );
  return data.button_ids;
}

export function setRoleMenuButtons(
  workspaceId: string,
  roleId: string,
  input: import("@sparkops/contracts/governance").SetRoleMenuButtonsCommand,
) {
  return bffFetch<{ updated: true }>(`/workspaces/${workspaceId}/roles/${roleId}/menu-buttons`, {
    method: "PUT",
    body: JSON.stringify(input),
    workspaceId,
  });
}
