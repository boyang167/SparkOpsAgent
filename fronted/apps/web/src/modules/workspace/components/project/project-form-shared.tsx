"use client";

export function FormRow({
  label,
  required,
  hint,
  error,
  labelExtra,
  compactHint,
  align = "start",
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  error?: string | null;
  labelExtra?: React.ReactNode;
  compactHint?: boolean;
  align?: "start" | "center";
  children: React.ReactNode;
}) {
  return (
    <div className={`project-form-row${error ? " has-error" : ""}${align === "center" ? " is-center" : ""}`}>
      <label className="project-form-label">
        {required ? <span className="project-required">*</span> : null}
        {label}
      </label>
      <div className="project-form-control">
        {labelExtra}
        {children}
        {error ? <p className="project-form-error">{error}</p> : null}
        {!error && hint ? (
          <p
            className={`project-form-hint${compactHint ? " is-compact" : ""}`}
            title={compactHint ? hint : undefined}
          >
            {hint}
          </p>
        ) : null}
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
