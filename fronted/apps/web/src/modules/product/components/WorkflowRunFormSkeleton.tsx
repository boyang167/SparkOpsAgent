export function WorkflowRunFormSkeleton() {
  return (
    <div className="wf-run-loading" role="status" aria-label="加载执行配置">
      <div className="wf-run-loading-pills" aria-hidden="true">
        <span className="wf-run-loading-pill" />
        <span className="wf-run-loading-pill" />
      </div>
      <div className="wf-run-loading-block" />
      <div className="wf-run-loading-block" />
      <div className="wf-run-loading-block short" />
    </div>
  );
}
