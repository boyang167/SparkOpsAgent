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
