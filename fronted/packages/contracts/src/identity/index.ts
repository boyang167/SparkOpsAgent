import { z } from "zod";

export const LoginCommandSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
});

export type LoginCommand = z.infer<typeof LoginCommandSchema>;

export const RegisterCommandSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
  display_name: z.string().min(1),
});

export type RegisterCommand = z.infer<typeof RegisterCommandSchema>;

export const AuthSessionVMSchema = z.object({
  token: z.string(),
  memberId: z.string(),
  email: z.string().email(),
  displayName: z.string().optional(),
});

export type AuthSessionVM = z.infer<typeof AuthSessionVMSchema>;

export const MeVMSchema = z.object({
  id: z.string(),
  email: z.string().email(),
  display_name: z.string(),
  is_super_admin: z.boolean().optional().default(false),
});

export type MeVM = z.infer<typeof MeVMSchema>;

export const UpdateProfileCommandSchema = z.object({
  display_name: z.string().min(1),
});

export type UpdateProfileCommand = z.infer<typeof UpdateProfileCommandSchema>;

export const ChangePasswordCommandSchema = z.object({
  current_password: z.string().min(1),
  new_password: z.string().min(8),
});

export type ChangePasswordCommand = z.infer<typeof ChangePasswordCommandSchema>;

export const LoginUIConfigSchema = z.object({
  enabled: z.boolean().optional().default(false),
  provider: z.string().optional().default("oidc"),
  show_password_login: z.boolean().optional().default(true),
});

export type LoginUIConfigVM = z.infer<typeof LoginUIConfigSchema>;

export const AuthProvidersVMSchema = z.object({
  oidc_enabled: z.boolean(),
  ldap_enabled: z.boolean(),
  sso_ui: LoginUIConfigSchema.optional(),
});

export type AuthProvidersVM = z.infer<typeof AuthProvidersVMSchema>;

export const SSOProviderSchema = z.enum(["oidc", "ldap"]);

export const SSOLoginUISettingsSchema = z.object({
  enabled: z.boolean(),
  provider: SSOProviderSchema,
  show_password_login: z.boolean(),
}).strict();

const OIDCAdminBaseSchema = z.object({
  enabled: z.boolean(),
  issuer: z.string(),
  client_id: z.string(),
  redirect_url: z.string(),
  scopes: z.string(),
  authorization_endpoint: z.string(),
  token_endpoint: z.string(),
  userinfo_endpoint: z.string(),
  tested_fingerprint: z.string(),
  tested_at: z.string().optional(),
}).strict();

const LDAPAdminBaseSchema = z.object({
  enabled: z.boolean(),
  url: z.string(),
  bind_dn: z.string(),
  user_search_base: z.string(),
  user_search_filter: z.string(),
  email_attribute: z.string(),
  name_attribute: z.string(),
  tested_fingerprint: z.string(),
  tested_at: z.string().optional(),
}).strict();

export const SSOOIDCAdminVMSchema = OIDCAdminBaseSchema.extend({
  has_client_secret: z.boolean(),
  runtime_enabled: z.boolean(),
}).strict();

export const SSOLDAPAdminVMSchema = LDAPAdminBaseSchema.extend({
  has_bind_password: z.boolean(),
  runtime_enabled: z.boolean(),
}).strict();

export const SSOAdminConfigVMSchema = z.object({
  version: z.number().int().nonnegative(),
  login_ui: SSOLoginUISettingsSchema,
  oidc: SSOOIDCAdminVMSchema,
  ldap: SSOLDAPAdminVMSchema,
}).strict();

export type SSOAdminConfigVM = z.infer<typeof SSOAdminConfigVMSchema>;

export const UpdateSSOConfigCommandSchema = z.object({
  version: z.number().int().nonnegative(),
  login_ui: SSOLoginUISettingsSchema,
  oidc: OIDCAdminBaseSchema.extend({
    client_secret: z.string(),
    clear_client_secret: z.boolean(),
  }).strict(),
  ldap: LDAPAdminBaseSchema.extend({
    bind_password: z.string(),
    clear_bind_password: z.boolean(),
  }).strict(),
}).strict();

export type UpdateSSOConfigCommand = z.infer<typeof UpdateSSOConfigCommandSchema>;

export const TestSSOProviderCommandSchema = z.object({
  provider: SSOProviderSchema,
  config: UpdateSSOConfigCommandSchema,
}).strict();

export type TestSSOProviderCommand = z.infer<typeof TestSSOProviderCommandSchema>;

export const TestSSOProviderResultVMSchema = z.object({
  provider: SSOProviderSchema,
  fingerprint: z.string().length(64),
  tested_at: z.string(),
  message: z.string(),
}).strict();

export type TestSSOProviderResultVM = z.infer<typeof TestSSOProviderResultVMSchema>;

export const LDAPLoginCommandSchema = z.object({
  username: z.string().min(1),
  password: z.string().min(1),
});

export type LDAPLoginCommand = z.infer<typeof LDAPLoginCommandSchema>;

export const PlatformMemberVMSchema = z.object({
  id: z.string(),
  email: z.string().email(),
  display_name: z.string(),
  status: z.enum(["active", "disabled"]),
  is_super_admin: z.boolean(),
  created_at: z.string(),
});

export type PlatformMemberVM = z.infer<typeof PlatformMemberVMSchema>;

export const CreatePlatformMemberCommandSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
  display_name: z.string().min(1),
  is_super_admin: z.boolean().optional().default(false),
});

export type CreatePlatformMemberCommand = z.infer<typeof CreatePlatformMemberCommandSchema>;

export const UpdatePlatformMemberCommandSchema = z.object({
  display_name: z.string().min(1).optional(),
  status: z.enum(["active", "disabled"]).optional(),
  is_super_admin: z.boolean().optional(),
});

export type UpdatePlatformMemberCommand = z.infer<typeof UpdatePlatformMemberCommandSchema>;

export const AdminResetPasswordCommandSchema = z.object({
  new_password: z.string().min(8),
});

export type AdminResetPasswordCommand = z.infer<typeof AdminResetPasswordCommandSchema>;
