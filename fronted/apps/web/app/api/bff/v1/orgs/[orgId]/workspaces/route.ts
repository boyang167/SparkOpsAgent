import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createWorkspace } from "@sparkops/bff-modules/tenant";
import { CreateWorkspaceCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest, context: { params: Promise<{ orgId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateWorkspaceCommandSchema.parse(await req.json());
    const workspace = await createWorkspace(ctx, params.orgId, body);
    return ok(workspace, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
