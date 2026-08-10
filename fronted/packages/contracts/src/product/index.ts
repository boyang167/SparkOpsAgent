import { z } from "zod";

export const TemplateVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  kind: z.string().optional(),
  created_at: z.string().optional(),
});

export type TemplateVM = z.infer<typeof TemplateVMSchema>;

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
