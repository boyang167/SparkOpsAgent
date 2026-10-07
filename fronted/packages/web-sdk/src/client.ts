import { ApiEnvelope, BffError } from "@sparkops/contracts";

const BFF_BASE = "/api/bff/v1";

export interface BffFetchOptions extends RequestInit {
  workspaceId?: string;
}

function isAbortError(err: unknown) {
  return err instanceof DOMException && err.name === "AbortError";
}

/** Browsers surface dropped connections as bare TypeErrors (e.g. "Error in input stream"). */
function toNetworkError(err: unknown, status = 0) {
  if (isAbortError(err)) return err;
  return new BffError("网络连接中断，请稍后重试", "BFF_NETWORK_ERROR", status);
}

export async function bffFetch<T>(path: string, options: BffFetchOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const hasMultipartBody =
    typeof FormData !== "undefined" && options.body instanceof FormData;
  if (!headers.has("Content-Type") && options.body && !hasMultipartBody) {
    headers.set("Content-Type", "application/json");
  }
  if (options.workspaceId) {
    headers.set("X-Workspace-Id", options.workspaceId);
  }

  let res: Response;
  try {
    res = await fetch(`${BFF_BASE}${path}`, {
      ...options,
      headers,
      credentials: "include",
    });
  } catch (err) {
    throw toNetworkError(err);
  }

  let body: ApiEnvelope<T>;
  try {
    body = (await res.json()) as ApiEnvelope<T>;
  } catch (err) {
    if (isAbortError(err)) throw err;
    if (err instanceof SyntaxError) {
      throw new BffError("服务返回了无法解析的响应", "BFF_INVALID_RESPONSE", res.status || 502);
    }
    throw toNetworkError(err, res.status);
  }
  if (!res.ok || body.code !== 0) {
    const errorCode =
      typeof body.data === "object" &&
      body.data !== null &&
      "error_code" in body.data
        ? String((body.data as { error_code?: string }).error_code)
        : "BFF_ERROR";
    throw new BffError(body.message || "bff request failed", errorCode, res.status, body.trace_id);
  }
  return body.data as T;
}
