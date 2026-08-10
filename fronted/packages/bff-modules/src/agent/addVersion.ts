import type { BffContext } from "@sparkops/bff-core";
import type { AddAgentVersionCommand, AddAgentVersionResult } from "@sparkops/contracts/agent";

export async function addAgentVersion(
  ctx: BffContext,
  workspaceId: string,
  agentId: string,
  input: AddAgentVersionCommand,
): Promise<AddAgentVersionResult> {
  return ctx.upstream.agent.addVersion(workspaceId, agentId, input);
}
