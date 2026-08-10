import type { BffContext } from "@sparkops/bff-core";

export async function archiveAgent(
  ctx: BffContext,
  workspaceId: string,
  agentId: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.agent.archiveAgent(workspaceId, agentId);
}
