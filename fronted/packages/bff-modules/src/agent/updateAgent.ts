import type { BffContext } from "@sparkops/bff-core";
import type { AgentVM, UpdateAgentCommand } from "@sparkops/contracts/agent";

export async function updateAgent(
  ctx: BffContext,
  workspaceId: string,
  agentId: string,
  input: UpdateAgentCommand,
): Promise<AgentVM> {
  const data = await ctx.upstream.agent.updateAgent(workspaceId, agentId, input);
  return {
    id: data.id,
    project_id: data.project_id ?? "",
    name: data.name,
    status: data.status,
    current_version_id: data.current_version_id,
  };
}
