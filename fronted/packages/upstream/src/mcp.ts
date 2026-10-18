import type { McpToolListVM } from "@sparkops/contracts/mcp";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createMcpUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listTools(workspaceId: string) {
      return upstreamFetch<McpToolListVM>("/api/v1/mcp/tools", {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },
  };
}

export type McpUpstream = ReturnType<typeof createMcpUpstream>;
