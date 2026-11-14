import type { BffContext } from "@sparkops/bff-core";
import type {
  CreateDirectoryCommand,
  CreateStorageBindingCommand,
  CreateStoragePoolCommand,
  CreateTransferCommand,
  FileAuditEventPageVM,
  FileAuditQuery,
  FileNamespaceVM,
  FileNodePageVM,
  FileNodeVM,
  FileRevisionPageVM,
  LifecyclePolicyPageVM,
  RegisterStorageProviderCommand,
  RenameFileNodeCommand,
  StorageBindingPageVM,
  StoragePoolPageVM,
  StorageProviderPageVM,
  TransferListQuery,
  TransferSessionPageVM,
  TransferSessionVM,
  UpsertLifecyclePolicyCommand,
} from "@sparkops/contracts/filesystem";
import {
  FileAuditEventPageVMSchema,
  FileNamespaceVMSchema,
  FileNodePageVMSchema,
  FileNodeVMSchema,
  FileRevisionPageVMSchema,
  LifecyclePolicyPageVMSchema,
  StorageBindingPageVMSchema,
  StoragePoolPageVMSchema,
  StorageProviderPageVMSchema,
  TransferSessionPageVMSchema,
  TransferSessionVMSchema,
} from "@sparkops/contracts/filesystem";

function mapTransfer(data: TransferSessionVM): TransferSessionVM {
  return TransferSessionVMSchema.parse(data);
}

function mapNamespace(data: FileNamespaceVM): FileNamespaceVM {
  return FileNamespaceVMSchema.parse(data);
}

function mapNodePage(data: FileNodePageVM): FileNodePageVM {
  return FileNodePageVMSchema.parse(data);
}

function mapNode(data: FileNodeVM): FileNodeVM {
  return FileNodeVMSchema.parse(data);
}

function mapRevisionPage(data: FileRevisionPageVM): FileRevisionPageVM {
  return FileRevisionPageVMSchema.parse(data);
}

export async function getFileNamespace(
  ctx: BffContext,
  workspaceId: string,
): Promise<FileNamespaceVM> {
  const data = await ctx.upstream.filesystem.getNamespace(workspaceId);
  return mapNamespace(data);
}

export async function listFileNodes(
  ctx: BffContext,
  workspaceId: string,
  query: { parent_id?: string | null; cursor?: string; limit?: number } = {},
): Promise<FileNodePageVM> {
  const data = await ctx.upstream.filesystem.listFiles(workspaceId, query);
  return mapNodePage(data);
}

export async function getFileNode(
  ctx: BffContext,
  workspaceId: string,
  nodeId: string,
): Promise<FileNodeVM> {
  const data = await ctx.upstream.filesystem.getFileNode(workspaceId, nodeId);
  return mapNode(data);
}

export async function createDirectoryNode(
  ctx: BffContext,
  workspaceId: string,
  input: CreateDirectoryCommand,
  idempotencyKey: string,
): Promise<FileNodeVM> {
  const data = await ctx.upstream.filesystem.createDirectory(workspaceId, input, idempotencyKey);
  return mapNode(data);
}

export async function renameFileNode(
  ctx: BffContext,
  workspaceId: string,
  nodeId: string,
  input: RenameFileNodeCommand,
  idempotencyKey: string,
): Promise<void> {
  await ctx.upstream.filesystem.renameFileNode(workspaceId, nodeId, input, idempotencyKey);
}

export async function listFileRevisions(
  ctx: BffContext,
  workspaceId: string,
  nodeId: string,
  query: { cursor?: string; limit?: number } = {},
): Promise<FileRevisionPageVM> {
  const data = await ctx.upstream.filesystem.listRevisions(workspaceId, nodeId, query);
  return mapRevisionPage(data);
}

export async function createTransferSession(
  ctx: BffContext,
  workspaceId: string,
  input: CreateTransferCommand,
  idempotencyKey: string,
): Promise<TransferSessionVM> {
  const data = await ctx.upstream.filesystem.createTransfer(workspaceId, input, idempotencyKey);
  return mapTransfer(data);
}

export async function listTransferSessions(
  ctx: BffContext,
  workspaceId: string,
  query: TransferListQuery = {},
): Promise<TransferSessionPageVM> {
  const data = await ctx.upstream.filesystem.listTransfers(workspaceId, query);
  return TransferSessionPageVMSchema.parse(data);
}

export async function getTransferSession(
  ctx: BffContext,
  workspaceId: string,
  sessionId: string,
): Promise<TransferSessionVM> {
  const data = await ctx.upstream.filesystem.getTransfer(workspaceId, sessionId);
  return mapTransfer(data);
}

export async function abortTransferSession(
  ctx: BffContext,
  workspaceId: string,
  sessionId: string,
  idempotencyKey: string,
): Promise<void> {
  await ctx.upstream.filesystem.abortTransfer(workspaceId, sessionId, idempotencyKey);
}

export async function listStorageProviders(
  ctx: BffContext,
  workspaceId: string,
): Promise<StorageProviderPageVM> {
  const data = await ctx.upstream.filesystem.listStorageProviders(workspaceId);
  return StorageProviderPageVMSchema.parse(data);
}

export async function registerStorageProvider(
  ctx: BffContext,
  workspaceId: string,
  input: RegisterStorageProviderCommand,
  idempotencyKey: string,
): Promise<{ id: string }> {
  return ctx.upstream.filesystem.registerStorageProvider(workspaceId, input, idempotencyKey);
}

export async function listStoragePools(
  ctx: BffContext,
  workspaceId: string,
): Promise<StoragePoolPageVM> {
  const data = await ctx.upstream.filesystem.listStoragePools(workspaceId);
  return StoragePoolPageVMSchema.parse(data);
}

export async function createStoragePool(
  ctx: BffContext,
  workspaceId: string,
  input: CreateStoragePoolCommand,
  idempotencyKey: string,
): Promise<{ id: string; name: string }> {
  return ctx.upstream.filesystem.createStoragePool(workspaceId, input, idempotencyKey);
}

export async function listStorageBindings(
  ctx: BffContext,
  workspaceId: string,
): Promise<StorageBindingPageVM> {
  const data = await ctx.upstream.filesystem.listStorageBindings(workspaceId);
  return StorageBindingPageVMSchema.parse(data);
}

export async function createStorageBinding(
  ctx: BffContext,
  workspaceId: string,
  input: CreateStorageBindingCommand,
  idempotencyKey: string,
): Promise<{ id: string }> {
  return ctx.upstream.filesystem.createStorageBinding(workspaceId, input, idempotencyKey);
}

export async function listLifecyclePolicies(
  ctx: BffContext,
  workspaceId: string,
): Promise<LifecyclePolicyPageVM> {
  const data = await ctx.upstream.filesystem.listLifecyclePolicies(workspaceId);
  return LifecyclePolicyPageVMSchema.parse(data);
}

export async function upsertLifecyclePolicy(
  ctx: BffContext,
  workspaceId: string,
  input: UpsertLifecyclePolicyCommand,
  idempotencyKey: string,
): Promise<{ id: string; version: number }> {
  return ctx.upstream.filesystem.upsertLifecyclePolicy(workspaceId, input, idempotencyKey);
}

export async function listFileAuditEvents(
  ctx: BffContext,
  workspaceId: string,
  query: FileAuditQuery = {},
): Promise<FileAuditEventPageVM> {
  const data = await ctx.upstream.filesystem.listFileAuditEvents(workspaceId, query);
  return FileAuditEventPageVMSchema.parse(data);
}
