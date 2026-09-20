import { createBffHandler } from "@sparkops/bff-core";
import { getRunPreview } from "@sparkops/bff-modules/delivery";

function requireParams(params?: Record<string, string>) {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return { wsId, projectId };
}

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const { wsId, projectId } = requireParams(params);
  const search = new URL(req.url).searchParams;
  const environmentId = search.get("environment_id")?.trim() ?? "";
  if (!environmentId) throw new Error("environment_id is required");
  const imageTag = search.get("image_tag")?.trim() ?? undefined;
  return getRunPreview(ctx, wsId, projectId, { environmentId, imageTag });
});
