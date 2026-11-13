import { createBffHandler } from "@sparkops/bff-core";
import { getFileNamespace } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  return getFileNamespace(ctx, wsId);
});
