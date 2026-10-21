"use client";

import {
  Background,
  Controls,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Connection,
  type Edge,
  type NodeChange,
  type NodeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useCallback, useMemo, type DragEvent } from "react";
import { definitionToFlow, layoutFromFlowNodes, STEP_PALETTE_MIME } from "../../lib/flow-adapter";
import type { PipelineLayout, PipelineStage } from "../../types";
import { StageNode, type StageFlowNode } from "./StageNode";

const nodeTypes = { stage: StageNode } satisfies NodeTypes;

interface PipelineCanvasProps {
  stages: PipelineStage[];
  layout: PipelineLayout;
  selectedStageId: string | null;
  onSelectStage: (stageId: string | null) => void;
  onLayoutChange: (layout: PipelineLayout) => void;
  onConnectStages: (sourceId: string, targetId: string) => void;
  onDisconnectStages: (sourceId: string, targetId: string) => void;
  onDropStep: (uses: string, position: { x: number; y: number }) => void;
  onDeleteStages: (stageIds: string[]) => void;
}

function PipelineCanvasInner({
  stages,
  layout,
  selectedStageId,
  onSelectStage,
  onLayoutChange,
  onConnectStages,
  onDisconnectStages,
  onDropStep,
  onDeleteStages,
}: PipelineCanvasProps) {
  const { screenToFlowPosition } = useReactFlow();
  const graph = useMemo(() => definitionToFlow(stages, layout), [stages, layout]);

  const nodes = useMemo<StageFlowNode[]>(
    () =>
      graph.nodes.map((node) => {
        const stage = stages.find((item) => item.id === node.id);
        return {
          id: node.id,
          type: "stage",
          position: node.position,
          selected: selectedStageId === node.id,
          data: { stage: stage ?? { id: node.id, steps: [] } },
        };
      }),
    [graph.nodes, stages, selectedStageId],
  );

  const edges = useMemo<Edge[]>(
    () =>
      graph.edges.map((edge) => ({
        id: edge.id,
        source: edge.source,
        target: edge.target,
      })),
    [graph.edges],
  );

  const onNodesChange = useCallback(
    (changes: NodeChange<StageFlowNode>[]) => {
      const removed = changes.filter((change) => change.type === "remove").map((change) => change.id);
      if (removed.length > 0) {
        onDeleteStages(removed);
      }

      const nextPositions = new Map(nodes.map((node) => [node.id, node.position]));
      let moved = false;
      for (const change of changes) {
        if (change.type === "position" && change.position) {
          nextPositions.set(change.id, change.position);
          moved = true;
        }
      }
      if (moved) {
        onLayoutChange(
          layoutFromFlowNodes(
            [...nextPositions.entries()].map(([id, position]) => ({ id, position })),
          ),
        );
      }
    },
    [nodes, onDeleteStages, onLayoutChange],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (connection.source && connection.target) {
        onConnectStages(connection.source, connection.target);
      }
    },
    [onConnectStages],
  );

  const onDragOver = useCallback((event: DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  const onDrop = useCallback(
    (event: DragEvent) => {
      event.preventDefault();
      const uses = event.dataTransfer.getData(STEP_PALETTE_MIME);
      if (!uses) return;
      const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });
      onDropStep(uses, position);
    },
    [onDropStep, screenToFlowPosition],
  );

  return (
    <div className="cd-canvas-shell">
      {stages.length === 0 ? <p className="cd-flow-empty">从左侧拖入步骤</p> : null}
      <ReactFlow<StageFlowNode>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        onConnect={onConnect}
        onEdgesDelete={(deleted) => {
          for (const edge of deleted) {
            onDisconnectStages(edge.source, edge.target);
          }
        }}
        onNodeClick={(_, node) => onSelectStage(node.id)}
        onPaneClick={() => onSelectStage(null)}
        onDragOver={onDragOver}
        onDrop={onDrop}
        deleteKeyCode={["Backspace", "Delete"]}
        proOptions={{ hideAttribution: true }}
        defaultEdgeOptions={{ type: "smoothstep" }}
        nodesConnectable
        elementsSelectable
      >
        <Background gap={20} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}

export function PipelineCanvas(props: PipelineCanvasProps) {
  return (
    <ReactFlowProvider>
      <PipelineCanvasInner {...props} />
    </ReactFlowProvider>
  );
}
