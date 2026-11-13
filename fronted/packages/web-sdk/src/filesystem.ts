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
import { bffFetch, type BffFetchOptions } from "./client";

export type TransferProgress = {
  uploadedBytes: number;
  totalBytes: number;
  completedParts: number;
  totalParts: number;
};

type UploadTransferOptions = {
  workspaceId: string;
  session?: TransferSessionVM;
  partSize?: number;
  idempotencyKey?: string;
  onProgress?: (progress: TransferProgress) => void;
  fetchImpl?: typeof fetch;
};

type UploadTransferResult = {
  session: TransferSessionVM;
  requestedParts: number[];
};

function defaultIdempotencyKey() {
  return `transfer-${Date.now()}`;
}

function mutationIdempotencyKey(prefix: string) {
  return `${prefix}-${Date.now()}`;
}

type ListFilesQuery = {
  parentId?: string | null;
  cursor?: string;
  limit?: number;
};

function listFilesQuery(query: ListFilesQuery = {}) {
  const params = new URLSearchParams();
  if (query.parentId) params.set("parent_id", query.parentId);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

export function getFileNamespace(workspaceId: string, options: BffFetchOptions = {}) {
  return bffFetch<FileNamespaceVM>(`/workspaces/${workspaceId}/namespaces`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function listFileNodes(
  workspaceId: string,
  query: ListFilesQuery = {},
  options: BffFetchOptions = {},
) {
  return bffFetch<FileNodePageVM>(`/workspaces/${workspaceId}/files${listFilesQuery(query)}`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function createDirectoryNode(
  workspaceId: string,
  input: CreateDirectoryCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("directory");
  return bffFetch<FileNodeVM>(`/workspaces/${workspaceId}/directories`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function renameFileNode(
  workspaceId: string,
  nodeId: string,
  input: RenameFileNodeCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("rename");
  return bffFetch<void>(`/workspaces/${workspaceId}/files/${nodeId}/name`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function listFileRevisions(
  workspaceId: string,
  nodeId: string,
  query: { cursor?: string; limit?: number } = {},
  options: BffFetchOptions = {},
) {
  const params = new URLSearchParams();
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  const qs = suffix ? `?${suffix}` : "";
  return bffFetch<FileRevisionPageVM>(
    `/workspaces/${workspaceId}/files/${nodeId}/revisions${qs}`,
    {
      method: "GET",
      workspaceId,
      ...options,
    },
  );
}

export function createTransferSession(
  workspaceId: string,
  input: CreateTransferCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? defaultIdempotencyKey();
  return bffFetch<TransferSessionVM>(`/workspaces/${workspaceId}/transfers`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function getTransferSession(sessionId: string, workspaceId: string) {
  return bffFetch<TransferSessionVM>(`/workspaces/${workspaceId}/transfers/${sessionId}`, {
    method: "GET",
    workspaceId,
  });
}

export function abortTransfer(
  sessionId: string,
  workspaceId: string,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? defaultIdempotencyKey();
  return bffFetch<void>(`/workspaces/${workspaceId}/transfers/${sessionId}`, {
    method: "DELETE",
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

function completedPartNumbers(session: TransferSessionVM): Set<number> {
  const numbers = new Set<number>();
  for (const part of session.parts) {
    if (part.etag) {
      numbers.add(part.part_number);
    }
  }
  return numbers;
}

function partUploadURL(session: TransferSessionVM, partNumber: number): string | undefined {
  const existing = session.parts.find((part) => part.part_number === partNumber);
  return existing?.upload_url;
}

export async function uploadTransferParts(
  file: Blob,
  session: TransferSessionVM,
  options: Pick<UploadTransferOptions, "partSize" | "onProgress" | "fetchImpl"> = {},
): Promise<UploadTransferResult> {
  const fetchImpl = options.fetchImpl ?? fetch;
  const partSize = options.partSize ?? session.part_size;
  const totalBytes = file.size;
  const totalParts =
    session.total_parts > 0 ? session.total_parts : Math.ceil(totalBytes / Math.max(partSize, 1));
  const completed = completedPartNumbers(session);
  const requestedParts: number[] = [];
  let uploadedBytes = 0;
  let completedParts = completed.size;

  for (let partNumber = 1; partNumber <= totalParts; partNumber++) {
    if (completed.has(partNumber)) {
      const done = session.parts.find((part) => part.part_number === partNumber);
      uploadedBytes += done?.size ?? 0;
      continue;
    }
    const offset = (partNumber - 1) * partSize;
    const size = Math.min(partSize, totalBytes - offset);
    if (size <= 0) {
      continue;
    }
    const uploadUrl = partUploadURL(session, partNumber);
    if (!uploadUrl) {
      throw new Error(`missing upload_url for part ${partNumber}`);
    }
    requestedParts.push(partNumber);
    const chunk = file.slice(offset, offset + size);
    await fetchImpl(uploadUrl, { method: "PUT", body: chunk });
    uploadedBytes += size;
    completedParts += 1;
    options.onProgress?.({
      uploadedBytes,
      totalBytes,
      completedParts,
      totalParts,
    });
  }

  return { session, requestedParts };
}

export async function uploadTransfer(
  file: Blob,
  options: UploadTransferOptions,
): Promise<UploadTransferResult> {
  if (!options.session) {
    throw new Error("transfer session required");
  }
  return uploadTransferParts(file, options.session, options);
}
