import { createBffHandler } from "@sparkops/bff-core";
import {
  listAuthorizedWorkspaceClusters,
  registerWorkspaceCluster,
} from "@sparkops/bff-modules/platform";
import { RegisterWorkspaceClusterCommandSchema } from "@sparkops/contracts/platform";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) throw new Error("workspace required");
  return { items: await listAuthorizedWorkspaceClusters(ctx, wsId) };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) throw new Error("workspace required");
  const input = RegisterWorkspaceClusterCommandSchema.parse(await req.json());
  return registerWorkspaceCluster(ctx, wsId, input);
});
