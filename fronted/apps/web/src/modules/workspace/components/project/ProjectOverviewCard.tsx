"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import {
  formatDateTime,
  languageColor,
  languageLabel,
  projectDisplayName,
} from "./project-utils";
import { computeProjectProgress, type ProjectProgressStep } from "./project-progress";

interface ProjectOverviewCardProps {
  project: ProjectVM;
  heading?: "h2" | "h3";
  detailsHref: string;
  continueHref?: string;
  onContinueConfig?: () => void;
  onArchive?: (project: ProjectVM) => void;
}

function statusTagClass(label: string) {
  if (label === "提交审核") return "project-tag project-tag-blue";
  return "project-tag project-tag-green";
}

function ProgressTrack({ steps }: { steps: ProjectProgressStep[] }) {
  return (
    <section className="project-workbench-progress">
      <div className="project-workbench-progress-title">项目进度</div>
      <ol className="project-workbench-steps">
        {steps.map((step, index) => (
          <li key={step.key} className={`project-workbench-step is-${step.status}`}>
            <div className="project-workbench-step-node">
              <span className="project-workbench-step-circle">
                {step.status === "done" ? "✓" : index + 1}
              </span>
            </div>
            <div className="project-workbench-step-copy">
              <strong>{step.title}</strong>
              <span>{step.subtitle}</span>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

function MoreMenu({ onArchive }: { onArchive: () => void }) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onPointerDown(event: PointerEvent) {
      if (!wrapRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  return (
    <div className="project-card-more-wrap" ref={wrapRef}>
      <button
        type="button"
        className="project-card-more"
        aria-label="更多操作"
        onClick={() => setOpen((value) => !value)}
      >
        ···
      </button>
      {open ? (
        <div className="project-card-more-menu">
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              onArchive();
            }}
          >
            归档
          </button>
        </div>
      ) : null}
    </div>
  );
}

export function ProjectOverviewCard({
  project,
  heading = "h3",
  detailsHref,
  continueHref,
  onContinueConfig,
  onArchive,
}: ProjectOverviewCardProps) {
  const { steps, statusLabel } = computeProjectProgress(project);
  const langColor = languageColor(project.language);
  const initial = languageLabel(project.language).charAt(0).toUpperCase() || "P";
  const TitleTag = heading;

  return (
    <article className="project-overview-card">
      <header className="project-workbench-header">
        <div className="project-workbench-title-wrap">
          <div className="project-lang-icon" style={{ backgroundColor: langColor }}>
            {initial}
          </div>
          <div className="project-overview-heading">
            <div className="project-overview-heading-row">
              <TitleTag>{projectDisplayName(project)}</TitleTag>
              <div className="project-card-tags">
                {project.namespace ? (
                  <span className="project-tag project-tag-gray">{project.namespace}</span>
                ) : null}
                <span className="project-tag project-tag-gray">草稿</span>
                <span className={statusTagClass(statusLabel)}>{statusLabel}</span>
              </div>
            </div>
            <p className="project-card-subtitle">{project.name}</p>
          </div>
        </div>
        {onArchive ? <MoreMenu onArchive={() => onArchive(project)} /> : null}
      </header>

      <p className="project-card-desc">{project.description || "暂无描述"}</p>

      <ProgressTrack steps={steps} />

      <div className="project-workbench-stats">
        <div>
          <span>部署次数</span>
          <strong>0</strong>
        </div>
        <div>
          <span>构建次数</span>
          <strong>0</strong>
        </div>
        <div>
          <span>最后部署</span>
          <strong>N/A</strong>
        </div>
      </div>

      <footer className="project-workbench-footer">
        <span className="text-muted">创建于 {formatDateTime(project.created_at)}</span>
        <div className="project-workbench-actions">
          <Link href={detailsHref} className="project-action-btn is-view">
            查看详情
          </Link>
          {onContinueConfig ? (
            <button type="button" className="project-action-btn is-config" onClick={onContinueConfig}>
              继续配置
            </button>
          ) : (
            <Link href={continueHref ?? detailsHref} className="project-action-btn is-config">
              继续配置
            </Link>
          )}
        </div>
      </footer>
    </article>
  );
}
