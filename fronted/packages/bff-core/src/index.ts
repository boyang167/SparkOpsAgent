import { randomUUID } from "crypto";
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { BffError } from "@sparkops/contracts";
import {
  apiPathEntitlement,
  bffPathToApiPath,
  isModuleEnabled,
} from "./entitlements";
import {
  invalidateEntitlementsCache,
  resolveWorkspaceEntitlements,
} from "./entitlements-cache";
import {
  createAgentUpstream,
  type AgentUpstream,
  createIdentityUpstream,
  type IdentityUpstream,
  createTenantUpstream,
  type TenantUpstream,
  createWorkspaceUpstream,
  type WorkspaceUpstream,
  createRuntimeUpstream,
  type RuntimeUpstream,
  createGovernanceUpstream,
  type GovernanceUpstream,
  createIntegrationUpstream,
  type IntegrationUpstream,
  createWorkflowUpstream,
  type WorkflowUpstream,
  createKnowledgeUpstream,
  type KnowledgeUpstream,
  createNotificationUpstream,
  type NotificationUpstream,
  createDeliveryUpstream,
  type DeliveryUpstream,
  createClusterUpstream,
  type ClusterUpstream,
  createPlatformUpstream,
  type PlatformUpstream,
  createProductUpstream,
  type ProductUpstream,
  createSecretUpstream,
  type SecretUpstream,
} from "@sparkops/upstream";

export const AUTH_COOKIE = "sparkops_token";
export const WORKSPACE_COOKIE = "sparkops_workspace_id";

export interface BffContext {
  traceId: string;
  token?: string;
  workspaceId?: string;
  upstream: {
    identity: IdentityUpstream;
    tenant: TenantUpstream;
    workspace: WorkspaceUpstream;
    agent: AgentUpstream;
    runtime: RuntimeUpstream;
    governance: GovernanceUpstream;
    integration: IntegrationUpstream;
    workflow: WorkflowUpstream;
    knowledge: KnowledgeUpstream;
    notification: NotificationUpstream;
    delivery: DeliveryUpstream;
    cluster: ClusterUpstream;
    platform: PlatformUpstream;
    product: ProductUpstream;
    secret: SecretUpstream;
  };
}

export interface BffHandlerArgs {
  params?: Record<string, string>;
}

export type BffHandler<T = unknown> = (
  ctx: BffContext,
  req: NextRequest,
  args: BffHandlerArgs,
) => Promise<T>;

async function assertApiEntitlement(ctx: BffContext, apiPath: string, traceId: string) {
  const moduleKey = apiPathEntitlement(apiPath);
  if (!moduleKey) return;

  const entitlements = await resolveWorkspaceEntitlements(ctx.workspaceId, (wsId) =>
    ctx.upstream.platform.getWorkspaceEntitlements(wsId),
  );
  if (!isModuleEnabled(entitlements, moduleKey)) {
    throw new BffError(
      `feature module disabled: ${moduleKey}`,
      "ENTITLEMENT_MODULE_DISABLED",
      403,
      traceId,
    );
  }
}

async function assertBffEntitlement(ctx: BffContext, req: NextRequest, traceId: string) {
  await assertApiEntitlement(ctx, bffPathToApiPath(req.nextUrl.pathname), traceId);
}

export {
  apiPathEntitlement,
  bffPathEntitlement,
  bffPathToApiPath,
  DEFAULT_ENTITLEMENTS,
  isModuleEnabled,
} from "./entitlements";
export type { EntitlementModuleKey } from "./entitlements";
export {
  configureEntitlementsCacheTtlSeconds,
  getEntitlementsCacheTtlMs,
  isEntitlementsCacheEnabled,
  readEntitlementsCacheTtlMs,
} from "./entitlements-cache-config";
export {
  getCachedEntitlements,
  invalidateEntitlementsCache,
  resolveWorkspaceEntitlements,
} from "./entitlements-cache";

export function createTraceId(req: NextRequest): string {
  return req.headers.get("x-trace-id") ?? randomUUID();
}

export async function createBffContext(req: NextRequest): Promise<BffContext> {
  const traceId = createTraceId(req);
  const token = req.cookies.get(AUTH_COOKIE)?.value;
  const workspaceId =
    req.headers.get("x-workspace-id") ??
    req.cookies.get(WORKSPACE_COOKIE)?.value ??
    undefined;
  return buildBffContext({ traceId, token, workspaceId });
}

export function createBffContextFromToken(token?: string, workspaceId?: string): BffContext {
  return buildBffContext({ traceId: randomUUID(), token, workspaceId });
}

function buildBffContext(input: {
  traceId: string;
  token?: string;
  workspaceId?: string;
}): BffContext {
  const baseOptions = {
    traceId: input.traceId,
    token: input.token,
    workspaceId: input.workspaceId,
  };
  return {
    traceId: input.traceId,
    token: input.token,
    workspaceId: input.workspaceId,
    upstream: {
      identity: createIdentityUpstream(baseOptions),
      tenant: createTenantUpstream(baseOptions),
      workspace: createWorkspaceUpstream(baseOptions),
      agent: createAgentUpstream(baseOptions),
      runtime: createRuntimeUpstream(baseOptions),
      governance: createGovernanceUpstream(baseOptions),
      integration: createIntegrationUpstream(baseOptions),
      workflow: createWorkflowUpstream(baseOptions),
      knowledge: createKnowledgeUpstream(baseOptions),
      notification: createNotificationUpstream(baseOptions),
      delivery: createDeliveryUpstream(baseOptions),
      cluster: createClusterUpstream(baseOptions),
      platform: createPlatformUpstream(baseOptions),
      product: createProductUpstream(baseOptions),
      secret: createSecretUpstream(baseOptions),
    },
  };
}

export function ok<T>(data: T, traceId: string, status = 200) {
  return NextResponse.json(
    { code: 0, message: "ok", data, trace_id: traceId },
    { status },
  );
}

export function fail(err: unknown, traceId: string) {
  if (err instanceof BffError) {
    return NextResponse.json(
      {
        code: 1,
        message: err.message,
        data: { error_code: err.code },
        trace_id: err.traceId ?? traceId,
      },
      { status: err.status },
    );
  }
  const message = err instanceof Error ? err.message : "internal error";
  return NextResponse.json(
    {
      code: 1,
      message,
      data: { error_code: "INTERNAL_ERROR" },
      trace_id: traceId,
    },
    { status: 500 },
  );
}

export function createBffHandler(handler: BffHandler) {
  return async (req: NextRequest, context: { params: Promise<Record<string, string>> }) => {
    const traceId = createTraceId(req);
    try {
      const ctx = await createBffContext(req);
      await assertBffEntitlement(ctx, req, traceId);
      const params = await context.params;
      const data = await handler(ctx, req, { params });
      return ok(data, traceId);
    } catch (err) {
      return fail(err, traceId);
    }
  };
}

export function setAuthCookie(res: NextResponse, token: string) {
  res.cookies.set(AUTH_COOKIE, token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 24,
  });
}

export function clearAuthCookie(res: NextResponse) {
  res.cookies.set(AUTH_COOKIE, "", {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 0,
  });
}

export function clearWorkspaceCookie(res: NextResponse) {
  res.cookies.set(WORKSPACE_COOKIE, "", {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 0,
  });
}

export function setWorkspaceCookie(res: NextResponse, workspaceId: string) {
  res.cookies.set(WORKSPACE_COOKIE, workspaceId, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });
}

export async function proxyUpstreamStream(ctx: BffContext, path: string): Promise<Response> {
  if (!ctx.token) {
    throw new BffError("unauthorized", "AUTH_UNAUTHORIZED", 401, ctx.traceId);
  }
  await assertApiEntitlement(ctx, path, ctx.traceId);
  const { buildUpstreamStreamHeaders, getUpstreamBaseUrl } = await import("@sparkops/upstream");
  const upstream = await fetch(`${getUpstreamBaseUrl()}${path}`, {
    headers: buildUpstreamStreamHeaders({
      token: ctx.token,
      workspaceId: ctx.workspaceId,
      traceId: ctx.traceId,
    }),
    cache: "no-store",
  });

  if (!upstream.ok || !upstream.body) {
    const text = await upstream.text();
    throw new BffError(text || "stream failed", "UPSTREAM_ERROR", upstream.status, ctx.traceId);
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
    },
  });
}
