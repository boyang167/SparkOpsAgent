import { describe, expect, it } from "vitest";
import type { NamespaceSummaryVM } from "@sparkops/contracts/filesystem";
import {
  canDeleteGovernedFileSpace,
  canWriteGovernedFileSpace,
  hasFilesystemPageAccess,
  resolveFilesystemAuditMode,
  resolveWorkbenchPermissions,
} from "./workbench-permissions";

const baseSummary = {
  id: "ns-1",
  organization_id: "org-1",
  owner_user_id: null,
  root_node_id: "root-1",
  status: "ACTIVE",
  capabilities: ["READ", "WRITE"] as const,
  entry_node_ids: [],
  entry_nodes_truncated: false,
} satisfies Omit<NamespaceSummaryVM, "namespace_type" | "workspace_id">;

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

describe("canWriteGovernedFileSpace", () => {
  it("allows writes in personal namespace when governed capabilities include WRITE", () => {
    expect(
      canWriteGovernedFileSpace({
        permissions: ["file:write"],
        fileSpace: "my",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: {
          ...baseSummary,
          namespace_type: "USER",
          workspace_id: null,
          owner_user_id: "user-1",
        },
      }),
    ).toBe(true);
  });

  it("rejects personal namespace without WRITE capability", () => {
    expect(
      canWriteGovernedFileSpace({
        permissions: ["file:write"],
        fileSpace: "my",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: {
          ...baseSummary,
          namespace_type: "USER",
          workspace_id: null,
          capabilities: ["READ"],
        },
      }),
    ).toBe(false);
  });

  it("still gates workspace namespace on matching workspace id", () => {
    expect(
      canWriteGovernedFileSpace({
        permissions: ["file:write"],
        fileSpace: "workspace",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: {
          ...baseSummary,
          namespace_type: "WORKSPACE",
          workspace_id: "ws-2",
        },
      }),
    ).toBe(false);
  });
});

describe("canDeleteGovernedFileSpace", () => {
  it("allows delete in a personal namespace when governed capabilities include DELETE", () => {
    expect(
      canDeleteGovernedFileSpace({
        permissions: ["file:delete"],
        fileSpace: "my",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: {
          ...baseSummary,
          namespace_type: "USER",
          workspace_id: null,
          owner_user_id: "user-1",
          capabilities: ["READ", "DELETE"],
        },
      }),
    ).toBe(true);
  });

  it("rejects delete without the file:delete permission or DELETE capability", () => {
    const summary: NamespaceSummaryVM = {
      ...baseSummary,
      namespace_type: "USER",
      workspace_id: null,
      capabilities: ["READ", "WRITE"],
    };
    expect(
      canDeleteGovernedFileSpace({
        permissions: ["file:write"],
        fileSpace: "my",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: summary,
      }),
    ).toBe(false);
    expect(
      canDeleteGovernedFileSpace({
        permissions: ["file:delete"],
        fileSpace: "my",
        namespace: { id: "ns-1" },
        workspaceId: "ws-1",
        namespaceSummary: summary,
      }),
    ).toBe(false);
  });
});

describe("hasFilesystemPageAccess", () => {
  it("requires file read for page access", () => {
    expect(hasFilesystemPageAccess([])).toBe(false);
    expect(hasFilesystemPageAccess(["file:write"])).toBe(false);
    expect(hasFilesystemPageAccess(["file:read"])).toBe(true);
  });
});
