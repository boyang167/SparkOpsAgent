import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createGovernedGrant, listGovernedGrants } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  CreateGovernedGrantCommandSchema,
  GovernanceMetadataQuerySchema,
} from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ nodeId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const { nodeId } = await context.params;
    const query = GovernanceMetadataQuerySchema.parse(
      Object.fromEntries(req.nextUrl.searchParams.entries()),
    );
    return ok(
      await listGovernedGrants(ctx, organizationId, ctx.workspaceId, nodeId, query),
      traceId,
    );
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ nodeId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new BffError(
        "Idempotency-Key header required",
        "FILESYSTEM_IDEMPOTENCY_KEY_REQUIRED",
        400,
        traceId,
      );
    }
    const ctx = await createBffContext(req);
    const { nodeId } = await context.params;
    const body = CreateGovernedGrantCommandSchema.parse(await req.json());
    const result = await createGovernedGrant(
      ctx,
      organizationId,
      ctx.workspaceId,
      nodeId,
      body,
      idempotencyKey,
    );
    const status = "status" in result && result.status === "PENDING" ? 202 : 201;
    return ok(result, traceId, status);
  } catch (err) {
    return fail(err, traceId);
  }
}
