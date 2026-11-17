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
  createFilesystemUpstream,
  type FilesystemUpstream,
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
  createMcpUpstream,
  type McpUpstream,
  createPluginUpstream,
  type PluginUpstream,
  createKubernetesPluginUpstream,
  type KubernetesPluginUpstream,
  getUpstreamBaseUrl,
  upstreamRawFetch,
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
    filesystem: FilesystemUpstream;
    notification: NotificationUpstream;
    delivery: DeliveryUpstream;
    cluster: ClusterUpstream;
    platform: PlatformUpstream;
    product: ProductUpstream;
    secret: SecretUpstream;
    mcp: McpUpstream;
    plugin: PluginUpstream;
    kubernetesPlugin: KubernetesPluginUpstream;
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

const KUBERNETES_PLUGIN_API_PREFIX = "/api/v1/plugins/kubernetes";
const KUBERNETES_PLUGIN_BFF_PREFIX = "/api/bff/v1/plugins/kubernetes";
const MAX_PROXY_BODY_BYTES = 50 * 1024 * 1024;
const REQUEST_HEADER_ALLOWLIST = new Set(["accept", "content-type"]);
const RESPONSE_HEADER_ALLOWLIST = new Set([
  "content-type",
  "content-disposition",
  "cache-control",
  "etag",
  "last-modified",
  "x-accel-buffering",
]);
const HTML_SANDBOX_CSP = "sandbox allow-scripts allow-forms allow-popups allow-downloads";

function invalidKubernetesPath(message = "invalid kubernetes plugin path") {
  return new BffError(message, "INVALID_REQUEST", 400);
}

function assertSafeKubernetesSegmentLayer(segment: string) {
  if (
    !segment ||
    segment === "." ||
    segment === ".." ||
    segment.includes("/") ||
    segment.includes("\\") ||
    segment.includes("\u0000") ||
    segment.includes("://") ||
    /^[a-z][a-z0-9+.-]*:/i.test(segment)
  ) {
    throw invalidKubernetesPath();
  }
}

function normalizeKubernetesSegment(segment: string): string {
  let current = segment;
  for (let depth = 0; depth < 3; depth += 1) {
    assertSafeKubernetesSegmentLayer(current);

    let decoded: string;
    try {
      decoded = decodeURIComponent(current);
    } catch {
      throw invalidKubernetesPath();
    }
    if (decoded === current) {
      return current;
    }
    current = decoded;
  }

  assertSafeKubernetesSegmentLayer(current);
  if (/%[0-9a-fA-F]{2}/.test(current)) {
    throw invalidKubernetesPath();
  }
  return current;
}

function normalizeKubernetesServiceProxySegment(segment: string): string {
  if (segment.includes("%") || segment.includes("/") || segment.includes("\\") || segment.includes("@")) {
    throw invalidKubernetesPath();
  }
  const match = /^(https?):([^:]+):([0-9]+)$/.exec(segment);
  if (!match) throw invalidKubernetesPath();
  const [, scheme, name, rawPort] = match;
  const port = Number(rawPort);
  if (!name || name.length > 253 || !name.split(".").every((label) =>
    label.length > 0 &&
    label.length <= 63 &&
    /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/.test(label)
  ) || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw invalidKubernetesPath();
  }
  return `${scheme}:${name}:${port}`;
}

function looksLikeKubernetesServiceProxySegment(segment: string): boolean {
  return /^(https?):([^:]+):([0-9]+)$/.test(segment);
}

export function buildKubernetesPluginApiPath(
  segments: readonly string[],
  search = "",
): `/api/v1/plugins/kubernetes/${string}` {
  if (segments.length === 0) {
    throw invalidKubernetesPath();
  }
  if (search && !search.startsWith("?")) {
    throw invalidKubernetesPath("invalid kubernetes plugin query");
  }
  const encoded: string[] = [];
  for (const segment of segments) {
    if (encoded.at(-1) === "services" && looksLikeKubernetesServiceProxySegment(segment)) {
      encoded.push(normalizeKubernetesServiceProxySegment(segment));
    } else {
      encoded.push(encodeURIComponent(normalizeKubernetesSegment(segment)));
    }
  }
  return `${KUBERNETES_PLUGIN_API_PREFIX}/${encoded.join("/")}${search}`;
}

function buildProxyRequestHeaders(headers: Headers): Headers {
  const forwarded = new Headers();
  headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (REQUEST_HEADER_ALLOWLIST.has(lower) || lower.startsWith("if-")) {
      forwarded.set(key, value);
    }
  });
  return forwarded;
}

function rewriteRedirectLocation(
  headers: Headers,
  upstreamPath: string,
  status: number,
  traceId: string,
): string | undefined {
  if (status < 300 || status >= 400) {
    return undefined;
  }
  const location = headers.get("location");
  if (!location) {
    return undefined;
  }

  let parsed: URL;
  let upstreamUrl: URL;
  try {
    upstreamUrl = new URL(upstreamPath, getUpstreamBaseUrl());
    parsed = new URL(location, upstreamUrl);
  } catch {
    throw new BffError("invalid upstream redirect", "UPSTREAM_REDIRECT_INVALID", 502, traceId);
  }
  if (
    parsed.origin !== upstreamUrl.origin ||
    !parsed.pathname.startsWith(`${KUBERNETES_PLUGIN_API_PREFIX}/`)
  ) {
    throw new BffError("invalid upstream redirect", "UPSTREAM_REDIRECT_INVALID", 502, traceId);
  }

  const suffix = parsed.pathname.slice(KUBERNETES_PLUGIN_API_PREFIX.length);
  return `${KUBERNETES_PLUGIN_BFF_PREFIX}${suffix}${parsed.search}`;
}

function buildProxyResponseHeaders(
  headers: Headers,
  upstreamPath: string,
  status: number,
  traceId: string,
): Headers {
  const forwarded = new Headers();
  headers.forEach((value, key) => {
    if (RESPONSE_HEADER_ALLOWLIST.has(key.toLowerCase())) {
      forwarded.set(key, value);
    }
  });
  const rewrittenLocation = rewriteRedirectLocation(headers, upstreamPath, status, traceId);
  if (rewrittenLocation) {
    forwarded.set("Location", rewrittenLocation);
  }
  if (headers.get("content-type")?.toLowerCase().includes("text/html")) {
    forwarded.set("Content-Security-Policy", HTML_SANDBOX_CSP);
    forwarded.set("X-Content-Type-Options", "nosniff");
    forwarded.set("Referrer-Policy", "no-referrer");
  }
  return forwarded;
}

async function readBoundedBody(req: Request, traceId: string): Promise<BodyInit | undefined> {
  if (req.method === "GET" || req.method === "HEAD" || !req.body) {
    return undefined;
  }

  const contentLength = req.headers.get("content-length");
  if (contentLength && Number(contentLength) > MAX_PROXY_BODY_BYTES) {
    throw new BffError("request body too large", "PAYLOAD_TOO_LARGE", 413, traceId);
  }

  const reader = req.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_PROXY_BODY_BYTES) {
      throw new BffError("request body too large", "PAYLOAD_TOO_LARGE", 413, traceId);
    }
    chunks.push(value);
  }

  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

export async function assertBffEntitlement(ctx: BffContext, req: NextRequest, traceId: string) {
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
      filesystem: createFilesystemUpstream(baseOptions),
      notification: createNotificationUpstream(baseOptions),
      delivery: createDeliveryUpstream(baseOptions),
      cluster: createClusterUpstream(baseOptions),
      platform: createPlatformUpstream(baseOptions),
      product: createProductUpstream(baseOptions),
      secret: createSecretUpstream(baseOptions),
      mcp: createMcpUpstream(baseOptions),
      plugin: createPluginUpstream(baseOptions),
      kubernetesPlugin: createKubernetesPluginUpstream(baseOptions),
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

export type BffRawHandler = (
  ctx: BffContext,
  req: NextRequest,
  args: BffHandlerArgs,
) => Promise<NextResponse>;

export function createBffRawHandler(handler: BffRawHandler) {
  return async (req: NextRequest, context: { params: Promise<Record<string, string>> }) => {
    const traceId = createTraceId(req);
    try {
      const ctx = await createBffContext(req);
      await assertBffEntitlement(ctx, req, traceId);
      const params = await context.params;
      return await handler(ctx, req, { params });
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

export async function proxyUpstreamStreamRequest(
  ctx: BffContext,
  path: `/api/v1/plugins/kubernetes/${string}`,
  req: Request,
): Promise<Response> {
  if (!ctx.token) {
    throw new BffError("unauthorized", "AUTH_UNAUTHORIZED", 401, ctx.traceId);
  }
  if (!ctx.workspaceId) {
    throw new BffError("workspace required", "WORKSPACE_REQUIRED", 400, ctx.traceId);
  }

  await assertApiEntitlement(ctx, path, ctx.traceId);
  const body = await readBoundedBody(req, ctx.traceId);
  const upstream = await upstreamRawFetch(path, {
    method: req.method,
    headers: buildProxyRequestHeaders(req.headers),
    body,
    signal: req.signal,
    options: {
      token: ctx.token,
      workspaceId: ctx.workspaceId,
      traceId: ctx.traceId,
    },
  });

  return new Response(req.method === "HEAD" ? null : upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: buildProxyResponseHeaders(upstream.headers, path, upstream.status, ctx.traceId),
  });
}
