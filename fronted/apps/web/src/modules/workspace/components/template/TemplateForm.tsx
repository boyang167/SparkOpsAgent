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

export function TemplateForm({
  form,
  isEdit,
  submitting,
  onChange,
  onSubmit,
  onCancel,
}: TemplateFormProps) {
  return (
    <form onSubmit={onSubmit} className="template-form stack-md">
      <label className="template-field">
        <span className="template-field-label">模板名称</span>
        <input
          className="input"
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder="请输入模板名称"
          required
          minLength={2}
          maxLength={50}
        />
        <span className="template-field-hint">{form.name.length}/50</span>
      </label>

      <div className="template-form-grid">
        <label className="template-field">
          <span className="template-field-label">模板分类</span>
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
          <span className="template-field-label">开发语言</span>
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
      </div>

      <div className="template-form-grid">
        <label className="template-field">
          <span className="template-field-label">模板状态</span>
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
        <label className="template-switch-field">
          <span className="template-field-label">是否公开</span>
          <div className="template-switch-row">
            <button
              type="button"
              className={`template-switch ${form.is_public ? "is-on" : ""}`}
              role="switch"
              aria-checked={form.is_public}
              onClick={() => onChange({ ...form, is_public: !form.is_public })}
            >
              <span className="template-switch-thumb" />
            </button>
            <span className="text-sm text-muted-foreground">
              {form.is_public ? "组织内所有工作区可见" : "仅当前工作区可见"}
            </span>
          </div>
        </label>
      </div>

      <label className="template-field">
        <span className="template-field-label">模板内容</span>
        <textarea
          className="input template-content-input"
          value={form.detail}
          onChange={(e) => onChange({ ...form, detail: e.target.value })}
          placeholder="请输入模板内容"
          required
          minLength={10}
          maxLength={5000}
        />
        <span className="template-field-hint">{form.detail.length}/5000</span>
      </label>

      <div className="template-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>取消</button>
        <button type="submit" className="btn" disabled={submitting}>
          {submitting ? "保存中…" : isEdit ? "更新" : "创建"}
        </button>
      </div>
    </form>
  );
}
