import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteSecret, getSecret, updateSecret } from "@sparkops/bff-modules/secret";
import { UpdateSecretCommandSchema } from "@sparkops/contracts/secret";
import { NextRequest } from "next/server";

export async function GET(
  _req: NextRequest,
  context: { params: Promise<{ wsId: string; secretId: string }> },
) {
  const traceId = createTraceId(_req);
  try {
    const ctx = await createBffContext(_req);
    const params = await context.params;
    const item = await getSecret(ctx, params.wsId, params.secretId);
    return ok(item, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; secretId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateSecretCommandSchema.parse(await req.json());
    const item = await updateSecret(ctx, params.wsId, params.secretId, body);
    return ok(item, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; secretId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await deleteSecret(ctx, params.wsId, params.secretId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
