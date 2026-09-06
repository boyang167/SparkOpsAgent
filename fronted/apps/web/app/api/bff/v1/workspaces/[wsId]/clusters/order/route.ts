import { createBffHandler } from "@sparkops/bff-core";
import { reorderAuthorizedWorkspaceClusters } from "@sparkops/bff-modules/platform";
import { ReorderWorkspaceClustersCommandSchema } from "@sparkops/contracts/platform";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) throw new Error("workspace required");
  const input = ReorderWorkspaceClustersCommandSchema.parse(await req.json());
  return reorderAuthorizedWorkspaceClusters(ctx, wsId, input.cluster_ids);
});
