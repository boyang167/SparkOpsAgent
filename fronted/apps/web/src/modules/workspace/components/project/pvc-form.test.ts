import { describe, expect, it } from "vitest";
import { composeStorage, isValidMountPath, isValidPvcName } from "./pvc-form";

describe("pvc-form", () => {
  it("accepts dns-style pvc names", () => {
    expect(isValidPvcName("test-repo")).toBe(true);
    expect(isValidPvcName("Test")).toBe(false);
  });

  it("requires absolute mount paths", () => {
    expect(isValidMountPath("/data")).toBe(true);
    expect(isValidMountPath("/")).toBe(false);
    expect(isValidMountPath("data")).toBe(false);
  });

  it("composes kubernetes storage quantities", () => {
    expect(composeStorage("1", "Gi")).toEqual({ storage: "1Gi", error: null });
    expect(composeStorage("512", "Mi").storage).toBe("512Mi");
    expect(composeStorage("0", "Gi").error).toMatch(/正数/);
  });
});
