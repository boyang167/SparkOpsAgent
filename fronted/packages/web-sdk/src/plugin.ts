import type {
  ExamplePingVM,
  PluginBindingVM,
  PluginRecordVM,
} from "@sparkops/contracts/plugin";
import { bffFetch } from "./client";

export async function listPlugins() {
  const data = await bffFetch<{ items: PluginRecordVM[] }>("/admin/plugins");
  return data.items;
}

export function installPlugin(file: File) {
  const formData = new FormData();
  formData.set("file", file);
  return bffFetch<PluginRecordVM>("/admin/plugins/install", {
    method: "POST",
    body: formData,
  });
}

export function markPluginMerged(pluginId: string) {
  return bffFetch<PluginRecordVM>(
    `/admin/plugins/${encodeURIComponent(pluginId)}/merged`,
    { method: "POST" },
  );
}

export function markPluginDeployed(pluginId: string) {
  return bffFetch<PluginRecordVM | { deleted: true }>(
    `/admin/plugins/${encodeURIComponent(pluginId)}/deployed`,
    { method: "POST" },
  );
}

export function uninstallPlugin(pluginId: string) {
  return bffFetch<PluginRecordVM>(
    `/admin/plugins/${encodeURIComponent(pluginId)}/uninstall`,
    { method: "POST" },
  );
}

export async function listWorkspacePlugins(workspaceId: string) {
  const data = await bffFetch<{ items: PluginRecordVM[] }>(
    `/workspaces/${workspaceId}/plugins`,
    { workspaceId },
  );
  return data.items;
}

export function enableWorkspacePlugin(workspaceId: string, pluginId: string) {
  return bffFetch<PluginBindingVM>(
    `/workspaces/${workspaceId}/plugins/${encodeURIComponent(pluginId)}/enable`,
    { method: "POST", workspaceId },
  );
}

export function disableWorkspacePlugin(workspaceId: string, pluginId: string) {
  return bffFetch<PluginBindingVM>(
    `/workspaces/${workspaceId}/plugins/${encodeURIComponent(pluginId)}/disable`,
    { method: "POST", workspaceId },
  );
}

export function getExamplePing(workspaceId?: string) {
  return bffFetch<ExamplePingVM>("/plugins/example/ping", { workspaceId });
}
