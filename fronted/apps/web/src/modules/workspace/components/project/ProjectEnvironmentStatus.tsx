"use client";

import {
  environmentStatusLabel,
  environmentStatusTone,
  environmentVersionLabel,
  isRunOnline,
  type EnvironmentDeployRow,
} from "./project-environment-status";

interface ProjectEnvironmentStatusProps {
  items: EnvironmentDeployRow[];
  loading: boolean;
  error: string | null;
  onUndeploy?: (row: EnvironmentDeployRow) => void;
}

function StatusBadge({ label, tone }: { label: string; tone: string }) {
  return (
    <span className={`project-status-badge is-${tone}`}>
      <span className="project-status-badge-dot" aria-hidden />
      {label}
    </span>
  );
}

export function ProjectEnvironmentStatus({
  items,
  loading,
  error,
  onUndeploy,
}: ProjectEnvironmentStatusProps) {
  const deployedCount = items.filter((item) => isRunOnline(item.latest)).length;

  return (
    <section className="project-overview-panel project-env-panel" aria-label="环境部署">
      <header className="project-overview-panel-head">
        <h4 className="project-overview-panel-title">环境部署</h4>
        {!loading && !error && items.length > 0 ? (
          <span className="project-overview-panel-meta">
            {deployedCount}/{items.length} 已上线
          </span>
        ) : null}
      </header>
      {loading ? (
        <p className="project-overview-panel-empty" role="status">加载环境信息…</p>
      ) : error ? (
        <p className="project-overview-panel-empty">{error}</p>
      ) : items.length === 0 ? (
        <p className="project-overview-panel-empty">暂无可用部署环境</p>
      ) : (
        <ul className="project-env-list">
          {items.map((item) => {
            const statusLabel = environmentStatusLabel(item.latest);
            const statusTone = environmentStatusTone(item.latest);
            const version = environmentVersionLabel(item.latest);
            const canUndeploy = Boolean(onUndeploy) && isRunOnline(item.latest);
            return (
              <li key={item.clusterId} className={`project-env-item is-${statusTone}`}>
                <span className="project-env-chip" title={item.clusterName}>
                  {item.clusterName}
                </span>
                <span className="project-env-version" title={version}>
                  {version}
                </span>
                <StatusBadge label={statusLabel} tone={statusTone} />
                {canUndeploy ? (
                  <button
                    type="button"
                    className="project-env-undeploy"
                    aria-label={`下线 ${item.clusterName}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      onUndeploy?.(item);
                    }}
                  >
                    下线
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
