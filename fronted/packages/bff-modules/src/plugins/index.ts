import type { BffContext } from "@sparkops/bff-core";
import { invalidateEntitlementsCache } from "@sparkops/bff-core";

export async function listPlugins(ctx: BffContext) {
  const data = await ctx.upstream.plugin.listPlugins();
  return data.items;
}

export function installPlugin(ctx: BffContext, formData: FormData) {
  return ctx.upstream.plugin.installPlugin(formData);
}

export function markPluginMerged(ctx: BffContext, pluginId: string) {
  return ctx.upstream.plugin.markPluginMerged(pluginId);
}

export async function markPluginDeployed(ctx: BffContext, pluginId: string) {
  const result = await ctx.upstream.plugin.markPluginDeployed(pluginId);
  invalidateEntitlementsCache();
  return result;
}

export function uninstallPlugin(ctx: BffContext, pluginId: string) {
  return ctx.upstream.plugin.uninstallPlugin(pluginId);
}

export async function listWorkspacePlugins(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.plugin.listWorkspacePlugins(workspaceId);
  return data.items;
}

export async function enableWorkspacePlugin(
  ctx: BffContext,
  workspaceId: string,
  pluginId: string,
) {
  const result = await ctx.upstream.plugin.enableWorkspacePlugin(workspaceId, pluginId);
  invalidateEntitlementsCache();
  return result;
}

export async function disableWorkspacePlugin(
  ctx: BffContext,
  workspaceId: string,
  pluginId: string,
) {
  const result = await ctx.upstream.plugin.disableWorkspacePlugin(workspaceId, pluginId);
  invalidateEntitlementsCache();
  return result;
}

export function getExamplePing(ctx: BffContext) {
  return ctx.upstream.plugin.getExamplePing();
}
