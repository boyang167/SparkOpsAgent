interface K8sLoadingStateProps {
  label?: string;
  compact?: boolean;
}

export function K8sLoadingState({ label = "加载中…", compact = false }: K8sLoadingStateProps) {
  return (
    <div className={`k8s-loading-state${compact ? " is-compact" : ""}`} role="status" aria-live="polite">
      <span className="k8s-loading-spinner" aria-hidden />
      <span>{label}</span>
    </div>
  );
}
