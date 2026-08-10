import type { BffContext } from "@sparkops/bff-core";
import type { AgentVM } from "@sparkops/contracts/agent";

export async function listAgents(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<AgentVM[]> {
  const data = await ctx.upstream.agent.listAgents(workspaceId, projectId);
  return data.items;
}
