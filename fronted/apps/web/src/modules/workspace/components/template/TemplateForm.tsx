"use client";

import type { CreateTemplateCommand } from "@sparkops/contracts/product";
import {
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
} from "./template-utils";

interface TemplateFormProps {
  form: CreateTemplateCommand;
  isEdit: boolean;
  submitting: boolean;
  onChange: (form: CreateTemplateCommand) => void;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
}

const NAME_MAX = 50;
const DETAIL_MAX = 5000;

function nearLimit(length: number, max: number) {
  return length >= max * 0.8;
}

export function TemplateForm({
  form,
  isEdit,
  submitting,
  onChange,
  onSubmit,
  onCancel,
}: TemplateFormProps) {
  return (
    <form onSubmit={onSubmit} className="template-form">
      <label className="template-field">
        <span className="template-field-label">名称</span>
        <input
          className="input"
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          required
          minLength={2}
          maxLength={NAME_MAX}
        />
        {nearLimit(form.name.length, NAME_MAX) ? (
          <span className="template-field-hint">{form.name.length}/{NAME_MAX}</span>
        ) : null}
      </label>

      <div className="template-form-row3">
        <label className="template-field">
          <span className="template-field-label">分类</span>
          <select
            className="input"
            value={form.kind}
            onChange={(e) => onChange({ ...form, kind: e.target.value })}
            required
          >
            {TEMPLATE_KIND_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </label>
        <label className="template-field">
          <span className="template-field-label">语言</span>
          <select
            className="input"
            value={form.language}
            onChange={(e) => onChange({ ...form, language: e.target.value })}
            required
          >
            {TEMPLATE_LANGUAGE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </label>
        <label className="template-field">
          <span className="template-field-label">状态</span>
          <select
            className="input"
            value={form.status}
            onChange={(e) => onChange({ ...form, status: e.target.value as CreateTemplateCommand["status"] })}
          >
            {TEMPLATE_STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </label>
      </div>

      <label className="template-field">
        <span className="template-field-label">内容</span>
        <textarea
          className="input template-content-input template-content-editor"
          value={form.detail}
          onChange={(e) => onChange({ ...form, detail: e.target.value })}
          spellCheck={false}
          required
          minLength={10}
          maxLength={DETAIL_MAX}
        />
        {nearLimit(form.detail.length, DETAIL_MAX) ? (
          <span className="template-field-hint">{form.detail.length}/{DETAIL_MAX}</span>
        ) : null}
      </label>

      <div className="template-form-footer">
        <label className="template-switch-inline">
          <button
            type="button"
            className={`template-switch ${form.is_public ? "is-on" : ""}`}
            role="switch"
            aria-checked={form.is_public}
            onClick={() => onChange({ ...form, is_public: !form.is_public })}
          >
            <span className="template-switch-thumb" />
          </button>
          <span>组织内公开</span>
        </label>
        <div className="template-form-actions">
          <button type="button" className="btn-secondary" onClick={onCancel}>取消</button>
          <button type="submit" className="btn" disabled={submitting}>
            {submitting ? "保存中…" : isEdit ? "保存" : "创建"}
          </button>
        </div>
      </div>
    </form>
  );
}
