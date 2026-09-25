"use client";

import { useState } from "react";
import type { ProductWorkflowVM, SprintStage } from "@sparkops/contracts/product";
import { isValidStageKey, nextStageColor, slugifyStageKey, withSortOrder } from "./sprint-process-templates";

interface SprintStageEditorProps {
  stages: SprintStage[];
  workflows: ProductWorkflowVM[];
  onChange: (stages: SprintStage[]) => void;
  minStages?: number;
  maxStages?: number;
  /** board：看板列编辑（简洁）；full：流程设置（完整字段） */
  variant?: "board" | "full";
}

export function SprintStageEditor({
  stages,
  workflows,
  onChange,
  minStages = 2,
  maxStages = 12,
  variant = "full",
}: SprintStageEditorProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());

  function toggleExpanded(stageId: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(stageId)) next.delete(stageId);
      else next.add(stageId);
      return next;
    });
  }

  function addStage() {
    if (stages.length >= maxStages) return;
    const index = stages.length;
    const label = variant === "board" ? `新列 ${index + 1}` : `阶段 ${index + 1}`;
    onChange(
      withSortOrder([
        ...stages,
        {
          id: crypto.randomUUID(),
          key: slugifyStageKey(label, index),
          name: label,
          sort_order: index,
          color: nextStageColor(index),
          workflows: [],
        },
      ]),
    );
  }

  function removeStage(stageId: string) {
    if (stages.length <= minStages) return;
    onChange(withSortOrder(stages.filter((stage) => stage.id !== stageId)));
    setExpandedIds((prev) => {
      const next = new Set(prev);
      next.delete(stageId);
      return next;
    });
  }

  function moveStage(stageId: string, direction: -1 | 1) {
    const index = stages.findIndex((stage) => stage.id === stageId);
    if (index < 0) return;
    const target = index + direction;
    if (target < 0 || target >= stages.length) return;
    const next = [...stages];
    const [item] = next.splice(index, 1);
    next.splice(target, 0, item);
    onChange(withSortOrder(next));
  }

  function updateStage(stageId: string, patch: Partial<SprintStage>) {
    onChange(withSortOrder(stages.map((stage) => (stage.id === stageId ? { ...stage, ...patch } : stage))));
  }

  function bindWorkflow(stageId: string, workflowId: string) {
    onChange(
      withSortOrder(
        stages.map((stage) => {
          if (stage.id !== stageId) return stage;
          if (!workflowId) return { ...stage, workflows: [] };
          const wf = workflows.find((item) => item.id === workflowId);
          return {
            ...stage,
            workflows: [{ workflow_id: workflowId, workflow_name: wf?.name, trigger_mode: "manual" as const }],
          };
        }),
      ),
    );
  }

  function syncStageKey(stageId: string, name: string, index: number) {
    const stage = stages.find((item) => item.id === stageId);
    if (!stage) return;
    const nextKey = slugifyStageKey(name, index);
    if (stage.key === nextKey || !isValidStageKey(stage.key)) {
      updateStage(stageId, { key: nextKey });
    }
  }

  if (variant === "board") {
    return (
      <div className="sprint-stage-editor-board">
        <div className="sprint-stage-preview" aria-label="列顺序预览">
          {stages.map((stage) => (
            <div
              key={stage.id}
              className="sprint-stage-preview-chip"
              style={{ borderTopColor: stage.color ?? "#3b82f6" }}
              title={stage.name}
            >
              <span className="sprint-stage-preview-chip-label">{stage.name || "未命名"}</span>
            </div>
          ))}
        </div>

        <ul className="sprint-stage-rows">
          {stages.map((stage, index) => {
            const keyInvalid = !isValidStageKey(stage.key);
            const expanded = expandedIds.has(stage.id);
            return (
              <li key={stage.id} className={`sprint-stage-row${keyInvalid ? " sprint-stage-row-invalid" : ""}`}>
                <div className="sprint-stage-row-main">
                  <label className="sprint-stage-color-btn" title="列颜色">
                    <span className="sprint-stage-color-swatch" style={{ backgroundColor: stage.color ?? "#3b82f6" }} />
                    <input
                      type="color"
                      className="sprint-stage-color-input"
                      value={stage.color ?? "#3b82f6"}
                      onChange={(e) => updateStage(stage.id, { color: e.target.value })}
                      aria-label={`${stage.name || "列"} 颜色`}
                    />
                  </label>
                  <input
                    className="input sprint-stage-name-input"
                    value={stage.name}
                    placeholder="列名称"
                    onChange={(e) => updateStage(stage.id, { name: e.target.value })}
                    onBlur={(e) => syncStageKey(stage.id, e.target.value, index)}
                    aria-label={`第 ${index + 1} 列名称`}
                  />
                  <button
                    type="button"
                    className={`sprint-stage-more-btn${expanded ? " sprint-stage-more-btn-active" : ""}`}
                    onClick={() => toggleExpanded(stage.id)}
                    aria-expanded={expanded}
                    aria-label={expanded ? "收起高级设置" : "展开高级设置"}
                  >
                    {expanded ? "收起" : "高级"}
                  </button>
                  <div className="sprint-stage-row-actions">
                    <button
                      type="button"
                      className="sprint-stage-icon-btn"
                      disabled={index === 0}
                      onClick={() => moveStage(stage.id, -1)}
                      aria-label="上移"
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      className="sprint-stage-icon-btn"
                      disabled={index === stages.length - 1}
                      onClick={() => moveStage(stage.id, 1)}
                      aria-label="下移"
                    >
                      ↓
                    </button>
                    <button
                      type="button"
                      className="sprint-stage-icon-btn sprint-stage-icon-btn-danger"
                      disabled={stages.length <= minStages}
                      onClick={() => removeStage(stage.id)}
                      aria-label="删除列"
                    >
                      ×
                    </button>
                  </div>
                </div>

                {expanded ? (
                  <div className="sprint-stage-row-detail">
                    <label className="sprint-stage-detail-field">
                      <span className="sprint-stage-detail-label">标识 key</span>
                      <input
                        className={`input text-sm font-mono sprint-stage-detail-input${keyInvalid ? " border-red-500" : ""}`}
                        value={stage.key}
                        onChange={(e) => updateStage(stage.id, { key: e.target.value.toLowerCase() })}
                        placeholder="dev"
                      />
                    </label>
                    <label className="sprint-stage-detail-field sprint-stage-detail-field-narrow">
                      <span className="sprint-stage-detail-label">WIP 上限</span>
                      <input
                        type="number"
                        min={0}
                        className="input text-sm sprint-stage-detail-input"
                        value={stage.wip_limit ?? ""}
                        onChange={(e) => {
                          const raw = e.target.value.trim();
                          updateStage(stage.id, { wip_limit: raw ? Number(raw) : undefined });
                        }}
                        placeholder="不限"
                      />
                    </label>
                    <label className="sprint-stage-detail-field sprint-stage-detail-field-wide">
                      <span className="sprint-stage-detail-label">默认工作流</span>
                      <select
                        className="input text-sm sprint-stage-detail-input"
                        value={stage.workflows?.[0]?.workflow_id ?? ""}
                        onChange={(e) => bindWorkflow(stage.id, e.target.value)}
                      >
                        <option value="">不绑定</option>
                        {workflows.map((wf) => (
                          <option key={wf.id} value={wf.id}>{wf.name}</option>
                        ))}
                      </select>
                    </label>
                    {keyInvalid ? (
                      <p className="sprint-stage-row-hint sprint-stage-row-hint-detail">key 须以小写字母开头，仅含 a-z、0-9、下划线</p>
                    ) : null}
                  </div>
                ) : null}

                {!expanded && keyInvalid ? (
                  <p className="sprint-stage-row-hint">key 无效，点击「高级」修正</p>
                ) : null}
              </li>
            );
          })}
        </ul>

        <div className="sprint-stage-editor-footer">
          <span className="sprint-stage-editor-count">{stages.length} / {maxStages} 列</span>
          <button
            type="button"
            className="btn-ghost text-sm"
            disabled={stages.length >= maxStages}
            onClick={addStage}
          >
            + 添加列
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="stack-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">看板阶段（{stages.length}/{maxStages}）</span>
        <button type="button" className="btn-secondary text-sm" disabled={stages.length >= maxStages} onClick={addStage}>
          添加阶段
        </button>
      </div>
      <ul className="stack-sm">
        {stages.map((stage, index) => (
          <li key={stage.id} className="rounded border p-3 stack-sm">
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-block w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: stage.color ?? "#3b82f6" }} />
              <span className="text-xs text-muted-foreground">#{index + 1}</span>
              <div className="flex gap-1 ml-auto">
                <button type="button" className="btn-ghost text-xs" disabled={index === 0} onClick={() => moveStage(stage.id, -1)}>↑</button>
                <button type="button" className="btn-ghost text-xs" disabled={index === stages.length - 1} onClick={() => moveStage(stage.id, 1)}>↓</button>
                <button type="button" className="btn-ghost text-xs text-red-600" disabled={stages.length <= minStages} onClick={() => removeStage(stage.id)}>删除</button>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <label className="stack-xs">
                <span className="text-sm">显示名称</span>
                <input
                  className="input"
                  value={stage.name}
                  onChange={(e) => updateStage(stage.id, { name: e.target.value })}
                  onBlur={(e) => syncStageKey(stage.id, e.target.value, index)}
                />
              </label>
              <label className="stack-xs">
                <span className="text-sm">标识 key</span>
                <input
                  className={`input font-mono text-sm${!isValidStageKey(stage.key) ? " border-red-500" : ""}`}
                  value={stage.key}
                  onChange={(e) => updateStage(stage.id, { key: e.target.value.toLowerCase() })}
                  placeholder="如 dev、stage_5"
                />
                {!isValidStageKey(stage.key) ? (
                  <span className="text-xs text-red-600">须以小写字母开头，仅含 a-z、0-9、下划线</span>
                ) : null}
              </label>
              <label className="stack-xs">
                <span className="text-sm">列颜色</span>
                <input type="color" className="input h-10 p-1" value={stage.color ?? "#3b82f6"} onChange={(e) => updateStage(stage.id, { color: e.target.value })} />
              </label>
              <label className="stack-xs">
                <span className="text-sm">WIP 上限</span>
                <input
                  type="number"
                  min={0}
                  className="input"
                  value={stage.wip_limit ?? ""}
                  onChange={(e) => {
                    const raw = e.target.value.trim();
                    updateStage(stage.id, { wip_limit: raw ? Number(raw) : undefined });
                  }}
                  placeholder="不限"
                />
              </label>
              <label className="stack-xs md:col-span-2">
                <span className="text-sm">默认绑定工作流</span>
                <select className="input" value={stage.workflows?.[0]?.workflow_id ?? ""} onChange={(e) => bindWorkflow(stage.id, e.target.value)}>
                  <option value="">不绑定</option>
                  {workflows.map((wf) => (
                    <option key={wf.id} value={wf.id}>{wf.name}</option>
                  ))}
                </select>
              </label>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
