"use client";

import { useEffect, useRef, useState, type DragEvent } from "react";
import { useCanvasViewport } from "./use-canvas-viewport";
import {
  applyWorkflowDrop,
  encodeWorkflowDragPayload,
  readWorkflowDragPayload,
  type WorkflowDragPayload,
  type WorkflowDropTarget,
  WORKFLOW_DRAG_MIME,
  jobDragEnabled,
} from "./workflow-editor-dnd";
import { jobTypeLabel, stageExecModeLabel, type EditorJob, type EditorStage } from "./workflow-editor-utils";

type Selection =
  | { kind: "stage"; stageIndex: number }
  | { kind: "job"; stageIndex: number; jobIndex: number }
  | null;

interface WorkflowEditorFlowCanvasProps {
  stages: EditorStage[];
  selection: Selection;
  onStagesChange: (stages: EditorStage[]) => void;
  onSelectionChange: (selection: Selection) => void;
  onInsertStage: (atIndex: number) => void;
  onOpenExecMode: (stageIndex: number) => void;
  onOpenEditStage: (stageIndex: number) => void;
  onRemoveStage: (stageIndex: number) => void;
  onAddJob: (stageIndex: number) => void;
  onRemoveJob: (stageIndex: number, jobIndex: number) => void;
  onUpdateStageName: (stageIndex: number, name: string) => void;
}

function dropTargetKey(target: WorkflowDropTarget | null): string {
  if (!target) return "";
  return target.kind === "stage" ? `stage:${target.stageIndex}` : `job:${target.stageIndex}:${target.jobIndex}`;
}

export function WorkflowEditorFlowCanvas({
  stages,
  selection,
  onStagesChange,
  onSelectionChange,
  onInsertStage,
  onOpenExecMode,
  onOpenEditStage,
  onRemoveStage,
  onAddJob,
  onRemoveJob,
  onUpdateStageName,
}: WorkflowEditorFlowCanvasProps) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const {
    viewport,
    zoomIn,
    zoomOut,
    reset,
    fitToView,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onWheel,
    dragging: panning,
  } = useCanvasViewport(undefined, {
    panBlockSelector:
      ".pwf-stage-card, .pwf-flow-toolbar, .pwf-flow-zoom, .pwf-connector-add, .pwf-connector-mode",
  });
  const [dragging, setDragging] = useState<WorkflowDragPayload | null>(null);
  const [dropTarget, setDropTarget] = useState<WorkflowDropTarget | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      fitToView(viewportRef.current, contentRef.current);
    }, 0);
    return () => window.clearTimeout(timer);
  }, [stages.length, fitToView]);

  function beginDrag(event: DragEvent, payload: WorkflowDragPayload) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData(WORKFLOW_DRAG_MIME, encodeWorkflowDragPayload(payload));
    setDragging(payload);
  }

  function endDrag() {
    setDragging(null);
    setDropTarget(null);
  }

  function acceptDrop(event: DragEvent, target: WorkflowDropTarget) {
    event.preventDefault();
    event.stopPropagation();
    const payload = dragging ?? readWorkflowDragPayload(event.dataTransfer);
    if (!payload) return;
    const next = applyWorkflowDrop(stages, payload, target);
    if (next !== stages) {
      onStagesChange(next);
      onSelectionChange(null);
    }
    endDrag();
  }

  function handleDragOver(event: DragEvent, target: WorkflowDropTarget) {
    const payload = dragging ?? readWorkflowDragPayload(event.dataTransfer);
    if (!payload) return;
    if (payload.kind === "stage" && target.kind === "job") return;
    if (payload.kind === "job" && target.kind === "stage" && payload.stageIndex === target.stageIndex) {
      event.preventDefault();
      setDropTarget(target);
      return;
    }
    if (payload.kind === "job" && target.kind === "job") {
      if (payload.stageIndex === target.stageIndex && payload.jobIndex === target.jobIndex) return;
      if (
        payload.stageIndex === target.stageIndex &&
        payload.jobIndex === target.jobIndex - 1
      ) {
        return;
      }
    }
    if (payload.kind === "stage" && target.kind === "stage" && payload.stageIndex === target.stageIndex) {
      return;
    }
    event.preventDefault();
    setDropTarget(target);
  }

  const dropKey = dropTargetKey(dropTarget);

  return (
    <div className="pwf-flow-canvas-wrap">
      <div className="pwf-flow-toolbar">
        <p className="pwf-flow-hint">
          拖拽空白区域平移画布 · Ctrl + 滚轮缩放 · 阶段/任务卡片可拖动排序
        </p>
        <div className="pwf-flow-zoom" aria-label="画布视图">
          <button type="button" className="pwf-flow-zoom-btn" onClick={zoomOut} aria-label="缩小">
            −
          </button>
          <span className="pwf-flow-zoom-value">{Math.round(viewport.scale * 100)}%</span>
          <button type="button" className="pwf-flow-zoom-btn" onClick={zoomIn} aria-label="放大">
            +
          </button>
          <button
            type="button"
            className="pwf-flow-zoom-btn pwf-flow-zoom-action"
            onClick={() => fitToView(viewportRef.current, contentRef.current)}
            title="自动居中"
          >
            自动居中
          </button>
          <button
            type="button"
            className="pwf-flow-zoom-btn pwf-flow-zoom-reset"
            onClick={reset}
            title="重置视图"
          >
            ⟲
          </button>
        </div>
      </div>

      <div
        ref={viewportRef}
        className={`pwf-flow-viewport${panning ? " is-dragging" : ""}`}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerLeave={onPointerUp}
        onWheel={onWheel}
        role="presentation"
      >
        <div
          className="pwf-flow-inner"
          style={{
            transform: `translate(${viewport.offsetX}px, ${viewport.offsetY}px) scale(${viewport.scale})`,
          }}
        >
          <div ref={contentRef} className="pwf-flow-canvas">
            <div className="pwf-flow-track">
        <div className="pwf-endpoint">
          <span className="pwf-endpoint-dot">开始</span>
        </div>

        {stages.map((stage, stageIndex) => (
          <div key={`stage-wrap-${stage.name}-${stageIndex}`} className="pwf-flow-segment">
            <div className="pwf-connector">
              <span className="pwf-connector-line" />
              <button
                type="button"
                className="pwf-connector-add"
                title="在此插入阶段"
                onClick={() => onInsertStage(stageIndex)}
              >
                +
              </button>
              <button type="button" className="pwf-connector-mode" onClick={() => onOpenExecMode(stageIndex)}>
                {stageExecModeLabel(stage.exec_mode)}
              </button>
              <span className="pwf-connector-line" />
            </div>

            <article
              className={`pwf-stage-card${
                selection?.stageIndex === stageIndex ? " is-active" : ""
              }${dropKey === `stage:${stageIndex}` ? " is-drop-target" : ""}${
                dragging?.kind === "stage" && dragging.stageIndex === stageIndex ? " is-dragging" : ""
              }`}
              onDragOver={(event) => handleDragOver(event, { kind: "stage", stageIndex })}
              onDrop={(event) => acceptDrop(event, { kind: "stage", stageIndex })}
            >
              <header className="pwf-stage-card-head">
                <span
                  className="pwf-drag-handle"
                  draggable
                  aria-label="拖动阶段"
                  title="拖动阶段"
                  onDragStart={(event) => beginDrag(event, { kind: "stage", stageIndex })}
                  onDragEnd={endDrag}
                >
                  ⠿
                </span>
                <input
                  className="pwf-stage-name"
                  value={stage.name}
                  onChange={(e) => onUpdateStageName(stageIndex, e.target.value)}
                />
                <button
                  type="button"
                  className="pwf-icon-btn"
                  title="编辑阶段"
                  onClick={() => onOpenEditStage(stageIndex)}
                >
                  ⚙
                </button>
              </header>

              <div
                className={`pwf-stage-jobs${dropKey === `stage:${stageIndex}` && dragging?.kind === "job" ? " is-drop-target" : ""}`}
                onDragOver={(event) => handleDragOver(event, { kind: "stage", stageIndex })}
                onDrop={(event) => acceptDrop(event, { kind: "stage", stageIndex })}
              >
                {stage.jobs.map((job, jobIndex) => (
                  <JobRow
                    key={`${job.name}-${jobIndex}`}
                    job={job}
                    stageIndex={stageIndex}
                    jobIndex={jobIndex}
                    active={
                      selection?.kind === "job" &&
                      selection.stageIndex === stageIndex &&
                      selection.jobIndex === jobIndex
                    }
                    dragging={dragging}
                    dropTarget={dropTarget}
                    onSelect={() => onSelectionChange({ kind: "job", stageIndex, jobIndex })}
                    onRemove={() => onRemoveJob(stageIndex, jobIndex)}
                    onDragStart={beginDrag}
                    onDragEnd={endDrag}
                    onDragOver={handleDragOver}
                    onDrop={acceptDrop}
                  />
                ))}
                {stage.jobs.length === 0 ? (
                  <div className="pwf-stage-jobs-empty text-xs text-muted">拖拽任务到此处</div>
                ) : null}
              </div>

              <div className="pwf-stage-footer">
                <button type="button" className="pwf-add-task" onClick={() => onAddJob(stageIndex)}>
                  + 任务
                </button>
                {stages.length > 1 ? (
                  <button type="button" className="btn btn-ghost text-xs" onClick={() => onRemoveStage(stageIndex)}>
                    删除阶段
                  </button>
                ) : null}
              </div>
            </article>
          </div>
        ))}

        <div className="pwf-flow-segment">
          <div className="pwf-connector">
            <span className="pwf-connector-line" />
            <button
              type="button"
              className="pwf-connector-add is-stage"
              title="添加阶段"
              onClick={() => onInsertStage(stages.length)}
            >
              +
            </button>
            <span className="pwf-connector-label">+ 阶段</span>
            <span className="pwf-connector-line" />
          </div>
          <div className="pwf-endpoint">
            <span className="pwf-endpoint-dot">结束</span>
          </div>
        </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function JobRow({
  job,
  stageIndex,
  jobIndex,
  active,
  dragging,
  dropTarget,
  onSelect,
  onRemove,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
}: {
  job: EditorJob;
  stageIndex: number;
  jobIndex: number;
  active: boolean;
  dragging: WorkflowDragPayload | null;
  dropTarget: WorkflowDropTarget | null;
  onSelect: () => void;
  onRemove: () => void;
  onDragStart: (event: DragEvent, payload: WorkflowDragPayload) => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent, target: WorkflowDropTarget) => void;
  onDrop: (event: DragEvent, target: WorkflowDropTarget) => void;
}) {
  const dropKey = dropTargetKey(dropTarget);
  const rowDropKey = `job:${stageIndex}:${jobIndex}`;
  const isDragging =
    dragging?.kind === "job" && dragging.stageIndex === stageIndex && dragging.jobIndex === jobIndex;

  return (
    <div
      className={`pwf-task-row-wrap${dropKey === rowDropKey ? " is-drop-before" : ""}`}
      onDragOver={(event) => onDragOver(event, { kind: "job", stageIndex, jobIndex })}
      onDrop={(event) => onDrop(event, { kind: "job", stageIndex, jobIndex })}
    >
      <button
        type="button"
        className={`pwf-task-row${active ? " is-active" : ""}${isDragging ? " is-dragging" : ""}`}
        onClick={onSelect}
      >
        <span
          className="pwf-drag-handle pwf-drag-handle-compact"
          draggable={jobDragEnabled(job)}
          aria-label="拖动任务"
          title="拖动任务"
          onDragStart={(event) => {
            event.stopPropagation();
            onDragStart(event, { kind: "job", stageIndex, jobIndex });
          }}
          onDragEnd={(event) => {
            event.stopPropagation();
            onDragEnd();
          }}
          onClick={(event) => event.stopPropagation()}
        >
          ⠿
        </span>
        <span className="pwf-task-dot" aria-hidden />
        <span className="pwf-task-name">{job.name}</span>
        <span className="pwf-task-type">{jobTypeLabel(job.type)}</span>
        <span
          className="pwf-task-remove"
          role="button"
          tabIndex={0}
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              e.stopPropagation();
              onRemove();
            }
          }}
        >
          ×
        </span>
      </button>
    </div>
  );
}
