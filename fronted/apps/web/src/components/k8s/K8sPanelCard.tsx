interface K8sPanelCardProps {
  children: React.ReactNode;
  className?: string;
}

/** Standard card shell for cluster resource panels. */
export function K8sPanelCard({ children, className }: K8sPanelCardProps) {
  return <section className={`k8s-resource-panel${className ? ` ${className}` : ""}`}>{children}</section>;
}
