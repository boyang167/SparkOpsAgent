"use client";

export function WorkflowStagePath({ labels, fallback }: { labels: string[]; fallback?: string }) {
  if (labels.length === 0) {
    return <span className="wf-list-path-empty">{fallback ?? "未配置阶段"}</span>;
  }
  return (
    <div className="wf-list-path-trail">
      {labels.map((label, index) => (
        <span key={`${label}-${index}`} className="wf-list-path-seg">
          {index > 0 ? <span className="wf-list-path-sep" aria-hidden="true">&gt;</span> : null}
          <span className="wf-list-path-chip">{label}</span>
        </span>
      ))}
    </div>
  );
}

export function WorkflowSparkline({ tones }: { tones: string[] }) {
  return (
    <div className="wf-list-sparkline" aria-hidden="true" title="最近运行结果">
      {tones.map((tone, index) => (
        <span key={index} className={`wf-list-spark is-${tone}`} />
      ))}
    </div>
  );
}

function SkeletonBar({ width }: { width: string }) {
  return <span className="wf-list-skeleton" style={{ width }} />;
}

export function WorkflowListSkeletonRows({ count = 5 }: { count?: number }) {
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="wf-list-row wf-list-row-skeleton">
          <div className="wf-list-row-main">
            <div className="wf-list-row-head">
              <SkeletonBar width="42%" />
            </div>
            <div className="wf-list-row-body">
              <span className="wf-list-skeleton wf-list-skeleton-fav" />
              <SkeletonBar width="8rem" />
              <SkeletonBar width="4rem" />
              <SkeletonBar width="3rem" />
              <div className="wf-list-last-run">
                <div className="wf-list-sparkline">
                  {Array.from({ length: 20 }, (_, j) => (
                    <span key={j} className="wf-list-spark is-empty" />
                  ))}
                </div>
                <SkeletonBar width="4.5rem" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </>
  );
}
