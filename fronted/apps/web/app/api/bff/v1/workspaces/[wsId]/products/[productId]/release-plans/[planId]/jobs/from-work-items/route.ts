import { createBffHandler } from "@sparkops/bff-core";
import { addReleasePlanJobsFromWorkItems } from "@sparkops/bff-modules/product";
import type { AddJobsFromWorkItemsCommand } from "@sparkops/contracts/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const body = (await req.json()) as AddJobsFromWorkItemsCommand;
  return addReleasePlanJobsFromWorkItems(ctx, wsId, productId, planId, body.work_item_ids);
});
