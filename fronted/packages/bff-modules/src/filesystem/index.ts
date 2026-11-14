import type { BffContext } from "@sparkops/bff-core";
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
  GovernanceMetadataQuery,
  GovernedShareSubmissionVM,
  EffectiveAccessVM,
  LifecyclePolicyPageVM,
  NamespaceSummaryPageVM,
  OrganizationShareRequestQuery,
  RegisterStorageProviderCommand,
  RejectShareRequestCommand,
  RenameFileNodeCommand,
  StorageBindingPageVM,
  StoragePoolPageVM,
  StorageProviderPageVM,
  SharedItemPageVM,
  SharePreviewVM,
  ShareRequestPageVM,
  ShareRequestVM,
  SharedWithMeQuery,
  UpdateStoragePoolCommand,
  UpdateStorageProviderCommand,
  TransferListQuery,
  TransferSessionPageVM,
  TransferSessionVM,
  UpsertLifecyclePolicyCommand,
} from "@sparkops/contracts/filesystem";
import {
  EffectiveAccessVMSchema,
  FileAuditEventPageVMSchema,
  FileNamespaceVMSchema,
  FileNodePageVMSchema,
  FileNodeVMSchema,
  FileRevisionPageVMSchema,
  LifecyclePolicyPageVMSchema,
  GovernedGrantPageVMSchema,
  GovernedShareSubmissionVMSchema,
  NamespaceSummaryPageVMSchema,
  SharedItemPageVMSchema,
  SharePreviewVMSchema,
  ShareRequestPageVMSchema,
  ShareRequestVMSchema,
  StorageBindingPageVMSchema,
  StoragePoolPageVMSchema,
  StorageProviderPageVMSchema,
  TransferSessionPageVMSchema,
  TransferSessionVMSchema,
} from "@sparkops/contracts/filesystem";

function governedContext(organizationId: string, workspaceId?: string) {
  return { organizationId, ...(workspaceId ? { workspaceId } : {}) };
}

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
  query: {
    parent_id?: string | null;
    type?: "FILE" | "DIRECTORY";
    q?: string;
    cursor?: string;
    limit?: number;
  } = {},
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

export async function moveFileNode(
  ctx: BffContext,
  workspaceId: string,
  nodeId: string,
  input: { parent_id: string | null; name: string },
  idempotencyKey: string,
): Promise<void> {
  await ctx.upstream.filesystem.moveFileNode(
    workspaceId,
    nodeId,
    input,
    idempotencyKey,
  );
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

export async function updateStorageProvider(
  ctx: BffContext,
  workspaceId: string,
  providerId: string,
  input: UpdateStorageProviderCommand,
  idempotencyKey: string,
): Promise<{ updated: true }> {
  return ctx.upstream.filesystem.updateStorageProvider(workspaceId, providerId, input, idempotencyKey);
}

export async function deleteStorageProvider(
  ctx: BffContext,
  workspaceId: string,
  providerId: string,
  idempotencyKey: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.filesystem.deleteStorageProvider(workspaceId, providerId, idempotencyKey);
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

export async function updateStoragePool(
  ctx: BffContext,
  workspaceId: string,
  poolId: string,
  input: UpdateStoragePoolCommand,
  idempotencyKey: string,
): Promise<{ id: string; name: string }> {
  return ctx.upstream.filesystem.updateStoragePool(workspaceId, poolId, input, idempotencyKey);
}

export async function deleteStoragePool(
  ctx: BffContext,
  workspaceId: string,
  poolId: string,
  idempotencyKey: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.filesystem.deleteStoragePool(workspaceId, poolId, idempotencyKey);
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

export async function listGovernedNamespaces(
  ctx: BffContext,
  organizationId: string,
  workspaceId?: string,
): Promise<NamespaceSummaryPageVM> {
  const data = await ctx.upstream.filesystem.listGovernedNamespaces(
    governedContext(organizationId, workspaceId),
  );
  return NamespaceSummaryPageVMSchema.parse(data);
}

export async function listGovernedChildren(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  namespaceId: string,
  query: GovernedChildrenQuery = {},
): Promise<FileNodePageVM> {
  const data = await ctx.upstream.filesystem.listGovernedChildren(
    governedContext(organizationId, workspaceId),
    namespaceId,
    query,
  );
  return FileNodePageVMSchema.parse(data);
}

export async function listSharedWithMe(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  query: SharedWithMeQuery = {},
): Promise<SharedItemPageVM> {
  const data = await ctx.upstream.filesystem.listSharedWithMe(
    governedContext(organizationId, workspaceId),
    query,
  );
  return SharedItemPageVMSchema.parse(data);
}

export async function getEffectiveAccess(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
): Promise<EffectiveAccessVM> {
  const data = await ctx.upstream.filesystem.getEffectiveAccess(
    governedContext(organizationId, workspaceId),
    nodeId,
  );
  return EffectiveAccessVMSchema.parse(data);
}

export async function listGovernedGrants(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  query: GovernanceMetadataQuery = {},
): Promise<GovernedGrantPageVM> {
  const data = await ctx.upstream.filesystem.listGovernedGrants(
    governedContext(organizationId, workspaceId),
    nodeId,
    query,
  );
  return GovernedGrantPageVMSchema.parse(data);
}

export async function listShareRequests(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  query: GovernanceMetadataQuery = {},
): Promise<ShareRequestPageVM> {
  const data = await ctx.upstream.filesystem.listShareRequests(
    governedContext(organizationId, workspaceId),
    nodeId,
    query,
  );
  return ShareRequestPageVMSchema.parse(data);
}

export async function listOrganizationShareRequests(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  query: OrganizationShareRequestQuery = { status: "PENDING" },
): Promise<ShareRequestPageVM> {
  const data = await ctx.upstream.filesystem.listOrganizationShareRequests(
    governedContext(organizationId, workspaceId),
    query,
  );
  return ShareRequestPageVMSchema.parse(data);
}

export async function approveShareRequest(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  requestId: string,
  idempotencyKey: string,
): Promise<ShareRequestVM> {
  return ShareRequestVMSchema.parse(await ctx.upstream.filesystem.approveShareRequest(
    governedContext(organizationId, workspaceId), requestId, idempotencyKey,
  ));
}

export async function rejectShareRequest(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  requestId: string,
  input: RejectShareRequestCommand,
  idempotencyKey: string,
): Promise<ShareRequestVM> {
  return ShareRequestVMSchema.parse(await ctx.upstream.filesystem.rejectShareRequest(
    governedContext(organizationId, workspaceId), requestId, input, idempotencyKey,
  ));
}

export async function createGovernedGrant(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  input: CreateGovernedGrantCommand,
  idempotencyKey: string,
): Promise<GovernedShareSubmissionVM> {
  const data = await ctx.upstream.filesystem.createGovernedGrant(
    governedContext(organizationId, workspaceId),
    nodeId,
    input,
    idempotencyKey,
  );
  return GovernedShareSubmissionVMSchema.parse(data);
}

export async function revokeGovernedGrant(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  grantId: string,
  idempotencyKey: string,
): Promise<void> {
  await ctx.upstream.filesystem.revokeGovernedGrant(
    governedContext(organizationId, workspaceId), nodeId, grantId, idempotencyKey,
  );
}

export async function previewGovernedShare(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  input: CreateGovernedGrantCommand,
  idempotencyKey: string,
): Promise<SharePreviewVM> {
  const data = await ctx.upstream.filesystem.previewGovernedShare(
    governedContext(organizationId, workspaceId),
    nodeId,
    input,
    idempotencyKey,
  );
  return SharePreviewVMSchema.parse(data);
}

export async function createShareRequest(
  ctx: BffContext,
  organizationId: string,
  workspaceId: string | undefined,
  nodeId: string,
  input: CreateGovernedGrantCommand,
  idempotencyKey: string,
): Promise<GovernedShareSubmissionVM> {
  const data = await ctx.upstream.filesystem.createShareRequest(
    governedContext(organizationId, workspaceId),
    nodeId,
    input,
    idempotencyKey,
  );
  return GovernedShareSubmissionVMSchema.parse(data);
}
