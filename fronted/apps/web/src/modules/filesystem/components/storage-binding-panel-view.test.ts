import { describe, expect, it } from "vitest";
import type { StorageBindingVM } from "@sparkops/contracts/filesystem";
import {
  buildBindingDisplayRow,
  defaultScopeIdForType,
  formatBindingMountPoint,
  formatScopeTargetLabel,
  STORAGE_BINDING_SCOPE_LABELS,
  scopeTargetOptions,
  validateStorageBindingForm,
} from "./storage-binding-panel-view";

const catalog = {
  organizations: [{ id: "org-1", name: "Acme", slug: "acme", is_member: true, read_only: false }],
  workspaces: [
    {
      id: "ws-1",
      org_id: "org-1",
      name: "研发工作区",
      slug: "dev",
      max_agents: 1,
      max_projects: 1,
      is_member: true,
      read_only: false,
    },
  ],
  projects: [
    {
      id: "proj-1",
      name: "portal",
      display_name: "门户",
      description: "",
    },
  ],
  orgMembers: [
    {
      member_id: "user-1",
      email: "alice@example.com",
      display_name: "Alice",
    },
  ],
};

describe("storage-binding-panel-view", () => {
  it("labels scope types and workspace targets in Chinese", () => {
    expect(STORAGE_BINDING_SCOPE_LABELS.WORKSPACE).toBe("工作区");
    const options = scopeTargetOptions("WORKSPACE", catalog);
    expect(options[0]?.label).toBe("研发工作区");
    expect(formatScopeTargetLabel("WORKSPACE", "ws-1", catalog)).toBe("研发工作区（dev）");
  });

  it("defaults scope id by type", () => {
    expect(defaultScopeIdForType("WORKSPACE", "ws-1", catalog)).toBe("ws-1");
    expect(defaultScopeIdForType("ORGANIZATION", "ws-1", catalog)).toBe("org-1");
    expect(defaultScopeIdForType("PROJECT", "ws-1", catalog)).toBe("proj-1");
    expect(defaultScopeIdForType("USER", "ws-1", catalog)).toBe("user-1");
  });

  it("tolerates catalogs without orgMembers", () => {
    const legacy = {
      organizations: catalog.organizations,
      workspaces: catalog.workspaces,
      projects: catalog.projects,
    } as typeof catalog;
    expect(defaultScopeIdForType("USER", "ws-1", legacy)).toBe("");
    expect(scopeTargetOptions("USER", legacy)).toEqual([]);
  });

  it("lists tenant users for user scope bindings", () => {
    const options = scopeTargetOptions("USER", catalog);
    expect(options).toEqual([
      { id: "user-1", label: "Alice", detail: "alice@example.com" },
    ]);
    expect(formatScopeTargetLabel("USER", "user-1", catalog)).toBe("Alice（alice@example.com）");
  });

  it("formats mount bindings rows for the table", () => {
    const binding: StorageBindingVM = {
      id: "11111111-1111-4111-8111-111111111111",
      organization_id: "org-1",
      scope_type: "WORKSPACE",
      scope_id: "ws-1",
      pool_id: "22222222-2222-4222-8222-222222222222",
      priority: 0,
    };
    expect(formatBindingMountPoint(binding, catalog)).toBe("/workspace/dev");
    const row = buildBindingDisplayRow(binding, catalog, "pool-cmc-primary", "ACTIVE");
    expect(row.namespace).toBe("acme/dev");
    expect(row.priority).toBe(0);
    expect(row.mountType).toBe("Virtual Mount");
    expect(row.status).toEqual({ label: "Mounted", tone: "mounted" });
  });

  it("validates binding form", () => {
    expect(
      validateStorageBindingForm({ pool_id: "", scope_id: "ws-1", scope_type: "WORKSPACE" }),
    ).toMatch(/存储池/);
    expect(
      validateStorageBindingForm({ pool_id: "p1", scope_id: "", scope_type: "WORKSPACE" }),
    ).toMatch(/绑定目标/);
    expect(
      validateStorageBindingForm({ pool_id: "p1", scope_id: "ws-1", scope_type: "WORKSPACE" }),
    ).toBeNull();
  });
});