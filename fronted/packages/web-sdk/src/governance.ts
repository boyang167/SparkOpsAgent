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

export function updateRole(workspaceId: string, roleId: string, permissions: string[]) {
  return bffFetch<{ id: string; name: string; permissions: string[] }>(
    `/workspaces/${workspaceId}/roles/${roleId}`,
    {
      method: "PATCH",
      body: JSON.stringify({ permissions }),
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

export async function previewApiSync() {
  return bffFetch<{ new: unknown[]; delete: unknown[]; ignore?: unknown[] }>("/platform/apis/sync");
}

export function confirmApiSync(input: { new: unknown[]; delete: unknown[] }) {
  return bffFetch<{ synced: boolean }>("/platform/apis/sync", {
    method: "POST",
    body: JSON.stringify(input),
  });
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

export async function myPermissions(workspaceId: string) {
  const data = await bffFetch<{ permissions: string[] }>(
    `/workspaces/${workspaceId}/me/permissions`,
    { workspaceId },
  );
  return data.permissions;
}
