import type {
  ApiEnvelope,
  ExamplePingVM,
  PluginBindingVM,
  PluginRecordVM,
} from "@sparkops/contracts";
import { BffError } from "@sparkops/contracts";
import type { UpstreamRequestOptions } from "./core";
import { getUpstreamBaseUrl, upstreamFetch } from "./core";

export function createPluginUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listPlugins() {
      return upstreamFetch<{ items: PluginRecordVM[] }>("/api/v1/admin/plugins", {
        method: "GET",
        options,
      });
    },

    async installPlugin(formData: FormData) {
      return upstreamFetch<PluginRecordVM>("/api/v1/admin/plugins/install", {
        method: "POST",
        body: formData,
        options,
      });
    },
    async markPluginMerged(pluginId: string) {
      return upstreamFetch<PluginRecordVM>(
        `/api/v1/admin/plugins/${encodeURIComponent(pluginId)}/merged`,
        { method: "POST", options },
      );
    },

    async markPluginDeployed(pluginId: string) {
      return upstreamFetch<PluginRecordVM | { deleted: true }>(
        `/api/v1/admin/plugins/${encodeURIComponent(pluginId)}/deployed`,
        { method: "POST", options },
      );
    },

    async uninstallPlugin(pluginId: string) {
      return upstreamFetch<PluginRecordVM>(
        `/api/v1/admin/plugins/${encodeURIComponent(pluginId)}/uninstall`,
        { method: "POST", options },
      );
    },

    async listWorkspacePlugins(workspaceId: string) {
      return upstreamFetch<{ items: PluginRecordVM[] }>(
        `/api/v1/workspaces/${workspaceId}/plugins`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async enableWorkspacePlugin(workspaceId: string, pluginId: string) {
      return upstreamFetch<PluginBindingVM>(
        `/api/v1/workspaces/${workspaceId}/plugins/${encodeURIComponent(pluginId)}/enable`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async disableWorkspacePlugin(workspaceId: string, pluginId: string) {
      return upstreamFetch<PluginBindingVM>(
        `/api/v1/workspaces/${workspaceId}/plugins/${encodeURIComponent(pluginId)}/disable`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async getExamplePing() {
      const headers = new Headers();
      if (options.token) headers.set("Authorization", `Bearer ${options.token}`);
      if (options.workspaceId) headers.set("X-Workspace-Id", options.workspaceId);
      if (options.traceId) headers.set("X-Trace-Id", options.traceId);

      let response: Response;
      try {
        response = await fetch(`${getUpstreamBaseUrl()}/api/v1/plugins/example/ping`, {
          method: "GET",
          headers,
          cache: "no-store",
        });
      } catch {
        throw new BffError("upstream unavailable", "UPSTREAM_UNAVAILABLE", 503);
      }

      let payload: ExamplePingVM | ApiEnvelope<ExamplePingVM>;
      try {
        payload = (await response.json()) as ExamplePingVM | ApiEnvelope<ExamplePingVM>;
      } catch {
        throw new BffError(
          "upstream returned invalid response",
          "UPSTREAM_ERROR",
          response.status || 502,
        );
      }
      if (!response.ok) {
        throw new BffError("plugin ping failed", "UPSTREAM_ERROR", response.status);
      }
      if ("code" in payload) {
        if (payload.code !== 0) {
          throw new BffError(
            payload.message || "plugin ping failed",
            "UPSTREAM_ERROR",
            response.status,
            payload.trace_id,
          );
        }
        return payload.data;
      }
      return payload;
    },
  };
}

export type PluginUpstream = ReturnType<typeof createPluginUpstream>;
