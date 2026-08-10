import { ApiEnvelope, BffError } from "@sparkops/contracts";

const BFF_BASE = "/api/bff/v1";

export interface BffFetchOptions extends RequestInit {
  workspaceId?: string;
}

export async function bffFetch<T>(path: string, options: BffFetchOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  if (options.workspaceId) {
    headers.set("X-Workspace-Id", options.workspaceId);
  }

  const res = await fetch(`${BFF_BASE}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });

  const body = (await res.json()) as ApiEnvelope<T>;
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
