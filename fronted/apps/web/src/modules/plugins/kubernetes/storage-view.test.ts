import { describe, expect, it } from "vitest";
import {
  emptyStorageTemplate,
  formatAccessModes,
  storagePhaseBadge,
  storageTabLabel,
} from "./storage-view";

describe("storage view", () => {
  it("labels storage tabs", () => {
    expect(storageTabLabel("PersistentVolumeClaim")).toBe("Persistent VolumeClaims");
    expect(storageTabLabel("StorageClass")).toBe("Storage Class");
  });

  it("formats phase badges", () => {
    expect(storagePhaseBadge("Bound")).toEqual({ text: "Bound", tone: "ok" });
    expect(storagePhaseBadge("Pending")).toEqual({ text: "Pending", tone: "bad" });
  });

  it("formats access modes", () => {
    expect(formatAccessModes(["ReadWriteOnce", "ReadOnlyMany"])).toBe("ReadWriteOnce, ReadOnlyMany");
  });

  it("creates pvc template", () => {
    expect(emptyStorageTemplate("PersistentVolumeClaim", "default")).toMatchObject({
      kind: "PersistentVolumeClaim",
      metadata: { namespace: "default" },
    });
  });
});
