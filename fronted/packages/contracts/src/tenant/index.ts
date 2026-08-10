import { z } from "zod";

export const WorkspaceVMSchema = z.object({
  id: z.string(),
  org_id: z.string(),
  name: z.string(),
  slug: z.string(),
  max_agents: z.number(),
  max_projects: z.number(),
});

export type WorkspaceVM = z.infer<typeof WorkspaceVMSchema>;

export const WorkspaceListVMSchema = z.object({
  items: z.array(WorkspaceVMSchema),
});

export type WorkspaceListVM = z.infer<typeof WorkspaceListVMSchema>;

export const SelectWorkspaceCommandSchema = z.object({
  workspaceId: z.string().uuid(),
});

export type SelectWorkspaceCommand = z.infer<typeof SelectWorkspaceCommandSchema>;

export const CreateOrganizationCommandSchema = z.object({
  name: z.string().min(1),
  slug: z.string().min(1),
});

export type CreateOrganizationCommand = z.infer<typeof CreateOrganizationCommandSchema>;

export const OrganizationVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  my_role: z.string().optional(),
});

export type OrganizationVM = z.infer<typeof OrganizationVMSchema>;

export const OrganizationListVMSchema = z.object({
  items: z.array(OrganizationVMSchema),
});

export type OrganizationListVM = z.infer<typeof OrganizationListVMSchema>;

export const WorkspaceCreatedVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
});

export type WorkspaceCreatedVM = z.infer<typeof WorkspaceCreatedVMSchema>;

export const CreateWorkspaceCommandSchema = z.object({
  name: z.string().min(1),
  slug: z.string().min(1),
});

export type CreateWorkspaceCommand = z.infer<typeof CreateWorkspaceCommandSchema>;

export const OrgMemberVMSchema = z.object({
  member_id: z.string(),
  email: z.string().email(),
  display_name: z.string(),
  role: z.enum(["owner", "admin", "member"]),
  joined_at: z.string(),
});

export type OrgMemberVM = z.infer<typeof OrgMemberVMSchema>;

export const OrgMemberListVMSchema = z.object({
  items: z.array(OrgMemberVMSchema),
});

export type OrgMemberListVM = z.infer<typeof OrgMemberListVMSchema>;

export const InviteOrgMemberCommandSchema = z.object({
  email: z.string().email(),
  role: z.enum(["admin", "member"]).optional().default("member"),
});

export type InviteOrgMemberCommand = z.infer<typeof InviteOrgMemberCommandSchema>;

export const UpdateOrgMemberRoleCommandSchema = z.object({
  role: z.enum(["admin", "member"]),
});

export type UpdateOrgMemberRoleCommand = z.infer<typeof UpdateOrgMemberRoleCommandSchema>;

export const OrgPermissionsVMSchema = z.object({
  permissions: z.array(z.string()),
});

export type OrgPermissionsVM = z.infer<typeof OrgPermissionsVMSchema>;

export const PlatformOrganizationVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  status: z.enum(["active", "suspended"]),
  created_at: z.string(),
});

export type PlatformOrganizationVM = z.infer<typeof PlatformOrganizationVMSchema>;

export const CreatePlatformOrganizationCommandSchema = z.object({
  name: z.string().min(1),
  slug: z.string().min(1),
  owner_email: z.string().email(),
});

export type CreatePlatformOrganizationCommand = z.infer<typeof CreatePlatformOrganizationCommandSchema>;

export const UpdateOrgStatusCommandSchema = z.object({
  status: z.enum(["active", "suspended"]),
});

export type UpdateOrgStatusCommand = z.infer<typeof UpdateOrgStatusCommandSchema>;

export const TransferOrgOwnerCommandSchema = z.object({
  owner_email: z.string().email(),
});

export type TransferOrgOwnerCommand = z.infer<typeof TransferOrgOwnerCommandSchema>;

export const AuditLogVMSchema = z.object({
  id: z.string(),
  actor_id: z.string(),
  action: z.string(),
  resource_type: z.string(),
  resource_id: z.string(),
  org_id: z.string().optional(),
  detail: z.record(z.unknown()),
  created_at: z.string(),
});

export type AuditLogVM = z.infer<typeof AuditLogVMSchema>;

export const AuditLogListVMSchema = z.object({
  items: z.array(AuditLogVMSchema),
});

export type AuditLogListVM = z.infer<typeof AuditLogListVMSchema>;

export const OrgQuotaVMSchema = z.object({
  max_workspaces: z.number(),
  max_agents: z.number(),
  max_projects: z.number(),
  token_budget: z.number(),
});

export type OrgQuotaVM = z.infer<typeof OrgQuotaVMSchema>;

export const OrgQuotaUsageVMSchema = z.object({
  workspaces: z.number(),
  agents: z.number(),
  projects: z.number(),
});

export type OrgQuotaUsageVM = z.infer<typeof OrgQuotaUsageVMSchema>;

export const UpdateOrgQuotaCommandSchema = z.object({
  max_workspaces: z.number().optional(),
  max_agents: z.number().optional(),
  max_projects: z.number().optional(),
  token_budget: z.number().optional(),
});

export type UpdateOrgQuotaCommand = z.infer<typeof UpdateOrgQuotaCommandSchema>;
