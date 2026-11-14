import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import {
  createTransferSession,
  listTransferSessions,
} from "@sparkops/bff-modules/filesystem";
import {
  CreateTransferCommandSchema,
  TransferListQuerySchema,
} from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const search = req.nextUrl.searchParams;
    const rawLimit = search.get("limit");
    const query = TransferListQuerySchema.parse({
      status: search.get("status") || undefined,
      cursor: search.get("cursor") || undefined,
      limit: rawLimit == null ? undefined : Number(rawLimit),
    });
    const page = await listTransferSessions(ctx, params.wsId, query);
    return ok(page, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateTransferCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const session = await createTransferSession(ctx, params.wsId, body, idempotencyKey);
    return ok(session, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
