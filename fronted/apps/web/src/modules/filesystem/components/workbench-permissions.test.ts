import { describe, expect, it } from "vitest";
import {
  hasFilesystemPageAccess,
  resolveWorkbenchPermissions,
} from "./workbench-permissions";

describe("resolveWorkbenchPermissions", () => {
  it("fails closed when workspace permissions are unavailable", () => {
    expect(resolveWorkbenchPermissions(undefined, false)).toEqual([]);
  });

  it("preserves real workspace permissions without inventing file read", () => {
    expect(resolveWorkbenchPermissions(["storage:read"], false)).toEqual(["storage:read"]);
  });

  it("grants explicit super admin access to the complete workbench", () => {
    expect(resolveWorkbenchPermissions(undefined, true)).toEqual([
      "file:read",
      "storage:read",
      "file:admin",
    ]);
  });
});

describe("hasFilesystemPageAccess", () => {
  it("requires file read for page access", () => {
    expect(hasFilesystemPageAccess([])).toBe(false);
    expect(hasFilesystemPageAccess(["file:write"])).toBe(false);
    expect(hasFilesystemPageAccess(["file:read"])).toBe(true);
  });
});
