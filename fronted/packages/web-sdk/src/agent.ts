import type { AgentDetailVM, AgentVM } from "@sparkops/contracts/agent";
import { bffFetch, type BffFetchOptions } from "./client";

export async function listAgents(workspaceId: string, projectId: string) {
  const options: BffFetchOptions = { workspaceId };
  const data = await bffFetch<{ items: AgentVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/agents`,
    options,
  );
  return data.items;
}

export function createAgent(
  workspaceId: string,
  projectId: string,
  input: { name: string; system_prompt: string; model_name: string },
) {
  return bffFetch<AgentVM>(`/workspaces/${workspaceId}/projects/${projectId}/agents`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function getAgent(agentId: string, workspaceId: string) {
  return bffFetch<AgentDetailVM>(`/agents/${agentId}`, {
    workspaceId,
  });
}

export function publishAgent(agentId: string, workspaceId: string, versionId: string) {
  return bffFetch<{ published: true }>(`/agents/${agentId}/publish`, {
    method: "POST",
    body: JSON.stringify({ version_id: versionId }),
    workspaceId,
  });
}

export function addAgentVersion(
  agentId: string,
  workspaceId: string,
  input: { system_prompt: string; model_name: string; knowledge_base_ids?: string[] },
) {
  return bffFetch<{ id: string; version: number }>(`/agents/${agentId}/versions`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function updateAgent(agentId: string, workspaceId: string, input: { name: string }) {
  return bffFetch<AgentVM>(`/agents/${agentId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function archiveAgent(agentId: string, workspaceId: string) {
  return bffFetch<{ deleted: true }>(`/agents/${agentId}`, {
    method: "DELETE",
    workspaceId,
  });
}
