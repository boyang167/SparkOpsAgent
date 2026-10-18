import { z } from "zod";

export const WorkflowJobSpecSchema = z.record(z.unknown()).default({});

export const WorkflowJobVMSchema = z.object({
  name: z.string(),
  type: z.string(),
  spec: WorkflowJobSpecSchema.optional().default({}),
});

export type WorkflowJobVM = z.infer<typeof WorkflowJobVMSchema>;

export const WorkflowStageVMSchema = z.object({
  name: z.string(),
  parallel: z.boolean().optional().default(false),
  exec_mode: z.enum(["auto", "manual"]).optional().default("auto"),
  jobs: z.array(WorkflowJobVMSchema).default([]),
});

export type WorkflowStageVM = z.infer<typeof WorkflowStageVMSchema>;

export const WorkflowNotificationVMSchema = z.object({
  id: z.string(),
  enabled: z.boolean().optional().default(true),
  events: z.array(z.string()).default([]),
  channel: z.string(),
  webhook_url: z.string().optional().default(""),
  targets: z.string().optional().default(""),
});

export type WorkflowNotificationVM = z.infer<typeof WorkflowNotificationVMSchema>;

export const WorkflowVariableVMSchema = z.object({
  id: z.string(),
  key: z.string(),
  value: z.string().optional().default(""),
  description: z.string().optional().default(""),
  runtime_input: z.boolean().optional().default(false),
  secret: z.boolean().optional().default(false),
  vault_path: z.string().optional().default(""),
});

export type WorkflowVariableVM = z.infer<typeof WorkflowVariableVMSchema>;

export const WorkflowTriggerVMSchema = z.object({
  id: z.string(),
  type: z.enum(["git", "webhook", "cron"]),
  enabled: z.boolean().optional().default(true),
  project_id: z.string().optional().default(""),
  events: z.array(z.string()).optional().default([]),
  branch_filter: z.string().optional().default(""),
  token: z.string().optional().default(""),
  schedule: z.string().optional().default(""),
});

export type WorkflowTriggerVM = z.infer<typeof WorkflowTriggerVMSchema>;

export const WorkflowAdvancedVMSchema = z.object({
  concurrency: z.number().int().optional().default(1),
  timeout_minutes: z.number().int().optional().default(0),
  debug: z.boolean().optional().default(false),
});

export type WorkflowAdvancedVM = z.infer<typeof WorkflowAdvancedVMSchema>;

export const WorkflowRuntimeInputRefSchema = z.object({
  stage_index: z.number().int(),
  stage_name: z.string(),
  job_name: z.string(),
  job_type: z.string(),
  field: z.string(),
});

export type WorkflowRuntimeInputRef = z.infer<typeof WorkflowRuntimeInputRefSchema>;

export const WorkflowRuntimeInputSchema = z.object({
  key: z.string(),
  label: z.string(),
  declared: z.boolean(),
  secret: z.boolean().optional(),
  refs: z.array(WorkflowRuntimeInputRefSchema).optional(),
});

export type WorkflowRuntimeInput = z.infer<typeof WorkflowRuntimeInputSchema>;

export const ProductWorkflowVMSchema = z.object({
  id: z.string(),
  product_id: z.string(),
  name: z.string(),
  key: z.string(),
  status: z.string(),
  stage_count: z.number().int(),
  description: z.string().optional(),
  stages: z.array(WorkflowStageVMSchema).optional(),
  notifications: z.array(WorkflowNotificationVMSchema).optional().default([]),
  variables: z.array(WorkflowVariableVMSchema).optional().default([]),
  triggers: z.array(WorkflowTriggerVMSchema).optional().default([]),
  advanced: WorkflowAdvancedVMSchema.optional(),
  runtime_inputs: z.array(WorkflowRuntimeInputSchema).optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export type ProductWorkflowVM = z.infer<typeof ProductWorkflowVMSchema>;

export const ProjectBuildRunVMSchema = z.object({
  project_id: z.string(),
  branch: z.string().optional(),
  git_ref_type: z.string().optional(),
  image_tag: z.string().optional(),
  image_repository: z.string().optional(),
  build_id: z.string().optional(),
  status: z.string(),
  error_message: z.string().optional(),
});

export type ProjectBuildRunVM = z.infer<typeof ProjectBuildRunVMSchema>;

export const ProjectDeployRunVMSchema = z.object({
  project_id: z.string(),
  environment_id: z.string().optional(),
  source_environment_id: z.string().optional(),
  deployment_run_id: z.string().optional(),
  version_id: z.string().optional(),
  image_tag: z.string().optional(),
  image_repository: z.string().optional(),
  sync_image: z.boolean().optional(),
  sync_yaml: z.boolean().optional(),
  first_sync: z.boolean().optional(),
  status: z.string(),
  error_message: z.string().optional(),
});

export type ProjectDeployRunVM = z.infer<typeof ProjectDeployRunVMSchema>;

export const ProjectVersionRunVMSchema = z.object({
  project_id: z.string(),
  version_id: z.string(),
  version_tag: z.string(),
  status: z.string(),
  error_message: z.string().optional(),
});

export type ProjectVersionRunVM = z.infer<typeof ProjectVersionRunVMSchema>;

export const ApprovalRunVMSchema = z.object({
  status: z.string(),
  actor_id: z.string().optional(),
  message: z.string().optional(),
  started_at: z.string().optional(),
  timeout_minutes: z.number().int().optional(),
  required_approvals: z.number().int().optional(),
  approved_by: z.array(z.string()).optional(),
});

export type ApprovalRunVM = z.infer<typeof ApprovalRunVMSchema>;

export const WorkflowJobRunVMSchema = z.object({
  name: z.string(),
  type: z.string(),
  status: z.string(),
  builds: z.array(ProjectBuildRunVMSchema).optional(),
  versions: z.array(ProjectVersionRunVMSchema).optional(),
  deploys: z.array(ProjectDeployRunVMSchema).optional(),
  approval: ApprovalRunVMSchema.optional(),
  message: z.string().optional(),
  error_message: z.string().optional(),
});

export type WorkflowJobRunVM = z.infer<typeof WorkflowJobRunVMSchema>;

export const WorkflowStageRunVMSchema = z.object({
  name: z.string(),
  status: z.string(),
  jobs: z.array(WorkflowJobRunVMSchema),
});

export type WorkflowStageRunVM = z.infer<typeof WorkflowStageRunVMSchema>;

export const WorkflowRunBranchVMSchema = z.object({
  branch: z.string(),
  ref_type: z.string().optional(),
  project_id: z.string().optional(),
});

export type WorkflowRunBranchVM = z.infer<typeof WorkflowRunBranchVMSchema>;

export const ProductWorkflowRunSummaryVMSchema = z.object({
  id: z.string(),
  run_number: z.number().int().optional(),
  status: z.string(),
  current_stage: z.number().int(),
  created_at: z.string().optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
  duration_ms: z.number().int().optional(),
  trigger_source: z.string().optional(),
  executor_id: z.string().optional(),
  remark: z.string().optional(),
  branches: z.array(WorkflowRunBranchVMSchema).optional(),
  build_services: z.array(z.string()).optional(),
  deploy_services: z.array(z.string()).optional(),
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
  remark: z.string().optional(),
  pause_kind: z.string().optional(),
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

export const CreateBlankProductWorkflowCommandSchema = z.object({
  name: z.string().min(1),
  key: z.string().optional(),
  description: z.string().optional(),
  stages: z.array(WorkflowStageVMSchema).optional(),
  activate: z.boolean().optional(),
});

export type CreateBlankProductWorkflowCommand = z.infer<typeof CreateBlankProductWorkflowCommandSchema>;

export const UpdateProductWorkflowCommandSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
  status: z.enum(["draft", "active"]).optional(),
  stages: z.array(WorkflowStageVMSchema).optional(),
  notifications: z.array(WorkflowNotificationVMSchema).optional(),
  variables: z.array(WorkflowVariableVMSchema).optional(),
  triggers: z.array(WorkflowTriggerVMSchema).optional(),
  advanced: WorkflowAdvancedVMSchema.optional(),
});

export type UpdateProductWorkflowCommand = z.infer<typeof UpdateProductWorkflowCommandSchema>;

export const StartProductWorkflowRunCommandSchema = z.object({
  input: z
    .object({
      branch: z.string().optional(),
      branch_overrides: z.record(z.string()).optional(),
      ref_type_overrides: z.record(z.string()).optional(),
      environment_overrides: z.record(z.string()).optional(),
      version_tag_overrides: z.record(z.string()).optional(),
      variables: z.record(z.string()).optional(),
      remark: z.string().optional(),
      /** Confirmed environment-sync preview IDs keyed by workflow job name. */
      environment_sync_preview_ids: z.record(z.string()).optional(),
      /** 0-based indices of workflow stages to execute; omit or include all to run every stage. */
      stage_indices: z.array(z.number().int().nonnegative()).optional(),
    })
    .optional(),
});

export type StartProductWorkflowRunCommand = z.infer<typeof StartProductWorkflowRunCommandSchema>;

export const UpdateProductWorkflowRunRemarkCommandSchema = z.object({
  remark: z.string(),
});

export type UpdateProductWorkflowRunRemarkCommand = z.infer<typeof UpdateProductWorkflowRunRemarkCommandSchema>;

export const RejectWorkflowJobCommandSchema = z.object({
  message: z.string().optional(),
});

export type RejectWorkflowJobCommand = z.infer<typeof RejectWorkflowJobCommandSchema>;

export const CancelProductWorkflowRunCommandSchema = z.object({
  message: z.string().optional(),
});

export type CancelProductWorkflowRunCommand = z.infer<typeof CancelProductWorkflowRunCommandSchema>;

export const PRODUCT_WORKFLOW_JOB_CATEGORIES = [
  { id: "build", label: "构建" },
  { id: "deploy", label: "部署" },
  { id: "flow", label: "流程控制" },
  { id: "tools", label: "通用工具" },
] as const;

export const PRODUCT_WORKFLOW_JOB_TYPES = [
  {
    type: "sparkops-build",
    category: "build",
    label: "构建",
    title: "SparkOps 构建",
    description: "触发关联项目构建",
    official: true,
  },
  {
    type: "sparkops-create-version",
    category: "build",
    label: "创建版本",
    title: "创建交付版本",
    description: "从构建产物快照 YAML 并创建版本",
    official: true,
  },
  {
    type: "sparkops-deploy",
    category: "deploy",
    label: "部署",
    title: "SparkOps 部署",
    description: "部署构建产物或交付版本到产品环境",
    official: true,
  },
  {
    type: "sparkops-environment-sync",
    category: "deploy",
    label: "环境同步",
    title: "环境部署同步",
    description: "将源环境中的项目部署同步到目标环境",
    official: true,
  },
  {
    type: "sparkops-approval",
    category: "flow",
    label: "审批",
    title: "人工审批",
    description: "审批通过后才可执行后续操作",
    official: true,
  },
  {
    type: "freestyle",
    category: "tools",
    label: "自由脚本",
    title: "自由脚本",
    description: "自定义脚本步骤",
    official: true,
  },
  {
    type: "notify",
    category: "tools",
    label: "通知",
    title: "通知",
    description: "发送通知消息",
    official: true,
  },
] as const;
