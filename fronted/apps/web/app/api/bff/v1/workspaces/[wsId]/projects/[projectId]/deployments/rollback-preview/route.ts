import { createBffHandler } from "@sparkops/bff-core";
import { getRollbackPreview } from "@sparkops/bff-modules/delivery";

function requireParams(params?: Record<string, string>) {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return { wsId, projectId };
}

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  const targetRunId = new URL(req.url).searchParams.get("target_run_id") ?? undefined;
  return getRollbackPreview(ctx, wsId, projectId, targetRunId);
});
