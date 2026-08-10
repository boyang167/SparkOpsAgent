import { createBffHandler } from "@sparkops/bff-core";
import { createCluster, listClusters } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => {
  const items = await listClusters(ctx);
  return { items };
});

export const POST = createBffHandler(async (ctx, req) => {
  const body = (await req.json()) as Record<string, unknown>;
  return createCluster(ctx, body);
});
