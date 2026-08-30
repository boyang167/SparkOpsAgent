import type { UpstreamRawFetchInit, UpstreamRequestOptions } from "./core";
import { upstreamRawFetch } from "./core";

export type KubernetesPluginApiPath = `/api/v1/plugins/kubernetes/${string}`;

export function createKubernetesPluginUpstream(options: UpstreamRequestOptions = {}) {
  return {
    request(path: KubernetesPluginApiPath, init: Omit<UpstreamRawFetchInit, "options"> = {}) {
      return upstreamRawFetch(path, {
        ...init,
        options,
      });
    },
  };
}

export type KubernetesPluginUpstream = ReturnType<typeof createKubernetesPluginUpstream>;
