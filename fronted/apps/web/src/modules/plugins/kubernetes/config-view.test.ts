import { describe, expect, it } from "vitest";
import {
  configTabLabel,
  configTabsForUser,
  emptyConfigTemplate,
  secretEditableType,
} from "./config-view";

describe("config view", () => {
  it("labels config tabs", () => {
    expect(configTabLabel("ConfigMap")).toBe("Config Maps");
    expect(configTabLabel("HorizontalPodAutoscaler")).toBe("HPA");
  });

  it("hides secret tab for non-admin users", () => {
    expect(configTabsForUser(false)).not.toContain("Secret");
    expect(configTabsForUser(true)).toContain("Secret");
  });

  it("detects editable secret types", () => {
    expect(secretEditableType("Opaque")).toBe(true);
    expect(secretEditableType("kubernetes.io/service-account-token")).toBe(false);
  });

  it("creates config templates", () => {
    expect(emptyConfigTemplate("ConfigMap", "default")).toEqual({
      apiVersion: "v1",
      kind: "ConfigMap",
      metadata: { namespace: "default" },
      data: {},
    });
    expect(emptyConfigTemplate("Secret", "default")).toMatchObject({
      kind: "Secret",
      type: "Opaque",
      data: {},
    });
  });
});
