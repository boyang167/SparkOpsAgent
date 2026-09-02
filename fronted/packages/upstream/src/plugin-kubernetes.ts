import type { UpstreamRawFetchInit, UpstreamRequestOptions } from "./core";
import { upstreamRawFetch } from "./core";

export type KubernetesPluginApiPath = `/api/v1/plugins/kubernetes/${string}`;

const KUBERNETES_PLUGIN_API_PREFIX = "/api/v1/plugins/kubernetes";

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

function looksLikeServiceProxySegment(segment: string): boolean {
  return /^(https?):([^:]+):([0-9]+)$/.test(segment);
}

function isDNS1123Name(value: string) {
  return value.length > 0 && value.length <= 253 && value.split(".").every((label) =>
    label.length > 0 && label.length <= 63 && /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/.test(label)
  );
}

export function buildKubernetesPluginApiPath(
  segments: readonly string[],
  search = "",
): KubernetesPluginApiPath {
  if (segments.length === 0 || (search && !search.startsWith("?"))) {
    throw invalidPath();
  }
  const encoded: string[] = [];
  for (const segment of segments) {
    encoded.push(
      encoded.at(-1) === "services" && looksLikeServiceProxySegment(segment)
        ? normalizeServiceProxySegment(segment)
        : encodeURIComponent(normalizeSegment(segment)),
    );
  }
  return `${KUBERNETES_PLUGIN_API_PREFIX}/${encoded.join("/")}${search}`;
}

export function createKubernetesPluginUpstream(options: UpstreamRequestOptions = {}) {
  return {
    request(segments: readonly string[], init: Omit<UpstreamRawFetchInit, "options"> = {}) {
      return upstreamRawFetch(buildKubernetesPluginApiPath(segments), {
        ...init,
        options,
      });
    },
  };
}

export type KubernetesPluginUpstream = ReturnType<typeof createKubernetesPluginUpstream>;
