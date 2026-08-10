import type { BffContext } from "@sparkops/bff-core";
import type { ConversationSummaryVM } from "@sparkops/contracts/runtime";

export async function listConversations(
  ctx: BffContext,
  agentId: string,
): Promise<ConversationSummaryVM[]> {
  const data = await ctx.upstream.runtime.listConversations(agentId);
  return data.items;
}
