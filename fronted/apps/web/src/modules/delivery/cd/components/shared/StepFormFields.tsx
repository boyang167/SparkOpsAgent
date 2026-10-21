"use client";

import type { StepCatalogEntry, StepFieldDef } from "../../types";

interface StepFormFieldsProps {
  entry: StepCatalogEntry;
  values: Record<string, unknown>;
  onChange: (values: Record<string, unknown>) => void;
}

export function StepFormFields({ entry, values, onChange }: StepFormFieldsProps) {
  if (entry.fields.length === 0) {
    return <p className="muted text-sm">此步骤无需额外参数。</p>;
  }

  function updateField(key: string, value: unknown) {
    onChange({ ...values, [key]: value });
  }

  return (
    <div className="cd-form-grid">
      {entry.fields.map((field) => (
        <FieldControl
          key={field.key}
          field={field}
          value={values[field.key]}
          onChange={(next) => updateField(field.key, next)}
        />
      ))}
    </div>
  );
}

function FieldControl({
  field,
  value,
  onChange,
}: {
  field: StepFieldDef;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const label = (
    <label className="cd-field-label">
      {field.label}
      {field.required ? <span className="cd-required">*</span> : null}
    </label>
  );

  if (field.type === "select") {
    return (
      <div className="cd-field">
        {label}
        <select
          className="input"
          value={String(value ?? field.options?.[0]?.value ?? "")}
          onChange={(e) => onChange(e.target.value)}
        >
          {(field.options ?? []).map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
    );
  }

  if (field.type === "boolean") {
    return (
      <div className="cd-field cd-field-inline">
        <input
          type="checkbox"
          checked={Boolean(value)}
          onChange={(e) => onChange(e.target.checked)}
        />
        {label}
      </div>
    );
  }

  if (field.type === "number") {
    return (
      <div className="cd-field">
        {label}
        <input
          className="input"
          type="number"
          value={value === undefined || value === null ? "" : String(value)}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
        />
      </div>
    );
  }

  if (field.type === "string-list") {
    const list = Array.isArray(value) ? value.map(String) : value ? [String(value)] : [""];
    return (
      <div className="cd-field">
        {label}
        <div className="stack-xs">
          {list.map((item, index) => (
            <div key={index} className="cd-inline-row">
              <input
                className="input"
                value={item}
                placeholder={field.placeholder}
                onChange={(e) => {
                  const next = [...list];
                  next[index] = e.target.value;
                  onChange(next.filter(Boolean));
                }}
              />
              <button
                type="button"
                className="btn-ghost text-sm"
                onClick={() => onChange(list.filter((_, i) => i !== index))}
              >
                删除
              </button>
            </div>
          ))}
          <button
            type="button"
            className="btn-ghost text-sm"
            onClick={() => onChange([...list, ""])}
          >
            + 添加
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="cd-field">
      {label}
      <input
        className="input"
        value={value === undefined || value === null ? "" : String(value)}
        placeholder={field.placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}
