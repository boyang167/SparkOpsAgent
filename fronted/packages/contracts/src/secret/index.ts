import { z } from "zod";

export const SecretStatusSchema = z.enum(["active", "inactive"]);

export const SecretCredentialsSchema = z.object({
  password: z.string().optional(),
  token: z.string().optional(),
  api_key: z.string().optional(),
  private_key: z.string().optional(),
  public_key: z.string().optional(),
  certificate: z.string().optional(),
  extra_config: z.string().optional(),
});

export const SecretCreatorVMSchema = z.object({
  id: z.string(),
  email: z.string().optional(),
  display_name: z.string().optional(),
});

export type SecretCreatorVM = z.infer<typeof SecretCreatorVMSchema>;

export const SecretVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  display_name: z.string().optional(),
  description: z.string().optional(),
  service_type: z.string(),
  service_url: z.string(),
  username: z.string().optional(),
  status: SecretStatusSchema.optional(),
  is_default: z.boolean().optional(),
  is_shared: z.boolean().optional(),
  created_by: z.string().optional(),
  creator: SecretCreatorVMSchema.optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  credentials: SecretCredentialsSchema.optional(),
  has_password: z.boolean().optional(),
  has_token: z.boolean().optional(),
  has_api_key: z.boolean().optional(),
  has_private_key: z.boolean().optional(),
  has_public_key: z.boolean().optional(),
  has_certificate: z.boolean().optional(),
  has_extra_config: z.boolean().optional(),
});

export type SecretVM = z.infer<typeof SecretVMSchema>;

export const CreateSecretCommandSchema = z.object({
  name: z.string().min(3).max(100),
  display_name: z.string().optional(),
  description: z.string().optional(),
  service_type: z.string().min(1),
  service_url: z.string().min(1),
  username: z.string().optional(),
  credentials: SecretCredentialsSchema.default({}),
  status: SecretStatusSchema.default("active"),
  is_default: z.boolean().default(false),
  is_shared: z.boolean().default(false),
});

export type CreateSecretCommand = z.infer<typeof CreateSecretCommandSchema>;

export const UpdateSecretCommandSchema = CreateSecretCommandSchema.omit({ name: true });

export type UpdateSecretCommand = z.infer<typeof UpdateSecretCommandSchema>;

export const SECRET_SERVICE_TYPES = [
  { value: "GitLab", label: "GitLab" },
  { value: "GitHub", label: "GitHub" },
  { value: "Jenkins", label: "Jenkins" },
  { value: "Harbor", label: "Harbor" },
  { value: "Docker", label: "Docker Registry" },
  { value: "Kubernetes", label: "Kubernetes" },
  { value: "Nexus", label: "Nexus" },
  { value: "SonarQube", label: "SonarQube" },
  { value: "Prometheus", label: "Prometheus" },
  { value: "Grafana", label: "Grafana" },
  { value: "Other", label: "其他" },
] as const;

export const SECRET_STATUS_OPTIONS = [
  { value: "active", label: "活跃" },
  { value: "inactive", label: "非活跃" },
] as const;

export function serviceTypeLabel(value?: string) {
  return SECRET_SERVICE_TYPES.find((o) => o.value === value)?.label ?? value ?? "-";
}
