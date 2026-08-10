import type { ModelListVM } from "@sparkops/contracts/integration";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createIntegrationUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listModels() {
      return upstreamFetch<ModelListVM>("/api/v1/models", {
        method: "GET",
        options,
      });
    },
  };
}

export type IntegrationUpstream = ReturnType<typeof createIntegrationUpstream>;
