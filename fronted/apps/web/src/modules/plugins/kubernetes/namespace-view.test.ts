import { describe, expect, it } from "vitest";
import {
  buildNamespaceFromDraft,
  emptyNamespaceCreateDraft,
  emptyNamespaceTemplate,
  namespacePhaseBadge,
  validateNamespaceCreateDraft,
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

  it("rejects an invalid namespace name", () => {
    expect(validateNamespaceCreateDraft({ ...emptyNamespaceCreateDraft(), name: "My App" }).name).toBeTruthy();
  });

  it("builds a namespace manifest from the draft", () => {
    expect(
      buildNamespaceFromDraft({
        name: "my-app",
        labels: [
          { key: "team", value: "platform" },
          { key: "", value: "" },
        ],
      }),
    ).toEqual({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "my-app", labels: { team: "platform" } },
    });
  });
});
