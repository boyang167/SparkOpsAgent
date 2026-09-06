"use client";

import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { STRATEGY_LABELS } from "../../lib/flow-adapter";
import { getStepCatalogEntry } from "../../mock/data";
import type { PipelineStage } from "../../types";

export type StageFlowNode = Node<{ stage: PipelineStage }, "stage">;

export function StageNode({ data, selected }: NodeProps<StageFlowNode>) {
  const stage = data.stage;
  const primary = stage.steps[0];
  const catalog = primary ? getStepCatalogEntry(primary.uses) : undefined;
  const strategy = primary?.uses === "fleet.deploy" ? String(primary.with?.strategy ?? "") : "";
  const badge = STRATEGY_LABELS[strategy];

  return (
    <div className={`cd-stage-node${selected ? " is-selected" : ""}${catalog?.pauses ? " is-pause" : ""}`}>
      <Handle type="target" position={Position.Left} className="cd-stage-handle" />
      <span className="cd-stage-node-id">{stage.id}</span>
      <span className="cd-stage-node-step">{catalog?.label ?? primary?.uses ?? "空 Stage"}</span>
      {badge ? <span className="cd-strategy-badge">{badge}</span> : null}
      {stage.steps.length > 1 ? (
        <span className="cd-stage-node-meta">+{stage.steps.length - 1} steps</span>
      ) : null}
      <Handle type="source" position={Position.Right} className="cd-stage-handle" />
    </div>
  );
}
