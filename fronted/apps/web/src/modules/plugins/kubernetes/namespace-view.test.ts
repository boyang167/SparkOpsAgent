import { describe, expect, it } from "vitest";
import {
  buildNamespaceFromDraft,
  emptyNamespaceTemplate,
  namespaceDisplayLabels,
  namespacePhaseBadge,
  validateNamespaceCreateDraft,
  validateNamespaceName,
} from "./namespace-view";

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

  it("filters metadata.name label from display list", () => {
    expect(
      namespaceDisplayLabels({
        "kubernetes.io/metadata.name": "default",
        team: "platform",
      }),
    ).toEqual([{ key: "team", value: "platform" }]);
  });

  it("validates namespace name as DNS label", () => {
    expect(validateNamespaceName("")).toMatch(/请填写/);
    expect(validateNamespaceName("My-App")).toMatch(/小写/);
    expect(validateNamespaceName("my-app")).toBeNull();
  });

  it("builds namespace manifest from form draft", () => {
    expect(
      buildNamespaceFromDraft({
        name: "my-app",
        labels: [{ key: "team", value: "platform" }],
      }),
    ).toEqual({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "my-app", labels: { team: "platform" } },
    });
  });

  it("rejects duplicate label keys in draft", () => {
    expect(
      validateNamespaceCreateDraft({
        name: "demo",
        labels: [
          { key: "env", value: "dev" },
          { key: "env", value: "prod" },
        ],
      }),
    ).toEqual({ labels: "标签键「env」重复" });
  });
});
