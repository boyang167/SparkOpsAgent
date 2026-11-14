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
  UpdateStoragePoolCommand,
  UpdateStorageProviderCommand,
  TransferListQuery,
  TransferSessionPageVM,
  TransferSessionVM,
  UpsertLifecyclePolicyCommand,
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
  signal?: AbortSignal;
};

type UploadTransferResult = {
  session: TransferSessionVM;
  requestedParts: number[];
};

let idempotencySequence = 0;

function uniqueIdempotencySuffix() {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") {
    return cryptoApi.randomUUID();
  }
  if (typeof cryptoApi?.getRandomValues === "function") {
    const values = new Uint32Array(4);
    cryptoApi.getRandomValues(values);
    return [...values].map((value) => value.toString(36)).join("-");
  }
  idempotencySequence += 1;
  return `${Date.now().toString(36)}-${idempotencySequence.toString(36)}-${Math.random()
    .toString(36)
    .slice(2)}`;
}

function defaultIdempotencyKey() {
  return `transfer-${uniqueIdempotencySuffix()}`;
}

function mutationIdempotencyKey(prefix: string) {
  return `${prefix}-${uniqueIdempotencySuffix()}`;
}

type ListFilesQuery = {
  parentId?: string | null;
  cursor?: string;
  limit?: number;
  nodeType?: "FILE" | "DIRECTORY";
  query?: string;
};

function listFilesQuery(query: ListFilesQuery = {}) {
  const params = new URLSearchParams();
  if (query.parentId) params.set("parent_id", query.parentId);
  if (query.nodeType) params.set("type", query.nodeType);
  if (query.query) params.set("q", query.query);
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

export function getFileNode(
  workspaceId: string,
  nodeId: string,
  options: BffFetchOptions = {},
) {
  return bffFetch<FileNodeVM>(`/workspaces/${workspaceId}/files/${nodeId}`, {
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

export function moveFileNode(
  workspaceId: string,
  nodeId: string,
  targetParentId: string,
  nodeName: string,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("move");
  return bffFetch<void>(`/workspaces/${workspaceId}/files/${nodeId}/move`, {
    ...options,
    method: "POST",
    body: JSON.stringify({ parent_id: targetParentId, name: nodeName }),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
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

export function listTransferSessions(
  workspaceId: string,
  query: TransferListQuery = {},
  options: BffFetchOptions = {},
) {
  const params = new URLSearchParams();
  if (query.status) params.set("status", query.status);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  const qs = suffix ? `?${suffix}` : "";
  return bffFetch<TransferSessionPageVM>(`/workspaces/${workspaceId}/transfers${qs}`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function getTransferSession(
  sessionId: string,
  workspaceId: string,
  options: BffFetchOptions = {},
) {
  return bffFetch<TransferSessionVM>(`/workspaces/${workspaceId}/transfers/${sessionId}`, {
    method: "GET",
    workspaceId,
    ...options,
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
  options: Pick<
    UploadTransferOptions,
    "partSize" | "onProgress" | "fetchImpl" | "signal"
  > = {},
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
    const response = await fetchImpl(uploadUrl, {
      method: "PUT",
      body: chunk,
      signal: options.signal,
    });
    if (!response.ok) {
      throw new Error(`transfer upload failed: ${response.status}`);
    }
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
  if (options.session.protocol === "DIRECT") {
    if (!options.session.upload_url || !options.session.capability_token) {
      throw new Error("direct transfer upload capability required");
    }
    const response = await (options.fetchImpl ?? fetch)(options.session.upload_url, {
      method: "PUT",
      body: file,
      headers: { Authorization: `Bearer ${options.session.capability_token}` },
      signal: options.signal,
    });
    if (!response.ok) {
      throw new Error(`transfer upload failed: ${response.status}`);
    }
    options.onProgress?.({
      uploadedBytes: file.size,
      totalBytes: file.size,
      completedParts: file.size > 0 ? 1 : 0,
      totalParts: file.size > 0 ? 1 : 0,
    });
    return {
      session: options.session,
      requestedParts: file.size > 0 ? [1] : [],
    };
  }
  return uploadTransferParts(file, options.session, options);
}

function fileAuditQuery(query: FileAuditQuery = {}) {
  const params = new URLSearchParams();
  if (query.actor_id) params.set("actor_id", query.actor_id);
  if (query.action) params.set("action", query.action);
  if (query.resource_id) params.set("resource_id", query.resource_id);
  if (query.result) params.set("result", query.result);
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

export function listStorageProviders(workspaceId: string, options: BffFetchOptions = {}) {
  return bffFetch<StorageProviderPageVM>(`/workspaces/${workspaceId}/storage-providers`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function registerStorageProvider(
  workspaceId: string,
  input: RegisterStorageProviderCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-provider");
  return bffFetch<{ id: string }>(`/workspaces/${workspaceId}/storage-providers`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function updateStorageProvider(
  workspaceId: string,
  providerId: string,
  input: UpdateStorageProviderCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-provider-update");
  return bffFetch<{ updated: true }>(`/workspaces/${workspaceId}/storage-providers/${providerId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function deleteStorageProvider(
  workspaceId: string,
  providerId: string,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-provider-delete");
  return bffFetch<{ deleted: true }>(`/workspaces/${workspaceId}/storage-providers/${providerId}`, {
    method: "DELETE",
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function listStoragePools(workspaceId: string, options: BffFetchOptions = {}) {
  return bffFetch<StoragePoolPageVM>(`/workspaces/${workspaceId}/storage-pools`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function createStoragePool(
  workspaceId: string,
  input: CreateStoragePoolCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-pool");
  return bffFetch<{ id: string; name: string }>(`/workspaces/${workspaceId}/storage-pools`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function updateStoragePool(
  workspaceId: string,
  poolId: string,
  input: UpdateStoragePoolCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-pool-update");
  return bffFetch<{ id: string; name: string }>(`/workspaces/${workspaceId}/storage-pools/${poolId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function deleteStoragePool(
  workspaceId: string,
  poolId: string,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-pool-delete");
  return bffFetch<{ deleted: true }>(`/workspaces/${workspaceId}/storage-pools/${poolId}`, {
    method: "DELETE",
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function listStorageBindings(workspaceId: string, options: BffFetchOptions = {}) {
  return bffFetch<StorageBindingPageVM>(`/workspaces/${workspaceId}/storage-bindings`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function createStorageBinding(
  workspaceId: string,
  input: CreateStorageBindingCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("storage-binding");
  return bffFetch<{ id: string }>(`/workspaces/${workspaceId}/storage-bindings`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
    headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
    ...options,
  });
}

export function listLifecyclePolicies(workspaceId: string, options: BffFetchOptions = {}) {
  return bffFetch<LifecyclePolicyPageVM>(`/workspaces/${workspaceId}/lifecycle-policies`, {
    method: "GET",
    workspaceId,
    ...options,
  });
}

export function upsertLifecyclePolicy(
  workspaceId: string,
  input: UpsertLifecyclePolicyCommand,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  const idempotencyKey = options.idempotencyKey ?? mutationIdempotencyKey("lifecycle-policy");
  return bffFetch<{ id: string; version: number }>(
    `/workspaces/${workspaceId}/lifecycle-policies`,
    {
      method: "POST",
      body: JSON.stringify(input),
      workspaceId,
      headers: { ...options.headers, "Idempotency-Key": idempotencyKey },
      ...options,
    },
  );
}

export function listFileAuditEvents(
  workspaceId: string,
  query: FileAuditQuery = {},
  options: BffFetchOptions = {},
) {
  return bffFetch<FileAuditEventPageVM>(
    `/workspaces/${workspaceId}/filesystem-audits${fileAuditQuery(query)}`,
    {
      method: "GET",
      workspaceId,
      ...options,
    },
  );
}
