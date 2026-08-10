import { listAgents, getAgent } from "@sparkops/bff-modules/agent";
import type { AgentDetailVM, AgentVM } from "@sparkops/contracts/agent";
import { loadBffContext } from "./context";

export async function loadAgents(workspaceId: string, projectId: string): Promise<AgentVM[]> {
  const ctx = await loadBffContext();
  return listAgents(ctx, workspaceId, projectId);
}

export async function loadAgent(workspaceId: string, agentId: string): Promise<AgentDetailVM> {
  const ctx = await loadBffContext();
  return getAgent(ctx, workspaceId, agentId);
}
