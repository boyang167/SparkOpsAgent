import type { BffContext } from "@sparkops/bff-core";
import type {
  CreateDirectoryCommand,
  CreateTransferCommand,
  FileNamespaceVM,
  FileNodePageVM,
  FileNodeVM,
  FileRevisionPageVM,
  RenameFileNodeCommand,
  TransferSessionVM,
} from "@sparkops/contracts/filesystem";
import {
  FileNamespaceVMSchema,
  FileNodePageVMSchema,
  FileNodeVMSchema,
  FileRevisionPageVMSchema,
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
