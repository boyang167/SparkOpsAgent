import { describe, expect, it } from "vitest";
import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "./yaml-document";

describe("kubernetes YAML editor document helpers", () => {
  it("roundtrips a deployment YAML document", () => {
    const yaml = [
      "apiVersion: apps/v1",
      "kind: Deployment",
      "metadata:",
      "  name: spark-api",
      "  namespace: apps",
      "spec:",
      "  replicas: 2",
      "",
    ].join("\n");

    const parsed = parseYamlEditorDocument(yaml);
    expect(parsed).toMatchObject({
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { name: "spark-api", namespace: "apps" },
      spec: { replicas: 2 },
    });
    expect(stringifyYamlEditorDocument(parsed)).toContain("kind: Deployment");
  });

  it("rejects scalar, array, and dangerous prototype keys", () => {
    expect(() => parseYamlEditorDocument("just-a-string")).toThrow("YAML/JSON 必须是对象");
    expect(() => parseYamlEditorDocument("- item")).toThrow("YAML/JSON 必须是对象");
    expect(() => parseYamlEditorDocument("metadata:\n  __proto__: polluted")).toThrow("YAML/JSON 包含不允许的键");
    expect(() => parseYamlEditorDocument('{"constructor":{"prototype":{"polluted":true}}}')).toThrow(
      "YAML/JSON 包含不允许的键",
    );
  });
});
