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

export const AuthProvidersVMSchema = z.object({
  oidc_enabled: z.boolean(),
  ldap_enabled: z.boolean(),
});

export type AuthProvidersVM = z.infer<typeof AuthProvidersVMSchema>;

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
