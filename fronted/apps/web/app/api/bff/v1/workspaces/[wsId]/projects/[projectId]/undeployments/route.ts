import { createBffHandler } from "@sparkops/bff-core";
import { undeploy } from "@sparkops/bff-modules/delivery";
import { UndeployCommandSchema } from "@sparkops/contracts/delivery";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  const input = UndeployCommandSchema.parse(await req.json());
  return undeploy(ctx, wsId, projectId, input);
});
