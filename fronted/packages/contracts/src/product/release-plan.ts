import { z } from "zod";

export const WorkflowRefSpecSchema = z.object({
  mode: z.string().optional(),
  workflow_id: z.string().optional(),
  parameters: z.record(z.string()).optional(),
});

export type WorkflowRefSpec = z.infer<typeof WorkflowRefSpecSchema>;

export const ExecutionRefSchema = z.object({
  workflow_run_id: z.string().optional(),
  build_id: z.string().optional(),
  error_message: z.string().optional(),
});

export type ExecutionRef = z.infer<typeof ExecutionRefSchema>;

export const ReleasePlanJobVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.enum(["text", "workflow"]),
  sort_order: z.number().int().optional(),
  owner_id: z.string().optional(),
  status: z.string(),
  text_content: z.string().optional(),
  text_checked: z.boolean().optional(),
  workflow_ref: WorkflowRefSpecSchema.optional(),
  execution_ref: ExecutionRefSchema.optional(),
});

export type ReleasePlanJobVM = z.infer<typeof ReleasePlanJobVMSchema>;

export const ReleasePlanVMSchema = z.object({
  id: z.string(),
  product_id: z.string(),
  sprint_id: z.string().optional(),
  name: z.string(),
  description: z.string().optional(),
  status: z.string(),
  release_window_start: z.string(),
  release_window_end: z.string(),
  approval_enabled: z.boolean().optional(),
  created_at: z.string().optional(),
  manager_id: z.string().optional(),
  jobs: z.array(ReleasePlanJobVMSchema).optional(),
  audit_logs: z.array(z.record(z.unknown())).optional(),
});

export type ReleasePlanVM = z.infer<typeof ReleasePlanVMSchema>;

export const CreateReleasePlanCommandSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  release_window_start: z.string(),
  release_window_end: z.string(),
  approval_enabled: z.boolean().optional(),
  sprint_id: z.string().optional(),
});

export type CreateReleasePlanCommand = z.infer<typeof CreateReleasePlanCommandSchema>;

export const AddReleasePlanJobCommandSchema = z.object({
  name: z.string().min(1),
  type: z.enum(["text", "workflow"]),
  text_content: z.string().optional(),
  sort_order: z.number().int().optional(),
  workflow_ref: WorkflowRefSpecSchema.optional(),
});

export type AddReleasePlanJobCommand = z.infer<typeof AddReleasePlanJobCommandSchema>;
