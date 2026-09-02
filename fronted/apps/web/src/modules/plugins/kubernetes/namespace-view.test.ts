import { describe, expect, it } from "vitest";
import { emptyNamespaceTemplate, namespacePhaseBadge } from "./namespace-view";

describe("namespace view", () => {
  it("marks Active phase as ok badge", () => {
    expect(namespacePhaseBadge("Active")).toEqual({ text: "Active", tone: "ok" });
  });

  it("marks unknown phase as bad badge", () => {
    expect(namespacePhaseBadge("Terminating")).toEqual({ text: "Terminating", tone: "bad" });
  });

  it("creates empty namespace template", () => {
    expect(emptyNamespaceTemplate()).toEqual({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "", labels: {} },
    });
  });
});
