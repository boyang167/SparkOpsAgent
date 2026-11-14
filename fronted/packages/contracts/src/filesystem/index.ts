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
