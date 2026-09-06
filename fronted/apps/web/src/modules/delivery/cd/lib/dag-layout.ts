import type { PipelineLayout, PipelineStage } from "../types";

export interface DagNodeLayout {
  id: string;
  x: number;
  y: number;
  layer: number;
}

export interface DagEdge {
  from: string;
  to: string;
}

const NODE_WIDTH = 200;
const NODE_HEIGHT = 88;
const LAYER_GAP_X = 260;
const ROW_GAP_Y = 120;
const CANVAS_PADDING = 48;

export function computeDagLayout(
  stages: PipelineStage[],
  savedLayout?: PipelineLayout,
): { nodes: DagNodeLayout[]; edges: DagEdge[]; width: number; height: number } {
  const edges: DagEdge[] = [];
  for (const stage of stages) {
    for (const need of stage.needs ?? []) {
      edges.push({ from: need, to: stage.id });
    }
  }

  const layers = assignLayers(stages);
  const layerBuckets = new Map<number, string[]>();
  for (const [id, layer] of layers) {
    const bucket = layerBuckets.get(layer) ?? [];
    bucket.push(id);
    layerBuckets.set(layer, bucket);
  }

  const nodes: DagNodeLayout[] = [];
  let maxLayer = 0;
  let maxRows = 1;

  for (const [layer, ids] of [...layerBuckets.entries()].sort((a, b) => a[0] - b[0])) {
    maxLayer = Math.max(maxLayer, layer);
    maxRows = Math.max(maxRows, ids.length);
    ids.forEach((id, row) => {
      const saved = savedLayout?.stages[id];
      nodes.push({
        id,
        layer,
        x: saved?.x ?? CANVAS_PADDING + layer * LAYER_GAP_X,
        y: saved?.y ?? CANVAS_PADDING + row * ROW_GAP_Y,
      });
    });
  }

  const width = CANVAS_PADDING * 2 + (maxLayer + 1) * LAYER_GAP_X + NODE_WIDTH;
  const height = CANVAS_PADDING * 2 + maxRows * ROW_GAP_Y + NODE_HEIGHT;
  return { nodes, edges, width, height };
}

function assignLayers(stages: PipelineStage[]): Map<string, number> {
  const byId = new Map(stages.map((s) => [s.id, s]));
  const memo = new Map<string, number>();

  function depth(id: string, visiting: Set<string>): number {
    if (memo.has(id)) return memo.get(id)!;
    if (visiting.has(id)) return 0;
    visiting.add(id);
    const stage = byId.get(id);
    const needs = stage?.needs ?? [];
    const next = needs.length === 0 ? 0 : Math.max(...needs.map((n) => depth(n, visiting))) + 1;
    visiting.delete(id);
    memo.set(id, next);
    return next;
  }

  for (const stage of stages) {
    depth(stage.id, new Set());
  }
  return memo;
}

export function autoLayoutStages(stages: PipelineStage[]): PipelineLayout {
  const { nodes } = computeDagLayout(stages);
  const layout: PipelineLayout = { stages: {} };
  for (const node of nodes) {
    layout.stages[node.id] = { x: node.x, y: node.y };
  }
  return layout;
}

export function edgePath(from: DagNodeLayout, to: DagNodeLayout): string {
  const x1 = from.x + NODE_WIDTH;
  const y1 = from.y + NODE_HEIGHT / 2;
  const x2 = to.x;
  const y2 = to.y + NODE_HEIGHT / 2;
  const midX = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;
}

export const DAG_NODE_SIZE = { width: NODE_WIDTH, height: NODE_HEIGHT };

export function nextStageId(stages: PipelineStage[]): string {
  let index = stages.length + 1;
  let candidate = `stage-${index}`;
  const ids = new Set(stages.map((s) => s.id));
  while (ids.has(candidate)) {
    index += 1;
    candidate = `stage-${index}`;
  }
  return candidate;
}
