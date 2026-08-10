import { createBffHandler } from "@sparkops/bff-core";
import { listConversations } from "@sparkops/bff-modules/runtime";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const agentId = params?.agentId;
  if (!agentId) {
    throw new Error("agent id required");
  }
  const items = await listConversations(ctx, agentId);
  return { items };
});
