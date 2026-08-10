import { createBffHandler } from "@sparkops/bff-core";
import { createSyncJob, listSyncJobs } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => {
  const items = await listSyncJobs(ctx);
  return { items };
});

export const POST = createBffHandler(async (ctx, req) => {
  const body = (await req.json()) as Record<string, unknown>;
  return createSyncJob(ctx, body);
});
