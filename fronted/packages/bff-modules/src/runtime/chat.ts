import type { BffContext } from "@sparkops/bff-core";
import type { ChatStartVM, MessageVM, StartChatCommand } from "@sparkops/contracts/runtime";

export async function startChat(
  ctx: BffContext,
  agentId: string,
  input: StartChatCommand,
): Promise<ChatStartVM> {
  return ctx.upstream.runtime.startChat(agentId, input);
}

export async function getConversation(
  ctx: BffContext,
  conversationId: string,
): Promise<MessageVM[]> {
  const data = await ctx.upstream.runtime.getConversation(conversationId);
  return data.items;
}
