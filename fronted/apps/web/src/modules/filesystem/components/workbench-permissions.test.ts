import { describe, expect, it } from "vitest";
import { resolveWorkbenchPermissions } from "./workbench-permissions";

describe("resolveWorkbenchPermissions", () => {
  it("defaults to file read when workspace permissions are unavailable", () => {
    expect(resolveWorkbenchPermissions(undefined, false)).toEqual(["file:read"]);
  });

  it("preserves real workspace permissions", () => {
    expect(resolveWorkbenchPermissions(["storage:read"], false)).toEqual([
      "file:read",
      "storage:read",
    ]);
  });

  it("grants management tabs to an explicit super admin", () => {
    expect(resolveWorkbenchPermissions(undefined, true)).toEqual([
      "file:read",
      "storage:read",
      "file:admin",
    ]);
  });
});
