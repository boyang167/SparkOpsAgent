import { describe, expect, it } from "vitest";
import { getMockPipeline } from "../mock/data";
import { pipelineToYaml, validatePipelineDefinition, yamlToPipeline } from "./pipeline-yaml";

describe("delivery pipeline YAML", () => {
  it("roundtrips a mock pipeline definition", () => {
    const pipeline = getMockPipeline("pipe-staging-only");
    expect(pipeline).toBeDefined();
    const yaml = pipelineToYaml(pipeline!.definition);
    const parsed = yamlToPipeline(yaml);
    expect(parsed.metadata.name).toBe("staging-only");
    expect(parsed.spec.stages).toHaveLength(3);
    expect(validatePipelineDefinition(parsed)).toEqual([]);
  });

  it("detects cyclic needs", () => {
    const cyclic = structuredClone(getMockPipeline("pipe-staging-only")!.definition);
    cyclic.spec.stages.push({
      id: "cycle-back",
      needs: ["verify"],
      steps: [{ uses: "notify.webhook" }],
    });
    const verify = cyclic.spec.stages.find((stage: { id: string }) => stage.id === "verify");
    verify!.needs = ["cycle-back"];
    expect(validatePipelineDefinition(cyclic).some((e) => e.includes("环"))).toBe(true);
  });

  it("rejects uses that are not in the step catalog", () => {
    const definition = structuredClone(getMockPipeline("pipe-staging-only")!.definition);
    definition.spec.stages[0]!.steps = [{ uses: "not.a.real.step" }];
    const errors = validatePipelineDefinition(definition);
    expect(errors.some((e) => e.includes("not.a.real.step") && e.includes("Catalog"))).toBe(true);
  });
});
