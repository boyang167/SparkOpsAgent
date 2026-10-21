import { MOCK_FLEETS } from "../mock/data";
import type { PipelineLayout, PipelineStage, PipelineStep } from "../types";
import { computeDagLayout, nextStageId } from "./dag-layout";
import { validatePipelineDefinition } from "./pipeline-yaml";

export const STEP_PALETTE_MIME = "application/sparkops-step";

export const STRATEGY_LABELS: Record<string, string> = {
  parallel: "并行",
  sequential: "顺序",
  wave: "分波",
};

export interface FlowNodeSnapshot {
  id: string;
  position: { x: number; y: number };
}

export interface FlowEdgeSnapshot {
  id: string;
  source: string;
  target: string;
}

export function definitionToFlow(
  stages: PipelineStage[],
  layout: PipelineLayout,
): { nodes: FlowNodeSnapshot[]; edges: FlowEdgeSnapshot[] } {
  const computed = computeDagLayout(stages, layout);
  const nodes = computed.nodes.map((node) => ({
    id: node.id,
    position: { x: node.x, y: node.y },
  }));
  const edges = computed.edges.map((edge) => ({
    id: `${edge.from}->${edge.to}`,
    source: edge.from,
    target: edge.to,
  }));
  return { nodes, edges };
}

export function layoutFromFlowNodes(nodes: FlowNodeSnapshot[]): PipelineLayout {
  const stages: PipelineLayout["stages"] = {};
  for (const node of nodes) {
    stages[node.id] = { x: node.position.x, y: node.position.y };
  }
  return { stages };
}

export function connectStages(
  stages: PipelineStage[],
  sourceId: string,
  targetId: string,
): { stages: PipelineStage[] | null; errors: string[] } {
  if (sourceId === targetId) {
    return { stages, errors: [] };
  }
  const target = stages.find((stage) => stage.id === targetId);
  if (!target) {
    return { stages: null, errors: [`目标节点不存在: ${targetId}`] };
  }
  if ((target.needs ?? []).includes(sourceId)) {
    return { stages, errors: [] };
  }

  const next = stages.map((stage) =>
    stage.id === targetId ? { ...stage, needs: [...(stage.needs ?? []), sourceId] } : stage,
  );
  const errors = validatePipelineDefinition({
    apiVersion: "cd.sparkops.io/v1",
    kind: "Pipeline",
    metadata: { name: "draft" },
    spec: { parameters: [], stages: next },
  });
  if (errors.length > 0) {
    return { stages: null, errors };
  }
  return { stages: next, errors: [] };
}

export function disconnectStages(
  stages: PipelineStage[],
  sourceId: string,
  targetId: string,
): PipelineStage[] {
  return stages.map((stage) => {
    if (stage.id !== targetId) return stage;
    const needs = (stage.needs ?? []).filter((need) => need !== sourceId);
    return { ...stage, needs: needs.length > 0 ? needs : undefined };
  });
}

export function createDroppedStage(
  stages: PipelineStage[],
  uses: string,
  position: { x: number; y: number },
): { stage: PipelineStage; position: { x: number; y: number } } {
  const withValues = uses === "fleet.deploy" ? { strategy: "parallel" } : {};
  return {
    stage: {
      id: nextStageId(stages),
      steps: [{ uses, with: withValues }],
    },
    position,
  };
}

export function reconcileLayout(layout: PipelineLayout, stages: PipelineStage[]): PipelineLayout {
  const computed = computeDagLayout(stages, layout);
  const next: PipelineLayout["stages"] = {};
  for (const node of computed.nodes) {
    next[node.id] = { x: node.x, y: node.y };
  }
  return { stages: next };
}

export function layoutWithNewStage(
  layout: PipelineLayout,
  stages: PipelineStage[],
  newId: string,
  position?: { x: number; y: number },
): PipelineLayout {
  if (position) {
    return { stages: { ...layout.stages, [newId]: position } };
  }
  const computed = computeDagLayout(stages, layout);
  const placed = computed.nodes.find((node) => node.id === newId);
  return {
    stages: {
      ...layout.stages,
      [newId]: placed ? { x: placed.x, y: placed.y } : { x: 48, y: 48 },
    },
  };
}

export function fleetWaveChain(fleetName: string): string | null {
  const fleet = MOCK_FLEETS.find((item) => item.name === fleetName);
  if (!fleet?.topology.waves.length) return null;
  return fleet.topology.waves.map((wave) => wave.name).join(" → ");
}

export function fleetDeploySoftWarnings(step: PipelineStep): string[] {
  if (step.uses !== "fleet.deploy") return [];
  const warnings: string[] = [];
  const fleet = String(step.with?.fleet ?? "").trim();
  if (!fleet) warnings.push("未选择 Fleet");
  if (step.with?.strategy === "wave" && !fleetWaveChain(fleet)) {
    warnings.push("该 Fleet 没有分波拓扑");
  }
  return warnings;
}
