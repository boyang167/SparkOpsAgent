import { createBffHandler } from "@sparkops/bff-core";
import { createSprint, listSprints } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const url = new URL(req.url);
  const status = url.searchParams.get("status") ?? undefined;
  const items = await listSprints(ctx, wsId, productId, status);
  return { items };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return createSprint(ctx, wsId, productId, body);
});
