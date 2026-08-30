export type ApiErrorInfo = {
  status?: number;
  code?: string;
  message: string;
  detail?: string;
  traceId?: string;
};

export class KubernetesApiError extends Error {
  status?: number;
  code?: string;
  detail?: string;
  traceId?: string;

  constructor(info: ApiErrorInfo) {
    super(info.message);
    this.name = "KubernetesApiError";
    this.status = info.status;
    this.code = info.code;
    this.detail = info.detail;
    this.traceId = info.traceId;
  }
}

export async function k8sFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new KubernetesApiError(await parseApiError(res));
  }
  if (res.status === 204) return undefined as T;
  const contentType = res.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) return unwrapBffResponse((await res.json()) as unknown) as T;
  return (await res.text()) as T;
}

export async function parseApiError(res: Response): Promise<ApiErrorInfo> {
  const status = res.status;
  const fallback = res.statusText || "请求失败";
  const contentType = res.headers.get("content-type") ?? "";
  const text = await res.text().catch(() => "");
  if (!text) return { status, message: fallback };
  if (!contentType.includes("application/json")) {
    return { status, message: fallback, detail: text };
  }
  try {
    const body = JSON.parse(text) as Record<string, unknown>;
    const data = objectValue(body.data);
    const code = stringValue(body.code) ?? stringValue(body.error_code) ?? stringValue(data?.error_code) ?? stringValue(data?.code);
    const message =
      stringValue(body.message) ??
      stringValue(body.error) ??
      stringValue(data?.message) ??
      stringValue(data?.error) ??
      stringValue(body.reason) ??
      stringValue(body.status) ??
      fallback;
    const detail = stringValue(body.detail) ?? stringValue(body.details) ?? stringValue(data?.message) ?? text;
    const traceId = stringValue(body.trace_id) ?? stringValue(body.traceId);
    return { status, code, message, detail, traceId };
  } catch {
    return { status, message: fallback, detail: text };
  }
}

export function errorInfo(err: unknown, fallback: string): ApiErrorInfo {
  if (err instanceof KubernetesApiError) {
    return { status: err.status, code: err.code, message: err.message, detail: err.detail, traceId: err.traceId };
  }
  if (err instanceof Error) return { message: err.message };
  return { message: fallback };
}

export function unwrapBffResponse(body: unknown): unknown {
  const envelope = objectValue(body);
  if (!envelope) return body;
  if ("data" in envelope && ("code" in envelope || "message" in envelope || "trace_id" in envelope)) {
    return envelope.data;
  }
  return body;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}
