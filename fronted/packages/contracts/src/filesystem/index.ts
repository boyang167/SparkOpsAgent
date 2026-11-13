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

export const CreateTransferCommandSchema = z.object({
  namespace_id: z.string().uuid().optional(),
  parent_id: z.string().uuid().optional(),
  name: z.string().min(1),
  protocol: TransferProtocolSchema,
  size: z.number().int().nonnegative(),
  sha256: z.string().optional(),
  classification: TransferClassificationSchema.optional().default("INTERNAL"),
  provider_id: z.string().uuid(),
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

export const StoragePoolVMSchema = z.object({
  id: z.string().uuid(),
  name: z.string(),
  status: z.string(),
});

export type StoragePoolVM = z.infer<typeof StoragePoolVMSchema>;
