"use client";

import Link from "next/link";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import {
  languageColor,
  languageLabel,
  labelOf,
  PROJECT_STATUS_OPTIONS,
  projectDisplayName,
  statusBadgeClass,
} from "./project-utils";

interface ProjectCardProps {
  project: ProjectVM;
  onArchive?: (project: ProjectVM) => void;
}

export function ProjectCard({ project, onArchive }: ProjectCardProps) {
  const langColor = languageColor(project.language);
  const initial = languageLabel(project.language).charAt(0).toUpperCase();

  return (
    <article className="project-card">
      <div className="project-card-header">
        <div className="project-card-title-wrap">
          <div className="project-lang-icon" style={{ backgroundColor: langColor }}>
            {initial}
          </div>
          <div>
            <h3>{projectDisplayName(project)}</h3>
            <p className="project-card-subtitle">{project.name}</p>
            <div className="project-card-tags">
              {project.namespace ? <span className="project-tag project-tag-blue">{project.namespace}</span> : null}
              <span className={statusBadgeClass(project.status)}>
                {labelOf(PROJECT_STATUS_OPTIONS, project.status)}
              </span>
              {project.language ? (
                <span className="project-tag project-tag-gray">{languageLabel(project.language)}</span>
              ) : null}
            </div>
          </div>
        </div>
      </div>
      <p className="project-card-desc">{project.description || "暂无描述"}</p>
      <div className="project-card-actions">
        <Link href={`/projects/${project.id}`} className="project-action-btn is-view">
          查看详情
        </Link>
        <Link href={`/projects/${project.id}`} className="project-action-btn is-edit">
          进入项目
        </Link>
        {onArchive ? (
          <button type="button" className="project-action-btn is-delete" onClick={() => onArchive(project)}>
            归档
          </button>
        ) : null}
      </div>
    </article>
  );
}
