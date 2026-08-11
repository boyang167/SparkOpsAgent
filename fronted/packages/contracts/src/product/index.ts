import { z } from "zod";

export const TemplateStatusSchema = z.enum(["active", "inactive", "draft"]);

export const TemplateCreatorVMSchema = z.object({
  id: z.string(),
  email: z.string().optional(),
  display_name: z.string().optional(),
});

export type TemplateCreatorVM = z.infer<typeof TemplateCreatorVMSchema>;

export const TemplateVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  kind: z.string().optional(),
  language: z.string().optional(),
  detail: z.string().optional(),
  rules: z.string().optional(),
  status: TemplateStatusSchema.optional(),
  is_public: z.boolean().optional(),
  created_by: z.string().optional(),
  creator: TemplateCreatorVMSchema.optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export type TemplateVM = z.infer<typeof TemplateVMSchema>;

export const CreateTemplateCommandSchema = z.object({
  name: z.string().min(2).max(50),
  kind: z.string().min(1),
  language: z.string().min(1),
  detail: z.string().min(10).max(5000),
  rules: z.string().optional(),
  status: TemplateStatusSchema.default("active"),
  is_public: z.boolean().default(false),
});

export type CreateTemplateCommand = z.infer<typeof CreateTemplateCommandSchema>;

export const UpdateTemplateCommandSchema = CreateTemplateCommandSchema;

export type UpdateTemplateCommand = z.infer<typeof UpdateTemplateCommandSchema>;

export const TEMPLATE_KIND_OPTIONS = [
  { value: "Deployment", label: "Deployment" },
  { value: "Service", label: "Service" },
  { value: "Ingress", label: "Ingress" },
  { value: "ConfigMap", label: "ConfigMap" },
  { value: "Secret", label: "Secret" },
  { value: "PersistentVolumeClaim", label: "PersistentVolumeClaim" },
  { value: "PersistentVolume", label: "PersistentVolume" },
  { value: "StorageClass", label: "StorageClass" },
  { value: "StatefulSet", label: "StatefulSet" },
  { value: "Job", label: "Job" },
  { value: "VirtualService", label: "VirtualService" },
  { value: "DestinationRule", label: "DestinationRule" },
  { value: "Gateway", label: "Gateway" },
  { value: "jenkinsFile", label: "jenkinsFile" },
  { value: "other", label: "other" },
] as const;

export const TEMPLATE_LANGUAGE_OPTIONS = [
  { value: "java", label: "Java" },
  { value: "javascript", label: "JavaScript" },
  { value: "typescript", label: "TypeScript" },
  { value: "python", label: "Python" },
  { value: "go", label: "Go" },
  { value: "php", label: "PHP" },
  { value: "ruby", label: "Ruby" },
  { value: "csharp", label: "C#" },
  { value: "cpp", label: "C++" },
  { value: "other", label: "其他" },
] as const;

export const TEMPLATE_STATUS_OPTIONS = [
  { value: "active", label: "启用" },
  { value: "inactive", label: "禁用" },
  { value: "draft", label: "草稿" },
] as const;

export const ProductVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  owner: z.string().optional(),
  status: z.string().optional(),
  created_at: z.string().optional(),
});

export type ProductVM = z.infer<typeof ProductVMSchema>;

export const ProductPlanVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  version: z.string().optional(),
  description: z.string().optional(),
  product_id: z.string().optional(),
  created_at: z.string().optional(),
});

export type ProductPlanVM = z.infer<typeof ProductPlanVMSchema>;

export const ProductRequirementVMSchema = z.object({
  id: z.string(),
  product_id: z.string().optional(),
  product_plan_id: z.string().optional(),
  project_id: z.string().optional(),
  project_name: z.string().optional(),
  project_tag: z.string().optional(),
  build_id: z.string().optional(),
  description: z.string().optional(),
  status: z.string(),
  multicluster_status: z.string().optional(),
  operator: z.string().optional(),
  created_at: z.string().optional(),
});

export type ProductRequirementVM = z.infer<typeof ProductRequirementVMSchema>;

export const DockerfileVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  base_image: z.string().optional(),
  created_at: z.string().optional(),
});

export type DockerfileVM = z.infer<typeof DockerfileVMSchema>;
