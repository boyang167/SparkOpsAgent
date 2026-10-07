import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { bffFetch } from "./client";

export async function listOrganizations() {
  const data = await bffFetch<{ items: OrganizationVM[] }>("/orgs");
  return data.items;
}

export async function listWorkspaces() {
  const data = await bffFetch<{ items: WorkspaceVM[] }>("/workspaces");
  return data.items;
}

export function selectWorkspace(workspaceId: string) {
  return bffFetch<{ workspaceId: string }>("/workspaces/select", {
    method: "POST",
    body: JSON.stringify({ workspaceId }),
  });
}

export function createOrganization(input: { name: string; slug: string }) {
  return bffFetch<{ id: string; name: string; slug: string }>("/orgs", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function createWorkspace(orgId: string, input: { name: string; slug: string }) {
  return bffFetch<{ id: string; name: string; slug: string }>(`/orgs/${orgId}/workspaces`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getOrganization(orgId: string) {
  return bffFetch<import("@sparkops/contracts/tenant").OrganizationVM & { my_role?: string }>(
    `/orgs/${orgId}`,
  );
}

export function listOrgMembers(orgId: string) {
  return bffFetch<{ items: import("@sparkops/contracts/tenant").OrgMemberVM[] }>(
    `/orgs/${orgId}/members`,
  ).then((data) => data.items);
}

export async function searchOrgMemberCandidates(orgId: string, q: string, limit = 10) {
  const params = new URLSearchParams({ q, limit: String(limit) });
  const data = await bffFetch<{ items: import("@sparkops/contracts/governance").MemberCandidateVM[] }>(
    `/orgs/${orgId}/member-candidates?${params.toString()}`,
  );
  return data.items;
}

export function inviteOrgMember(orgId: string, input: { email: string; role?: "admin" | "member" }) {
  return bffFetch<{ member_id: string; email: string; display_name: string; role: string }>(
    `/orgs/${orgId}/members`,
    {
      method: "POST",
      body: JSON.stringify(input),
    },
  );
}

export function updateOrgMemberRole(orgId: string, memberId: string, role: "admin" | "member") {
  return bffFetch<{ updated: true }>(`/orgs/${orgId}/members/${memberId}`, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

export function removeOrgMember(orgId: string, memberId: string) {
  return bffFetch<{ removed: true }>(`/orgs/${orgId}/members/${memberId}`, {
    method: "DELETE",
  });
}

export function getMyOrgPermissions(orgId: string) {
  return bffFetch<{ permissions: string[] }>(`/orgs/${orgId}/me/permissions`).then(
    (data) => data.permissions,
  );
}

export function listPlatformOrganizations() {
  return bffFetch<{ items: import("@sparkops/contracts/tenant").PlatformOrganizationVM[] }>(
    "/platform/orgs",
  ).then((d) => d.items);
}

export function createPlatformOrganization(input: {
  name: string;
  slug: string;
  owner_email?: string;
}) {
  return bffFetch<import("@sparkops/contracts/tenant").PlatformOrganizationVM>("/platform/orgs", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updatePlatformOrganizationStatus(orgId: string, status: "active" | "suspended") {
  return bffFetch<{ updated: true }>(`/platform/orgs/${orgId}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export function deletePlatformOrganization(orgId: string) {
  return bffFetch<{ deleted: true }>(`/platform/orgs/${orgId}`, {
    method: "DELETE",
  });
}

export function transferPlatformOrgOwner(orgId: string, ownerEmail: string) {
  return bffFetch<{ new_owner_email: string }>(`/platform/orgs/${orgId}/owners/transfer`, {
    method: "POST",
    body: JSON.stringify({ owner_email: ownerEmail }),
  });
}

export function transferOrgOwner(orgId: string, ownerEmail: string) {
  return bffFetch<{ new_owner_email: string }>(`/orgs/${orgId}/owners/transfer`, {
    method: "POST",
    body: JSON.stringify({ owner_email: ownerEmail }),
  });
}

export function listOrgAuditLogs(orgId: string) {
  return bffFetch<{ items: import("@sparkops/contracts/tenant").AuditLogVM[] }>(
    `/orgs/${orgId}/audit-logs`,
  ).then((d) => d.items);
}

export function listPlatformAuditLogs() {
  return bffFetch<{ items: import("@sparkops/contracts/tenant").AuditLogVM[] }>(
    "/platform/audit-logs",
  ).then((d) => d.items);
}

export function getOrgQuota(orgId: string) {
  return bffFetch<import("@sparkops/contracts/tenant").OrgQuotaVM>(`/orgs/${orgId}/quota`);
}

export function getOrgQuotaUsage(orgId: string) {
  return bffFetch<import("@sparkops/contracts/tenant").OrgQuotaUsageVM>(`/orgs/${orgId}/quota/usage`);
}

export function updateOrgQuota(
  orgId: string,
  input: import("@sparkops/contracts/tenant").UpdateOrgQuotaCommand,
) {
  return bffFetch<import("@sparkops/contracts/tenant").OrgQuotaVM>(`/orgs/${orgId}/quota`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}
