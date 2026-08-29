import { createBffHandler } from "@sparkops/bff-core";
import { createDeployment, listDeployments } from "@sparkops/bff-modules/delivery";
import { CreateDeploymentCommandSchema } from "@sparkops/contracts/delivery";

function requireParams(params?: Record<string, string>) {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return { wsId, projectId };
}

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  return listDeployments(ctx, wsId, projectId);
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  const input = CreateDeploymentCommandSchema.parse(await req.json());
  return createDeployment(ctx, wsId, projectId, input);
});
