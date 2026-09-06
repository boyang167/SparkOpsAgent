"use client";

import { fleetDeploySoftWarnings, fleetWaveChain } from "../../lib/flow-adapter";
import { getStepCatalogEntry, STEP_CATALOG } from "../../mock/data";
import type { PipelineStage, PipelineStep } from "../../types";
import { StepFormFields } from "../shared/StepFormFields";

interface StepPropertyPanelProps {
  stage: PipelineStage | null;
  allStageIds: string[];
  onChange: (stage: PipelineStage) => void;
  onDelete: () => void;
}

export function StepPropertyPanel({ stage, allStageIds, onChange, onDelete }: StepPropertyPanelProps) {
  if (!stage) {
    return (
      <div className="cd-side-panel">
        <h3 className="cd-side-title">Stage 属性</h3>
        <p className="muted text-sm">在画布中选择 Stage，或从左侧步骤库添加新节点。</p>
      </div>
    );
  }

  const activeStage = stage;
  const activeStepIndex = 0;
  const activeStep = activeStage.steps[activeStepIndex];
  const catalog = activeStep ? getStepCatalogEntry(activeStep.uses) : undefined;

  function patchStage(patch: Partial<PipelineStage>) {
    onChange({
      ...activeStage,
      ...patch,
      id: patch.id ?? activeStage.id,
      steps: patch.steps ?? activeStage.steps,
    });
  }

  function patchStep(stepIndex: number, patch: Partial<PipelineStep>) {
    const steps = activeStage.steps.map((step, index) =>
      index === stepIndex ? { ...step, ...patch } : step,
    );
    patchStage({ steps });
  }

  function toggleNeed(id: string) {
    const needs = new Set(activeStage.needs ?? []);
    if (needs.has(id)) needs.delete(id);
    else needs.add(id);
    patchStage({ needs: [...needs] });
  }

  return (
    <div className="cd-side-panel">
      <div className="cd-side-header">
        <h3 className="cd-side-title">Stage 属性</h3>
        <button type="button" className="btn-ghost text-sm" onClick={onDelete}>
          删除
        </button>
      </div>

      <div className="cd-form-grid">
        <div className="cd-field">
          <label className="cd-field-label">Stage ID</label>
          <input
            className="input"
            value={activeStage.id}
            onChange={(e) => patchStage({ id: e.target.value.trim() })}
          />
        </div>
        <div className="cd-field">
          <label className="cd-field-label">条件 (if)</label>
          <input
            className="input"
            value={activeStage.if ?? ""}
            placeholder="可选 CEL 表达式"
            onChange={(e) => patchStage({ if: e.target.value || undefined })}
          />
        </div>
      </div>

      <div className="cd-field">
        <label className="cd-field-label">依赖 (needs)</label>
        <div className="cd-chip-list">
          {allStageIds
            .filter((id) => id !== activeStage.id)
            .map((id) => {
              const active = (activeStage.needs ?? []).includes(id);
              return (
                <button
                  key={id}
                  type="button"
                  className={`cd-chip${active ? " is-active" : ""}`}
                  onClick={() => toggleNeed(id)}
                >
                  {id}
                </button>
              );
            })}
        </div>
      </div>

      <div className="cd-field">
        <label className="cd-field-label">Step 类型</label>
        <select
          className="input"
          value={activeStep?.uses ?? ""}
          onChange={(e) => {
            const uses = e.target.value;
            const entry = STEP_CATALOG.find((item) => item.uses === uses);
            const withDefaults: Record<string, unknown> = {};
            for (const field of entry?.fields ?? []) {
              if (field.options?.[0]) withDefaults[field.key] = field.options[0].value;
            }
            patchStep(activeStepIndex, { uses, with: withDefaults });
          }}
        >
          {STEP_CATALOG.map((item) => (
            <option key={item.uses} value={item.uses}>
              {item.label}
            </option>
          ))}
        </select>
        {catalog ? <p className="field-hint">{catalog.description}</p> : null}
      </div>

      {catalog && activeStep ? (
        <StepFormFields
          entry={catalog}
          values={activeStep.with ?? {}}
          onChange={(withValues) => patchStep(activeStepIndex, { with: withValues })}
        />
      ) : null}

      {activeStep?.uses === "fleet.deploy" ? (
        <FleetDeployHints step={activeStep} />
      ) : null}
    </div>
  );
}

function FleetDeployHints({ step }: { step: PipelineStep }) {
  const strategy = String(step.with?.strategy ?? "");
  const fleet = String(step.with?.fleet ?? "");
  const waveChain = strategy === "wave" ? fleetWaveChain(fleet) : null;
  const warnings = fleetDeploySoftWarnings(step);

  if (!waveChain && warnings.length === 0) return null;

  return (
    <div className="cd-deploy-hints">
      {waveChain ? (
        <p className="cd-wave-preview">
          分波顺序：{waveChain}
        </p>
      ) : null}
      {warnings.map((warning) => (
        <p key={warning} className="cd-soft-warning">
          {warning}
        </p>
      ))}
    </div>
  );
}
