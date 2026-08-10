import type { WorkflowRunVM } from "@sparkops/contracts/workflow";
import {
  formatWorkflowTime,
  parseWorkflowRunLogs,
  statusBadgeClass,
} from "../utils/workflowRun";

interface WorkflowRunDetailPanelProps {
  run: WorkflowRunVM;
}

function stepTypeLabel(type: string) {
  if (type === "log") return "日志";
  if (type === "noop") return "空操作";
  return type;
}

export function WorkflowRunDetailPanel({ run }: WorkflowRunDetailPanelProps) {
  const steps = parseWorkflowRunLogs(run.output);

  return (
    <div className="stack-lg">
      <section className="card stack-md">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-sm text-muted">运行 ID</p>
            <p className="font-mono text-sm break-all">{run.id}</p>
          </div>
          <span className={`badge ${statusBadgeClass(run.status)}`}>{run.status}</span>
        </div>

        <dl className="grid gap-3 sm:grid-cols-2 text-sm">
          <div>
            <dt className="text-muted">开始时间</dt>
            <dd>{formatWorkflowTime(run.started_at)}</dd>
          </div>
          <div>
            <dt className="text-muted">结束时间</dt>
            <dd>{run.completed_at ? formatWorkflowTime(run.completed_at) : "—"}</dd>
          </div>
        </dl>

        {run.input?.trim() ? (
          <div>
            <p className="text-sm text-muted mb-2">输入</p>
            <pre className="text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap">
              {run.input}
            </pre>
          </div>
        ) : null}

        {run.error ? (
          <div>
            <p className="text-sm text-muted mb-2">错误</p>
            <p className="error text-sm">{run.error}</p>
          </div>
        ) : null}
      </section>

      <section className="section stack-md">
        <div>
          <h2 className="section-title">步骤日志</h2>
          <p className="text-muted mt-1">
            共 {steps.length} 步 · 由 definition steps 同步执行产生
          </p>
        </div>

        {steps.length === 0 ? (
          <div className="empty-state">暂无步骤日志</div>
        ) : (
          <ol className="stack-sm">
            {steps.map((step) => (
              <li key={`${step.index}-${step.type}`} className="card flex gap-3">
                <span className="badge badge-neutral shrink-0 h-fit">#{step.index}</span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium">{stepTypeLabel(step.type)}</span>
                    <span className="badge badge-neutral text-xs">{step.type}</span>
                  </div>
                  {step.message ? (
                    <p className="text-sm mt-2 whitespace-pre-wrap break-words">{step.message}</p>
                  ) : null}
                </div>
              </li>
            ))}
          </ol>
        )}

        {run.output && steps.length === 0 ? (
          <pre className="text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap">
            {run.output}
          </pre>
        ) : null}
      </section>
    </div>
  );
}
