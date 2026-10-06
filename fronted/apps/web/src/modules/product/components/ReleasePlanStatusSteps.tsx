import { releasePlanStatusLabel } from "./release-plan-utils";

const STEPS = ["planning", "wait_for_approval", "executing", "success"] as const;

interface ReleasePlanStatusStepsProps {
  status: string;
  approvalEnabled?: boolean;
}

function stepState(step: (typeof STEPS)[number], status: string, approvalEnabled: boolean) {
  if (status === step) return "active";
  const done =
    (step === "planning" && status !== "planning") ||
    (step === "wait_for_approval" && !["planning", "wait_for_approval"].includes(status) && approvalEnabled) ||
    (step === "executing" && ["executing", "success"].includes(status)) ||
    (step === "success" && status === "success");
  return done ? "done" : "pending";
}

export function ReleasePlanStatusSteps({ status, approvalEnabled = false }: ReleasePlanStatusStepsProps) {
  const visibleSteps = approvalEnabled ? STEPS : STEPS.filter((s) => s !== "wait_for_approval");

  return (
    <ol className="release-plan-steps" aria-label="发布计划进度">
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
