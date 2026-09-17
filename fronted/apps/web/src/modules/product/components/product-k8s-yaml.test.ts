import { describe, expect, it } from "vitest";
import { belongsToProject, parseYamlDocuments, resourceListLabel } from "./product-k8s-yaml";

describe("product k8s yaml helpers", () => {
  it("matches resources by name or app label", () => {
    expect(belongsToProject({ metadata: { name: "reviews" } }, "reviews")).toBe(true);
    expect(belongsToProject({ metadata: { name: "bookinfo-reviews" } }, "reviews")).toBe(true);
    expect(
      belongsToProject({ metadata: { name: "web", labels: { app: "reviews" } } }, "reviews"),
    ).toBe(true);
    expect(belongsToProject({ metadata: { name: "details" } }, "reviews")).toBe(false);
    expect(belongsToProject({ metadata: { name: "test-repo2" } }, "test-repo")).toBe(false);
    expect(belongsToProject({ metadata: { name: "test-repo" } }, "test-repo2")).toBe(false);
    expect(belongsToProject({ metadata: { name: "test-repo-api" } }, "test-repo")).toBe(true);
  });

  it("parses kind and name from yaml documents", () => {
    const docs = parseYamlDocuments(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews
  labels:
    app: reviews
---
apiVersion: v1
kind: Service
metadata:
  name: reviews
`);
    expect(docs.map((doc) => resourceListLabel(String(doc.kind), String(doc.metadata?.name)))).toEqual([
      "deployment/reviews",
      "service/reviews",
    ]);
  });
});
