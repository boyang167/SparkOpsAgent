import { createBffHandler } from "@sparkops/bff-core";
import { EnvironmentSyncSourceResourcesCommandSchema } from "@sparkops/contracts/product";
import { listEnvironmentSyncSourceResources } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  const input = EnvironmentSyncSourceResourcesCommandSchema.parse(body);
  return listEnvironmentSyncSourceResources(ctx, wsId, productId, input);
});
