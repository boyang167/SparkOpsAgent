import {
  getFileNode,
  getFileNamespace,
  listGovernedChildren,
  listGovernedNamespaces,
  listSharedWithMe,
  listFileNodes,
  listFileRevisions,
} from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import type {
  FileNodePageVM,
  FileNamespaceVM,
  FileNodeVM,
  FileRevisionPageVM,
  NamespaceSummaryPageVM,
  SharedItemPageVM,
} from "@sparkops/contracts/filesystem";
import { loadBffContext } from "./context";

export async function loadFileNamespace(workspaceId: string): Promise<FileNamespaceVM> {
  const ctx = await loadBffContext(workspaceId);
  return getFileNamespace(ctx, workspaceId);
}

export async function loadFileNodes(
  workspaceId: string,
  query: {
    parentId?: string | null;
    cursor?: string;
    query?: string;
    limit?: number;
  } = {},
): Promise<FileNodePageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listFileNodes(ctx, workspaceId, {
    parent_id: query.parentId ?? undefined,
    cursor: query.cursor,
    q: query.query,
    limit: query.limit,
  });
}

export async function loadAccessibleFileNamespaces(
  organizationId: string,
  workspaceId?: string,
): Promise<NamespaceSummaryPageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listGovernedNamespaces(ctx, organizationId, workspaceId);
}

export async function loadGovernedFileChildren(
  organizationId: string,
  workspaceId: string | undefined,
  namespaceId: string,
  query: {
    parentId?: string;
    cursor?: string;
    query?: string;
    limit?: number;
  } = {},
): Promise<FileNodePageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listGovernedChildren(ctx, organizationId, workspaceId, namespaceId, {
    parent_id: query.parentId,
    cursor: query.cursor,
    q: query.query,
    limit: query.limit,
  });
}

export async function loadSharedFileItems(
  organizationId: string,
  workspaceId: string | undefined,
  query: {
    source?: "organization" | "workspace" | "user" | "agent";
    cursor?: string;
    query?: string;
    limit?: number;
  } = {},
): Promise<SharedItemPageVM> {
  const ctx = await loadBffContext(workspaceId);
  return listSharedWithMe(ctx, organizationId, workspaceId, {
    source_type: query.source,
    cursor: query.cursor,
    q: query.query,
    limit: query.limit,
  });
}

export type FileSpaceLoad<T> =
  | { kind: "loaded"; value: T }
  | { kind: "forbidden" | "not-found" | "error" };

export async function resolveFileSpaceLoad<T>(
  load: () => Promise<T>,
): Promise<FileSpaceLoad<T>> {
  try {
    return { kind: "loaded", value: await load() };
  } catch (error) {
    if (error instanceof BffError && error.status === 403) return { kind: "forbidden" };
    if (error instanceof BffError && error.status === 404) return { kind: "not-found" };
    return { kind: "error" };
  }
}

export async function loadFileNode(workspaceId: string, nodeId: string): Promise<FileNodeVM> {
  const ctx = await loadBffContext(workspaceId);
  return getFileNode(ctx, workspaceId, nodeId);
}

export type FileNodeSelection = {
  node?: FileNodeVM;
  error?: "forbidden" | "not-found" | "error";
};

export function isInvalidFilesystemScopeError(error: unknown): boolean {
  if (!(error instanceof BffError)) return false;
  return (
    (error.status === 400 && error.code === "FILESYSTEM_INVALID_REQUEST") ||
    (error.status === 403 && error.code === "FILESYSTEM_RESOURCE_FORBIDDEN") ||
    (error.status === 404 && error.code === "FILESYSTEM_NOT_FOUND")
  );
}

export async function resolveFileNodeSelection(
  namespaceId: string,
  load: () => Promise<FileNodeVM>,
): Promise<FileNodeSelection> {
  try {
    const node = await load();
    return node.namespace_id === namespaceId
      ? { node }
      : { error: "not-found" };
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
