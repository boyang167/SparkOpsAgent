import { createBffHandler } from "@sparkops/bff-core";
import { listWorkspaceMenuButtons } from "@sparkops/bff-modules/governance";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const items = await listWorkspaceMenuButtons(ctx, wsId);
  return { items };
});
