export type K8sStatusTone = "success" | "warning" | "danger" | "neutral" | "info";

interface K8sStatusBadgeProps {
  tone?: K8sStatusTone;
  children: React.ReactNode;
  dot?: boolean;
  title?: string;
}

export function K8sStatusBadge({ tone = "neutral", children, dot = false, title }: K8sStatusBadgeProps) {
  return (
    <span className={`k8s-status-badge is-${tone}${dot ? " has-dot" : ""}`} title={title}>
      {dot ? <span className="k8s-status-dot" aria-hidden /> : null}
      {children}
    </span>
  );
}

/** Map common K8s phases to badge tones. */
export function k8sPhaseTone(phase: string): K8sStatusTone {
  const p = phase.toLowerCase();
  if (p === "running" || p === "active" || p === "bound" || p === "ready" || p === "succeeded") {
    return "success";
  }
  if (p === "pending" || p === "progressing" || p === "terminating") return "warning";
  if (p === "failed" || p === "error" || p === "crashloopbackoff" || p === "lost") return "danger";
  return "neutral";
}
