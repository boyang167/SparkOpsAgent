import { z } from "zod";

export const ProductWorkflowVMSchema = z.object({
  id: z.string(),
  product_id: z.string(),
  name: z.string(),
  key: z.string(),
  status: z.string(),
  stage_count: z.number().int(),
  created_at: z.string().optional(),
});

export type ProductWorkflowVM = z.infer<typeof ProductWorkflowVMSchema>;

export const ProjectBuildRunVMSchema = z.object({
  project_id: z.string(),
  branch: z.string().optional(),
  build_id: z.string().optional(),
  status: z.string(),
  error_message: z.string().optional(),
});

export type ProjectBuildRunVM = z.infer<typeof ProjectBuildRunVMSchema>;

export const ProjectDeployRunVMSchema = z.object({
  project_id: z.string(),
  environment_id: z.string().optional(),
  deployment_run_id: z.string().optional(),
  image_tag: z.string().optional(),
  status: z.string(),
  error_message: z.string().optional(),
});

export type ProjectDeployRunVM = z.infer<typeof ProjectDeployRunVMSchema>;

export const ApprovalRunVMSchema = z.object({
  status: z.string(),
  actor_id: z.string().optional(),
  message: z.string().optional(),
});

export type ApprovalRunVM = z.infer<typeof ApprovalRunVMSchema>;

export const WorkflowJobRunVMSchema = z.object({
  name: z.string(),
  type: z.string(),
  status: z.string(),
  builds: z.array(ProjectBuildRunVMSchema).optional(),
  deploys: z.array(ProjectDeployRunVMSchema).optional(),
  approval: ApprovalRunVMSchema.optional(),
  error_message: z.string().optional(),
});

export type WorkflowJobRunVM = z.infer<typeof WorkflowJobRunVMSchema>;

export const WorkflowStageRunVMSchema = z.object({
  name: z.string(),
  status: z.string(),
  jobs: z.array(WorkflowJobRunVMSchema),
});

export type WorkflowStageRunVM = z.infer<typeof WorkflowStageRunVMSchema>;

export const ProductWorkflowRunSummaryVMSchema = z.object({
  id: z.string(),
  status: z.string(),
  current_stage: z.number().int(),
  created_at: z.string().optional(),
});

export type ProductWorkflowRunSummaryVM = z.infer<typeof ProductWorkflowRunSummaryVMSchema>;

export const ProductWorkflowRunVMSchema = z.object({
  id: z.string(),
  workflow_id: z.string(),
  product_id: z.string(),
  status: z.string(),
  current_stage: z.number().int(),
  stage_runs: z.array(WorkflowStageRunVMSchema),
  error_message: z.string().optional(),
  created_at: z.string().optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
});

export type ProductWorkflowRunVM = z.infer<typeof ProductWorkflowRunVMSchema>;

export const CreateProductWorkflowCommandSchema = z.object({
  template_id: z.string().min(1),
  name: z.string().optional(),
  key_vals: z.record(z.string()).optional(),
  project_ids: z.array(z.string()).min(1),
});

export type CreateProductWorkflowCommand = z.infer<typeof CreateProductWorkflowCommandSchema>;

export const RejectWorkflowJobCommandSchema = z.object({
  message: z.string().optional(),
});

export type RejectWorkflowJobCommand = z.infer<typeof RejectWorkflowJobCommandSchema>;
