"use client";

import { useEffect, useState } from "react";
import {
  buildNamespaceFromDraft,
  emptyNamespaceCreateDraft,
  emptyNamespaceTemplate,
  type NamespaceCreateDraft,
  type NamespaceCreateDraftErrors,
  type NamespaceLabelRow,
  validateNamespaceCreateDraft,
} from "../namespace-view";
import { YamlEditor } from "../YamlEditor";

type CreateMode = "form" | "yaml";

type NamespaceCreateFormProps = {
  open: boolean;
  canSave: boolean;
  saving: boolean;
  onSave: (manifest: unknown) => Promise<void>;
  onClose: () => void;
};

export function NamespaceCreateForm({ open, canSave, saving, onSave, onClose }: NamespaceCreateFormProps) {
  const [mode, setMode] = useState<CreateMode>("form");
  const [draft, setDraft] = useState<NamespaceCreateDraft>(() => emptyNamespaceCreateDraft());
  const [errors, setErrors] = useState<NamespaceCreateDraftErrors>({});

  useEffect(() => {
    if (!open) return;
    setMode("form");
    setDraft(emptyNamespaceCreateDraft());
    setErrors({});
  }, [open]);

  async function submitForm() {
    const nextErrors = validateNamespaceCreateDraft(draft);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    await onSave(buildNamespaceFromDraft(draft));
  }

  return (
    <div className="stack-md">
      <div className="k8s-segmented" role="tablist" aria-label="创建方式">
        <button
          type="button"
          role="tab"
          aria-selected={mode === "form"}
          className={`k8s-segmented-btn${mode === "form" ? " is-active" : ""}`}
          onClick={() => setMode("form")}
        >
          表单
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mode === "yaml"}
          className={`k8s-segmented-btn${mode === "yaml" ? " is-active" : ""}`}
          onClick={() => setMode("yaml")}
        >
          YAML
        </button>
      </div>

      {mode === "form" ? (
        <form
          className="stack-md"
          onSubmit={(e) => {
            e.preventDefault();
            void submitForm();
          }}
        >
          <label className="k8s-form-field">
            <span className="label">
              名称 <span className="text-danger">*</span>
            </span>
            <input
              className="input"
              value={draft.name}
              placeholder="例如 my-app"
              autoComplete="off"
              spellCheck={false}
              disabled={!canSave || saving}
              onChange={(e) => {
                setDraft((current) => ({ ...current, name: e.target.value }));
                if (errors.name) setErrors((current) => ({ ...current, name: undefined }));
              }}
            />
            <span className="k8s-form-field-hint">小写字母、数字与连字符，最长 63 字符</span>
            {errors.name ? <span className="error mt-1 block">{errors.name}</span> : null}
          </label>

          <NamespaceLabelEditor
            rows={draft.labels}
            disabled={!canSave || saving}
            error={errors.labels}
            onChange={(labels) => {
              setDraft((current) => ({ ...current, labels }));
              if (errors.labels) setErrors((current) => ({ ...current, labels: undefined }));
            }}
          />

          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" disabled={saving} onClick={onClose}>
              取消
            </button>
            <button type="submit" className="btn" disabled={!canSave || saving}>
              {saving ? "创建中…" : "创建"}
            </button>
          </div>
        </form>
      ) : (
        <YamlEditor
          value={emptyNamespaceTemplate()}
          canSave={canSave}
          saving={saving}
          confirmLabel="创建"
          cancelLabel="取消"
          onSave={onSave}
          onClose={onClose}
        />
      )}
    </div>
  );
}

function NamespaceLabelEditor({
  rows,
  disabled,
  error,
  onChange,
}: {
  rows: NamespaceLabelRow[];
  disabled?: boolean;
  error?: string;
  onChange: (next: NamespaceLabelRow[]) => void;
}) {
  return (
    <div className="stack-sm">
      <span className="label">标签（可选）</span>
      <div className="k8s-kv-table">
        <div className="k8s-kv-head">
          <span>键</span>
          <span>值</span>
          <span />
        </div>
        {rows.length === 0 ? <p className="text-sm text-muted px-1">暂无标签，可按需添加。</p> : null}
        {rows.map((row, index) => (
          <div key={index} className="k8s-kv-row">
            <input
              className="input"
              placeholder="例如 team"
              value={row.key}
              disabled={disabled}
              onChange={(e) => onChange(rows.map((item, i) => (i === index ? { ...item, key: e.target.value } : item)))}
            />
            <input
              className="input"
              placeholder="例如 platform"
              value={row.value}
              disabled={disabled}
              onChange={(e) => onChange(rows.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))}
            />
            <button
              type="button"
              className="k8s-action-link k8s-kv-delete"
              disabled={disabled}
              onClick={() => onChange(rows.filter((_, i) => i !== index))}
            >
              删除
            </button>
          </div>
        ))}
        <button
          type="button"
          className="k8s-action-link"
          disabled={disabled}
          onClick={() => onChange([...rows, { key: "", value: "" }])}
        >
          + 添加标签
        </button>
      </div>
      {error ? <span className="error mt-1 block">{error}</span> : null}
    </div>
  );
}
