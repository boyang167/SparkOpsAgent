export interface KubernetesPluginFetchOptions extends RequestInit {
  workspaceId?: string;
}

const KUBERNETES_PLUGIN_BFF_PREFIX = "/api/bff/v1/plugins/kubernetes";
const REQUEST_HEADER_ALLOWLIST = new Set(["accept", "content-type"]);

function invalidPath() {
  return new Error("invalid kubernetes plugin path");
}

function assertSafeSegmentLayer(segment: string) {
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
    throw invalidPath();
  }
}

function normalizeSegment(segment: string): string {
  let current = segment;
  for (let depth = 0; depth < 3; depth += 1) {
    assertSafeSegmentLayer(current);
    let decoded: string;
    try {
      decoded = decodeURIComponent(current);
    } catch {
      throw invalidPath();
    }
    if (decoded === current) {
      return current;
    }
    current = decoded;
  }

  assertSafeSegmentLayer(current);
  if (/%[0-9a-fA-F]{2}/.test(current)) {
    throw invalidPath();
  }
  return current;
}

function normalizeServiceProxySegment(segment: string): string {
  if (segment.includes("%") || segment.includes("/") || segment.includes("\\") || segment.includes("@")) throw invalidPath();
  const match = /^(https?):([^:]+):([0-9]+)$/.exec(segment);
  const port = Number(match?.[3]);
  const name = match?.[2] ?? "";
  if (!match || !isDNS1123Name(name) || !Number.isInteger(port) || port < 1 || port > 65535) throw invalidPath();
  return `${match[1]}:${name}:${port}`;
}

function isDNS1123Name(value: string) {
  return value.length > 0 && value.length <= 253 && value.split(".").every((label) =>
    label.length > 0 && label.length <= 63 && /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/.test(label)
  );
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
    throw invalidPath();
  }
  const encoded: string[] = [];
  for (const segment of segments) {
    encoded.push(encoded.at(-1) === "services"
      ? normalizeServiceProxySegment(segment)
      : encodeURIComponent(normalizeSegment(segment)));
  }
  return `${KUBERNETES_PLUGIN_BFF_PREFIX}/${encoded.join("/")}${search}`;
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
