import { describe, expect, it } from "vitest";
import {
  formatSyncResourceKinds,
  normalizeSyncResourceKinds,
  toggleSyncResourceKind,
} from "./environment-sync-resource-kinds";

describe("environment sync resource kinds", () => {
  it("defaults to all kinds when yaml sync is enabled", () => {
    expect(normalizeSyncResourceKinds([], true, false)).toEqual([
      "Deployment", "Service", "ConfigMap", "Ingress",
    ]);
  });

  it("formats selected kinds for display", () => {
    expect(formatSyncResourceKinds(["Deployment", "Service"])).toBe("Deployment、Service");
  });

  it("toggles kinds while preserving canonical order", () => {
    expect(toggleSyncResourceKind(["Deployment", "Service"], "Ingress", true)).toEqual([
      "Deployment", "Service", "Ingress",
    ]);
  });
});
