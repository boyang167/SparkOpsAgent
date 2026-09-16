import { z } from "zod";
import { ProductWorkflowVMSchema } from "./product-workflow";

export const WorkflowTemplateKeyValSchema = z.object({
  key: z.string(),
  value: z.string().optional(),
  description: z.string().optional(),
  type: z.string().optional(),
  choice_options: z.array(z.string()).optional(),
  required: z.boolean().optional(),
  source: z.string().optional(),
});

export type WorkflowTemplateKeyVal = z.infer<typeof WorkflowTemplateKeyValSchema>;

export const WorkflowTemplateVMSchema = z.object({
  id: z.string(),
  scope: z.string(),
  workspace_id: z.string().optional(),
  name: z.string(),
  key: z.string(),
  description: z.string().optional(),
  category: z.string().optional(),
  status: z.string().optional(),
  version: z.string().optional(),
  key_vals: z.array(WorkflowTemplateKeyValSchema).optional(),
  definition: z.record(z.unknown()).optional(),
  slot_schema: z.record(z.unknown()).optional(),
});

export type WorkflowTemplateVM = z.infer<typeof WorkflowTemplateVMSchema>;

export const WorkflowTemplatePreviewVMSchema = z.object({
  template_id: z.string(),
  key_vals: z.array(WorkflowTemplateKeyValSchema),
  definition: z.record(z.unknown()),
});

export type WorkflowTemplatePreviewVM = z.infer<typeof WorkflowTemplatePreviewVMSchema>;

export const WorkspaceProductWorkflowVMSchema = ProductWorkflowVMSchema.extend({
  product_name: z.string().optional(),
  workspace_id: z.string().optional(),
});

export type WorkspaceProductWorkflowVM = z.infer<typeof WorkspaceProductWorkflowVMSchema>;

export const CreateWorkflowTemplateCommandSchema = z.object({
  name: z.string().min(1),
  key: z.string().min(1),
  description: z.string().optional(),
  definition: z.record(z.unknown()),
});

export type CreateWorkflowTemplateCommand = z.infer<typeof CreateWorkflowTemplateCommandSchema>;

export const UpdateWorkflowTemplateCommandSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
  definition: z.record(z.unknown()).optional(),
});

export type UpdateWorkflowTemplateCommand = z.infer<typeof UpdateWorkflowTemplateCommandSchema>;
