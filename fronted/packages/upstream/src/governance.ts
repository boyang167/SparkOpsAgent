import type {
  AssignRoleCommand,
  CreateRoleCommand,
  InviteMemberCommand,
  InvitedMemberVM,
  MyPermissionsVM,
  UpdateRoleCommand,
  RoleCreatedVM,
  RoleListVM,
  WorkspaceMemberListVM,
} from "@sparkops/contracts/governance";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createGovernanceUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listRoleTree(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").RoleTreeVM[] }>(
        `/api/v1/workspaces/${wsId}/roles/tree`,
        { method: "GET", options },
      );
    },
    async copyRole(wsId: string, roleId: string, name?: string) {
      return upstreamFetch<{ id: string; name: string; permissions: string[] }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/copy`,
        { method: "POST", body: JSON.stringify({ name }), options },
      );
    },
    async listRoleAPIPolicies(wsId: string, roleId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").RoleAPIPolicy[] }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/api-policies`,
        { method: "GET", options },
      );
    },
    async updateRoleAPIPolicies(wsId: string, roleId: string, items: import("@sparkops/contracts/governance").RoleAPIPolicy[]) {
      return upstreamFetch<{ updated: boolean }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/api-policies`,
        { method: "PUT", body: JSON.stringify({ items }), options },
      );
    },
    async listDataResources(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").DataResourceVM[] }>(
        `/api/v1/workspaces/${wsId}/data-resources`,
        { method: "GET", options },
      );
    },
    async listRoleDataPolicies(wsId: string, roleId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").RoleDataPolicyVM[] }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/data-policies`,
        { method: "GET", options },
      );
    },
    async updateRoleDataPolicies(
      wsId: string,
      roleId: string,
      input: import("@sparkops/contracts/governance").UpdateRoleDataPoliciesCommand,
    ) {
      return upstreamFetch<{ updated: boolean }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/data-policies`,
        { method: "PUT", body: JSON.stringify(input), options },
      );
    },
    async listWorkspaceApis(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").RegisteredAPI[] }>(
        `/api/v1/workspaces/${wsId}/apis`,
        { method: "GET", options },
      );
    },
    async listPlatformApis() {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").RegisteredAPI[] }>(
        "/api/v1/platform/apis",
        { method: "GET", options },
      );
    },
    async previewApiSync(wsId: string) {
      return upstreamFetch<import("@sparkops/contracts/governance").APISyncDiff>(
        `/api/v1/workspaces/${wsId}/apis/sync`,
        { method: "GET", options },
      );
    },
    async confirmApiSync(wsId: string, input: { new: unknown[]; delete: unknown[] }) {
      return upstreamFetch<{ synced: boolean }>(`/api/v1/workspaces/${wsId}/apis/sync`, {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async updateApiMeta(
      wsId: string,
      apiId: string,
      input: import("@sparkops/contracts/governance").UpdateAPIMetaCommand,
    ) {
      return upstreamFetch<{ updated: boolean }>(`/api/v1/workspaces/${wsId}/apis/${apiId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deleteApi(wsId: string, apiId: string) {
      return upstreamFetch<{ deleted: boolean }>(`/api/v1/workspaces/${wsId}/apis/${apiId}`, {
        method: "DELETE",
        options,
      });
    },
    async deleteApisByIds(wsId: string, input: import("@sparkops/contracts/governance").DeleteApisCommand) {
      return upstreamFetch<{ deleted: boolean; count: number }>(
        `/api/v1/workspaces/${wsId}/apis/batch-delete`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options,
        },
      );
    },
    async previewPlatformApiSync() {
      return upstreamFetch<import("@sparkops/contracts/governance").APISyncDiff>(
        "/api/v1/platform/apis/sync",
        { method: "GET", options },
      );
    },
    async confirmPlatformApiSync(input: { new: unknown[]; delete: unknown[] }) {
      return upstreamFetch<{ synced: boolean }>("/api/v1/platform/apis/sync", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async updatePlatformApiMeta(
      apiId: string,
      input: import("@sparkops/contracts/governance").UpdateAPIMetaCommand,
    ) {
      return upstreamFetch<{ updated: boolean }>(`/api/v1/platform/apis/${apiId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deletePlatformApi(apiId: string) {
      return upstreamFetch<{ deleted: boolean }>(`/api/v1/platform/apis/${apiId}`, {
        method: "DELETE",
        options,
      });
    },
    async deletePlatformApisByIds(input: import("@sparkops/contracts/governance").DeleteApisCommand) {
      return upstreamFetch<{ deleted: boolean; count: number }>("/api/v1/platform/apis/batch-delete", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async listApiTokens(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").APITokenVM[] }>(
        `/api/v1/workspaces/${wsId}/api-tokens`,
        { method: "GET", options },
      );
    },
    async createApiToken(wsId: string, input: import("@sparkops/contracts/governance").CreateAPITokenCommand) {
      return upstreamFetch<{ id: string; token: string; token_prefix: string; expires_at?: string }>(
        `/api/v1/workspaces/${wsId}/api-tokens`,
        { method: "POST", body: JSON.stringify(input), options },
      );
    },
    async revokeApiToken(wsId: string, tokenId: string) {
      return upstreamFetch<{ revoked: boolean }>(`/api/v1/workspaces/${wsId}/api-tokens/${tokenId}`, {
        method: "DELETE",
        options,
      });
    },
    async listRoles(wsId: string) {
      return upstreamFetch<RoleListVM>(`/api/v1/workspaces/${wsId}/roles`, {
        method: "GET",
        options,
      });
    },
    async createRole(wsId: string, input: CreateRoleCommand) {
      return upstreamFetch<RoleCreatedVM>(`/api/v1/workspaces/${wsId}/roles`, {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async updateRole(wsId: string, roleId: string, input: UpdateRoleCommand) {
      return upstreamFetch<RoleCreatedVM>(`/api/v1/workspaces/${wsId}/roles/${roleId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deleteRole(wsId: string, roleId: string) {
      return upstreamFetch<{ deleted: boolean }>(`/api/v1/workspaces/${wsId}/roles/${roleId}`, {
        method: "DELETE",
        options,
      });
    },
    async listMembers(wsId: string) {
      return upstreamFetch<WorkspaceMemberListVM>(`/api/v1/workspaces/${wsId}/members`, {
        method: "GET",
        options,
      });
    },
    async inviteMember(wsId: string, input: InviteMemberCommand) {
      return upstreamFetch<InvitedMemberVM>(`/api/v1/workspaces/${wsId}/members`, {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async assignRole(wsId: string, memberId: string, input: AssignRoleCommand) {
      return upstreamFetch<{ assigned: boolean }>(
        `/api/v1/workspaces/${wsId}/members/${memberId}/roles`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options,
        },
      );
    },
    async removeRoleBinding(wsId: string, memberId: string, roleId: string) {
      return upstreamFetch<{ removed: boolean }>(
        `/api/v1/workspaces/${wsId}/members/${memberId}/roles/${roleId}`,
        {
          method: "DELETE",
          options,
        },
      );
    },
    async myPermissions(wsId: string) {
      return upstreamFetch<MyPermissionsVM>(`/api/v1/workspaces/${wsId}/me/permissions`, {
        method: "GET",
        options,
      });
    },
    async listMenuTree(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").MenuTreeVM[] }>(
        `/api/v1/workspaces/${wsId}/menus/tree`,
        { method: "GET", options },
      );
    },
    async listNavMenus(wsId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").MenuTreeVM[] }>(
        `/api/v1/workspaces/${wsId}/me/menus`,
        { method: "GET", options },
      );
    },
    async listMyMenuButtons(wsId: string, path: string) {
      const query = new URLSearchParams({ path });
      return upstreamFetch<import("@sparkops/contracts/governance").MyMenuButtonsVM>(
        `/api/v1/workspaces/${wsId}/me/menu-buttons?${query.toString()}`,
        { method: "GET", options },
      );
    },
    async getMenu(wsId: string, menuId: string) {
      return upstreamFetch<import("@sparkops/contracts/governance").MenuVM>(
        `/api/v1/workspaces/${wsId}/menus/${menuId}`,
        { method: "GET", options },
      );
    },
    async createMenu(wsId: string, input: import("@sparkops/contracts/governance").UpsertMenuCommand) {
      return upstreamFetch<import("@sparkops/contracts/governance").MenuVM>(
        `/api/v1/workspaces/${wsId}/menus`,
        { method: "POST", body: JSON.stringify(input), options },
      );
    },
    async updateMenu(
      wsId: string,
      menuId: string,
      input: import("@sparkops/contracts/governance").UpsertMenuCommand,
    ) {
      return upstreamFetch<{ updated: true }>(`/api/v1/workspaces/${wsId}/menus/${menuId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deleteMenu(wsId: string, menuId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/workspaces/${wsId}/menus/${menuId}`, {
        method: "DELETE",
        options,
      });
    },
    async getMenuRoles(wsId: string, menuId: string) {
      return upstreamFetch<{ role_ids: string[] }>(
        `/api/v1/workspaces/${wsId}/menus/${menuId}/roles`,
        { method: "GET", options },
      );
    },
    async setMenuRoles(
      wsId: string,
      menuId: string,
      input: import("@sparkops/contracts/governance").SetMenuRolesCommand,
    ) {
      return upstreamFetch<{ updated: true }>(
        `/api/v1/workspaces/${wsId}/menus/${menuId}/roles`,
        { method: "PUT", body: JSON.stringify(input), options },
      );
    },
    async getRoleMenus(wsId: string, roleId: string) {
      return upstreamFetch<{ menu_ids: string[] }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/menus`,
        { method: "GET", options },
      );
    },
    async setRoleMenus(
      wsId: string,
      roleId: string,
      input: import("@sparkops/contracts/governance").SetRoleMenusCommand,
    ) {
      return upstreamFetch<{ updated: true }>(
        `/api/v1/workspaces/${wsId}/roles/${roleId}/menus`,
        { method: "PUT", body: JSON.stringify(input), options },
      );
    },
  };
}

export type GovernanceUpstream = ReturnType<typeof createGovernanceUpstream>;
