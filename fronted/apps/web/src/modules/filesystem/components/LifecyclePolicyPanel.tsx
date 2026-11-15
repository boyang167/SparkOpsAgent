"use client";

import { useCallback, useEffect, useState } from "react";
import type { LifecyclePolicyVM, LifecycleRuleAction } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { listLifecyclePolicies, upsertLifecyclePolicy } from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { canMutateLifecycle } from "./storage-admin-permissions";
import {
  buildLifecycleRulesPayload,
  emptyLifecycleRules,
  LIFECYCLE_ACTIONS,
  LIFECYCLE_CLASSIFICATION_VALUES,
  LIFECYCLE_SCOPE_VALUES,
  LIFECYCLE_WHEN_KEYS,
  type LifecycleRuleDraft,
} from "./lifecycle-policy-spec";
import { LIFECYCLE_ACTION_LABELS, LIFECYCLE_WHEN_LABELS } from "./lifecycle-policy-labels";

export type LifecyclePolicyPanelProps = {
  workspaceId: string;
  permissions: string[];
  policies?: LifecyclePolicyVM[];
};

function parseRules(rules: Record<string, unknown>): LifecycleRuleDraft[] {
  const raw = rules.rules;
  if (!Array.isArray(raw)) return [];
  const drafts: LifecycleRuleDraft[] = [];
  for (const item of raw) {
    if (!item || typeof item !== "object") continue;
    const record = item as Record<string, unknown>;
    const whenRaw = record.when;
    const when: Record<string, string | number> = {};
    if (whenRaw && typeof whenRaw === "object") {
      for (const key of LIFECYCLE_WHEN_KEYS) {
        const value = (whenRaw as Record<string, unknown>)[key];
        if (typeof value === "string" || typeof value === "number") {
          when[key] = value;
        }
      }
    }
    const actions = Array.isArray(record.actions)
      ? record.actions
          .map((action) => String(action).toLowerCase())
          .filter((action): action is LifecycleRuleAction =>
            LIFECYCLE_ACTIONS.includes(action as LifecycleRuleAction),
          )
      : [];
    drafts.push({
      id: typeof record.id === "string" ? record.id : `rule-${drafts.length + 1}`,
      when,
      actions,
    });
  }
  return drafts;
}

export function LifecyclePolicyPanel({
  workspaceId,
  permissions,
  policies: initialPolicies,
}: LifecyclePolicyPanelProps) {
  const canAdmin = canMutateLifecycle(permissions);
  const [policies, setPolicies] = useState<LifecyclePolicyVM[]>(initialPolicies ?? []);
  const [loading, setLoading] = useState(!initialPolicies);
  const [error, setError] = useState<string | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [policyName, setPolicyName] = useState("default");
  const [ruleDrafts, setRuleDrafts] = useState<LifecycleRuleDraft[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await listLifecyclePolicies(workspaceId);
      setPolicies(page.items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载生命周期策略失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    if (initialPolicies === undefined) {
      void load();
    }
  }, [initialPolicies, load]);

  function openEditor(policy?: LifecyclePolicyVM) {
    if (policy) {
      setPolicyName(policy.name);
      setRuleDrafts(parseRules(policy.rules));
    } else {
      setPolicyName("default");
      setRuleDrafts(emptyLifecycleRules().rules);
    }
    setEditorOpen(true);
  }

  function addRule() {
    setRuleDrafts((current) => [
      ...current,
      { id: `rule-${current.length + 1}`, when: { scope: "WORKSPACE" }, actions: ["archive"] },
    ]);
  }

  async function savePolicy() {
    if (!canAdmin || !policyName.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      await upsertLifecyclePolicy(workspaceId, {
        name: policyName.trim(),
        rules: buildLifecycleRulesPayload(ruleDrafts),
        status: "ACTIVE",
      });
      setEditorOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存策略失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="stack-lg">
      {canAdmin ? (
        <div className="flex flex-wrap items-start justify-end gap-3">
          <button type="button" className="btn btn-primary" onClick={() => openEditor()}>
            新建策略
          </button>
        </div>
      ) : null}

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : policies.length === 0 ? (
        <div className="empty-state">暂无生命周期策略</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>版本</th>
                <th>状态</th>
                <th>规则数</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {policies.map((policy) => {
                const ruleCount = parseRules(policy.rules).length;
                return (
                  <tr key={`${policy.id}-${policy.version}`}>
                    <td>{policy.name}</td>
                    <td>v{policy.version}</td>
                    <td>{policy.status}</td>
                    <td>{ruleCount}</td>
                    <td>
                      {canAdmin ? (
                        <button type="button" className="btn btn-ghost btn-sm" onClick={() => openEditor(policy)}>
                          编辑
                        </button>
                      ) : null}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal
        open={editorOpen}
        title="编辑生命周期策略"
        size="lg"
        panelClassName="filesystem-lifecycle-modal"
        onClose={() => setEditorOpen(false)}
        footer={
          <div className="modal-footer-actions">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={submitting}
              onClick={() => setEditorOpen(false)}
            >
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={submitting}
              onClick={() => void savePolicy()}
            >
              保存版本
            </button>
          </div>
        }
      >
        <div className="lifecycle-policy-modal-form">
          <label className="form-field">
            <span className="form-label">策略名称</span>
            <input
              className="input"
              value={policyName}
              onChange={(e) => setPolicyName(e.target.value)}
            />
          </label>
          <div className="lifecycle-policy-rules">
            {ruleDrafts.map((rule, index) => (
              <LifecycleRuleEditor
                key={`${rule.id}-${index}`}
                rule={rule}
                onChange={(next: LifecycleRuleDraft) =>
                  setRuleDrafts((current) => current.map((item, i) => (i === index ? next : item)))
                }
                onRemove={() => setRuleDrafts((current) => current.filter((_, i) => i !== index))}
              />
            ))}
          </div>
          <button type="button" className="btn btn-secondary btn-sm" onClick={addRule}>
            添加规则
          </button>
        </div>
      </Modal>
    </section>
  );
}

function LifecycleRuleEditor(props: {
  rule: LifecycleRuleDraft;
  onChange: (rule: LifecycleRuleDraft) => void;
  onRemove: () => void;
}) {
  const { rule, onChange, onRemove } = props;
  return (
    <article className="lifecycle-rule-card">
      <header className="lifecycle-rule-card-header">
        <h3 className="lifecycle-rule-card-title">规则 {rule.id}</h3>
        <button type="button" className="lifecycle-rule-remove" onClick={onRemove}>
          删除
        </button>
      </header>
      <label className="form-field lifecycle-rule-field">
        <span className="form-label">规则 ID</span>
        <input
          className="input"
          value={rule.id}
          onChange={(e) => onChange({ ...rule, id: e.target.value })}
        />
      </label>
      <section className="lifecycle-rule-section" aria-labelledby={`when-${rule.id}`}>
        <h4 id={`when-${rule.id}`} className="lifecycle-rule-section-title">
          When 条件
        </h4>
        <div className="lifecycle-rule-when-grid">
          {LIFECYCLE_WHEN_KEYS.map((key) => (
            <label key={key} className="form-field lifecycle-rule-field">
              <span className="form-label">{LIFECYCLE_WHEN_LABELS[key]}</span>
              {key === "scope" ? (
                <select
                  className="input"
                  value={String(rule.when[key] ?? "")}
                  onChange={(e) => {
                    const nextWhen = { ...rule.when };
                    if (e.target.value === "") {
                      delete nextWhen[key];
                    } else {
                      nextWhen[key] = e.target.value;
                    }
                    onChange({ ...rule, when: nextWhen });
                  }}
                >
                  <option value="">未设置</option>
                  {LIFECYCLE_SCOPE_VALUES.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              ) : key === "classification" ? (
                <select
                  className="input"
                  value={String(rule.when[key] ?? "")}
                  onChange={(e) => {
                    const nextWhen = { ...rule.when };
                    if (e.target.value === "") {
                      delete nextWhen[key];
                    } else {
                      nextWhen[key] = e.target.value;
                    }
                    onChange({ ...rule, when: nextWhen });
                  }}
                >
                  <option value="">未设置</option>
                  {LIFECYCLE_CLASSIFICATION_VALUES.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  className="input"
                  type={key === "min_size" || key === "max_age_days" ? "number" : "text"}
                  min={key === "min_size" || key === "max_age_days" ? 0 : undefined}
                  value={rule.when[key] ?? ""}
                  onChange={(e) => {
                    const raw = e.target.value;
                    const nextWhen = { ...rule.when };
                    if (key === "min_size" || key === "max_age_days") {
                      if (raw === "") {
                        delete nextWhen[key];
                      } else {
                        nextWhen[key] = Number(raw);
                      }
                    } else {
                      nextWhen[key] = raw;
                    }
                    onChange({ ...rule, when: nextWhen });
                  }}
                />
              )}
            </label>
          ))}
        </div>
      </section>
      <section className="lifecycle-rule-section" aria-labelledby={`actions-${rule.id}`}>
        <h4 id={`actions-${rule.id}`} className="lifecycle-rule-section-title">
          动作
        </h4>
        <div className="lifecycle-rule-actions">
          {LIFECYCLE_ACTIONS.map((action) => {
            const checked = rule.actions.includes(action);
            return (
              <label
                key={action}
                className={checked ? "lifecycle-rule-action is-checked" : "lifecycle-rule-action"}
              >
                <input
                  type="checkbox"
                  className="sr-only"
                  checked={checked}
                  onChange={(e) => {
                    const actions = e.target.checked
                      ? [...rule.actions, action]
                      : rule.actions.filter((item) => item !== action);
                    onChange({ ...rule, actions });
                  }}
                />
                <span>{LIFECYCLE_ACTION_LABELS[action] ?? action}</span>
              </label>
            );
          })}
        </div>
      </section>
    </article>
  );
}
