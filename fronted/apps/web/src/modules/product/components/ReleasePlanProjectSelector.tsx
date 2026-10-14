"use client";

import type { ProductProjectVM } from "@sparkops/contracts/product";

interface ReleasePlanProjectSelectorProps {
  projectIds: string[];
  projects: ProductProjectVM[];
  selectedIds: string[];
  disabled?: boolean;
  onChange?: (projectIds: string[]) => void;
}

export function ReleasePlanProjectSelector({
  projectIds,
  projects,
  selectedIds,
  disabled,
  onChange,
}: ReleasePlanProjectSelectorProps) {
  const projectById = new Map(projects.map((project) => [project.id, project]));
  const selected = new Set(selectedIds);

  if (projectIds.length === 0) return null;

  return (
    <fieldset className="release-plan-projects" disabled={disabled}>
      <legend className="release-plan-job-owner-label">本次执行项目</legend>
      <div className="release-plan-projects-list">
        {projectIds.map((projectId) => {
          const project = projectById.get(projectId);
          const checked = selected.has(projectId);
          const isLastSelected = checked && selectedIds.length === 1;
          return (
            <label
              key={projectId}
              className="release-plan-project-option"
              title={isLastSelected && !disabled ? "本次执行至少选择一个项目" : undefined}
            >
              <input
                type="checkbox"
                checked={checked}
                disabled={disabled || isLastSelected}
                onChange={(event) => {
                  if (!onChange) return;
                  const next = event.target.checked
                    ? [...selectedIds, projectId]
                    : selectedIds.filter((id) => id !== projectId);
                  onChange(projectIds.filter((id) => next.includes(id)));
                }}
              />
              <span>{project?.display_name || project?.name || projectId}</span>
            </label>
          );
        })}
      </div>
      <span className="release-plan-projects-count">
        已选择 {selectedIds.length} / {projectIds.length} 个项目
      </span>
    </fieldset>
  );
}
