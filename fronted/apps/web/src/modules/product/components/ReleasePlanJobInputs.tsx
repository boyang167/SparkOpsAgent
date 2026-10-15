"use client";

import { useState } from "react";
import type { JobInputState } from "./release-plan-runtime-inputs";

interface ReleasePlanJobInputsProps {
  states: JobInputState[];
  editable: boolean;
  busy?: boolean;
  onChange: (key: string, value: string | null) => void;
}

function display(state: JobInputState): string {
  const shown = state.secret ? "••••••" : state.value;
  if (state.source === "plan") return `继承：${shown}`;
  if (state.source === "missing") return "未填写";
  return shown;
}

export function ReleasePlanJobInputs({ states, editable, busy, onChange }: ReleasePlanJobInputsProps) {
  const [editing, setEditing] = useState<Set<string>>(() => new Set());

  function stopEditing(key: string) {
    setEditing((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });
  }

  return (
    <ul className="release-plan-job-inputs">
      {states.map((state) => {
        const ownInput = editable && state.overridable && (state.source === "job" || editing.has(state.key));
        const original = state.source === "job" ? state.value : "";
        return (
          <li
            key={state.key}
            className={`release-plan-job-input${state.source === "missing" && !ownInput ? " is-missing" : ""}`}
          >
            <span className="release-plan-job-input-key" title={state.label}>
              {state.key}
            </span>
            {ownInput ? (
              <input
                key={`${state.key}-${state.source}-${state.value}`}
                autoFocus={state.source !== "job"}
                className="input text-xs release-plan-job-field-control"
                type={state.secret ? "password" : "text"}
                defaultValue={original}
                disabled={busy}
                placeholder={state.source === "plan" ? display(state) : "填写本项取值"}
                aria-label={`${state.key} 本项取值`}
                onKeyDown={(e) => {
                  if (e.key === "Enter") e.currentTarget.blur();
                  if (e.key === "Escape") {
                    e.currentTarget.value = original;
                    e.currentTarget.blur();
                  }
                }}
                onBlur={(e) => {
                  const value = e.target.value.trim();
                  stopEditing(state.key);
                  if (value === original) return;
                  onChange(state.key, value || null);
                }}
              />
            ) : (
              <span className="release-plan-job-input-value">{display(state)}</span>
            )}
            {editable && state.overridable && state.source === "job" ? (
              <button
                type="button"
                className="release-plan-job-input-action"
                disabled={busy}
                onClick={() => onChange(state.key, null)}
              >
                恢复继承
              </button>
            ) : null}
            {editable && state.overridable && state.source !== "job" && !ownInput ? (
              <button
                type="button"
                className="release-plan-job-input-action"
                disabled={busy}
                onClick={() => setEditing((prev) => new Set(prev).add(state.key))}
              >
                单独填写
              </button>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
