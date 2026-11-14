import { describe, expect, it } from "vitest";
import {
  hasFilesystemPageAccess,
  resolveFilesystemAuditMode,
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

  it("uses authoritative organization permissions for approval management", () => {
    expect(resolveWorkbenchPermissions(["file:read"], false, ["org:file:admin"])).toEqual([
      "file:read",
      "org:file:admin",
    ]);
    expect(resolveWorkbenchPermissions(["file:read", "file:admin"], false)).not.toContain(
      "org:file:admin",
    );
  });
});

describe("resolveFilesystemAuditMode", () => {
  it("uses authoritative organization permission before workspace permission", () => {
    expect(resolveFilesystemAuditMode(["file:read", "org:file:admin"])).toBe("organization");
    expect(resolveFilesystemAuditMode(["file:read", "file:admin", "org:file:admin"])).toBe(
      "organization",
    );
  });

  it("uses workspace audit only for workspace file administrators", () => {
    expect(resolveFilesystemAuditMode(["file:read", "file:admin"])).toBe("workspace");
    expect(resolveFilesystemAuditMode(["file:read"])).toBeUndefined();
  });
});

describe("hasFilesystemPageAccess", () => {
  it("requires file read for page access", () => {
    expect(hasFilesystemPageAccess([])).toBe(false);
    expect(hasFilesystemPageAccess(["file:write"])).toBe(false);
    expect(hasFilesystemPageAccess(["file:read"])).toBe(true);
  });
});
