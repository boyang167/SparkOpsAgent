import { z } from "zod";

export const PERMISSION_OPTIONS = [
  { value: "project:read", label: "项目 · 查看" },
  { value: "project:write", label: "项目 · 创建/编辑" },
  { value: "project:submit", label: "项目 · 提交审核" },
  { value: "project:review", label: "项目 · 审批" },
  { value: "project:operate", label: "项目 · 运维操作" },
  { value: "secret:read", label: "密钥 · 查看" },
  { value: "secret:write", label: "密钥 · 管理" },
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

export const UpdateRoleCommandSchema = z
  .object({
    name: z.string().min(1).optional(),
    parent_id: z.string().uuid().nullable().optional(),
    permissions: z.array(z.string()).min(1).optional(),
  })
  .refine(
    (data) =>
      data.name !== undefined ||
      data.parent_id !== undefined ||
      data.permissions !== undefined,
    { message: "至少提供一个更新字段" },
  );

export type UpdateRoleCommand = z.infer<typeof UpdateRoleCommandSchema>;

export const SetRoleMenusCommandSchema = z.object({
  menu_ids: z.array(z.string()),
});

export type SetRoleMenusCommand = z.infer<typeof SetRoleMenusCommandSchema>;

export const SetRoleMenuButtonsCommandSchema = z.object({
  button_ids: z.array(z.string()),
});

export type SetRoleMenuButtonsCommand = z.infer<typeof SetRoleMenuButtonsCommandSchema>;

export const WorkspaceMenuButtonVMSchema = z.object({
  button_id: z.string(),
  menu_id: z.string(),
  menu_title: z.string(),
  menu_path: z.string(),
  name: z.string(),
  code: z.string(),
  remark: z.string().optional().default(""),
});

export type WorkspaceMenuButtonVM = z.infer<typeof WorkspaceMenuButtonVMSchema>;

export const RoleMenusVMSchema = z.object({
  menu_ids: z.array(z.string()),
});

export type RoleMenusVM = z.infer<typeof RoleMenusVMSchema>;

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

export const MemberCandidateVMSchema = z.object({
  member_id: z.string(),
  email: z.string(),
  display_name: z.string(),
});

export type MemberCandidateVM = z.infer<typeof MemberCandidateVMSchema>;

export const MemberCandidateListVMSchema = z.object({
  items: z.array(MemberCandidateVMSchema),
});

export type MemberCandidateListVM = z.infer<typeof MemberCandidateListVMSchema>;

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

export const UpdateAPIMetaCommandSchema = z.object({
  api_group: z.string().min(1),
  description: z.string().optional().default(""),
});

export type UpdateAPIMetaCommand = z.infer<typeof UpdateAPIMetaCommandSchema>;

export const DeleteApisCommandSchema = z.object({
  ids: z.array(z.string().uuid()).min(1),
});

export type DeleteApisCommand = z.infer<typeof DeleteApisCommandSchema>;

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

export const DATA_ACTIONS = ["select", "insert", "update", "delete"] as const;
export type DataAction = (typeof DATA_ACTIONS)[number];

export const DATA_SCOPES = [
  { value: "all", label: "全部数据" },
  { value: "member_set", label: "关联成员" },
  { value: "self", label: "仅本人" },
  { value: "custom", label: "自定义" },
] as const;

export type DataScope = (typeof DATA_SCOPES)[number]["value"];

export const RoleDataPolicySchema = z.object({
  resource_key: z.string().min(1),
  action: z.enum(DATA_ACTIONS),
  data_scope: z.string().min(1),
  custom_rules: z.array(z.record(z.unknown())).optional(),
});

export type RoleDataPolicyVM = z.infer<typeof RoleDataPolicySchema>;

export const UpdateRoleDataPoliciesCommandSchema = z.object({
  items: z.array(RoleDataPolicySchema),
});

export type UpdateRoleDataPoliciesCommand = z.infer<typeof UpdateRoleDataPoliciesCommandSchema>;

export const DataResourceVMSchema = z.object({
  id: z.string(),
  resource_key: z.string(),
  schema_name: z.string(),
  table_name: z.string(),
  owner_column: z.string(),
  workspace_column: z.string(),
  title: z.string(),
  description: z.string(),
  enabled: z.boolean(),
});

export type DataResourceVM = z.infer<typeof DataResourceVMSchema>;

export const MenuParamSchema = z.object({
  id: z.string().optional(),
  type: z.enum(["query", "params"]).default("query"),
  key: z.string().min(1),
  value: z.string().optional().default(""),
  sort: z.number().int().optional().default(0),
});

export type MenuParamVM = z.infer<typeof MenuParamSchema>;

export const MenuButtonSchema = z.object({
  id: z.string().optional(),
  name: z.string().min(1),
  code: z.string().min(1),
  remark: z.string().optional().default(""),
  sort: z.number().int().optional().default(0),
  role_ids: z.array(z.string()).optional().default([]),
});

export type MenuButtonVM = z.infer<typeof MenuButtonSchema>;

export const MenuTreeVMSchema: z.ZodType<MenuTreeVM> = z.lazy(() =>
  z.object({
    id: z.string(),
    parent_id: z.string().nullable().optional(),
    name: z.string(),
    path: z.string(),
    title: z.string(),
    icon: z.string().optional(),
    component: z.string().optional(),
    hidden: z.boolean(),
    sort: z.number(),
    keep_alive: z.boolean(),
    active_menu: z.string().optional(),
    close_tab: z.boolean().optional(),
    is_basic: z.boolean().optional(),
    transition_animation: z.string().optional(),
    children: z.array(MenuTreeVMSchema).optional(),
  }),
);

export interface MenuTreeVM {
  id: string;
  parent_id?: string | null;
  name: string;
  path: string;
  title: string;
  icon?: string;
  component?: string;
  hidden: boolean;
  sort: number;
  keep_alive: boolean;
  active_menu?: string;
  close_tab?: boolean;
  is_basic?: boolean;
  transition_animation?: string;
  children?: MenuTreeVM[];
}

export const MenuVMSchema = z.object({
  id: z.string(),
  parent_id: z.string().nullable().optional(),
  name: z.string(),
  path: z.string(),
  title: z.string(),
  icon: z.string().optional(),
  component: z.string().optional(),
  hidden: z.boolean(),
  sort: z.number(),
  keep_alive: z.boolean(),
  active_menu: z.string().optional(),
  close_tab: z.boolean().optional(),
  is_basic: z.boolean().optional(),
  transition_animation: z.string().optional(),
  params: z.array(MenuParamSchema).optional(),
  buttons: z.array(MenuButtonSchema).optional(),
});

export type MenuVM = z.infer<typeof MenuVMSchema>;

export const UpsertMenuCommandSchema = z.object({
  parent_id: z.string().nullable().optional(),
  name: z.string().min(1),
  path: z.string().min(1),
  title: z.string().min(1),
  icon: z.string().optional().default(""),
  component: z.string().optional().default(""),
  hidden: z.boolean().optional().default(false),
  sort: z.number().int().optional().default(0),
  keep_alive: z.boolean().optional().default(false),
  active_menu: z.string().optional().default(""),
  close_tab: z.boolean().optional().default(false),
  is_basic: z.boolean().optional().default(false),
  transition_animation: z.enum(["global", "fade", "none"]).optional().default("global"),
  params: z.array(MenuParamSchema).optional().default([]),
  buttons: z.array(MenuButtonSchema).optional().default([]),
});

export type UpsertMenuCommand = z.infer<typeof UpsertMenuCommandSchema>;

export const SetMenuRolesCommandSchema = z.object({
  role_ids: z.array(z.string()),
});

export type SetMenuRolesCommand = z.infer<typeof SetMenuRolesCommandSchema>;

export const MyMenuButtonItemSchema = z.object({
  code: z.string(),
  name: z.string(),
  remark: z.string().optional().default(""),
});

export type MyMenuButtonItemVM = z.infer<typeof MyMenuButtonItemSchema>;

export const MyMenuButtonsVMSchema = z.object({
  menu_id: z.string().optional(),
  path: z.string(),
  configured: z.boolean(),
  items: z.array(MyMenuButtonItemSchema),
});

export type MyMenuButtonsVM = z.infer<typeof MyMenuButtonsVMSchema>;
