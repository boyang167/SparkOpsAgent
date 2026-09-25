import { createBffHandler } from "@sparkops/bff-core";
import { createReleasePlan, listReleasePlans } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const sprintId = new URL(req.url).searchParams.get("sprint_id") ?? undefined;
  const items = await listReleasePlans(ctx, wsId, productId, sprintId);
  return { items };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return createReleasePlan(ctx, wsId, productId, body);
});
