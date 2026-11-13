import type { BffContext } from "@sparkops/bff-core";
import type {
  CreateTransferCommand,
  TransferSessionVM,
} from "@sparkops/contracts/filesystem";
import { TransferSessionVMSchema } from "@sparkops/contracts/filesystem";

function mapTransfer(data: TransferSessionVM): TransferSessionVM {
  return TransferSessionVMSchema.parse(data);
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
