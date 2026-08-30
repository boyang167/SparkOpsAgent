export type ApiErrorInfo = {
  status?: number;
  code?: string;
  message: string;
  detail?: string;
};

export class KubernetesApiError extends Error {
  status?: number;
  code?: string;
  detail?: string;

  constructor(info: ApiErrorInfo) {
    super(info.message);
    this.name = "KubernetesApiError";
    this.status = info.status;
    this.code = info.code;
    this.detail = info.detail;
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
  if (contentType.includes("application/json")) return (await res.json()) as T;
  return (await res.text()) as T;
}

export async function parseApiError(res: Response): Promise<ApiErrorInfo> {
  const status = res.status;
  const fallback = res.statusText || "请求失败";
  const text = await res.text().catch(() => "");
  if (!text) return { status, message: fallback };
  try {
    const body = JSON.parse(text) as Record<string, unknown>;
    const code = stringValue(body.code) ?? stringValue(body.error_code);
    const message =
      stringValue(body.message) ??
      stringValue(body.error) ??
      stringValue(body.reason) ??
      stringValue(body.status) ??
      fallback;
    const detail = stringValue(body.detail) ?? stringValue(body.details) ?? text;
    return { status, code, message, detail };
  } catch {
    return { status, message: text };
  }
}

export function errorInfo(err: unknown, fallback: string): ApiErrorInfo {
  if (err instanceof KubernetesApiError) {
    return { status: err.status, code: err.code, message: err.message, detail: err.detail };
  }
  if (err instanceof Error) return { message: err.message };
  return { message: fallback };
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}
