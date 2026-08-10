import { createBffHandler } from "@sparkops/bff-core";
import { createCronJob, listCronJobs } from "@sparkops/bff-modules/delivery";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("workspace and project required");
  const items = await listCronJobs(ctx, wsId, projectId);
  return { items };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("workspace and project required");
  const body = (await req.json()) as { name: string; schedule: string; branch?: string };
  return createCronJob(ctx, wsId, projectId, body);
});
