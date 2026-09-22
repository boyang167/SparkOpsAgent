"use client";

import type { PipelineStageView, PipelineTask } from "./workflow-run-canvas-utils";
import { taskStatusTone } from "./workflow-run-canvas-utils";
import { useCanvasViewport } from "./use-canvas-viewport";
import { WorkflowRunStatusIcon } from "./WorkflowRunStatusIcon";

interface WorkflowRunCanvasProps {
  stages: PipelineStageView[];
  selectedTaskId: string | null;
  onSelectTask: (task: PipelineTask) => void;
}

function Connector() {
  return (
    <div className="wf-run-live-connector" aria-hidden="true">
      <span className="wf-run-live-connector-line" />
      <span className="wf-run-live-connector-node" />
    </div>
  );
}

function TaskCard({
  task,
  selected,
  onSelect,
}: {
  task: PipelineTask;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      className={`wf-run-live-task${selected ? " is-selected" : ""}${task.status === "running" ? " is-active" : ""}`}
      onClick={onSelect}
    >
      <WorkflowRunStatusIcon status={task.status} size={16} />
      <div className="wf-run-live-task-body">
        <span className="wf-run-live-task-type">{task.jobTypeLabel}</span>
        <span className="wf-run-live-task-name">
          {task.projectLabel}
          {task.projectId && task.projectLabel !== task.projectId ? (
            <span className="wf-run-live-task-sub">({task.projectId.slice(0, 8)})</span>
          ) : null}
        </span>
      </div>
      <span className={`wf-run-live-task-state ${taskStatusTone(task.status)}`} />
    </button>
  );
}

export function WorkflowRunCanvas({ stages, selectedTaskId, onSelectTask }: WorkflowRunCanvasProps) {
  const { viewport, zoomIn, zoomOut, reset, onPointerDown, onPointerMove, onPointerUp, onWheel, dragging } =
    useCanvasViewport();

  return (
    <div className="wf-run-live-canvas-wrap">
      <div className="wf-run-live-zoom" aria-label="画布缩放">
        <button type="button" className="wf-run-live-zoom-btn" onClick={zoomOut} aria-label="缩小">
          −
        </button>
        <span className="wf-run-live-zoom-value">{Math.round(viewport.scale * 100)}%</span>
        <button type="button" className="wf-run-live-zoom-btn" onClick={zoomIn} aria-label="放大">
          +
        </button>
        <button type="button" className="wf-run-live-zoom-btn wf-run-live-zoom-reset" onClick={reset} title="重置视图">
          ⟲
        </button>
      </div>

      <div
        className={`wf-run-live-canvas-viewport${dragging ? " is-dragging" : ""}`}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerLeave={onPointerUp}
        onWheel={onWheel}
        role="presentation"
      >
        <div
          className="wf-run-live-canvas-inner"
          style={{
            transform: `translate(${viewport.offsetX}px, ${viewport.offsetY}px) scale(${viewport.scale})`,
          }}
        >
          <div className="wf-run-live-canvas">
            <div className="wf-run-live-node wf-run-live-node-terminal">
              <span className="wf-run-live-node-dot" />
              <span>开始</span>
            </div>

            <Connector />

            {stages.map((stage, index) => (
              <div key={`${stage.name}-${stage.index}`} className="wf-run-live-stage-group">
                <article className={`wf-run-live-stage ${taskStatusTone(stage.status)}`}>
                  <header className="wf-run-live-stage-head">{stage.name}</header>
                  <div className="wf-run-live-stage-tasks">
                    {stage.tasks.length ? (
                      stage.tasks.map((task) => (
                        <TaskCard
                          key={task.id}
                          task={task}
                          selected={selectedTaskId === task.id}
                          onSelect={() => onSelectTask(task)}
                        />
                      ))
                    ) : (
                      <div className="wf-run-live-task is-empty">暂无任务</div>
                    )}
                  </div>
                </article>
                {index < stages.length - 1 ? <Connector /> : null}
              </div>
            ))}

            <Connector />

            <div className="wf-run-live-node wf-run-live-node-terminal">
              <span className="wf-run-live-node-dot" />
              <span>结束</span>
            </div>
          </div>
        </div>
      </div>

      <p className="wf-run-live-canvas-hint text-xs text-muted">拖拽空白区域平移 · Ctrl + 滚轮缩放</p>
    </div>
  );
}
