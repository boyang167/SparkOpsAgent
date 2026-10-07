import { createBffHandler } from "@sparkops/bff-core";
import { getReleasePlanVersionDiff } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  const version = Number(params?.version);
  if (!wsId || !productId || !planId || !Number.isFinite(version) || version <= 0) {
    throw new Error("workspace, product, plan and version required");
  }
  const compareRaw = req.nextUrl.searchParams.get("compare");
  const compareVersion = compareRaw != null && compareRaw !== "" ? Number(compareRaw) : undefined;
  if (compareVersion != null && (!Number.isFinite(compareVersion) || compareVersion < 0)) {
    throw new Error("invalid compare version");
  }
  return getReleasePlanVersionDiff(ctx, wsId, productId, planId, version, compareVersion);
});
