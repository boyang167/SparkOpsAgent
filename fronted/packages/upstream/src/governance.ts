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
    async previewApiSync() {
      return upstreamFetch<import("@sparkops/contracts/governance").APISyncDiff>(
        "/api/v1/platform/apis/sync",
        { method: "GET", options },
      );
    },
    async confirmApiSync(input: { new: unknown[]; delete: unknown[] }) {
      return upstreamFetch<{ synced: boolean }>("/api/v1/platform/apis/sync", {
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
  };
}

export type GovernanceUpstream = ReturnType<typeof createGovernanceUpstream>;
