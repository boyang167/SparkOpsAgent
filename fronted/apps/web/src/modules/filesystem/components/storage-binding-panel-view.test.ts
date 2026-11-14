import { describe, expect, it } from "vitest";
import {
  defaultScopeIdForType,
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