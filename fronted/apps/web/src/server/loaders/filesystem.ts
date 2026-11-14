import {
  getFileNode,
  getFileNamespace,
  listFileNodes,
  listFileRevisions,
} from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import type { FileNodePageVM, FileNamespaceVM, FileNodeVM, FileRevisionPageVM } from "@sparkops/contracts/filesystem";
import { loadBffContext } from "./context";

export async function loadFileNamespace(workspaceId: string): Promise<FileNamespaceVM> {
  const ctx = await loadBffContext(workspaceId);
  return getFileNamespace(ctx, workspaceId);
}

export async function loadFileNodes(
  workspaceId: string,
  parentId?: string | null,
): Promise<FileNodePageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listFileNodes(ctx, workspaceId, { parent_id: parentId ?? undefined });
}

export async function loadFileNode(workspaceId: string, nodeId: string): Promise<FileNodeVM> {
  const ctx = await loadBffContext(workspaceId);
  return getFileNode(ctx, workspaceId, nodeId);
}

export type FileNodeSelection = {
  node?: FileNodeVM;
  error?: "forbidden" | "not-found" | "error";
};

export async function resolveFileNodeSelection(
  load: () => Promise<FileNodeVM>,
): Promise<FileNodeSelection> {
  try {
    return { node: await load() };
  } catch (error) {
    if (error instanceof BffError && error.status === 403) return { error: "forbidden" };
    if (error instanceof BffError && error.status === 404) return { error: "not-found" };
    return { error: "error" };
  }
}

export async function loadFileRevisions(
  workspaceId: string,
  nodeId: string,
): Promise<FileRevisionPageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listFileRevisions(ctx, workspaceId, nodeId);
}
