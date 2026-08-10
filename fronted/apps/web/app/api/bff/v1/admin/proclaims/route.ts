import { createBffHandler } from "@sparkops/bff-core";
import { createProclaim, listProclaims } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => {
  const items = await listProclaims(ctx);
  return { items };
});

export const POST = createBffHandler(async (ctx, req) => {
  const body = (await req.json()) as Record<string, unknown>;
  return createProclaim(ctx, body);
});
