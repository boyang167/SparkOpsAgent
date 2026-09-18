import { z } from "zod";

export const PRODUCT_ENVIRONMENT_TYPES = [
  { id: "dev", label: "开发" },
  { id: "test", label: "测试" },
  { id: "staging", label: "预发" },
  { id: "production", label: "生产" },
] as const;

export const ProductEnvironmentTypeSchema = z.enum(["dev", "test", "staging", "production"]);
export type ProductEnvironmentType = z.infer<typeof ProductEnvironmentTypeSchema>;

export function environmentTypeLabel(type: string | undefined): string {
  return PRODUCT_ENVIRONMENT_TYPES.find((item) => item.id === type)?.label || "测试";
}

export function environmentTypeBadgeClass(type: string | undefined): string {
  if (type === "production") return "badge-warning";
  if (type === "staging") return "badge-primary";
  if (type === "dev") return "badge-neutral";
  return "badge-primary";
}

export function environmentRequiresApproval(type: string | undefined): boolean {
  return type === "staging" || type === "production";
}

export const EnvironmentVMSchema = z.object({
  id: z.string(),
  product_id: z.string(),
  name: z.string(),
  env_type: z.string(),
  cluster_id: z.string(),
  namespace: z.string(),
  variables: z.record(z.string()).optional(),
  status: z.string(),
  created_at: z.string().optional(),
});

export type EnvironmentVM = z.infer<typeof EnvironmentVMSchema>;

export const EnvironmentProjectStatusVMSchema = z.object({
  project_id: z.string(),
  project_name: z.string(),
  display_name: z.string().optional(),
  enabled: z.boolean(),
  current_image_tag: z.string().optional(),
  current_deployment_run_id: z.string().optional(),
  last_deployed_at: z.string().optional(),
});

export type EnvironmentProjectStatusVM = z.infer<typeof EnvironmentProjectStatusVMSchema>;

export const EnvironmentStatusVMSchema = z.object({
  environment_id: z.string(),
  name: z.string(),
  namespace: z.string(),
  cluster_id: z.string(),
  projects: z.array(EnvironmentProjectStatusVMSchema),
});

export type EnvironmentStatusVM = z.infer<typeof EnvironmentStatusVMSchema>;

export const CreateEnvironmentCommandSchema = z.object({
  name: z.string().min(1),
  env_type: ProductEnvironmentTypeSchema.optional(),
  cluster_id: z.string().min(1),
  namespace: z.string().min(1),
  variables: z.record(z.string()).optional(),
  project_ids: z.array(z.string()).optional(),
});

export type CreateEnvironmentCommand = z.infer<typeof CreateEnvironmentCommandSchema>;

export const UpdateEnvironmentCommandSchema = z.object({
  name: z.string().min(1).optional(),
  env_type: ProductEnvironmentTypeSchema.optional(),
  cluster_id: z.string().optional(),
  namespace: z.string().optional(),
  variables: z.record(z.string()).optional(),
  status: z.enum(["active", "disabled"]).optional(),
});

export type UpdateEnvironmentCommand = z.infer<typeof UpdateEnvironmentCommandSchema>;
