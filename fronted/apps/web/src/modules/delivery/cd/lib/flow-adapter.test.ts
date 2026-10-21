import { describe, expect, it } from "vitest";
import { mergePipelineFromYaml, pipelineToYaml } from "./pipeline-yaml";
import {
  connectStages,
  createDroppedStage,
  definitionToFlow,
  disconnectStages,
  fleetDeploySoftWarnings,
  fleetWaveChain,
  layoutFromFlowNodes,
} from "./flow-adapter";
import { getMockPipeline } from "../mock/data";
import type { PipelineStage } from "../types";

function stagesOf(id: string): PipelineStage[] {
  return structuredClone(getMockPipeline(id)!.definition.spec.stages);
}

describe("flow-adapter", () => {
  it("roundtrips definition and layout into flow nodes and edges", () => {
    const pipeline = getMockPipeline("pipe-staging-only")!;
    const { nodes, edges } = definitionToFlow(pipeline.definition.spec.stages, pipeline.layout);

    expect(nodes.map((n) => n.id)).toEqual(["build", "deploy", "verify"]);
    expect(edges).toEqual([
      { id: "build->deploy", source: "build", target: "deploy" },
      { id: "deploy->verify", source: "deploy", target: "verify" },
    ]);
    expect(nodes.find((n) => n.id === "build")?.position).toEqual(pipeline.layout.stages.build);

    const layout = layoutFromFlowNodes(nodes);
    expect(layout.stages.build).toEqual(pipeline.layout.stages.build);
  });

  it("adds needs when connecting two stages", () => {
    const stages: PipelineStage[] = [
      { id: "build", steps: [{ uses: "jenkins.build" }] },
      { id: "deploy", steps: [{ uses: "fleet.deploy", with: { strategy: "parallel" } }] },
    ];
    const result = connectStages(stages, "build", "deploy");
    expect(result.errors).toEqual([]);
    expect(result.stages?.find((s) => s.id === "deploy")?.needs).toEqual(["build"]);
  });

  it("rejects a cyclic connect and keeps stages unchanged", () => {
    const stages = stagesOf("pipe-staging-only");
    const before = structuredClone(stages);
    const result = connectStages(stages, "verify", "build");
    expect(result.stages).toBeNull();
    expect(result.errors.some((e) => e.includes("环"))).toBe(true);
    expect(stages).toEqual(before);
  });

  it("removes needs when disconnecting an edge", () => {
    const stages = stagesOf("pipe-staging-only");
    const next = disconnectStages(stages, "build", "deploy");
    expect(next.find((s) => s.id === "deploy")?.needs ?? []).toEqual([]);
  });

  it("creates a dropped fleet.deploy stage with parallel strategy and no auto needs", () => {
    const existing: PipelineStage[] = [{ id: "build", steps: [{ uses: "jenkins.build" }] }];
    const { stage, position } = createDroppedStage(existing, "fleet.deploy", { x: 120, y: 80 });
    expect(stage.needs).toBeUndefined();
    expect(stage.steps[0]).toEqual({ uses: "fleet.deploy", with: { strategy: "parallel" } });
    expect(position).toEqual({ x: 120, y: 80 });
    expect(stage.id).toMatch(/^stage-/);
  });

  it("keeps the previous definition when YAML introduces a cycle", () => {
    const pipeline = getMockPipeline("pipe-staging-only")!;
    const cyclic = structuredClone(pipeline.definition);
    cyclic.spec.stages[0]!.needs = ["verify"];
    const result = mergePipelineFromYaml(pipeline, pipelineToYaml(cyclic));
    expect(result.errors.some((e) => e.includes("环"))).toBe(true);
    expect(result.pipeline.definition).toEqual(pipeline.definition);
  });

  it("builds a wave chain for prod-cn and warns when wave has no topology", () => {
    expect(fleetWaveChain("prod-cn")).toBe("canary → primary → edge");
    expect(fleetWaveChain("missing-fleet")).toBeNull();
    expect(
      fleetDeploySoftWarnings({
        uses: "fleet.deploy",
        with: { fleet: "missing-fleet", strategy: "wave" },
      }),
    ).toContain("该 Fleet 没有分波拓扑");
  });
});
