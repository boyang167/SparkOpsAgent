import type { ProjectStorageOverviewVM } from "@sparkops/contracts/cluster";

const PROJECT_STORAGE_TTL_MS = 30_000;

interface ProjectStorageCacheEntry {
  value: ProjectStorageOverviewVM;
  writtenAt: number;
}

const cache = new Map<string, ProjectStorageCacheEntry>();
const inflight = new Map<string, Promise<ProjectStorageOverviewVM>>();

export function projectStorageKey(workspaceId: string, projectId: string) {
  return `${workspaceId}:${projectId}`;
}

export function readProjectStorageCache(key: string) {
  return cache.get(key)?.value ?? null;
}

export function invalidateProjectStorage(key: string) {
  cache.delete(key);
}

export function clearProjectStorageCache() {
  cache.clear();
  inflight.clear();
}

export function loadProjectStorage(
  key: string,
  fetcher: () => Promise<ProjectStorageOverviewVM>,
  options: { force?: boolean } = {},
) {
  const cached = cache.get(key);
  if (
    !options.force &&
    cached &&
    Date.now() - cached.writtenAt < PROJECT_STORAGE_TTL_MS
  ) {
    return Promise.resolve(cached.value);
  }

  const pending = inflight.get(key);
  if (pending) return pending;

  const request = fetcher()
    .then((value) => {
      cache.set(key, { value, writtenAt: Date.now() });
      return value;
    })
    .finally(() => {
      inflight.delete(key);
    });
  inflight.set(key, request);
  return request;
}
