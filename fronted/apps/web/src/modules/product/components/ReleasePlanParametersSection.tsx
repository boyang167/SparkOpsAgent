"use client";

import { useMemo } from "react";
import type { ReleasePlanVM } from "@sparkops/contracts/product";
import { planInputRows, planMissingInputs } from "./release-plan-runtime-inputs";

interface ReleasePlanParametersSectionProps {
  plan: ReleasePlanVM;
  editable: boolean;
  busy?: boolean;
  onChange: (parameters: Record<string, string>) => void | Promise<void>;
}

export function ReleasePlanParametersSection({ plan, editable, busy, onChange }: ReleasePlanParametersSectionProps) {
  const rows = useMemo(() => planInputRows(plan.jobs ?? []), [plan.jobs]);
  const missingKeys = useMemo(
    () => new Set(planMissingInputs(plan).flatMap((item) => item.keys)),
    [plan],
  );
  if (rows.length === 0) return null;
  const params = plan.parameters ?? {};

  function commit(key: string, raw: string) {
    const value = raw.trim();
    if (value === (params[key] ?? "")) return;
    const next = { ...params };
    if (value) next[key] = value;
    else delete next[key];
    void onChange(next);
  }

  return (
    <section className="release-plan-params" aria-label="发布参数">
      <header className="release-plan-params-head">
        <h3 className="release-plan-params-title">发布参数</h3>
        <p className="release-plan-params-desc">所有工作流发布项共用；单个发布项可在「运行参数」中单独填写。</p>
      </header>
      <ul className="release-plan-params-list">
        {rows.map((row) => {
          const value = params[row.key] ?? "";
          const missing = !value && missingKeys.has(row.key);
          return (
            <li key={row.key} className={`release-plan-param-row${missing ? " is-missing" : ""}`}>
              <div className="release-plan-param-meta">
                <span className="release-plan-param-key">{row.key}</span>
                {row.label !== row.key ? <span className="release-plan-param-label">{row.label}</span> : null}
                <span className="release-plan-param-usage" title={row.usedBy.join("\n")}>
                  {row.usedBy.join("、")}
                </span>
              </div>
              {editable ? (
                <input
                  key={`${row.key}-${value}`}
                  className="input text-sm release-plan-param-input"
                  type={row.secret ? "password" : "text"}
                  defaultValue={value}
                  disabled={busy}
                  placeholder={missing ? "必填" : "未填写"}
                  aria-label={`${row.key} 取值`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") e.currentTarget.blur();
                  }}
                  onBlur={(e) => commit(row.key, e.target.value)}
                />
              ) : (
                <span className="release-plan-param-value">
                  {value ? (row.secret ? "••••••" : value) : "未填写"}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
