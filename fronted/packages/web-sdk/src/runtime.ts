import type { ChatStartVM, ConversationSummaryVM, MessageVM, StartChatCommand } from "@sparkops/contracts/runtime";
import { bffFetch, type BffFetchOptions } from "./client";

export function startChat(agentId: string, input: StartChatCommand, workspaceId: string) {
  const options: BffFetchOptions = { workspaceId };
  return bffFetch<ChatStartVM>(`/agents/${agentId}/chat`, {
    method: "POST",
    body: JSON.stringify(input),
    ...options,
  });
}

export async function getConversation(conversationId: string, workspaceId: string) {
  const data = await bffFetch<{ items: MessageVM[] }>(`/conversations/${conversationId}`, {
    workspaceId,
  });
  return data.items;
}

export async function listConversations(agentId: string, workspaceId: string) {
  const data = await bffFetch<{ items: ConversationSummaryVM[] }>(
    `/agents/${agentId}/conversations`,
    { workspaceId },
  );
  return data.items;
}

export interface StreamRunHandlers {
  onDelta: (delta: string) => void;
  onDone: () => void;
  onError?: (error: Error) => void;
}

export function streamRun(runId: string, handlers: StreamRunHandlers): () => void {
  const es = new EventSource(`/api/bff/v1/runs/${runId}/stream`);

  es.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as { delta?: string; done?: string };
      if (payload.done) {
        handlers.onDone();
        es.close();
        return;
      }
      if (payload.delta) {
        handlers.onDelta(payload.delta);
      }
    } catch (err) {
      handlers.onError?.(err instanceof Error ? err : new Error("stream parse error"));
    }
  };

  es.onerror = () => {
    handlers.onError?.(new Error("stream connection error"));
    es.close();
  };

  return () => es.close();
}
