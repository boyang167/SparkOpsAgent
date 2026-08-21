import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { DEFAULT_ENTITLEMENTS } from "@/components/layout/entitlements";

const DEFAULT_CACHE_TTL_MS = 30_000;

function readClientCacheTtlMs(): number {
  const raw = process.env.NEXT_PUBLIC_ENTITLEMENTS_CACHE_TTL_SECONDS;
  if (raw === undefined || raw.trim() === "") {
    return DEFAULT_CACHE_TTL_MS;
  }
  const sec = Number.parseInt(raw, 10);
  if (!Number.isFinite(sec) || sec <= 0) {
    return sec === 0 ? 0 : DEFAULT_CACHE_TTL_MS;
  }
  return sec * 1000;
}

function isClientCacheEnabled() {
  return readClientCacheTtlMs() > 0;
}

interface ClientCacheEntry {
  workspaceId: string;
  value: WorkspaceEntitlementsVM;
  expiresAt: number;
}

let clientCache: ClientCacheEntry | null = null;
let inflight: Promise<WorkspaceEntitlementsVM> | null = null;
let inflightWorkspaceId: string | null = null;

export function invalidateClientEntitlementsCache() {
  clientCache = null;
  inflight = null;
  inflightWorkspaceId = null;
}

export async function fetchWorkspaceEntitlementsCached(
  workspaceId: string,
  fetcher: () => Promise<WorkspaceEntitlementsVM>,
  options?: { fresh?: boolean },
): Promise<WorkspaceEntitlementsVM> {
  const ttlMs = readClientCacheTtlMs();

  if (options?.fresh || !isClientCacheEnabled()) {
    const value = await fetcher();
    if (isClientCacheEnabled()) {
      clientCache = { workspaceId, value, expiresAt: Date.now() + ttlMs };
    } else {
      clientCache = null;
    }
    inflight = null;
    inflightWorkspaceId = null;
    return value;
  }

  const now = Date.now();
  if (clientCache && clientCache.workspaceId === workspaceId && clientCache.expiresAt > now) {
    return clientCache.value;
  }

  if (inflight && inflightWorkspaceId === workspaceId) {
    return inflight;
  }

  inflightWorkspaceId = workspaceId;
  inflight = fetcher()
    .then((value) => {
      clientCache = { workspaceId, value, expiresAt: Date.now() + ttlMs };
      inflight = null;
      inflightWorkspaceId = null;
      return value;
    })
    .catch((err) => {
      inflight = null;
      inflightWorkspaceId = null;
      throw err;
    });

  return inflight;
}

export function getCachedClientEntitlements(
  workspaceId?: string,
): WorkspaceEntitlementsVM | null {
  if (!workspaceId || !clientCache || clientCache.workspaceId !== workspaceId) {
    return null;
  }
  if (clientCache.expiresAt <= Date.now()) {
    return null;
  }
  return clientCache.value;
}

export function getDefaultEntitlements(): WorkspaceEntitlementsVM {
  return DEFAULT_ENTITLEMENTS;
}
