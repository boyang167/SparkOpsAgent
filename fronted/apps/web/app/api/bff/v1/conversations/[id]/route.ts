import { createBffHandler } from "@sparkops/bff-core";
import { getConversation } from "@sparkops/bff-modules/runtime";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const id = params?.id;
  if (!id) {
    throw new Error("conversation id required");
  }
  const items = await getConversation(ctx, id);
  return { items };
});
