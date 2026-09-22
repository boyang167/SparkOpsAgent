import { taskStatusTone } from "./workflow-run-canvas-utils";

export function WorkflowRunStatusIcon({ status, size = 18 }: { status: string; size?: number }) {
  const tone = taskStatusTone(status);
  if (tone === "is-running") {
    return <span className={`wf-run-live-icon ${tone}`} style={{ width: size, height: size }} aria-hidden="true" />;
  }
  if (tone === "is-success") {
    return (
      <svg className={`wf-run-live-icon ${tone}`} viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
        <circle cx="12" cy="12" r="10" fill="currentColor" opacity="0.14" />
        <path d="M8 12.5l2.5 2.5L16 9.5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  if (tone === "is-failed") {
    return (
      <svg className={`wf-run-live-icon ${tone}`} viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
        <circle cx="12" cy="12" r="10" fill="currentColor" opacity="0.14" />
        <path d="M9 9l6 6M15 9l-6 6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      </svg>
    );
  }
  if (tone === "is-paused") {
    return (
      <svg className={`wf-run-live-icon ${tone}`} viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
        <rect x="8" y="7" width="3" height="10" rx="1" fill="currentColor" />
        <rect x="13" y="7" width="3" height="10" rx="1" fill="currentColor" />
      </svg>
    );
  }
  return (
    <svg className={`wf-run-live-icon ${tone}`} viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
      <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" strokeWidth="1.6" opacity="0.45" />
      <path d="M12 8v5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      <circle cx="12" cy="16" r="0.9" fill="currentColor" />
    </svg>
  );
}
