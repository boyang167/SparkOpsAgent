import type { BffContext } from "@sparkops/bff-core";
import type { AgentVM, CreateAgentCommand } from "@sparkops/contracts/agent";

export async function createAgent(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: CreateAgentCommand,
): Promise<AgentVM> {
  const data = await ctx.upstream.agent.createAgent(workspaceId, projectId, input);
  return {
    id: data.id,
    project_id: data.project_id ?? projectId,
    name: data.name,
    status: data.status,
    current_version_id: data.current_version_id,
  };
}
