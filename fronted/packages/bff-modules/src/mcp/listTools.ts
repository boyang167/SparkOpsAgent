import type { BffContext } from "@sparkops/bff-core";
import type { McpToolVM } from "@sparkops/contracts/mcp";

export async function listMcpTools(ctx: BffContext, workspaceId: string): Promise<McpToolVM[]> {
  const data = await ctx.upstream.mcp.listTools(workspaceId);
  return data.items;
}
