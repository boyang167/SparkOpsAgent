import type { ModelVM } from "@sparkops/contracts/integration";
import type { McpToolVM } from "@sparkops/contracts/mcp";
import { bffFetch } from "./client";

export async function listModels() {
  const data = await bffFetch<{ items: ModelVM[] }>("/models");
  return data.items;
}

export async function listMcpTools(workspaceId: string) {
  const data = await bffFetch<{ items: McpToolVM[] }>("/mcp/tools", { workspaceId });
  return data.items;
}
