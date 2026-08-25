import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listGitTree } from "@sparkops/bff-modules/workspace";
import { ListGitTreeCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = ListGitTreeCommandSchema.parse(await req.json());
    const data = await listGitTree(ctx, params.wsId, body);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
