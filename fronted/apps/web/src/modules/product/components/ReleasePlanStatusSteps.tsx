import { releasePlanStatusLabel } from "./release-plan-utils";

const STEPS = ["planning", "wait_for_approval", "executing", "success"] as const;

interface ReleasePlanStatusStepsProps {
  status: string;
  approvalEnabled?: boolean;
  layout?: "horizontal" | "vertical";
}

const PLANNING_STATUSES = new Set(["planning", "wait_for_finish_planning_external_check", "finish_planning_external_check_failed"]);
const APPROVAL_STATUSES = new Set(["wait_for_approval"]);
const EXECUTE_WAIT_STATUSES = new Set(["wait_for_execute_external_check", "execute_external_check_failed"]);
const EXECUTING_STATUSES = new Set(["executing", "wait_for_all_done_external_check", "all_done_external_check_failed"]);

function stepState(step: (typeof STEPS)[number], status: string, approvalEnabled: boolean) {
  if (status === step) return "active";
  if (step === "planning" && PLANNING_STATUSES.has(status)) return "active";
  if (step === "wait_for_approval" && APPROVAL_STATUSES.has(status)) return "active";
  if (step === "executing" && (EXECUTE_WAIT_STATUSES.has(status) || EXECUTING_STATUSES.has(status))) return "active";
  const done =
    (step === "planning" && !PLANNING_STATUSES.has(status)) ||
    (step === "wait_for_approval" && approvalEnabled && !PLANNING_STATUSES.has(status) && !APPROVAL_STATUSES.has(status)) ||
    (step === "executing" && status === "success") ||
    (step === "success" && status === "success");
  return done ? "done" : "pending";
}

export function ReleasePlanStatusSteps({
  status,
  approvalEnabled = false,
  layout = "horizontal",
}: ReleasePlanStatusStepsProps) {
  const visibleSteps = approvalEnabled ? STEPS : STEPS.filter((s) => s !== "wait_for_approval");

  return (
    <ol
      className={`release-plan-steps${layout === "vertical" ? " release-plan-steps-vertical" : ""}`}
      aria-label="发布计划进度"
    >
      {visibleSteps.map((step, index) => {
        const state = stepState(step, status, approvalEnabled);
        const prevDone = index > 0 && stepState(visibleSteps[index - 1], status, approvalEnabled) === "done";
        return (
          <li key={step} className="release-plan-step-wrap">
            {index > 0 ? (
              <span className={`release-plan-step-connector${prevDone ? " release-plan-step-connector-done" : ""}`} aria-hidden />
            ) : null}
            <span
              className={`release-plan-step release-plan-step-${state}`}
              aria-current={state === "active" ? "step" : undefined}
            >
              <span className="release-plan-step-marker" aria-hidden />
              <span className="release-plan-step-label">{releasePlanStatusLabel(step)}</span>
            </span>
          </li>
        );
      })}
    </ol>
  );
}
