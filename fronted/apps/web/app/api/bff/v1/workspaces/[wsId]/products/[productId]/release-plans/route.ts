import { createBffHandler } from "@sparkops/bff-core";
import { createReleasePlan, listReleasePlansPage } from "@sparkops/bff-modules/product";

function parseNonNegativeInt(raw: string | null) {
  if (!raw) return undefined;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const search = new URL(req.url).searchParams;
  return listReleasePlansPage(ctx, wsId, productId, {
    sprintId: search.get("sprint_id") ?? undefined,
    q: search.get("q")?.trim() || undefined,
    limit: parseNonNegativeInt(search.get("limit")),
    offset: parseNonNegativeInt(search.get("offset")),
  });
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return createReleasePlan(ctx, wsId, productId, body);
});
