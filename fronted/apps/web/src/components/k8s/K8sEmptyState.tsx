interface K8sEmptyStateProps {
  variant?: "no-cluster" | "empty" | "missing-def" | "custom";
  title?: string;
  description?: string;
  action?: React.ReactNode;
  compact?: boolean;
  kind?: string;
}

const PRESETS: Record<
  Exclude<K8sEmptyStateProps["variant"], "custom" | undefined>,
  { title: string; description?: string }
> = {
  "no-cluster": {
    title: "请先选择集群",
    description: "在上方选择或接入一个 Kubernetes 集群后，即可管理资源。",
  },
  empty: {
    title: "暂无数据",
    description: "当前筛选条件下没有匹配的资源，可调整命名空间或搜索条件。",
  },
  "missing-def": {
    title: "资源类型不可用",
    description: "缺少对应的 API 资源定义，请联系平台管理员检查插件配置。",
  },
};

export function K8sEmptyState({
  variant = "custom",
  title,
  description,
  action,
  compact = false,
  kind,
}: K8sEmptyStateProps) {
  const preset = variant !== "custom" ? PRESETS[variant] : null;
  const resolvedTitle =
    title ?? (variant === "missing-def" && kind ? `缺少 ${kind} 资源定义` : preset?.title ?? "暂无内容");
  const resolvedDescription = description ?? preset?.description;

  return (
    <div
      className={`k8s-empty-state${compact ? " is-compact" : ""}`}
      role={variant === "empty" || variant === "no-cluster" ? "status" : undefined}
    >
      <div className="k8s-empty-state-icon" aria-hidden>
        {variant === "no-cluster" ? "⎈" : variant === "empty" ? "∅" : "!"}
      </div>
      <strong className="k8s-empty-state-title">{resolvedTitle}</strong>
      {resolvedDescription ? <p className="k8s-empty-state-desc">{resolvedDescription}</p> : null}
      {action ? <div className="k8s-empty-state-action">{action}</div> : null}
    </div>
  );
}
