import type { BffContext } from "@sparkops/bff-core";

export async function publishAgent(
  ctx: BffContext,
  workspaceId: string,
  agentId: string,
  versionId: string,
): Promise<{ published: true }> {
  return ctx.upstream.agent.publishAgent(workspaceId, agentId, versionId);
}
