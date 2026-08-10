import type { AgentDetailVM, AgentListVM, AgentVM } from "@sparkops/contracts/agent";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createAgentUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listAgents(workspaceId: string, projectId: string) {
      return upstreamFetch<AgentListVM>(`/api/v1/projects/${projectId}/agents`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async createAgent(
      workspaceId: string,
      projectId: string,
      input: { name: string; system_prompt: string; model_name: string },
    ) {
      return upstreamFetch<Partial<AgentVM> & Pick<AgentVM, "id" | "name" | "status">>(
        `/api/v1/projects/${projectId}/agents`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options: { ...options, workspaceId },
        },
      );
    },

    async getAgent(workspaceId: string, agentId: string) {
      return upstreamFetch<AgentDetailVM>(`/api/v1/agents/${agentId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async publishAgent(workspaceId: string, agentId: string, versionId: string) {
      return upstreamFetch<{ published: true }>(`/api/v1/agents/${agentId}/publish`, {
        method: "POST",
        body: JSON.stringify({ version_id: versionId }),
        options: { ...options, workspaceId },
      });
    },

    async addVersion(
      workspaceId: string,
      agentId: string,
      input: { system_prompt: string; model_name: string; knowledge_base_ids?: string[] },
    ) {
      return upstreamFetch<{ id: string; version: number }>(`/api/v1/agents/${agentId}/versions`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async updateAgent(workspaceId: string, agentId: string, input: { name: string }) {
      return upstreamFetch<Partial<AgentVM> & Pick<AgentVM, "id" | "name" | "status">>(
        `/api/v1/agents/${agentId}`,
        {
          method: "PATCH",
          body: JSON.stringify(input),
          options: { ...options, workspaceId },
        },
      );
    },

    async archiveAgent(workspaceId: string, agentId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/agents/${agentId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },
  };
}

export type AgentUpstream = ReturnType<typeof createAgentUpstream>;
