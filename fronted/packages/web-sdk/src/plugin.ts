import type {
  ExamplePingVM,
  PluginBindingVM,
  PluginRecordVM,
} from "@sparkops/contracts/plugin";
import { BffError } from "@sparkops/contracts";
import { bffFetch } from "./client";

export async function listPlugins() {
  const data = await bffFetch<{ items: PluginRecordVM[] }>("/admin/plugins");
  return data.items;
}

export function installPlugin(file: File) {
  const maxBytes = 50 * 1024 * 1024;
  if (file.size > maxBytes) {
    return Promise.reject(
      new BffError("插件 zip 不能超过 50 MiB", "INVALID_REQUEST", 413),
    );
  }
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
