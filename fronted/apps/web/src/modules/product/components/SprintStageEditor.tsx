"use client";

import { useState, type CSSProperties } from "react";
import type { ProductWorkflowVM, SprintStage, StageWorkflowBinding } from "@sparkops/contracts/product";
import { isValidStageKey, nextStageColor, slugifyStageKey, withSortOrder } from "./sprint-process-templates";

interface SprintStageEditorProps {
  stages: SprintStage[];
  workflows: ProductWorkflowVM[];
  onChange: (stages: SprintStage[]) => void;
  minStages?: number;
  maxStages?: number;
  /** board：看板列编辑（简洁）；full：流程设置（完整字段） */
  variant?: "board" | "full";
  /** 进行中的迭代：列结构锁定，只能调整工作流绑定 */
  structureLocked?: boolean;
  initialExpandedIds?: string[];
}

export function SprintStageEditor({
  stages,
  workflows,
  onChange,
  minStages = 2,
  maxStages = 12,
  variant = "full",
  structureLocked = false,
  initialExpandedIds,
}: SprintStageEditorProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set(initialExpandedIds));

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

  function setStageWorkflows(stageId: string, bindings: StageWorkflowBinding[]) {
    onChange(stages.map((stage) => (stage.id === stageId ? { ...stage, workflows: bindings } : stage)));
  }

  if (structureLocked) {
    return (
      <div className="stage-lock">
        <p className="stage-lock-hint">迭代进行中，看板列结构已锁定，只能调整各列绑定的工作流。</p>
        {workflows.length === 0 ? (
          <p className="stage-lock-hint">产品暂无工作流，请先在「工作流」中创建。</p>
        ) : (
          <ul className="stage-lock-list">
            {stages.map((stage) => (
              <li key={stage.id} className="stage-lock-row">
                <div className="stage-lock-name">
                  <span className="stage-lock-dot" style={{ background: stage.color || "var(--muted)" }} aria-hidden />
                  {stage.name}
                </div>
                <StageWorkflowFields
                  stage={stage}
                  workflows={workflows}
                  onChange={(bindings) => setStageWorkflows(stage.id, bindings)}
                />
              </li>
            ))}
          </ul>
        )}
      </div>
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
      <div className="stage-board">
        <ol className="stage-board-track">
          {stages.map((stage, index) => {
            const keyInvalid = !isValidStageKey(stage.key);
            const expanded = expandedIds.has(stage.id);
            const color = stage.color ?? "#3b82f6";
            return (
              <li
                key={stage.id}
                className={`stage-col${expanded ? " is-expanded" : ""}${keyInvalid ? " is-invalid" : ""}`}
                style={{ "--stage-color": color } as CSSProperties}
              >
                <div className="stage-col-bar" aria-hidden="true" />
                <div className="stage-col-head">
                  <label className="stage-col-color" title="列颜色">
                    <span className="stage-col-dot" />
                    <input
                      type="color"
                      value={color}
                      onChange={(e) => updateStage(stage.id, { color: e.target.value })}
                      aria-label={`${stage.name || "列"} 颜色`}
                    />
                  </label>
                  <input
                    className="stage-col-name"
                    value={stage.name}
                    placeholder="列名称"
                    onChange={(e) => updateStage(stage.id, { name: e.target.value })}
                    onBlur={(e) => syncStageKey(stage.id, e.target.value, index)}
                    aria-label={`第 ${index + 1} 列名称`}
                  />
                  {stage.wip_limit ? <span className="stage-col-wip" title="WIP 上限">{stage.wip_limit}</span> : null}
                </div>

                {expanded ? (
                  <div className="stage-col-detail">
                    <label>
                      <span>标识 key</span>
                      <input
                        className="input font-mono"
                        value={stage.key}
                        onChange={(e) => updateStage(stage.id, { key: e.target.value.toLowerCase() })}
                        placeholder="dev"
                      />
                    </label>
                    <label>
                      <span>WIP 上限</span>
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
                    {workflows.length > 0 ? (
                      <StageWorkflowFields
                        stage={stage}
                        workflows={workflows}
                        onChange={(bindings) => setStageWorkflows(stage.id, bindings)}
                      />
                    ) : null}
                  </div>
                ) : (
                  <div className="stage-col-body" aria-hidden="true">
                    <span className="stage-col-card" />
                    <span className="stage-col-card is-short" />
                  </div>
                )}

                {keyInvalid ? (
                  <p className="stage-col-hint">key 须以小写字母开头，仅含 a-z、0-9、下划线</p>
                ) : null}

                <div className="stage-col-actions">
                  <button
                    type="button"
                    disabled={index === 0}
                    onClick={() => moveStage(stage.id, -1)}
                    aria-label="左移"
                    title="左移"
                  >
                    <StageIcon d="M15 18l-6-6 6-6" />
                  </button>
                  <button
                    type="button"
                    disabled={index === stages.length - 1}
                    onClick={() => moveStage(stage.id, 1)}
                    aria-label="右移"
                    title="右移"
                  >
                    <StageIcon d="M9 18l6-6-6-6" />
                  </button>
                  <button
                    type="button"
                    className={expanded ? "is-active" : undefined}
                    onClick={() => toggleExpanded(stage.id)}
                    aria-expanded={expanded}
                    aria-label={expanded ? "收起设置" : "列设置"}
                    title={expanded ? "收起设置" : "列设置"}
                  >
                    <StageIcon d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" />
                  </button>
                  <button
                    type="button"
                    className="is-danger"
                    disabled={stages.length <= minStages}
                    onClick={() => removeStage(stage.id)}
                    aria-label="删除列"
                    title="删除列"
                  >
                    <StageIcon d="M6 6l12 12M18 6L6 18" />
                  </button>
                </div>
              </li>
            );
          })}
          {stages.length < maxStages ? (
            <li className="stage-col-add">
              <button type="button" onClick={addStage}>
                <StageIcon d="M12 5v14M5 12h14" />
                添加列
              </button>
            </li>
          ) : null}
        </ol>
        <p className="stage-board-count">{stages.length} / {maxStages} 列 · 至少 {minStages} 列</p>
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
              <div className="md:col-span-2">
                <StageWorkflowFields
                  stage={stage}
                  workflows={workflows}
                  onChange={(bindings) => setStageWorkflows(stage.id, bindings)}
                />
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function StageWorkflowFields({
  stage,
  workflows,
  onChange,
}: {
  stage: SprintStage;
  workflows: ProductWorkflowVM[];
  onChange: (bindings: StageWorkflowBinding[]) => void;
}) {
  const binding = stage.workflows?.[0];

  function patch(next: Partial<StageWorkflowBinding>) {
    if (!binding) return;
    onChange([{ ...binding, ...next }]);
  }

  return (
    <div className="stage-wf-fields">
      <label className="stage-wf-field">
        <span>工作流</span>
        <select
          className="input"
          value={binding?.workflow_id ?? ""}
          onChange={(e) => {
            const id = e.target.value;
            if (!id) {
              onChange([]);
              return;
            }
            onChange([
              {
                workflow_id: id,
                workflow_name: workflows.find((wf) => wf.id === id)?.name,
                trigger_mode: binding?.trigger_mode ?? "manual",
                default_parameters: binding?.default_parameters ?? {},
                required_for_exit: binding?.required_for_exit ?? false,
              },
            ]);
          }}
        >
          <option value="">不绑定</option>
          {workflows.map((wf) => (
            <option key={wf.id} value={wf.id}>
              {wf.name}
            </option>
          ))}
        </select>
      </label>
      {binding ? (
        <>
          <label className="stage-wf-field">
            <span>触发方式</span>
            <select
              className="input"
              value={binding.trigger_mode}
              onChange={(e) => patch({ trigger_mode: e.target.value as StageWorkflowBinding["trigger_mode"] })}
            >
              <option value="manual">手动执行</option>
              <option value="auto_on_enter">工作项进入此列时自动执行</option>
            </select>
          </label>
          <label className="stage-wf-check">
            <input
              type="checkbox"
              checked={binding.required_for_exit}
              onChange={(e) => patch({ required_for_exit: e.target.checked })}
            />
            <span>执行成功后才能移到后面的列</span>
          </label>
        </>
      ) : null}
    </div>
  );
}

function StageIcon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={d} />
    </svg>
  );
}
