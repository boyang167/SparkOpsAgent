import type { BffContext } from "@sparkops/bff-core";
import type { RegisteredAPI, RoleTreeVM } from "@sparkops/contracts/governance";

function normalizeRegisteredAPI(raw: RegisteredAPI & Record<string, unknown>): RegisteredAPI {
  return {
    id: String(raw.id ?? raw.ID ?? ""),
    path: String(raw.path ?? raw.Path ?? ""),
    method: String(raw.method ?? raw.Method ?? "GET").toUpperCase(),
    api_group: String(raw.api_group ?? raw.APIGroup ?? "default"),
    description: String(raw.description ?? raw.Description ?? ""),
  };
}

export async function listRoleTree(ctx: BffContext, workspaceId: string): Promise<RoleTreeVM[]> {
  const data = await ctx.upstream.governance.listRoleTree(workspaceId);
  return data.items;
}

export async function copyRole(ctx: BffContext, workspaceId: string, roleId: string, name?: string) {
  return ctx.upstream.governance.copyRole(workspaceId, roleId, name);
}

export async function listWorkspaceApis(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.governance.listWorkspaceApis(workspaceId);
  return data.items.map((item) => normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>));
}

export async function listPlatformApis(ctx: BffContext) {
  const data = await ctx.upstream.governance.listPlatformApis();
  return data.items.map((item) => normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>));
}

export async function previewApiSync(ctx: BffContext) {
  return ctx.upstream.governance.previewApiSync();
}

export async function confirmApiSync(
  ctx: BffContext,
  input: { new: unknown[]; delete: unknown[] },
) {
  return ctx.upstream.governance.confirmApiSync(input);
}

export async function listApiTokens(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.governance.listApiTokens(workspaceId);
  return data.items;
}

export async function createApiToken(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/governance").CreateAPITokenCommand,
) {
  return ctx.upstream.governance.createApiToken(workspaceId, input);
}

export async function revokeApiToken(ctx: BffContext, workspaceId: string, tokenId: string) {
  return ctx.upstream.governance.revokeApiToken(workspaceId, tokenId);
}

export async function listRoleAPIPolicies(ctx: BffContext, workspaceId: string, roleId: string) {
  const data = await ctx.upstream.governance.listRoleAPIPolicies(workspaceId, roleId);
  return data.items;
}

export async function updateRoleAPIPolicies(
  ctx: BffContext,
  workspaceId: string,
  roleId: string,
  items: import("@sparkops/contracts/governance").RoleAPIPolicy[],
) {
  return ctx.upstream.governance.updateRoleAPIPolicies(workspaceId, roleId, items);
}
