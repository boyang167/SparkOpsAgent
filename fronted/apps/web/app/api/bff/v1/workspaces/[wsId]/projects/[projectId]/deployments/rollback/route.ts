import { createBffHandler } from "@sparkops/bff-core";
import { rollbackDeployment } from "@sparkops/bff-modules/delivery";
import { RollbackDeploymentCommandSchema } from "@sparkops/contracts/delivery";

function requireParams(params?: Record<string, string>) {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return { wsId, projectId };
}

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  const raw = req.headers.get("content-length") === "0" ? {} : await req.json();
  const input = RollbackDeploymentCommandSchema.parse(raw);
  return rollbackDeployment(ctx, wsId, projectId, input);
});
