import { createBffHandler } from "@sparkops/bff-core";
import { createWebhook, listWebhooks } from "@sparkops/bff-modules/delivery";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("workspace and project required");
  return listWebhooks(ctx, wsId, projectId);
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("workspace and project required");
  const body = (await req.json()) as {
    name: string;
    url: string;
    token?: string;
    events: string[];
    enabled: boolean;
  };
  return createWebhook(ctx, wsId, projectId, body);
});
