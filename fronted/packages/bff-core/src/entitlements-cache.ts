import {
  WorkspaceEntitlementsVMSchema,
  type WorkspaceEntitlementsVM,
} from "@sparkops/contracts/platform";
import { DEFAULT_ENTITLEMENTS } from "./entitlements";
import {
  getEntitlementsCacheTtlMs,
  isEntitlementsCacheEnabled,
} from "./entitlements-cache-config";

interface CacheEntry {
  value: WorkspaceEntitlementsVM;
  expiresAt: number;
}

const cacheByWorkspace = new Map<string, CacheEntry>();
const inflightByWorkspace = new Map<string, Promise<WorkspaceEntitlementsVM>>();

function normalizeWorkspaceEntitlements(value: unknown): WorkspaceEntitlementsVM {
  const upstream = value && typeof value === "object" ? value : {};
  return WorkspaceEntitlementsVMSchema.parse({
    ...DEFAULT_ENTITLEMENTS,
    ...upstream,
  });
}

export function invalidateEntitlementsCache() {
  cacheByWorkspace.clear();
  inflightByWorkspace.clear();
}

export async function getCachedEntitlements(
  cacheKey: string,
  fetcher: () => Promise<WorkspaceEntitlementsVM>,
  options?: { fresh?: boolean },
): Promise<WorkspaceEntitlementsVM> {
  const ttlMs = getEntitlementsCacheTtlMs();

  if (options?.fresh || !isEntitlementsCacheEnabled()) {
    const value = normalizeWorkspaceEntitlements(await fetcher());
    if (isEntitlementsCacheEnabled()) {
      cacheByWorkspace.set(cacheKey, { value, expiresAt: Date.now() + ttlMs });
    } else {
      cacheByWorkspace.delete(cacheKey);
    }
    inflightByWorkspace.delete(cacheKey);
    return value;
  }

  const now = Date.now();
  const cached = cacheByWorkspace.get(cacheKey);
  if (cached && cached.expiresAt > now) {
    return cached.value;
  }

  const inflight = inflightByWorkspace.get(cacheKey);
  if (inflight) {
    return inflight;
  }

  const pending = fetcher()
    .then((value) => {
      const normalized = normalizeWorkspaceEntitlements(value);
      cacheByWorkspace.set(cacheKey, { value: normalized, expiresAt: Date.now() + ttlMs });
      inflightByWorkspace.delete(cacheKey);
      return normalized;
    })
    .catch((err) => {
      inflightByWorkspace.delete(cacheKey);
      throw err;
    });

  inflightByWorkspace.set(cacheKey, pending);
  return pending;
}

export async function resolveWorkspaceEntitlements(
  workspaceId: string | undefined,
  fetcher: (wsId: string) => Promise<WorkspaceEntitlementsVM>,
  options?: { fresh?: boolean },
): Promise<WorkspaceEntitlementsVM> {
  if (!workspaceId) {
    return DEFAULT_ENTITLEMENTS;
  }
  return getCachedEntitlements(workspaceId, () => fetcher(workspaceId), options);
}
