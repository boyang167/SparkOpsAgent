import type { BffContext } from "@sparkops/bff-core";
import type { AgentDetailVM } from "@sparkops/contracts/agent";

export async function getAgent(
  ctx: BffContext,
  workspaceId: string,
  agentId: string,
): Promise<AgentDetailVM> {
  return ctx.upstream.agent.getAgent(workspaceId, agentId);
}
