export interface KubernetesPluginFetchOptions extends RequestInit {
  workspaceId?: string;
}

const KUBERNETES_PLUGIN_BFF_PREFIX = "/api/bff/v1/plugins/kubernetes";
const REQUEST_HEADER_ALLOWLIST = new Set(["accept", "content-type"]);

function assertSafeSegment(segment: string) {
  const lower = segment.toLowerCase();
  if (
    !segment ||
    segment === "." ||
    segment === ".." ||
    segment.includes("/") ||
    segment.includes("\\") ||
    segment.includes("\u0000") ||
    segment.includes("://") ||
    /^[a-z][a-z0-9+.-]*:$/i.test(segment) ||
    /%(?:00|2e|2f|5c)/i.test(lower)
  ) {
    throw new Error("invalid kubernetes plugin path");
  }
}

function buildRequestHeaders(headersInit: HeadersInit = {}, workspaceId?: string) {
  const headers = new Headers();
  new Headers(headersInit).forEach((value, key) => {
    const lower = key.toLowerCase();
    if (REQUEST_HEADER_ALLOWLIST.has(lower) || lower.startsWith("if-")) {
      headers.set(key, value);
    }
  });
  if (workspaceId) {
    headers.set("X-Workspace-Id", workspaceId);
  }
  return headers;
}

export function buildKubernetesPluginBffPath(segments: readonly string[], search = "") {
  if (segments.length === 0 || (search && !search.startsWith("?"))) {
    throw new Error("invalid kubernetes plugin path");
  }
  return `${KUBERNETES_PLUGIN_BFF_PREFIX}/${segments
    .map((segment) => {
      assertSafeSegment(segment);
      return encodeURIComponent(segment);
    })
    .join("/")}${search}`;
}

export function kubernetesPluginFetch(
  segments: readonly string[],
  options: KubernetesPluginFetchOptions = {},
) {
  const { workspaceId, headers, ...rest } = options;
  return fetch(buildKubernetesPluginBffPath(segments), {
    ...rest,
    headers: buildRequestHeaders(headers, workspaceId),
    credentials: "include",
  });
}
