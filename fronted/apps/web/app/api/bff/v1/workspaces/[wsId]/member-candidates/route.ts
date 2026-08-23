import { createBffHandler } from "@sparkops/bff-core";
import { searchMemberCandidates } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, req: NextRequest, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const url = new URL(req.url);
  const q = url.searchParams.get("q") ?? "";
  const limitRaw = url.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : 10;
  const items = await searchMemberCandidates(ctx, wsId, q, Number.isFinite(limit) ? limit : 10);
  return { items };
});
