import { listConversations } from "@sparkops/bff-modules/runtime";
import type { ConversationSummaryVM } from "@sparkops/contracts/runtime";
import { loadBffContext } from "./context";

export async function loadConversations(agentId: string): Promise<ConversationSummaryVM[]> {
  const ctx = await loadBffContext();
  return listConversations(ctx, agentId);
}
