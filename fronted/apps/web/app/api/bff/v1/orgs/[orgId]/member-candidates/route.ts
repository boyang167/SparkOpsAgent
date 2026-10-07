import { createBffHandler } from "@sparkops/bff-core";
import { searchOrgMemberCandidates } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, req: NextRequest, { params }) => {
  const orgId = params?.orgId;
  if (!orgId) {
    throw new Error("organization id required");
  }
  const url = new URL(req.url);
  const q = url.searchParams.get("q") ?? "";
  const limitRaw = url.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : 10;
  const items = await searchOrgMemberCandidates(ctx, orgId, q, Number.isFinite(limit) ? limit : 10);
  return { items };
});
