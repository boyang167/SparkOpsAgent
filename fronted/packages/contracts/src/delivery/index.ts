import { z } from "zod";

export const BuildVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  status: z.string(),
  branch: z.string(),
  build_number: z.number(),
  jenkins_job: z.string().optional(),
  console_log: z.string().optional(),
  error: z.string().optional(),
  started_at: z.string(),
  completed_at: z.string().optional(),
  rollback_of_build_id: z.string().optional(),
});

export type BuildVM = z.infer<typeof BuildVMSchema>;

export const TriggerBuildCommandSchema = z.object({
  branch: z.string().optional().default("main"),
});

export type TriggerBuildCommand = z.infer<typeof TriggerBuildCommandSchema>;

export const DeploymentStatusSchema = z.enum([
  "queued",
  "preparing",
  "building",
  "deploying",
  "succeeded",
  "failed",
  "aborted",
]);

export type DeploymentStatus = z.infer<typeof DeploymentStatusSchema>;

export const DeploymentRunTypeSchema = z.enum(["normal", "rollback", "sync", "undeploy"]);

export type DeploymentRunType = z.infer<typeof DeploymentRunTypeSchema>;

export const DeploymentRunVMSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  project_id: z.string(),
  cluster_id: z.string(),
  git_ref_type: z.enum(["branch", "tag"]),
  git_ref_value: z.string(),
  image_tag: z.string(),
  image_repository: z.string().optional(),
  cross_cluster: z.boolean().optional().default(false),
  status: DeploymentStatusSchema,
  run_type: DeploymentRunTypeSchema.optional().default("normal"),
  rollback_of_run_id: z.string().optional(),
  deploy_namespace: z.string().optional(),
  undeploy_options: z
    .object({ delete_config: z.boolean(), delete_storage: z.boolean() })
    .optional(),
  build_id: z.string().optional(),
  error_message: z.string().default(""),
  current_stage: z.enum(["", "preparing", "building", "deploying"]),
  created_by: z.string(),
  created_at: z.string(),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
});

export type DeploymentRunVM = z.infer<typeof DeploymentRunVMSchema>;

export const DeploymentRunListVMSchema = z.object({
  items: z.array(DeploymentRunVMSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});

export type DeploymentRunListVM = z.infer<typeof DeploymentRunListVMSchema>;

export const SyncManifestKindSchema = z.enum([
  "Deployment",
  "Service",
  "ConfigMap",
  "Secret",
  "Ingress",
]);

export type SyncManifestKind = z.infer<typeof SyncManifestKindSchema>;

export const SyncManifestInputSchema = z.object({
  kind: SyncManifestKindSchema,
  name: z.string().min(1),
  yaml: z.string().optional(),
  managed_pull_secret: z.boolean().optional(),
});

export type SyncManifestInput = z.infer<typeof SyncManifestInputSchema>;

export const SyncPreviewItemVMSchema = z.object({
  id: z.string(),
  kind: SyncManifestKindSchema,
  name: z.string(),
  related_reason: z.string().optional().default(""),
  default_selected: z.boolean(),
  managed: z.boolean().optional().default(false),
  yaml: z.string().optional().default(""),
  note: z.string().optional().default(""),
});

export type SyncPreviewItemVM = z.infer<typeof SyncPreviewItemVMSchema>;

export const SyncPreviewVMSchema = z.object({
  build_cluster_id: z.string(),
  build_cluster_name: z.string(),
  namespace: z.string(),
  items: z.array(SyncPreviewItemVMSchema),
});

export type SyncPreviewVM = z.infer<typeof SyncPreviewVMSchema>;

export const RunPreviewItemVMSchema = z.object({
  kind: SyncManifestKindSchema,
  name: z.string(),
  rendered_yaml: z.string().default(""),
  current_yaml: z.string().default(""),
  current_found: z.boolean(),
});

export type RunPreviewItemVM = z.infer<typeof RunPreviewItemVMSchema>;

export const RunPreviewVMSchema = z.object({
  environment_id: z.string(),
  cluster_id: z.string().optional(),
  cluster_name: z.string(),
  namespace: z.string(),
  items: z.array(RunPreviewItemVMSchema),
});

export type RunPreviewVM = z.infer<typeof RunPreviewVMSchema>;

export const CreateDeploymentCommandSchema = z
  .object({
    cluster_id: z.string().optional(),
    environment_id: z.string().optional(),
    git_ref_type: z.enum(["branch", "tag"]).optional(),
    git_ref_value: z.string().optional(),
    image_tag: z.string().max(255).optional(),
    cross_cluster: z.boolean().optional(),
    sync_only: z.boolean().optional(),
    manifests: z.array(SyncManifestInputSchema).optional(),
  })
  .superRefine((value, ctx) => {
    if (value.sync_only) {
      if (!value.manifests || value.manifests.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "manifests are required for sync_only",
          path: ["manifests"],
        });
      }
      return;
    }
    if (!value.git_ref_type) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "git_ref_type is required",
        path: ["git_ref_type"],
      });
    }
    if (!value.git_ref_value?.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "git_ref_value is required",
        path: ["git_ref_value"],
      });
    }
  });

export type CreateDeploymentCommand = z.infer<typeof CreateDeploymentCommandSchema>;

export const RollbackDeploymentCommandSchema = z.object({
  target_run_id: z.string().optional(),
});

export type RollbackDeploymentCommand = z.infer<typeof RollbackDeploymentCommandSchema>;

export const RollbackDiffFieldSchema = z.object({
  label: z.string(),
  current: z.string(),
  target: z.string(),
  changed: z.boolean(),
});

export type RollbackDiffFieldVM = z.infer<typeof RollbackDiffFieldSchema>;

export const RollbackPreviewVMSchema = z.object({
  can_rollback: z.boolean(),
  reason: z.string().optional(),
  current_run: DeploymentRunVMSchema.optional(),
  target_run: DeploymentRunVMSchema.optional(),
  diff: z.array(RollbackDiffFieldSchema).optional(),
});

export type RollbackPreviewVM = z.infer<typeof RollbackPreviewVMSchema>;

export const AuthorizedClusterVMSchema = z.object({
  id: z.string(),
  cluster_name: z.string(),
  server: z.string().optional(),
  priority_id: z.number().int().nonnegative().optional().default(0),
  is_build_env: z.boolean().optional().default(false),
  available: z.boolean().optional().default(true),
});

export type AuthorizedClusterVM = z.infer<typeof AuthorizedClusterVMSchema>;

export const DeploymentOptionsVMSchema = z.object({
  clusters: z.array(AuthorizedClusterVMSchema),
  missing: z.array(z.string()),
});

export type DeploymentOptionsVM = z.infer<typeof DeploymentOptionsVMSchema>;

export const DeploymentEventVMSchema = z
  .object({
    delta: z.string().optional(),
    done: z.literal("true").optional(),
  })
  .refine((event) => Boolean(event.delta || event.done), {
    message: "deployment event requires delta or done",
  });

export type DeploymentEventVM = z.infer<typeof DeploymentEventVMSchema>;

export const CronJobVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  schedule: z.string(),
  branch: z.string().optional(),
  enabled: z.boolean().optional(),
  created_at: z.string().optional(),
});

export type CronJobVM = z.infer<typeof CronJobVMSchema>;

export const WEBHOOK_EVENTS = ["Build", "Deploy", "Rollback"] as const;
export type WebhookEventKey = (typeof WEBHOOK_EVENTS)[number];

export const ProjectWebhookVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  url: z.string(),
  token: z.string().optional(),
  events: z.array(z.string()),
  enabled: z.boolean(),
  created_at: z.string().optional(),
});

export type ProjectWebhookVM = z.infer<typeof ProjectWebhookVMSchema>;

export const WebhookEventCatalogItemSchema = z.object({
  key: z.string(),
  label: z.string(),
});

export type WebhookEventCatalogItem = z.infer<typeof WebhookEventCatalogItemSchema>;

export interface UpsertProjectWebhookInput {
  name: string;
  url: string;
  token?: string;
  events: string[];
  enabled: boolean;
}

export const UndeployCommandSchema = z.object({
  cluster_id: z.string().min(1),
  confirm_name: z.string().min(1),
  delete_config: z.boolean().default(false),
  delete_storage: z.boolean().default(false),
});

export type UndeployCommand = z.infer<typeof UndeployCommandSchema>;
