const DEFAULT_CACHE_TTL_SECONDS = 30;

function parseCacheTtlSeconds(raw: string | undefined): number {
  if (raw === undefined || raw.trim() === "") {
    return DEFAULT_CACHE_TTL_SECONDS;
  }
  const sec = Number.parseInt(raw, 10);
  if (!Number.isFinite(sec)) {
    return DEFAULT_CACHE_TTL_SECONDS;
  }
  return sec;
}

/** ENTITLEMENTS_CACHE_TTL_SECONDS — 0 disables cache. */
export function readEntitlementsCacheTtlMs(): number {
  const sec = parseCacheTtlSeconds(process.env.ENTITLEMENTS_CACHE_TTL_SECONDS);
  if (sec <= 0) return 0;
  return sec * 1000;
}

/** NEXT_PUBLIC_ENTITLEMENTS_CACHE_TTL_SECONDS — browser-side TTL. */
export function readPublicEntitlementsCacheTtlMs(): number {
  const sec = parseCacheTtlSeconds(process.env.NEXT_PUBLIC_ENTITLEMENTS_CACHE_TTL_SECONDS);
  if (sec <= 0) return 0;
  return sec * 1000;
}

let cacheTtlMs = readEntitlementsCacheTtlMs();

export function configureEntitlementsCacheTtlSeconds(seconds: number) {
  cacheTtlMs = seconds <= 0 ? 0 : seconds * 1000;
}

export function getEntitlementsCacheTtlMs() {
  return cacheTtlMs;
}

export function isEntitlementsCacheEnabled() {
  return cacheTtlMs > 0;
}
