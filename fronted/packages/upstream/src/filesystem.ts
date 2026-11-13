import type { CreateTransferCommand, TransferSessionVM } from "@sparkops/contracts/filesystem";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createFilesystemUpstream(options: UpstreamRequestOptions = {}) {
  return {
    createTransfer(workspaceId: string, input: CreateTransferCommand, idempotencyKey: string) {
      return upstreamFetch<TransferSessionVM>(`/api/v1/workspaces/${workspaceId}/transfers`, {
        method: "POST",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": idempotencyKey },
        options: { ...options, workspaceId },
      });
    },
    getTransfer(workspaceId: string, sessionId: string) {
      return upstreamFetch<TransferSessionVM>(
        `/api/v1/workspaces/${workspaceId}/transfers/${sessionId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
    abortTransfer(workspaceId: string, sessionId: string, idempotencyKey: string) {
      return upstreamFetch<void>(`/api/v1/workspaces/${workspaceId}/transfers/${sessionId}`, {
        method: "DELETE",
        headers: { "Idempotency-Key": idempotencyKey },
        options: { ...options, workspaceId },
      });
    },
  };
}

export type FilesystemUpstream = ReturnType<typeof createFilesystemUpstream>;
