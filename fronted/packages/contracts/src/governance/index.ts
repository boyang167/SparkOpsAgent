import { z } from "zod";

export const PERMISSION_OPTIONS = [
  { value: "project:read", label: "项目 · 查看" },
  { value: "project:write", label: "项目 · 创建/编辑" },
  { value: "agent:read", label: "Agent · 查看" },
  { value: "agent:write", label: "Agent · 创建/编辑" },
  { value: "agent:publish", label: "Agent · 发布" },
  { value: "chat:use", label: "对话 · 使用" },
  { value: "governance:admin", label: "治理 · RBAC 管理" },
] as const;

export const RoleVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  permissions: z.array(z.string()),
  parent_id: z.string().optional(),
});

export type RoleVM = z.infer<typeof RoleVMSchema>;

export const RoleTreeVMSchema: z.ZodType<RoleTreeVM> = z.lazy(() =>
  z.object({
    id: z.string(),
    name: z.string(),
    permissions: z.array(z.string()),
    parent_id: z.string().optional(),
    children: z.array(RoleTreeVMSchema).optional(),
  }),
);

export interface RoleTreeVM {
  id: string;
  name: string;
  permissions: string[];
  parent_id?: string;
  children?: RoleTreeVM[];
}

export type RoleTreeListVM = { items: RoleTreeVM[] };

export const RoleListVMSchema = z.object({
  items: z.array(RoleVMSchema),
});

export type RoleListVM = z.infer<typeof RoleListVMSchema>;

export const CreateRoleCommandSchema = z.object({
  name: z.string().min(1),
  permissions: z.array(z.string()).min(1),
  parent_id: z.string().uuid().optional(),
});

export type CreateRoleCommand = z.infer<typeof CreateRoleCommandSchema>;

export const RoleCreatedVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  permissions: z.array(z.string()),
});

export type RoleCreatedVM = z.infer<typeof RoleCreatedVMSchema>;

export const UpdateRoleCommandSchema = z.object({
  permissions: z.array(z.string()).min(1),
});

export type UpdateRoleCommand = z.infer<typeof UpdateRoleCommandSchema>;

export const MemberRoleVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  permissions: z.array(z.string()),
});

export type MemberRoleVM = z.infer<typeof MemberRoleVMSchema>;

export const WorkspaceMemberVMSchema = z.object({
  member_id: z.string(),
  email: z.string(),
  display_name: z.string(),
  roles: z.array(MemberRoleVMSchema),
});

export type WorkspaceMemberVM = z.infer<typeof WorkspaceMemberVMSchema>;

export const WorkspaceMemberListVMSchema = z.object({
  items: z.array(WorkspaceMemberVMSchema),
});

export type WorkspaceMemberListVM = z.infer<typeof WorkspaceMemberListVMSchema>;

export const AssignRoleCommandSchema = z.object({
  role_id: z.string().uuid(),
});

export type AssignRoleCommand = z.infer<typeof AssignRoleCommandSchema>;

export const MyPermissionsVMSchema = z.object({
  permissions: z.array(z.string()),
});

export type MyPermissionsVM = z.infer<typeof MyPermissionsVMSchema>;

export const InviteMemberCommandSchema = z.object({
  email: z.string().email(),
});

export type InviteMemberCommand = z.infer<typeof InviteMemberCommandSchema>;

export const InvitedMemberVMSchema = z.object({
  member_id: z.string(),
  email: z.string(),
  display_name: z.string(),
});

export type InvitedMemberVM = z.infer<typeof InvitedMemberVMSchema>;

export const RegisteredAPISchema = z.object({
  ID: z.string().optional(),
  id: z.string().optional(),
  Path: z.string().optional(),
  path: z.string().optional(),
  Method: z.string().optional(),
  method: z.string().optional(),
  APIGroup: z.string().optional(),
  api_group: z.string().optional(),
  Description: z.string().optional(),
  description: z.string().optional(),
});

export type RegisteredAPI = {
  id: string;
  path: string;
  method: string;
  api_group: string;
  description: string;
};

export const APISyncDiffSchema = z.object({
  new: z.array(z.record(z.unknown())),
  delete: z.array(z.record(z.unknown())),
  ignore: z.array(z.record(z.unknown())).optional(),
});

export type APISyncDiff = z.infer<typeof APISyncDiffSchema>;

export const APITokenVMSchema = z.object({
  id: z.string().optional(),
  ID: z.string().optional(),
  member_id: z.string(),
  workspace_id: z.string().nullable().optional(),
  role_id: z.string().nullable().optional(),
  token_prefix: z.string(),
  status: z.string(),
  expires_at: z.string().nullable().optional(),
  remark: z.string().optional(),
  created_at: z.string().optional(),
});

export type APITokenVM = {
  id: string;
  member_id: string;
  role_id?: string | null;
  token_prefix: string;
  status: string;
  expires_at?: string | null;
  remark?: string;
  created_at?: string;
};

export const CreateAPITokenCommandSchema = z.object({
  member_id: z.string().uuid(),
  role_id: z.string().uuid().optional(),
  remark: z.string().optional().default(""),
  expire_days: z.number().int().min(0).max(3650).default(30),
});

export type CreateAPITokenCommand = z.infer<typeof CreateAPITokenCommandSchema>;

export const RoleAPIPolicySchema = z.object({
  path: z.string(),
  method: z.string(),
});

export type RoleAPIPolicy = z.infer<typeof RoleAPIPolicySchema>;
