import type { ReactNode } from "react";

function ChevronIcon() {
  return (
    <svg className="k8s-action-sheet-chevron" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M8 5l4 5-4 5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function K8sActionSheet({ children }: { children: ReactNode }) {
  return (
    <div className="k8s-action-sheet" role="menu">
      {children}
    </div>
  );
}

export function K8sActionSheetDivider() {
  return <div className="k8s-action-sheet-divider" role="separator" />;
}

export function K8sActionSheetGroup({
  title,
  danger = false,
  children,
}: {
  title?: string;
  danger?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      className={`k8s-action-sheet-group${danger ? " is-danger" : ""}`}
      role="group"
      aria-label={title ?? (danger ? "危险操作" : undefined)}
    >
      {title ? <div className="k8s-action-sheet-group-label">{title}</div> : null}
      <div className="k8s-action-sheet-group-body">{children}</div>
    </div>
  );
}

export function K8sActionSheetItem({
  label,
  icon,
  danger = false,
  showChevron = false,
  onClick,
}: {
  label: string;
  icon?: ReactNode;
  danger?: boolean;
  showChevron?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      className={`k8s-action-sheet-item${danger ? " is-danger" : ""}${showChevron ? " has-chevron" : ""}`}
      onClick={onClick}
    >
      {icon ? <span className="k8s-action-sheet-icon">{icon}</span> : null}
      <span className="k8s-action-sheet-label">{label}</span>
      {showChevron ? <ChevronIcon /> : null}
    </button>
  );
}

export const K8sActionIcons = {
  detail: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <circle cx="10" cy="10" r="6.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 9v4M10 7h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  ),
  shell: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M5.5 8.5 8 10.5l-2.5 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M10.5 13h4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <rect x="3.5" y="4.5" width="13" height="11" rx="2" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  ),
  monitor: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M4 14V8.5M8 14V6M12 14v-3M16 14V5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  ),
  yaml: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M6.5 5.5 4 10l2.5 4.5M13.5 5.5 16 10l-2.5 4.5M11 5l-2 10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  drain: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M10 4v8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M7 9.5 10 12.5 13 9.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5.5 15.5h9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  ),
  schedule: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <circle cx="10" cy="10" r="6.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 6.5V10l2.5 1.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  delete: (
    <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M7.5 7.5v5.5M12.5 7.5v5.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M4.5 6.5h11M8 4.5h4l.5 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M6 6.5v8.5a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V6.5" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  ),
};
