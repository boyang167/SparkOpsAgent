import type {
  AuditLogListVM,
  CreateOrganizationCommand,
  CreatePlatformOrganizationCommand,
  CreateWorkspaceCommand,
  InviteOrgMemberCommand,
  OrganizationListVM,
  OrganizationVM,
  OrgMemberListVM,
  OrgPermissionsVM,
  PlatformOrganizationVM,
  TransferOrgOwnerCommand,
  UpdateOrgMemberRoleCommand,
  UpdateOrgStatusCommand,
  WorkspaceCreatedVM,
  WorkspaceListVM,
  WorkspaceVM,
} from "@sparkops/contracts/tenant";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createTenantUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listOrganizations() {
      return upstreamFetch<OrganizationListVM>("/api/v1/orgs", {
        method: "GET",
        options,
      });
    },
    async listWorkspaces() {
      return upstreamFetch<WorkspaceListVM>("/api/v1/workspaces", {
        method: "GET",
        options,
      });
    },
    async createOrganization(input: CreateOrganizationCommand) {
      return upstreamFetch<OrganizationVM>("/api/v1/orgs", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async createWorkspace(orgId: string, input: CreateWorkspaceCommand) {
      return upstreamFetch<WorkspaceCreatedVM>(`/api/v1/orgs/${orgId}/workspaces`, {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async getOrganization(orgId: string) {
      return upstreamFetch<OrganizationVM & { my_role?: string }>(`/api/v1/orgs/${orgId}`, {
        method: "GET",
        options,
      });
    },
    async listOrgMembers(orgId: string) {
      return upstreamFetch<OrgMemberListVM>(`/api/v1/orgs/${orgId}/members`, {
        method: "GET",
        options,
      });
    },
    async searchOrgMemberCandidates(orgId: string, q: string, limit = 10) {
      const params = new URLSearchParams({ q, limit: String(limit) });
      return upstreamFetch<{ items: import("@sparkops/contracts/governance").MemberCandidateVM[] }>(
        `/api/v1/orgs/${orgId}/member-candidates?${params.toString()}`,
        { method: "GET", options },
      );
    },
    async inviteOrgMember(orgId: string, input: InviteOrgMemberCommand) {
      return upstreamFetch<{ member_id: string; email: string; display_name: string; role: string }>(
        `/api/v1/orgs/${orgId}/members`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options,
        },
      );
    },
    async updateOrgMemberRole(orgId: string, memberId: string, input: UpdateOrgMemberRoleCommand) {
      return upstreamFetch<{ updated: true }>(`/api/v1/orgs/${orgId}/members/${memberId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async removeOrgMember(orgId: string, memberId: string) {
      return upstreamFetch<{ removed: true }>(`/api/v1/orgs/${orgId}/members/${memberId}`, {
        method: "DELETE",
        options,
      });
    },
    async getMyOrgPermissions(orgId: string) {
      return upstreamFetch<OrgPermissionsVM>(`/api/v1/orgs/${orgId}/me/permissions`, {
        method: "GET",
        options,
      });
    },
    async listPlatformOrganizations() {
      return upstreamFetch<{ items: PlatformOrganizationVM[] }>("/api/v1/platform/orgs", {
        method: "GET",
        options,
      });
    },
    async createPlatformOrganization(input: CreatePlatformOrganizationCommand) {
      return upstreamFetch<PlatformOrganizationVM>("/api/v1/platform/orgs", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },
    async updatePlatformOrganizationStatus(orgId: string, input: UpdateOrgStatusCommand) {
      return upstreamFetch<{ updated: true }>(`/api/v1/platform/orgs/${orgId}/status`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deletePlatformOrganization(orgId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/platform/orgs/${orgId}`, {
        method: "DELETE",
        options,
      });
    },
    async transferPlatformOrgOwner(orgId: string, input: TransferOrgOwnerCommand) {
      return upstreamFetch<{ previous_owner_id: string; new_owner_id: string; new_owner_email: string }>(
        `/api/v1/platform/orgs/${orgId}/owners/transfer`,
        { method: "POST", body: JSON.stringify(input), options },
      );
    },
    async transferOrgOwner(orgId: string, input: TransferOrgOwnerCommand) {
      return upstreamFetch<{ previous_owner_id: string; new_owner_id: string; new_owner_email: string }>(
        `/api/v1/orgs/${orgId}/owners/transfer`,
        { method: "POST", body: JSON.stringify(input), options },
      );
    },
    async listOrgAuditLogs(orgId: string, params?: { limit?: number; offset?: number }) {
      const qs = new URLSearchParams();
      if (params?.limit) qs.set("limit", String(params.limit));
      if (params?.offset) qs.set("offset", String(params.offset));
      const suffix = qs.toString() ? `?${qs}` : "";
      return upstreamFetch<AuditLogListVM>(`/api/v1/orgs/${orgId}/audit-logs${suffix}`, {
        method: "GET",
        options,
      });
    },
    async listPlatformAuditLogs(params?: { limit?: number; offset?: number }) {
      const qs = new URLSearchParams();
      if (params?.limit) qs.set("limit", String(params.limit));
      if (params?.offset) qs.set("offset", String(params.offset));
      const suffix = qs.toString() ? `?${qs}` : "";
      return upstreamFetch<AuditLogListVM>(`/api/v1/platform/audit-logs${suffix}`, {
        method: "GET",
        options,
      });
    },
    async getOrgQuota(orgId: string) {
      return upstreamFetch<import("@sparkops/contracts/tenant").OrgQuotaVM>(
        `/api/v1/orgs/${orgId}/quota`,
        { method: "GET", options },
      );
    },
    async getOrgQuotaUsage(orgId: string) {
      return upstreamFetch<import("@sparkops/contracts/tenant").OrgQuotaUsageVM>(
        `/api/v1/orgs/${orgId}/quota/usage`,
        { method: "GET", options },
      );
    },
    async updateOrgQuota(orgId: string, input: import("@sparkops/contracts/tenant").UpdateOrgQuotaCommand) {
      return upstreamFetch<import("@sparkops/contracts/tenant").OrgQuotaVM>(
        `/api/v1/orgs/${orgId}/quota`,
        { method: "PATCH", body: JSON.stringify(input), options },
      );
    },
  };
}

export type TenantUpstream = ReturnType<typeof createTenantUpstream>;
