interface ReleasePlanModalFooterProps {
  onCancel: () => void;
  onSubmit: () => void;
  submitLabel?: string;
  submittingLabel?: string;
  busy?: boolean;
  disabled?: boolean;
}

export function ReleasePlanModalFooter({
  onCancel,
  onSubmit,
  submitLabel = "保存",
  submittingLabel = "保存中…",
  busy,
  disabled,
}: ReleasePlanModalFooterProps) {
  return (
    <div className="modal-footer-actions">
      <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={onCancel}>
        取消
      </button>
      <button type="button" className="btn btn-primary text-sm" disabled={busy || disabled} onClick={onSubmit}>
        {busy ? submittingLabel : submitLabel}
      </button>
    </div>
  );
}
