import type { ChatStartVM, ConversationListVM, MessageVM, StartChatCommand } from "@sparkops/contracts/runtime";
import type { UpstreamRequestOptions } from "./core";
import { getUpstreamBaseUrl, upstreamFetch } from "./core";

export function createRuntimeUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async startChat(agentId: string, input: StartChatCommand) {
      return upstreamFetch<ChatStartVM>(`/api/v1/agents/${agentId}/chat`, {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async getConversation(conversationId: string) {
      return upstreamFetch<{ items: MessageVM[] }>(`/api/v1/conversations/${conversationId}`, {
        method: "GET",
        options,
      });
    },

    async listConversations(agentId: string) {
      return upstreamFetch<ConversationListVM>(`/api/v1/agents/${agentId}/conversations`, {
        method: "GET",
        options,
      });
    },

    streamRunUrl(runId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/runs/${runId}/stream`;
    },
  };
}

export type RuntimeUpstream = ReturnType<typeof createRuntimeUpstream>;

export function buildUpstreamStreamHeaders(options: UpstreamRequestOptions = {}): HeadersInit {
  const headers: Record<string, string> = {
    Accept: "text/event-stream",
  };
  if (options.token) {
    headers.Authorization = `Bearer ${options.token}`;
  }
  if (options.workspaceId) {
    headers["X-Workspace-Id"] = options.workspaceId;
  }
  if (options.traceId) {
    headers["X-Trace-Id"] = options.traceId;
  }
  return headers;
}
