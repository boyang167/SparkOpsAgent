import { createBffHandler } from "@sparkops/bff-core";
import { EnvironmentSyncPreviewCommandSchema } from "@sparkops/contracts/product";
import { createEnvironmentSyncPreview } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  const input = EnvironmentSyncPreviewCommandSchema.parse(body);
  return createEnvironmentSyncPreview(ctx, wsId, productId, input);
});
