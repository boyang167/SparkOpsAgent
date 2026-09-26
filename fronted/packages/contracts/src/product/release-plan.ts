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
  done_remark: z.string().optional(),
});

export type ExecutionRef = z.infer<typeof ExecutionRefSchema>;

export const ApprovalProviderSchema = z.enum(["internal", "feishu", "dingtalk"]);

export const ApprovalInstanceSchema = z.object({
  token: z.string().optional(),
  status: z.string().optional(),
  sent_at: z.string().optional(),
  acted_at: z.string().optional(),
  actor_note: z.string().optional(),
});

export const ApprovalConfigSchema = z.object({
  enabled: z.boolean().optional(),
  provider: ApprovalProviderSchema.optional(),
  webhook_url: z.string().optional(),
  approver_ids: z.array(z.string()).optional(),
  instance: ApprovalInstanceSchema.optional(),
});

export type ApprovalConfig = z.infer<typeof ApprovalConfigSchema>;

export const ReleasePlanJobVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.enum(["text", "workflow"]),
  sort_order: z.number().int().optional(),
  owner_id: z.string().optional(),
  status: z.string(),
  text_content: z.string().optional(),
  text_format: z.string().optional(),
  text_checked: z.boolean().optional(),
  workflow_ref: WorkflowRefSpecSchema.optional(),
  execution_ref: ExecutionRefSchema.optional(),
});

export type ReleasePlanJobVM = z.infer<typeof ReleasePlanJobVMSchema>;

export const ReleasePlanSprintVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.string(),
  external_ref: z.record(z.unknown()).optional(),
});

export type ReleasePlanSprintVM = z.infer<typeof ReleasePlanSprintVMSchema>;

export const LinkedWorkItemVMSchema = z.object({
  id: z.string(),
  title: z.string(),
  stage_id: z.string(),
  status: z.string(),
  pm_links: z
    .array(
      z.object({
        provider: z.string().optional(),
        external_key: z.string().optional(),
        url: z.string().optional(),
      }),
    )
    .optional(),
});

export type LinkedWorkItemVM = z.infer<typeof LinkedWorkItemVMSchema>;

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
  approval_config: ApprovalConfigSchema.optional(),
  schedule_execute_time: z.string().optional(),
  created_at: z.string().optional(),
  manager_id: z.string().optional(),
  jobs: z.array(ReleasePlanJobVMSchema).optional(),
  audit_logs: z.array(z.record(z.unknown())).optional(),
  sprint: ReleasePlanSprintVMSchema.optional(),
  linked_work_items: z.array(LinkedWorkItemVMSchema).optional(),
});

export type ReleasePlanVM = z.infer<typeof ReleasePlanVMSchema>;

export const WorkspaceReleasePlanVMSchema = ReleasePlanVMSchema.extend({
  product_name: z.string().optional(),
  sprint_name: z.string().optional(),
});

export type WorkspaceReleasePlanVM = z.infer<typeof WorkspaceReleasePlanVMSchema>;

export const BlockingReleasePlanVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.string(),
  product_id: z.string(),
});

export type BlockingReleasePlanVM = z.infer<typeof BlockingReleasePlanVMSchema>;

export const CreateReleasePlanCommandSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  manager_id: z.string().optional(),
  release_window_start: z.string(),
  release_window_end: z.string(),
  approval_enabled: z.boolean().optional(),
  approval_config: ApprovalConfigSchema.optional(),
  schedule_execute_time: z.string().optional(),
  sprint_id: z.string().optional(),
});

export type CreateReleasePlanCommand = z.infer<typeof CreateReleasePlanCommandSchema>;

export const UpdateReleasePlanCommandSchema = z.object({
  name: z.string().min(1).optional(),
  description: z.string().optional(),
  manager_id: z.string().optional(),
  approver_id: z.string().optional(),
  release_window_start: z.string().optional(),
  release_window_end: z.string().optional(),
  approval_enabled: z.boolean().optional(),
  approval_config: ApprovalConfigSchema.optional(),
  schedule_execute_time: z.string().optional(),
  clear_schedule_time: z.boolean().optional(),
  sprint_id: z.string().optional(),
  clear_sprint_id: z.boolean().optional(),
});

export type UpdateReleasePlanCommand = z.infer<typeof UpdateReleasePlanCommandSchema>;

export const RejectReleasePlanCommandSchema = z.object({
  reason: z.string().optional(),
});

export type RejectReleasePlanCommand = z.infer<typeof RejectReleasePlanCommandSchema>;

export const AddReleasePlanJobCommandSchema = z.object({
  name: z.string().min(1),
  type: z.enum(["text", "workflow"]),
  text_content: z.string().optional(),
  text_format: z.string().optional(),
  sort_order: z.number().int().optional(),
  owner_id: z.string().optional(),
  workflow_ref: WorkflowRefSpecSchema.optional(),
});

export type AddReleasePlanJobCommand = z.infer<typeof AddReleasePlanJobCommandSchema>;

export const UpdateReleasePlanJobCommandSchema = z.object({
  name: z.string().min(1).optional(),
  owner_id: z.string().optional(),
  text_content: z.string().optional(),
  text_format: z.string().optional(),
  sort_order: z.number().int().optional(),
  workflow_ref: WorkflowRefSpecSchema.optional(),
});

export type UpdateReleasePlanJobCommand = z.infer<typeof UpdateReleasePlanJobCommandSchema>;

export const ReorderReleasePlanJobsCommandSchema = z.object({
  job_ids: z.array(z.string()).min(1),
});

export type ReorderReleasePlanJobsCommand = z.infer<typeof ReorderReleasePlanJobsCommandSchema>;

export const AddJobsFromWorkItemsCommandSchema = z.object({
  work_item_ids: z.array(z.string()).min(1),
});

export type AddJobsFromWorkItemsCommand = z.infer<typeof AddJobsFromWorkItemsCommandSchema>;

export const CopyReleasePlanCommandSchema = z.object({
  name: z.string().optional(),
});

export type CopyReleasePlanCommand = z.infer<typeof CopyReleasePlanCommandSchema>;

export const SaveReleasePlanTemplateCommandSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
});

export type SaveReleasePlanTemplateCommand = z.infer<typeof SaveReleasePlanTemplateCommandSchema>;

export const ReleasePlanTemplateVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  job_count: z.number().int().optional(),
  created_at: z.string().optional(),
});

export type ReleasePlanTemplateVM = z.infer<typeof ReleasePlanTemplateVMSchema>;

export const CreateReleasePlanFromTemplateCommandSchema = z.object({
  product_id: z.string(),
  name: z.string().optional(),
  description: z.string().optional(),
  manager_id: z.string().optional(),
  release_window_start: z.string(),
  release_window_end: z.string(),
  schedule_execute_time: z.string().optional(),
  sprint_id: z.string().optional(),
});

export type CreateReleasePlanFromTemplateCommand = z.infer<typeof CreateReleasePlanFromTemplateCommandSchema>;

export const MarkReleasePlanJobDoneCommandSchema = z.object({
  remark: z.string().optional(),
});

export type MarkReleasePlanJobDoneCommand = z.infer<typeof MarkReleasePlanJobDoneCommandSchema>;
