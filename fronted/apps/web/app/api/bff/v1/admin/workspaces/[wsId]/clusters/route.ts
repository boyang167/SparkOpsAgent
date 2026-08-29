import { createBffHandler } from "@sparkops/bff-core";
import {
  grantWorkspaceCluster,
  listWorkspaceClusters,
} from "@sparkops/bff-modules/platform";
import { GrantWorkspaceClusterCommandSchema } from "@sparkops/contracts/platform";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) throw new Error("workspace required");
  return { items: await listWorkspaceClusters(ctx, wsId) };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) throw new Error("workspace required");
  const input = GrantWorkspaceClusterCommandSchema.parse(await req.json());
  return grantWorkspaceCluster(ctx, wsId, input.cluster_id);
});
