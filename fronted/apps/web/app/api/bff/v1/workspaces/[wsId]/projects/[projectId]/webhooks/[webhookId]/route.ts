import { createBffHandler } from "@sparkops/bff-core";
import { deleteWebhook, updateWebhook } from "@sparkops/bff-modules/delivery";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  const webhookId = params?.webhookId;
  if (!wsId || !projectId || !webhookId) throw new Error("missing params");
  const body = (await req.json()) as {
    name: string;
    url: string;
    token?: string;
    events: string[];
    enabled: boolean;
  };
  return updateWebhook(ctx, wsId, projectId, webhookId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  const webhookId = params?.webhookId;
  if (!wsId || !projectId || !webhookId) throw new Error("missing params");
  return deleteWebhook(ctx, wsId, projectId, webhookId);
});
