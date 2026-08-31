import { describe, expect, it } from "vitest";
import { formatRoleAge, isBuiltinRole } from "./cluster-authorize-roles";

describe("isBuiltinRole", () => {
  it("protects default RBAC and system-prefixed cluster roles", () => {
    expect(isBuiltinRole({ metadata: { name: "view" } }, "cluster")).toBe(true);
    expect(isBuiltinRole({ metadata: { name: "cluster-admin" } }, "cluster")).toBe(true);
    expect(isBuiltinRole({ metadata: { name: "system:coredns" } }, "cluster")).toBe(true);
    expect(
      isBuiltinRole(
        { metadata: { name: "custom", labels: { "kubernetes.io/bootstrapping": "rbac-defaults" } } },
        "cluster",
      ),
    ).toBe(true);
    expect(isBuiltinRole({ metadata: { name: "sparkops-view" } }, "cluster")).toBe(false);
  });

  it("does not treat admin/edit/view as builtin namespaced roles unless system-prefixed", () => {
    expect(isBuiltinRole({ metadata: { name: "admin" } }, "namespace")).toBe(false);
    expect(isBuiltinRole({ metadata: { name: "system:controller:bootstrap-signer" } }, "namespace")).toBe(true);
  });
});

describe("formatRoleAge", () => {
  it("formats relative age", () => {
    const now = Date.parse("2026-08-31T13:00:00Z");
    expect(formatRoleAge("2026-08-31T12:50:00Z", now)).toBe("10 分钟");
    expect(formatRoleAge("2026-08-30T13:00:00Z", now)).toBe("1 天");
  });
});
