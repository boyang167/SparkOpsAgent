"use client";

export function EndpointForm({
  children,
  disabled,
  submitLabel,
  onSubmit,
}: {
  children: React.ReactNode;
  disabled: boolean;
  submitLabel: string;
  onSubmit: () => Promise<void>;
}) {
  return (
    <form
      className="stack-md"
      onSubmit={(e) => {
        e.preventDefault();
        void onSubmit();
      }}
    >
      {disabled ? <div className="alert alert-info">需要选择集群并拥有 governance:admin 权限。</div> : null}
      <div className="grid gap-3 md:grid-cols-3">{children}</div>
      <div className="flex justify-end">
        <button type="submit" className="btn" disabled={disabled}>
          {submitLabel}
        </button>
      </div>
    </form>
  );
}

export function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label>
      <span className="label">{label}</span>
      {children}
      {error ? <span className="error mt-1 block">{error}</span> : null}
    </label>
  );
}
