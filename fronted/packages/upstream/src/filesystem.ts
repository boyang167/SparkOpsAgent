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
  TransferSessionVM,
  UpsertLifecyclePolicyCommand,
} from "@sparkops/contracts/filesystem";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

type ListFilesQuery = {
  parent_id?: string | null;
  cursor?: string;
  limit?: number;
};

type ListRevisionsQuery = {
  cursor?: string;
  limit?: number;
};

function filesQuery(query: ListFilesQuery = {}) {
  const params = new URLSearchParams();
  if (query.parent_id) params.set("parent_id", query.parent_id);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

function revisionsQuery(query: ListRevisionsQuery = {}) {
  const params = new URLSearchParams();
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
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

export function createFilesystemUpstream(options: UpstreamRequestOptions = {}) {
  return {
    getNamespace(workspaceId: string) {
      return upstreamFetch<FileNamespaceVM>(`/api/v1/workspaces/${workspaceId}/namespaces`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },
    listFiles(workspaceId: string, query: ListFilesQuery = {}) {
      return upstreamFetch<FileNodePageVM>(
        `/api/v1/workspaces/${workspaceId}/files${filesQuery(query)}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
    createDirectory(workspaceId: string, input: CreateDirectoryCommand, idempotencyKey: string) {
      return upstreamFetch<FileNodeVM>(`/api/v1/workspaces/${workspaceId}/directories`, {
        method: "POST",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": idempotencyKey },
        options: { ...options, workspaceId },
      });
    },
    renameFileNode(
      workspaceId: string,
      nodeId: string,
      input: RenameFileNodeCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<void>(`/api/v1/workspaces/${workspaceId}/files/${nodeId}/name`, {
        method: "PATCH",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": idempotencyKey },
        options: { ...options, workspaceId },
      });
    },
    listRevisions(workspaceId: string, nodeId: string, query: ListRevisionsQuery = {}) {
      return upstreamFetch<FileRevisionPageVM>(
        `/api/v1/workspaces/${workspaceId}/files/${nodeId}/revisions${revisionsQuery(query)}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
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
    listStorageProviders(workspaceId: string) {
      return upstreamFetch<StorageProviderPageVM>(
        `/api/v1/workspaces/${workspaceId}/storage-providers`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
    registerStorageProvider(
      workspaceId: string,
      input: RegisterStorageProviderCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<{ id: string }>(
        `/api/v1/workspaces/${workspaceId}/storage-providers`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    listStoragePools(workspaceId: string) {
      return upstreamFetch<StoragePoolPageVM>(`/api/v1/workspaces/${workspaceId}/storage-pools`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },
    createStoragePool(workspaceId: string, input: CreateStoragePoolCommand, idempotencyKey: string) {
      return upstreamFetch<{ id: string; name: string }>(
        `/api/v1/workspaces/${workspaceId}/storage-pools`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    listStorageBindings(workspaceId: string) {
      return upstreamFetch<StorageBindingPageVM>(
        `/api/v1/workspaces/${workspaceId}/storage-bindings`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
    createStorageBinding(
      workspaceId: string,
      input: CreateStorageBindingCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<{ id: string }>(
        `/api/v1/workspaces/${workspaceId}/storage-bindings`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    listLifecyclePolicies(workspaceId: string) {
      return upstreamFetch<LifecyclePolicyPageVM>(
        `/api/v1/workspaces/${workspaceId}/lifecycle-policies`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
    upsertLifecyclePolicy(
      workspaceId: string,
      input: UpsertLifecyclePolicyCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<{ id: string; version: number }>(
        `/api/v1/workspaces/${workspaceId}/lifecycle-policies`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    listFileAuditEvents(workspaceId: string, query: FileAuditQuery = {}) {
      return upstreamFetch<FileAuditEventPageVM>(
        `/api/v1/workspaces/${workspaceId}/filesystem-audits${fileAuditQuery(query)}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
  };
}

export type FilesystemUpstream = ReturnType<typeof createFilesystemUpstream>;
