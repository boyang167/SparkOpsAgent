import { describe, expect, it } from "vitest";
import {
  accessTabLabel,
  accessTabNamespaced,
  emptyAccessTemplate,
  formatBindingSubjects,
  formatRoleRef,
} from "./access-view";

describe("access view", () => {
  it("labels access tabs", () => {
    expect(accessTabLabel("ServiceAccount")).toBe("Service Accounts");
    expect(accessTabLabel("ClusterRoleBinding")).toBe("Cluster Role Bindings");
  });

  it("detects namespaced tabs", () => {
    expect(accessTabNamespaced("Role")).toBe(true);
    expect(accessTabNamespaced("ClusterRole")).toBe(false);
  });

  it("formats role ref", () => {
    expect(formatRoleRef({ kind: "ClusterRole", name: "admin" })).toBe("ClusterRole/admin");
  });

  it("formats binding subjects", () => {
    expect(
      formatBindingSubjects(
        [
          { kind: "User", name: "alice" },
          { kind: "Group", name: "devs" },
        ],
        "User",
      ),
    ).toBe("alice");
  });

  it("creates role binding template", () => {
    expect(emptyAccessTemplate("RoleBinding", "default")).toMatchObject({
      kind: "RoleBinding",
      metadata: { namespace: "default" },
    });
  });
});
