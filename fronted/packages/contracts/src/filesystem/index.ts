import { z } from "zod";

export const TransferStatusSchema = z.enum([
  "INITIATED",
  "UPLOADING",
  "PAUSED",
  "VERIFYING",
  "SCANNING",
  "COMMITTING",
  "COMPLETED",
  "FAILED",
  "REJECTED",
  "ABORTED",
  "EXPIRED",
]);
export type TransferStatus = z.infer<typeof TransferStatusSchema>;

export const TransferProtocolSchema = z.enum(["DIRECT", "MULTIPART"]);

export const TransferClassificationSchema = z.enum([
  "PUBLIC",
  "INTERNAL",
  "CONFIDENTIAL",
  "RESTRICTED",
]);

export const TransferPartVMSchema = z.object({
  part_number: z.number().int().positive(),
  offset: z.number().int().nonnegative(),
  size: z.number().int().positive(),
  etag: z.string().optional().default(""),
  checksum: z.string().optional(),
  upload_url: z.string().url().optional(),
});

export type TransferPartVM = z.infer<typeof TransferPartVMSchema>;

export const TransferSessionVMSchema = z.object({
  session_id: z.string().uuid(),
  workspace_id: z.string().uuid(),
  namespace_id: z.string().uuid(),
  target_parent_id: z.string().uuid().nullable().optional(),
  target_name: z.string(),
  status: TransferStatusSchema,
  protocol: TransferProtocolSchema,
  expected_size: z.number().int().nonnegative(),
  expected_sha256: z.string().optional(),
  part_size: z.number().int().nonnegative(),
  total_parts: z.number().int().nonnegative(),
  expires_at: z.string(),
  error_code: z.string().optional(),
  upload_url: z.string().url().optional(),
  capability_token: z.string().optional(),
  parts: z.array(TransferPartVMSchema).default([]),
});

export type TransferSessionVM = z.infer<typeof TransferSessionVMSchema>;

export const TransferSessionPageVMSchema = z.object({
  items: z.array(TransferSessionVMSchema),
  next_cursor: z.string().default(""),
});

export type TransferSessionPageVM = z.infer<typeof TransferSessionPageVMSchema>;

export const TransferListQuerySchema = z.object({
  status: TransferStatusSchema.optional(),
  cursor: z.string().max(256).optional(),
  limit: z.number().int().positive().max(100).optional(),
});

export type TransferListQuery = z.infer<typeof TransferListQuerySchema>;

export const CreateTransferCommandSchema = z.object({
  namespace_id: z.string().uuid().optional(),
  parent_id: z.string().uuid().optional(),
  name: z.string().min(1),
  protocol: TransferProtocolSchema,
  size: z.number().int().nonnegative(),
  sha256: z.string().optional(),
  classification: TransferClassificationSchema.optional().default("INTERNAL"),
  provider_id: z.string().uuid().optional(),
  expires_at: z.string().datetime().optional(),
});

export type CreateTransferCommand = z.infer<typeof CreateTransferCommandSchema>;

export const FileNodeTypeSchema = z.enum(["FILE", "DIRECTORY"]);
export const FileNodeStateSchema = z.enum([
  "ACTIVE",
  "TRASHED",
  "RETENTION_HOLD",
  "PURGE_PENDING",
  "PURGED",
]);

export const FileNodeVMSchema = z.object({
  id: z.string().uuid(),
  namespace_id: z.string().uuid(),
  parent_id: z.string().uuid().nullable(),
  type: FileNodeTypeSchema,
  name: z.string(),
  current_revision_id: z.string().uuid().nullable(),
  state: FileNodeStateSchema,
  owner_id: z.string().uuid(),
});

export type FileNodeVM = z.infer<typeof FileNodeVMSchema>;

export const FileNamespaceVMSchema = z.object({
  id: z.string().uuid(),
  workspace_id: z.string().uuid(),
  root_node_id: z.string().uuid().nullable(),
  status: z.string(),
});

export type FileNamespaceVM = z.infer<typeof FileNamespaceVMSchema>;

export const FileNodePageVMSchema = z.object({
  items: z.array(FileNodeVMSchema),
  next_cursor: z.string(),
});

export type FileNodePageVM = z.infer<typeof FileNodePageVMSchema>;

export const FileRevisionVMSchema = z.object({
  id: z.string().uuid(),
  node_id: z.string().uuid(),
  revision_no: z.number().int().positive(),
  size: z.number().int().nonnegative(),
  mime_type: z.string(),
  created_by: z.string().uuid(),
});

export type FileRevisionVM = z.infer<typeof FileRevisionVMSchema>;

export const FileRevisionPageVMSchema = z.object({
  items: z.array(FileRevisionVMSchema),
  next_cursor: z.string(),
});

export type FileRevisionPageVM = z.infer<typeof FileRevisionPageVMSchema>;

export const CreateDirectoryCommandSchema = z.object({
  namespace_id: z.string().uuid().optional(),
  parent_id: z.string().uuid().nullable().optional(),
  name: z.string().min(1),
});

export type CreateDirectoryCommand = z.infer<typeof CreateDirectoryCommandSchema>;

export const RenameFileNodeCommandSchema = z.object({
  name: z.string().min(1),
});

export type RenameFileNodeCommand = z.infer<typeof RenameFileNodeCommandSchema>;

/** Placeholder VMs for later G4 tasks; no upstream routes in G3 yet. */
export const FileShareVMSchema = z.object({
  id: z.string().uuid(),
  node_id: z.string().uuid(),
  expires_at: z.string().nullable(),
  max_downloads: z.number().int().nullable(),
});

export type FileShareVM = z.infer<typeof FileShareVMSchema>;

export const FileArtifactVMSchema = z.object({
  id: z.string().uuid(),
  file_id: z.string().uuid(),
  artifact_type: z.string(),
  producer_id: z.string().uuid(),
  run_id: z.string().uuid().optional(),
});

export type FileArtifactVM = z.infer<typeof FileArtifactVMSchema>;

export const StorageProviderTypeSchema = z.enum(["LOCAL", "NFS", "S3"]);
export type StorageProviderType = z.infer<typeof StorageProviderTypeSchema>;

export const StorageProviderVMSchema = z.object({
  id: z.string().uuid(),
  organization_id: z.string().uuid(),
  name: z.string(),
  provider_type: StorageProviderTypeSchema,
  endpoint: z.string().optional().default(""),
  secret_ref: z.string(),
  capabilities: z.record(z.unknown()).optional().default({}),
  config_version: z.number().int().nonnegative(),
  status: z.string(),
});

export type StorageProviderVM = z.infer<typeof StorageProviderVMSchema>;

export const StorageProviderPageVMSchema = z.object({
  items: z.array(StorageProviderVMSchema),
});

export type StorageProviderPageVM = z.infer<typeof StorageProviderPageVMSchema>;

export const RegisterStorageProviderCommandSchema = z.object({
  id: z.string().uuid().optional(),
  name: z.string().min(1),
  provider_type: StorageProviderTypeSchema,
  endpoint: z.string().optional(),
  secret_ref: z.string().min(1),
  capabilities: z.record(z.unknown()).optional(),
  status: z.string().optional(),
});

export type RegisterStorageProviderCommand = z.infer<typeof RegisterStorageProviderCommandSchema>;

export const StoragePoolMemberModeSchema = z.enum(["PRIMARY", "REPLICA", "ARCHIVE", "DRAINING"]);
export type StoragePoolMemberMode = z.infer<typeof StoragePoolMemberModeSchema>;

export const StoragePoolMemberVMSchema = z.object({
  provider_id: z.string().uuid(),
  mode: StoragePoolMemberModeSchema,
  priority: z.number().int(),
});

export type StoragePoolMemberVM = z.infer<typeof StoragePoolMemberVMSchema>;

export const StoragePoolStatusSchema = z.enum(["ACTIVE", "DISABLED"]);
export type StoragePoolStatus = z.infer<typeof StoragePoolStatusSchema>;

export const StoragePoolVMSchema = z.object({
  id: z.string().uuid(),
  name: z.string(),
  status: StoragePoolStatusSchema,
  placement_policy_version: z.number().int().nonnegative(),
  members: z.array(StoragePoolMemberVMSchema).default([]),
});

export type StoragePoolVM = z.infer<typeof StoragePoolVMSchema>;

export const StoragePoolPageVMSchema = z.object({
  items: z.array(StoragePoolVMSchema),
});

export type StoragePoolPageVM = z.infer<typeof StoragePoolPageVMSchema>;

export const CreateStoragePoolCommandSchema = z.object({
  name: z.string().min(1),
  members: z.array(StoragePoolMemberVMSchema).default([]),
});

export type CreateStoragePoolCommand = z.infer<typeof CreateStoragePoolCommandSchema>;

export const UpdateStorageProviderCommandSchema = z.object({
  name: z.string().min(1),
  endpoint: z.string().optional(),
  secret_ref: z.string().optional(),
  status: z.string().optional(),
});

export type UpdateStorageProviderCommand = z.infer<typeof UpdateStorageProviderCommandSchema>;

export const UpdateStoragePoolCommandSchema = z.object({
  name: z.string().min(1),
  status: StoragePoolStatusSchema.optional(),
  members: z.array(StoragePoolMemberVMSchema).default([]),
});

export type UpdateStoragePoolCommand = z.infer<typeof UpdateStoragePoolCommandSchema>;

export const StorageBindingScopeTypeSchema = z.enum([
  "ORGANIZATION",
  "WORKSPACE",
  "PROJECT",
  "USER",
]);

export type StorageBindingScopeType = z.infer<typeof StorageBindingScopeTypeSchema>;

export const StorageBindingVMSchema = z.object({
  id: z.string().uuid(),
  organization_id: z.string().uuid(),
  scope_type: StorageBindingScopeTypeSchema,
  scope_id: z.string().uuid(),
  pool_id: z.string().uuid(),
  priority: z.number().int(),
});

export type StorageBindingVM = z.infer<typeof StorageBindingVMSchema>;

export const StorageBindingPageVMSchema = z.object({
  items: z.array(StorageBindingVMSchema),
});

export type StorageBindingPageVM = z.infer<typeof StorageBindingPageVMSchema>;

export const CreateStorageBindingCommandSchema = z.object({
  scope_type: StorageBindingScopeTypeSchema,
  scope_id: z.string().uuid(),
  pool_id: z.string().uuid(),
  priority: z.number().int().optional().default(0),
});

export type CreateStorageBindingCommand = z.infer<typeof CreateStorageBindingCommandSchema>;

export const LifecyclePolicyStatusSchema = z.enum(["ACTIVE", "DISABLED"]);
export type LifecyclePolicyStatus = z.infer<typeof LifecyclePolicyStatusSchema>;

export const LifecyclePolicyVMSchema = z.object({
  id: z.string().uuid(),
  organization_id: z.string().uuid(),
  name: z.string(),
  version: z.number().int().positive(),
  rules: z.record(z.unknown()),
  status: LifecyclePolicyStatusSchema,
});

export type LifecyclePolicyVM = z.infer<typeof LifecyclePolicyVMSchema>;

export const LifecyclePolicyPageVMSchema = z.object({
  items: z.array(LifecyclePolicyVMSchema),
});

export type LifecyclePolicyPageVM = z.infer<typeof LifecyclePolicyPageVMSchema>;

export const UpsertLifecyclePolicyCommandSchema = z.object({
  name: z.string().min(1),
  version: z.number().int().positive().optional(),
  rules: z.record(z.unknown()).optional().default({}),
  status: LifecyclePolicyStatusSchema.optional().default("ACTIVE"),
});

export type UpsertLifecyclePolicyCommand = z.infer<typeof UpsertLifecyclePolicyCommandSchema>;

export const LifecycleRuleActionSchema = z.enum([
  "replicate",
  "archive",
  "trash",
  "purge",
  "rescan",
]);

export type LifecycleRuleAction = z.infer<typeof LifecycleRuleActionSchema>;

export const FileAuditEventVMSchema = z.object({
  id: z.string().uuid(),
  actor_id: z.string().uuid(),
  action: z.string(),
  resource_type: z.string(),
  resource_id: z.string(),
  result: z.string(),
  trace_id: z.string().optional(),
  created_at: z.string(),
  detail: z.record(z.unknown()).optional().default({}),
});

export type FileAuditEventVM = z.infer<typeof FileAuditEventVMSchema>;

export const FileAuditEventPageVMSchema = z.object({
  items: z.array(FileAuditEventVMSchema),
  next_cursor: z.string().optional().default(""),
});

export type FileAuditEventPageVM = z.infer<typeof FileAuditEventPageVMSchema>;

export const FileAuditQuerySchema = z.object({
  actor_id: z.string().uuid().optional(),
  action: z.string().optional(),
  resource_id: z.string().optional(),
  result: z.string().optional(),
  from: z.string().datetime().optional(),
  to: z.string().datetime().optional(),
  cursor: z.string().optional(),
  limit: z.number().int().positive().max(100).optional(),
});

export type FileAuditQuery = z.infer<typeof FileAuditQuerySchema>;

export const NamespaceTypeSchema = z.enum(["USER", "WORKSPACE", "ORGANIZATION"]);
export type NamespaceType = z.infer<typeof NamespaceTypeSchema>;

export const ResourceActionSchema = z.enum(["READ", "WRITE", "DELETE", "SHARE", "ADMIN"]);
export type ResourceAction = z.infer<typeof ResourceActionSchema>;

export const GovernedPermissionSchema = z.enum([
  "READ",
  "DOWNLOAD",
  "WRITE",
  "DELETE",
  "SHARE",
  "ADMIN",
]);
export type GovernedPermission = z.infer<typeof GovernedPermissionSchema>;

export const PrincipalTypeSchema = z.enum(["USER", "WORKSPACE", "ORGANIZATION", "ROLE"]);
export type PrincipalType = z.infer<typeof PrincipalTypeSchema>;

export const GrantEffectSchema = z.enum(["ALLOW", "DENY"]);
export type GrantEffect = z.infer<typeof GrantEffectSchema>;

export const NamespaceSummaryVMSchema = z.object({
  id: z.string().uuid(),
  namespace_type: NamespaceTypeSchema,
  organization_id: z.string().uuid(),
  workspace_id: z.string().uuid().nullable().optional(),
  owner_user_id: z.string().uuid().nullable().optional(),
  root_node_id: z.string().uuid().nullable().optional(),
  status: z.string(),
  capabilities: z.array(ResourceActionSchema),
  entry_node_ids: z.array(z.string().uuid()),
  entry_nodes_truncated: z.boolean(),
});
export type NamespaceSummaryVM = z.infer<typeof NamespaceSummaryVMSchema>;

export const NamespaceSummaryPageVMSchema = z.object({
  items: z.array(NamespaceSummaryVMSchema),
});
export type NamespaceSummaryPageVM = z.infer<typeof NamespaceSummaryPageVMSchema>;

export const GovernedGrantVMSchema = z.object({
  id: z.string().uuid(),
  node_id: z.string().uuid(),
  grantee_id: z.string().uuid(),
  principal_type: PrincipalTypeSchema,
  principal_id: z.string().uuid(),
  permission: GovernedPermissionSchema,
  effect: GrantEffectSchema,
  expires_at: z.string().datetime({ offset: true }).nullable().optional(),
});
export type GovernedGrantVM = z.infer<typeof GovernedGrantVMSchema>;

export const GovernedGrantPageVMSchema = z.object({
  items: z.array(GovernedGrantVMSchema),
});
export type GovernedGrantPageVM = z.infer<typeof GovernedGrantPageVMSchema>;

export const AuthorizationCacheKeyVMSchema = z.object({
  organization_id: z.string().uuid(),
  workspace_id: z.string().uuid().nullable(),
  actor_id: z.string().uuid(),
  node_id: z.string().uuid(),
  action: ResourceActionSchema,
  policy_version: z.number().int(),
});
export type AuthorizationCacheKeyVM = z.infer<typeof AuthorizationCacheKeyVMSchema>;

export const EffectiveAccessVMSchema = z.object({
  decision: z.enum(["ALLOW", "DENY"]),
  effective_actions: z.array(ResourceActionSchema),
  reason_code: z.string(),
  source_grant_ids: z.array(z.string().uuid()),
  policy_version: z.number().int(),
  cache_key: AuthorizationCacheKeyVMSchema,
});
export type EffectiveAccessVM = z.infer<typeof EffectiveAccessVMSchema>;

export const SharedItemVMSchema = z.object({
  node: FileNodeVMSchema,
  effective_actions: z.array(ResourceActionSchema),
  source_grant_ids: z.array(z.string().uuid()),
});
export type SharedItemVM = z.infer<typeof SharedItemVMSchema>;

export const SharedItemPageVMSchema = z.object({
  items: z.array(SharedItemVMSchema),
  next_cursor: z.string(),
});
export type SharedItemPageVM = z.infer<typeof SharedItemPageVMSchema>;

export const CreateGovernedGrantCommandSchema = z.object({
  principal_type: PrincipalTypeSchema,
  principal_ids: z.array(z.string().uuid()).min(1),
  actions: z.array(GovernedPermissionSchema).min(1),
  effect: GrantEffectSchema.optional(),
  inherit_to_children: z.boolean().optional(),
  expires_at: z.string().datetime({ offset: true }).nullable().optional(),
});
export type CreateGovernedGrantCommand = z.infer<typeof CreateGovernedGrantCommandSchema>;

export const ShareRequestStatusSchema = z.enum([
  "PENDING",
  "APPROVED",
  "REJECTED",
  "CANCELLED",
]);
export type ShareRequestStatus = z.infer<typeof ShareRequestStatusSchema>;

export const ShareRequestVMSchema = z.object({
  id: z.string().uuid(),
  organization_id: z.string().uuid(),
  namespace_id: z.string().uuid(),
  node_id: z.string().uuid(),
  requested_by: z.string().uuid(),
  principal_type: PrincipalTypeSchema,
  principal_ids: z.array(z.string().uuid()),
  actions: z.array(GovernedPermissionSchema),
  inherit_to_children: z.boolean().optional(),
  expires_at: z.string().datetime({ offset: true }).nullable().optional(),
  classification: TransferClassificationSchema,
  effect: GrantEffectSchema,
  status: ShareRequestStatusSchema,
  reviewed_by: z.string().uuid().nullable().optional(),
  reviewed_at: z.string().datetime({ offset: true }).nullable().optional(),
  rejection_reason: z.string().optional(),
  created_at: z.string().datetime({ offset: true }),
});
export type ShareRequestVM = z.infer<typeof ShareRequestVMSchema>;

export const GovernedShareResultVMSchema = z.object({
  request: ShareRequestVMSchema,
  grants: z.array(GovernedGrantVMSchema),
});
export type GovernedShareResultVM = z.infer<typeof GovernedShareResultVMSchema>;

export const GovernedShareSubmissionVMSchema = z.union([
  GovernedShareResultVMSchema,
  ShareRequestVMSchema,
]);
export type GovernedShareSubmissionVM = z.infer<typeof GovernedShareSubmissionVMSchema>;

export const GovernedChildrenQuerySchema = z.object({
  parent_id: z.string().uuid().optional(),
  type: FileNodeTypeSchema.optional(),
  q: z.string().max(200).optional(),
  cursor: z.string().uuid().max(128).optional(),
  limit: z.number().int().positive().max(200).optional(),
});
export type GovernedChildrenQuery = z.infer<typeof GovernedChildrenQuerySchema>;

export const SharedWithMeQuerySchema = z.object({
  q: z.string().max(200).optional(),
  cursor: z.string().max(512).optional(),
  limit: z.number().int().positive().max(200).optional(),
});
export type SharedWithMeQuery = z.infer<typeof SharedWithMeQuerySchema>;
