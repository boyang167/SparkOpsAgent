interface K8sResourceTabsProps<T extends string> {
  tabs: readonly T[];
  active: T;
  onChange: (tab: T) => void;
  ariaLabel: string;
  label?: (tab: T) => string;
}

export function K8sResourceTabs<T extends string>({
  tabs,
  active,
  onChange,
  ariaLabel,
  label = (tab) => tab,
}: K8sResourceTabsProps<T>) {
  return (
    <div className="k8s-resource-tabs" role="tablist" aria-label={ariaLabel}>
      {tabs.map((tab) => (
        <button
          key={tab}
          type="button"
          role="tab"
          aria-selected={active === tab}
          className={`k8s-resource-tab${active === tab ? " is-active" : ""}`}
          onClick={() => onChange(tab)}
        >
          {label(tab)}
        </button>
      ))}
    </div>
  );
}
