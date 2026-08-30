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

export function buildKubernetesPluginApiPath(
  segments: readonly string[],
  search = "",
): KubernetesPluginApiPath {
  if (segments.length === 0 || (search && !search.startsWith("?"))) {
    throw invalidPath();
  }
  return `${KUBERNETES_PLUGIN_API_PREFIX}/${segments
    .map((segment) => encodeURIComponent(normalizeSegment(segment)))
    .join("/")}${search}`;
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
