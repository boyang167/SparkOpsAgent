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
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">生命周期策略</h2>
          <p className="text-sm text-muted">仅允许规格定义的筛选条件与动作。</p>
        </div>
        {canAdmin ? (
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary" onClick={() => openEditor()}>
              新建策略
            </button>
          </div>
        ) : null}
      </div>

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

      <Modal open={editorOpen} title="编辑生命周期策略" onClose={() => setEditorOpen(false)}>
        <div className="stack-md">
          <label className="stack-xs">
            <span>策略名称</span>
            <input className="input" value={policyName} onChange={(e) => setPolicyName(e.target.value)} />
          </label>
          {ruleDrafts.map((rule, index) => (
            <LifecycleRuleEditor
              key={rule.id}
              rule={rule}
              onChange={(next: LifecycleRuleDraft) =>
                setRuleDrafts((current) => current.map((item, i) => (i === index ? next : item)))
              }
              onRemove={() => setRuleDrafts((current) => current.filter((_, i) => i !== index))}
            />
          ))}
          <button type="button" className="btn btn-secondary" onClick={addRule}>
            添加规则
          </button>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn btn-ghost" disabled={submitting} onClick={() => setEditorOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary" disabled={submitting} onClick={() => void savePolicy()}>
              保存版本
            </button>
          </div>
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
    <div className="rounded-lg border border-border p-3 stack-sm">
      <RuleSectionHeader id={rule.id} onRemove={onRemove} />
      <label className="stack-xs">
        <span>规则 ID</span>
        <input className="input" value={rule.id} onChange={(e) => onChange({ ...rule, id: e.target.value })} />
      </label>
      <fieldset className="stack-sm">
        <legend className="text-sm font-medium">When 条件</legend>
        {LIFECYCLE_WHEN_KEYS.map((key) => (
          <label key={key} className="stack-xs">
            <span>{key}</span>
            {key === "scope" ? (
              <select
                className="input"
                value={String(rule.when[key] ?? "")}
                onChange={(e) =>
                  onChange({ ...rule, when: { ...rule.when, [key]: e.target.value } })
                }
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
                onChange={(e) =>
                  onChange({ ...rule, when: { ...rule.when, [key]: e.target.value } })
                }
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
                value={rule.when[key] ?? ""}
                onChange={(e) =>
                  onChange({
                    ...rule,
                    when: {
                      ...rule.when,
                      [key]: key === "min_size" || key === "max_age_days" ? Number(e.target.value) : e.target.value,
                    },
                  })
                }
              />
            )}
          </label>
        ))}
      </fieldset>
      <fieldset className="stack-xs">
        <legend className="text-sm font-medium">动作</legend>
        {LIFECYCLE_ACTIONS.map((action) => (
          <label key={action} className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={rule.actions.includes(action)}
              onChange={(e) => {
                const actions = e.target.checked
                  ? [...rule.actions, action]
                  : rule.actions.filter((item) => item !== action);
                onChange({ ...rule, actions });
              }}
            />
            {action}
          </label>
        ))}
      </fieldset>
    </div>
  );
}

function RuleSectionHeader({ id, onRemove }: { id: string; onRemove: () => void }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm font-medium">规则 {id}</span>
      <button type="button" className="btn btn-ghost btn-sm" onClick={onRemove}>
        删除
      </button>
    </div>
  );
}
