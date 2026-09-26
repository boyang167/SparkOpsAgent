import { releasePlanStatusLabel } from "./release-plan-utils";

const STEPS = ["planning", "wait_for_approval", "executing", "success"] as const;

interface ReleasePlanStatusStepsProps {
  status: string;
  approvalEnabled?: boolean;
}

export function ReleasePlanStatusSteps({ status, approvalEnabled }: ReleasePlanStatusStepsProps) {
  const visibleSteps = approvalEnabled ? STEPS : STEPS.filter((s) => s !== "wait_for_approval");

  return (
    <ol className="flex flex-wrap gap-2 text-xs">
      {visibleSteps.map((step) => {
        const active = status === step;
        const done =
          (step === "planning" && status !== "planning") ||
          (step === "wait_for_approval" &&
            !["planning", "wait_for_approval"].includes(status) &&
            Boolean(approvalEnabled)) ||
          (step === "executing" && ["executing", "success"].includes(status)) ||
          (step === "success" && status === "success");
        return (
          <li
            key={step}
            className={`rounded px-2 py-1 border ${
              active ? "border-primary bg-primary/5 text-primary" : done ? "border-green-600/30 text-green-700" : "text-muted-foreground"
            }`}
          >
            {releasePlanStatusLabel(step)}
          </li>
        );
      })}
    </ol>
  );
}
