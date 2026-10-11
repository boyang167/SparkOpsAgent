import { createBffHandler } from "@sparkops/bff-core";
import { archiveProjectWithTeardown } from "@sparkops/bff-modules/workspace";
import { ArchiveProjectCommandSchema } from "@sparkops/contracts/workspace";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  const input = ArchiveProjectCommandSchema.parse(await req.json());
  return archiveProjectWithTeardown(ctx, wsId, projectId, input);
});
