import { createBffHandler, createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createSecret, listSecrets } from "@sparkops/bff-modules/secret";
import { CreateSecretCommandSchema } from "@sparkops/contracts/secret";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) {
    throw new Error("workspace required");
  }
  const items = await listSecrets(ctx, wsId);
  return { items };
});

export async function POST(req: NextRequest, context: { params: Promise<{ wsId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateSecretCommandSchema.parse(await req.json());
    const secret = await createSecret(ctx, params.wsId, body);
    return ok(secret, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
