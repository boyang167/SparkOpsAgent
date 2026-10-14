import { z } from "zod";

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

export const SprintStatusSchema = z.enum(["planning", "active", "completed", "archived"]);
export type SprintStatus = z.infer<typeof SprintStatusSchema>;

export const WorkItemTypeSchema = z.enum(["story", "task", "bug", "epic", "release", "custom"]);
export type WorkItemType = z.infer<typeof WorkItemTypeSchema>;

export const WorkItemPrioritySchema = z.enum(["critical", "high", "medium", "low"]);
export type WorkItemPriority = z.infer<typeof WorkItemPrioritySchema>;

export const WorkItemStatusSchema = z.enum(["open", "in_progress", "blocked", "done", "cancelled"]);
export type WorkItemStatus = z.infer<typeof WorkItemStatusSchema>;

export const StageWorkflowTriggerModeSchema = z.enum(["manual", "auto_on_enter"]);
export type StageWorkflowTriggerMode = z.infer<typeof StageWorkflowTriggerModeSchema>;

export const PmProviderSchema = z.enum(["jira", "tapd", "pingcode", "feishu"]);
export type PmProvider = z.infer<typeof PmProviderSchema>;

export const WorkItemSyncModeSchema = z.enum(["link_only", "bidirectional"]);
export type WorkItemSyncMode = z.infer<typeof WorkItemSyncModeSchema>;

export const WorkItemExecutionStatusSchema = z.enum([
  "running",
  "success",
  "failed",
  "cancelled",
]);
export type WorkItemExecutionStatus = z.infer<typeof WorkItemExecutionStatusSchema>;

// ---------------------------------------------------------------------------
// Sprint Process (流程模板)
// ---------------------------------------------------------------------------

export const StageWorkflowBindingSchema = z.object({
  workflow_id: z.string().uuid(),
  workflow_name: z.string().optional(),
  trigger_mode: StageWorkflowTriggerModeSchema.default("manual"),
  default_parameters: z.record(z.string()).optional().default({}),
  required_for_exit: z.boolean().optional().default(false),
});

export type StageWorkflowBinding = z.infer<typeof StageWorkflowBindingSchema>;

export const StageGatePolicySchema = z.object({
  require_branch: z.boolean().optional().default(false),
  require_workflow_success: z.array(z.string().uuid()).optional().default([]),
  require_jira_status: z.array(z.string()).optional().default([]),
  require_approval: z.boolean().optional().default(false),
  protected_branches: z.array(z.string()).optional().default([]),
});

export type StageGatePolicy = z.infer<typeof StageGatePolicySchema>;

export const SprintStageSchema = z.object({
  id: z.string().uuid(),
  key: z.string().min(1).max(64),
  name: z.string().min(1).max(128),
  sort_order: z.number().int().nonnegative(),
  color: z.string().optional(),
  wip_limit: z.number().int().positive().optional(),
  entry_policy: StageGatePolicySchema.optional(),
  workflows: z.array(StageWorkflowBindingSchema).default([]),
});

export type SprintStage = z.infer<typeof SprintStageSchema>;

export const SprintProcessVMSchema = z.object({
  id: z.string().uuid(),
  product_id: z.string().uuid(),
  name: z.string(),
  template_key: z.string().optional(),
  description: z.string().optional(),
  version: z.number().int().positive(),
  is_default: z.boolean().default(true),
  stages: z.array(SprintStageSchema).default([]),
  created_by: z.string().uuid().optional(),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
});

export type SprintProcessVM = z.infer<typeof SprintProcessVMSchema>;

export const WorkspaceSprintProcessTemplateVMSchema = z.object({
  id: z.string().uuid(),
  workspace_id: z.string().uuid(),
  name: z.string(),
  template_key: z.string().optional(),
  description: z.string().optional(),
  stages: z.array(SprintStageSchema).default([]),
  created_by: z.string().uuid().optional(),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
});

export type WorkspaceSprintProcessTemplateVM = z.infer<typeof WorkspaceSprintProcessTemplateVMSchema>;

export const UpsertSprintProcessCommandSchema = z.object({
  name: z.string().min(1).max(255),
  stages: z.array(
    z.object({
      id: z.string().uuid().optional(),
      key: z.string().min(1).max(64),
      name: z.string().min(1).max(128),
      sort_order: z.number().int().nonnegative(),
      color: z.string().optional(),
      wip_limit: z.number().int().positive().optional(),
      entry_policy: StageGatePolicySchema.optional(),
      workflows: z.array(StageWorkflowBindingSchema).default([]),
    }),
  ).min(1),
});

export type UpsertSprintProcessCommand = z.infer<typeof UpsertSprintProcessCommandSchema>;

export const CreateSprintProcessCommandSchema = UpsertSprintProcessCommandSchema.extend({
  is_default: z.boolean().optional(),
});

export type CreateSprintProcessCommand = z.infer<typeof CreateSprintProcessCommandSchema>;

export const CreateWorkspaceSprintProcessTemplateCommandSchema = UpsertSprintProcessCommandSchema;
export type CreateWorkspaceSprintProcessTemplateCommand = z.infer<
  typeof CreateWorkspaceSprintProcessTemplateCommandSchema
>;

export const UpdateWorkspaceSprintProcessTemplateCommandSchema = UpsertSprintProcessCommandSchema;
export type UpdateWorkspaceSprintProcessTemplateCommand = z.infer<
  typeof UpdateWorkspaceSprintProcessTemplateCommandSchema
>;

export const ImportWorkspaceSprintTemplateCommandSchema = z.object({
  workspace_template_id: z.string().uuid(),
  name: z.string().optional(),
});

export type ImportWorkspaceSprintTemplateCommand = z.infer<typeof ImportWorkspaceSprintTemplateCommandSchema>;

// ---------------------------------------------------------------------------
// External PM references
// ---------------------------------------------------------------------------

export const SprintExternalRefSchema = z.object({
  provider: PmProviderSchema,
  project_key: z.string().optional(),
  sprint_id: z.string().optional(),
  board_id: z.string().optional(),
  sprint_url: z.string().url().optional(),
});

export type SprintExternalRef = z.infer<typeof SprintExternalRefSchema>;

export const WorkItemExternalRefSchema = z.object({
  provider: PmProviderSchema,
  issue_key: z.string(),
  issue_id: z.string().optional(),
  issue_type: z.string().optional(),
  issue_url: z.string().url().optional(),
  pm_workspace_id: z.string().optional(),
  sync_mode: WorkItemSyncModeSchema.default("link_only"),
});

export type WorkItemExternalRef = z.infer<typeof WorkItemExternalRefSchema>;

// ---------------------------------------------------------------------------
// Sprint (迭代实例)
// ---------------------------------------------------------------------------

export const SprintSummaryVMSchema = z.object({
  id: z.string().uuid(),
  product_id: z.string().uuid(),
  name: z.string(),
  goal: z.string().optional(),
  status: SprintStatusSchema,
  planned_start: z.string().datetime().optional(),
  planned_end: z.string().datetime().optional(),
  actual_start: z.string().datetime().optional(),
  actual_end: z.string().datetime().optional(),
  owner_id: z.string().uuid().optional(),
  owner_name: z.string().optional(),
  process_version: z.number().int().positive(),
  work_item_count: z.number().int().nonnegative().optional(),
  done_work_item_count: z.number().int().nonnegative().optional(),
  external: SprintExternalRefSchema.optional(),
  legacy_plan_id: z.string().uuid().optional(),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
});

export type SprintSummaryVM = z.infer<typeof SprintSummaryVMSchema>;

export const SprintVMSchema = SprintSummaryVMSchema.extend({
  process_id: z.string().uuid(),
  stages: z.array(SprintStageSchema).default([]),
});

export type SprintVM = z.infer<typeof SprintVMSchema>;

export const CreateSprintCommandSchema = z.object({
  name: z.string().min(1).max(255),
  goal: z.string().max(2000).optional(),
  planned_start: z.string().datetime().optional(),
  planned_end: z.string().datetime().optional(),
  owner_id: z.string().uuid().optional(),
  process_id: z.string().uuid().optional(),
  external: SprintExternalRefSchema.optional(),
});

export type CreateSprintCommand = z.infer<typeof CreateSprintCommandSchema>;

export const UpdateSprintCommandSchema = z.object({
  name: z.string().min(1).max(255).optional(),
  goal: z.string().max(2000).optional(),
  planned_start: z.string().datetime().optional(),
  planned_end: z.string().datetime().optional(),
  owner_id: z.string().uuid().optional(),
  external: SprintExternalRefSchema.optional(),
  stages: z.array(SprintStageSchema).optional(),
  stage_migrations: z.record(z.string().uuid()).optional(),
});

export type UpdateSprintCommand = z.infer<typeof UpdateSprintCommandSchema>;

// ---------------------------------------------------------------------------
// Work Item (工作项)
// ---------------------------------------------------------------------------

export const WorkItemDeliverySchema = z.object({
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  image_tags: z.record(z.string()).optional(),
  last_workflow_run_id: z.string().uuid().optional(),
  last_workflow_status: z.string().optional(),
});

export type WorkItemDelivery = z.infer<typeof WorkItemDeliverySchema>;

export const WorkItemCardVMSchema = z.object({
  id: z.string().uuid(),
  sprint_id: z.string().uuid(),
  stage_id: z.string().uuid(),
  parent_id: z.string().uuid().optional(),
  sort_order: z.number().int().nonnegative(),
  type: WorkItemTypeSchema,
  title: z.string(),
  priority: WorkItemPrioritySchema,
  status: WorkItemStatusSchema,
  assignee_id: z.string().uuid().optional(),
  assignee_name: z.string().optional(),
  project_ids: z.array(z.string().uuid()).default([]),
  labels: z.array(z.string()).default([]),
  external: WorkItemExternalRefSchema.optional(),
  delivery: WorkItemDeliverySchema.optional(),
  blocked_reason: z.string().optional(),
  version: z.number().int().nonnegative().default(0),
  updated_at: z.string().datetime().optional(),
});

export type WorkItemCardVM = z.infer<typeof WorkItemCardVMSchema>;

export const WorkItemVMSchema = WorkItemCardVMSchema.extend({
  description: z.string().optional(),
  reporter_id: z.string().uuid().optional(),
  reporter_name: z.string().optional(),
  created_by: z.string().uuid().optional(),
  created_at: z.string().datetime().optional(),
});

export type WorkItemVM = z.infer<typeof WorkItemVMSchema>;

export const CreateWorkItemCommandSchema = z.object({
  stage_id: z.string().uuid(),
  type: WorkItemTypeSchema.default("story"),
  title: z.string().min(1).max(512),
  description: z.string().max(10000).optional(),
  priority: WorkItemPrioritySchema.default("medium"),
  assignee_id: z.string().uuid().optional(),
  project_ids: z.array(z.string().uuid()).default([]),
  labels: z.array(z.string()).default([]),
  parent_id: z.string().uuid().optional(),
  external: WorkItemExternalRefSchema.optional(),
  sort_order: z.number().int().nonnegative().optional(),
});

export type CreateWorkItemCommand = z.infer<typeof CreateWorkItemCommandSchema>;

export const UpdateWorkItemCommandSchema = z.object({
  title: z.string().min(1).max(512).optional(),
  description: z.string().max(10000).optional(),
  type: WorkItemTypeSchema.optional(),
  priority: WorkItemPrioritySchema.optional(),
  status: WorkItemStatusSchema.optional(),
  assignee_id: z.string().uuid().nullable().optional(),
  clear_assignee: z.boolean().optional(),
  project_ids: z.array(z.string().uuid()).optional(),
  labels: z.array(z.string()).optional(),
  blocked_reason: z.string().nullable().optional(),
  external: WorkItemExternalRefSchema.optional(),
});

export type UpdateWorkItemCommand = z.infer<typeof UpdateWorkItemCommandSchema>;

export const MoveWorkItemCommandSchema = z.object({
  stage_id: z.string().uuid(),
  sort_order: z.number().int().nonnegative(),
  version: z.number().int().nonnegative(),
});

export type MoveWorkItemCommand = z.infer<typeof MoveWorkItemCommandSchema>;

export const ExecuteWorkItemCommandSchema = z.object({
  workflow_id: z.string().uuid().optional(),
  stage_id: z.string().uuid().optional(),
  input: z
    .object({
      branch: z.string().optional(),
      branch_overrides: z.record(z.string()).optional(),
      ref_type_overrides: z.record(z.string()).optional(),
      environment_overrides: z.record(z.string()).optional(),
      variables: z.record(z.string()).optional(),
      remark: z.string().optional(),
      stage_indices: z.array(z.number().int().nonnegative()).optional(),
    })
    .optional(),
});

export type ExecuteWorkItemCommand = z.infer<typeof ExecuteWorkItemCommandSchema>;

export const LinkJiraIssueCommandSchema = z.object({
  issue_key: z.string().min(1).max(64),
  sync_mode: WorkItemSyncModeSchema.default("link_only"),
});

export type LinkJiraIssueCommand = z.infer<typeof LinkJiraIssueCommandSchema>;

// ---------------------------------------------------------------------------
// Board (看板聚合)
// ---------------------------------------------------------------------------

export const SprintBoardColumnVMSchema = z.object({
  stage: SprintStageSchema,
  work_items: z.array(WorkItemCardVMSchema).default([]),
  wip_count: z.number().int().nonnegative(),
  wip_limit_exceeded: z.boolean().optional(),
});

export type SprintBoardColumnVM = z.infer<typeof SprintBoardColumnVMSchema>;

export const SprintBoardVMSchema = z.object({
  sprint: SprintVMSchema,
  columns: z.array(SprintBoardColumnVMSchema).default([]),
});

export type SprintBoardVM = z.infer<typeof SprintBoardVMSchema>;

// ---------------------------------------------------------------------------
// Execution & Activity
// ---------------------------------------------------------------------------

export const WorkItemExecutionArtifactsSchema = z.object({
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  images: z.array(z.object({ project_id: z.string(), tag: z.string() })).optional(),
  deploy_envs: z.array(z.string()).optional(),
  logs_url: z.string().url().optional(),
});

export type WorkItemExecutionArtifacts = z.infer<typeof WorkItemExecutionArtifactsSchema>;

export const WorkItemExecutionVMSchema = z.object({
  id: z.string().uuid(),
  work_item_id: z.string().uuid(),
  stage_id: z.string().uuid(),
  workflow_id: z.string().uuid(),
  workflow_name: z.string().optional(),
  workflow_run_id: z.string().uuid(),
  triggered_by: z.string().uuid(),
  triggered_by_name: z.string().optional(),
  status: WorkItemExecutionStatusSchema,
  artifacts: WorkItemExecutionArtifactsSchema.optional(),
  error_message: z.string().optional(),
  started_at: z.string().datetime(),
  finished_at: z.string().datetime().optional(),
});

export type WorkItemExecutionVM = z.infer<typeof WorkItemExecutionVMSchema>;

export const WorkItemActivityVMSchema = z.object({
  id: z.string().uuid(),
  work_item_id: z.string().uuid(),
  actor_id: z.string().uuid(),
  actor_name: z.string().optional(),
  action: z.enum([
    "created",
    "updated",
    "moved_stage",
    "assigned",
    "linked_jira",
    "workflow_started",
    "workflow_completed",
    "workflow_failed",
    "comment",
    "blocked",
    "unblocked",
  ]),
  detail: z.record(z.unknown()).optional(),
  created_at: z.string().datetime(),
});

export type WorkItemActivityVM = z.infer<typeof WorkItemActivityVMSchema>;

// ---------------------------------------------------------------------------
// Metrics & Import
// ---------------------------------------------------------------------------

export const SprintMetricsVMSchema = z.object({
  sprint_id: z.string().uuid(),
  total_work_items: z.number().int().nonnegative(),
  done_work_items: z.number().int().nonnegative(),
  blocked_work_items: z.number().int().nonnegative(),
  burndown: z.array(
    z.object({
      date: z.string(),
      remaining: z.number().int().nonnegative(),
    }),
  ).optional(),
  stage_cycle_time_hours: z.record(z.number()).optional(),
  deployment_frequency: z.number().optional(),
  change_failure_rate: z.number().optional(),
});

export type SprintMetricsVM = z.infer<typeof SprintMetricsVMSchema>;

export const ImportJiraSprintCommandSchema = z.object({
  jira_sprint_id: z.string().min(1),
  jira_board_id: z.string().optional(),
  target_stage_id: z.string().uuid().optional(),
  issue_types: z.array(z.string()).optional(),
  sync_mode: WorkItemSyncModeSchema.default("link_only"),
});

export type ImportJiraSprintCommand = z.infer<typeof ImportJiraSprintCommandSchema>;

export const ImportJiraSprintResultSchema = z.object({
  sprint_id: z.string().uuid(),
  imported_count: z.number().int().nonnegative(),
  skipped_count: z.number().int().nonnegative(),
  work_item_ids: z.array(z.string().uuid()),
});

export type ImportJiraSprintResult = z.infer<typeof ImportJiraSprintResultSchema>;

export const ImportPmSprintCommandSchema = z.object({
  provider: PmProviderSchema,
  pm_workspace_id: z.string().optional(),
  iteration_id: z.string().min(1),
  target_stage_id: z.string().uuid().optional(),
  issue_types: z.array(z.string()).optional(),
  sync_mode: WorkItemSyncModeSchema.default("link_only"),
});

export type ImportPmSprintCommand = z.infer<typeof ImportPmSprintCommandSchema>;

export const ImportPmSprintResultSchema = ImportJiraSprintResultSchema;
export type ImportPmSprintResult = z.infer<typeof ImportPmSprintResultSchema>;

export const LinkPmIssueCommandSchema = z.object({
  provider: PmProviderSchema,
  pm_workspace_id: z.string().optional(),
  issue_key: z.string().min(1).max(64),
  sync_mode: WorkItemSyncModeSchema.default("link_only"),
});

export type LinkPmIssueCommand = z.infer<typeof LinkPmIssueCommandSchema>;

export const SprintBoardWsConfigSchema = z.object({
  websocket_path: z.string(),
  access_token: z.string(),
  workspace_id: z.string().uuid(),
});

export type SprintBoardWsConfig = z.infer<typeof SprintBoardWsConfigSchema>;

export const SprintBoardWsMessageSchema = z.object({
  event: z.literal("board_changed"),
  payload: z.object({
    reason: z.string(),
  }),
});

export type SprintBoardWsMessage = z.infer<typeof SprintBoardWsMessageSchema>;

// ---------------------------------------------------------------------------
// List query params (BFF / client helpers)
// ---------------------------------------------------------------------------

export const ListSprintsQuerySchema = z.object({
  status: SprintStatusSchema.optional(),
  page: z.coerce.number().int().positive().optional().default(1),
  page_size: z.coerce.number().int().min(1).max(100).optional().default(20),
});

export type ListSprintsQuery = z.infer<typeof ListSprintsQuerySchema>;

export const ActiveSprintSummaryVMSchema = SprintSummaryVMSchema.extend({
  product_name: z.string().optional(),
});

export type ActiveSprintSummaryVM = z.infer<typeof ActiveSprintSummaryVMSchema>;
