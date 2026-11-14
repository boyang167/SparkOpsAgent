import type {
  CreateGovernedGrantCommand,
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
  GovernedChildrenQuery,
  GovernedGrantPageVM,
  GovernedShareSubmissionVM,
  EffectiveAccessVM,
  LifecyclePolicyPageVM,
  NamespaceSummaryPageVM,
  RegisterStorageProviderCommand,
  RenameFileNodeCommand,
  StorageBindingPageVM,
  StoragePoolPageVM,
  StorageProviderPageVM,
  SharedItemPageVM,
  SharePreviewVM,
  ShareRequestPageVM,
  SharedWithMeQuery,
  UpdateStoragePoolCommand,
  UpdateStorageProviderCommand,
  TransferListQuery,
  TransferSessionPageVM,
  TransferSessionVM,
  UpsertLifecyclePolicyCommand,
} from "@sparkops/contracts/filesystem";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

type ListFilesQuery = {
  parent_id?: string | null;
  type?: "FILE" | "DIRECTORY";
  q?: string;
  cursor?: string;
  limit?: number;
};

type MoveFileNodeCommand = {
  parent_id: string | null;
  name: string;
};

type ListRevisionsQuery = {
  cursor?: string;
  limit?: number;
};

export type GovernedFilesystemContext = {
  organizationId: string;
  workspaceId?: string;
};

function governedHeaders(
  context: GovernedFilesystemContext,
  headers: HeadersInit = {},
): Headers {
  const result = new Headers(headers);
  result.set("X-Organization-ID", context.organizationId);
  return result;
}

function governedOptions(
  options: UpstreamRequestOptions,
  context: GovernedFilesystemContext,
): UpstreamRequestOptions {
  return { ...options, workspaceId: context.workspaceId };
}

function governedChildrenQuery(query: GovernedChildrenQuery = {}) {
  const params = new URLSearchParams();
  if (query.parent_id) params.set("parent_id", query.parent_id);
  if (query.type) params.set("type", query.type);
  if (query.q) params.set("q", query.q);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

function sharedWithMeQuery(query: SharedWithMeQuery = {}) {
  const params = new URLSearchParams();
  if (query.source_type) params.set("source_type", query.source_type);
  if (query.q) params.set("q", query.q);
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit != null) params.set("limit", String(query.limit));
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

function filesQuery(query: ListFilesQuery = {}) {
  const params = new URLSearchParams();
  if (query.parent_id) params.set("parent_id", query.parent_id);
  if (query.type) params.set("type", query.type);
  if (query.q) params.set("q", query.q);
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

function transfersQuery(query: TransferListQuery = {}) {
  const params = new URLSearchParams();
  if (query.status) params.set("status", query.status);
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
    listGovernedNamespaces(context: GovernedFilesystemContext) {
      return upstreamFetch<NamespaceSummaryPageVM>("/api/v1/filesystem/namespaces", {
        method: "GET",
        headers: governedHeaders(context),
        options: governedOptions(options, context),
      });
    },
    listGovernedChildren(
      context: GovernedFilesystemContext,
      namespaceId: string,
      query: GovernedChildrenQuery = {},
    ) {
      return upstreamFetch<FileNodePageVM>(
        `/api/v1/filesystem/namespaces/${encodeURIComponent(namespaceId)}/children${governedChildrenQuery(query)}`,
        {
          method: "GET",
          headers: governedHeaders(context),
          options: governedOptions(options, context),
        },
      );
    },
    listSharedWithMe(
      context: GovernedFilesystemContext,
      query: SharedWithMeQuery = {},
    ) {
      return upstreamFetch<SharedItemPageVM>(
        `/api/v1/filesystem/shared-with-me${sharedWithMeQuery(query)}`,
        {
          method: "GET",
          headers: governedHeaders(context),
          options: governedOptions(options, context),
        },
      );
    },
    getEffectiveAccess(context: GovernedFilesystemContext, nodeId: string) {
      return upstreamFetch<EffectiveAccessVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/effective-access`,
        {
          method: "GET",
          headers: governedHeaders(context),
          options: governedOptions(options, context),
        },
      );
    },
    listGovernedGrants(context: GovernedFilesystemContext, nodeId: string) {
      return upstreamFetch<GovernedGrantPageVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/grants`,
        {
          method: "GET",
          headers: governedHeaders(context),
          options: governedOptions(options, context),
        },
      );
    },
    listShareRequests(context: GovernedFilesystemContext, nodeId: string) {
      return upstreamFetch<ShareRequestPageVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/share-requests`,
        {
          method: "GET",
          headers: governedHeaders(context),
          options: governedOptions(options, context),
        },
      );
    },
    createGovernedGrant(
      context: GovernedFilesystemContext,
      nodeId: string,
      input: CreateGovernedGrantCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<GovernedShareSubmissionVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/grants`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: governedHeaders(context, { "Idempotency-Key": idempotencyKey }),
          options: governedOptions(options, context),
        },
      );
    },
    previewGovernedShare(
      context: GovernedFilesystemContext,
      nodeId: string,
      input: CreateGovernedGrantCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<SharePreviewVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/share-preview`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: governedHeaders(context, { "Idempotency-Key": idempotencyKey }),
          options: governedOptions(options, context),
        },
      );
    },
    createShareRequest(
      context: GovernedFilesystemContext,
      nodeId: string,
      input: CreateGovernedGrantCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<GovernedShareSubmissionVM>(
        `/api/v1/filesystem/nodes/${encodeURIComponent(nodeId)}/share-requests`,
        {
          method: "POST",
          body: JSON.stringify(input),
          headers: governedHeaders(context, { "Idempotency-Key": idempotencyKey }),
          options: governedOptions(options, context),
        },
      );
    },
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
    getFileNode(workspaceId: string, nodeId: string) {
      return upstreamFetch<FileNodeVM>(
        `/api/v1/workspaces/${workspaceId}/files/${nodeId}`,
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
    moveFileNode(
      workspaceId: string,
      nodeId: string,
      input: MoveFileNodeCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<void>(`/api/v1/workspaces/${workspaceId}/files/${nodeId}/move`, {
        method: "POST",
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
    listTransfers(workspaceId: string, query: TransferListQuery = {}) {
      return upstreamFetch<TransferSessionPageVM>(
        `/api/v1/workspaces/${workspaceId}/transfers${transfersQuery(query)}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
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
    updateStorageProvider(
      workspaceId: string,
      providerId: string,
      input: UpdateStorageProviderCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<{ updated: true }>(
        `/api/v1/workspaces/${workspaceId}/storage-providers/${providerId}`,
        {
          method: "PATCH",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    deleteStorageProvider(workspaceId: string, providerId: string, idempotencyKey: string) {
      return upstreamFetch<{ deleted: true }>(
        `/api/v1/workspaces/${workspaceId}/storage-providers/${providerId}`,
        {
          method: "DELETE",
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
    updateStoragePool(
      workspaceId: string,
      poolId: string,
      input: UpdateStoragePoolCommand,
      idempotencyKey: string,
    ) {
      return upstreamFetch<{ id: string; name: string }>(
        `/api/v1/workspaces/${workspaceId}/storage-pools/${poolId}`,
        {
          method: "PATCH",
          body: JSON.stringify(input),
          headers: { "Idempotency-Key": idempotencyKey },
          options: { ...options, workspaceId },
        },
      );
    },
    deleteStoragePool(workspaceId: string, poolId: string, idempotencyKey: string) {
      return upstreamFetch<{ deleted: true }>(
        `/api/v1/workspaces/${workspaceId}/storage-pools/${poolId}`,
        {
          method: "DELETE",
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
