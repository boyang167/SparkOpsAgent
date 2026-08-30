import { ApiEnvelope, BffError } from "@sparkops/contracts";

export interface UpstreamRequestOptions {
  token?: string;
  workspaceId?: string;
  traceId?: string;
}

export interface UpstreamRawFetchInit extends RequestInit {
  options?: UpstreamRequestOptions;
}

export function getUpstreamBaseUrl(): string {
  return process.env.UPSTREAM_URL ?? "http://localhost:8080";
}

export function buildUpstreamRequestHeaders(
  headersInit: HeadersInit = {},
  options: UpstreamRequestOptions = {},
): Headers {
  const headers = new Headers(headersInit);
  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }
  if (options.workspaceId) {
    headers.set("X-Workspace-Id", options.workspaceId);
  }
  if (options.traceId) {
    headers.set("X-Trace-Id", options.traceId);
  }
  return headers;
}

export async function upstreamRawFetch(
  path: string,
  init: UpstreamRawFetchInit = {},
): Promise<Response> {
  if (!path.startsWith("/") || path.startsWith("//")) {
    throw new BffError("invalid upstream path", "INVALID_REQUEST", 400, init.options?.traceId);
  }

  const { options, ...rest } = init;
  try {
    return await fetch(`${getUpstreamBaseUrl()}${path}`, {
      ...rest,
      headers: buildUpstreamRequestHeaders(rest.headers, options),
      cache: "no-store",
    });
  } catch {
    throw new BffError("upstream unavailable", "UPSTREAM_UNAVAILABLE", 502, options?.traceId);
  }
}

export async function upstreamFetch<T>(
  path: string,
  init: RequestInit & { options?: UpstreamRequestOptions } = {},
): Promise<T> {
  const { options, ...rest } = init;
  const headers = new Headers(rest.headers);
  const hasMultipartBody =
    typeof FormData !== "undefined" && rest.body instanceof FormData;
  if (!headers.has("Content-Type") && rest.body && !hasMultipartBody) {
    headers.set("Content-Type", "application/json");
  }
  if (options?.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }
  if (options?.workspaceId) {
    headers.set("X-Workspace-Id", options.workspaceId);
  }
  if (options?.traceId) {
    headers.set("X-Trace-Id", options.traceId);
  }

  let res: Response;
  try {
    res = await fetch(`${getUpstreamBaseUrl()}${path}`, {
      ...rest,
      headers,
      cache: "no-store",
    });
  } catch {
    throw new BffError("upstream unavailable", "UPSTREAM_UNAVAILABLE", 503);
  }

  let body: ApiEnvelope<T>;
  try {
    body = (await res.json()) as ApiEnvelope<T>;
  } catch {
    throw new BffError("upstream returned invalid response", "UPSTREAM_ERROR", res.status || 502);
  }
  if (!res.ok || body.code !== 0) {
    const errorCode =
      typeof body.data === "object" &&
      body.data !== null &&
      "error_code" in body.data
        ? String((body.data as { error_code?: string }).error_code)
        : "UPSTREAM_ERROR";
    throw new BffError(body.message || "upstream request failed", errorCode, res.status, body.trace_id);
  }
  return body.data as T;
}
