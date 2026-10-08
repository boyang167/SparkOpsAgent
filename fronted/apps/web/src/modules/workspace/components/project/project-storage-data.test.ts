import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearProjectStorageCache,
  invalidateProjectStorage,
  loadProjectStorage,
  projectStorageKey,
  readProjectStorageCache,
} from "./project-storage-data";

const emptyOverview = { items: [], classes: [], warnings: [] };

describe("project storage data", () => {
  beforeEach(() => {
    clearProjectStorageCache();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-08T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("deduplicates concurrent requests", async () => {
    let resolveRequest: (value: typeof emptyOverview) => void = () => undefined;
    const fetcher = vi.fn(
      () =>
        new Promise<typeof emptyOverview>((resolve) => {
          resolveRequest = resolve;
        }),
    );
    const key = projectStorageKey("ws", "project");

    const first = loadProjectStorage(key, fetcher);
    const second = loadProjectStorage(key, fetcher);
    resolveRequest(emptyOverview);

    await expect(first).resolves.toEqual(emptyOverview);
    await expect(second).resolves.toEqual(emptyOverview);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("returns fresh cache and refetches after the TTL", async () => {
    const fetcher = vi.fn().mockResolvedValue(emptyOverview);
    const key = projectStorageKey("ws", "project");

    await loadProjectStorage(key, fetcher);
    await loadProjectStorage(key, fetcher);
    expect(fetcher).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(30_001);
    await loadProjectStorage(key, fetcher);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("supports forced refresh and invalidation", async () => {
    const fetcher = vi.fn().mockResolvedValue(emptyOverview);
    const key = projectStorageKey("ws", "project");

    await loadProjectStorage(key, fetcher);
    await loadProjectStorage(key, fetcher, { force: true });
    expect(fetcher).toHaveBeenCalledTimes(2);

    invalidateProjectStorage(key);
    expect(readProjectStorageCache(key)).toBeNull();
  });
});
