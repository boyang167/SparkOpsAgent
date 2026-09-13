import { createBffHandler } from "@sparkops/bff-core";
import { getSyncPreview } from "@sparkops/bff-modules/delivery";

function requireParams(params?: Record<string, string>) {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return { wsId, projectId };
}

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  const imageTag = new URL(req.url).searchParams.get("image_tag") ?? undefined;
  return getSyncPreview(ctx, wsId, projectId, imageTag);
});
