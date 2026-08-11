"use client";

export function FormRow({
  label,
  required,
  hint,
  labelExtra,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  labelExtra?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="project-form-row">
      <label className="project-form-label">
        {required ? <span className="project-required">*</span> : null}
        {label}
      </label>
      <div className="project-form-control">
        {labelExtra}
        {children}
        {hint ? <p className="project-form-hint">{hint}</p> : null}
      </div>
    </div>
  );
}

export function CharCounter({ value, max }: { value: string; max: number }) {
  return (
    <span className="project-char-counter">
      {value.length} / {max}
    </span>
  );
}
