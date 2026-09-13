"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { DeploymentEntryAction } from "./deployment-gate";
import { ProjectEnvironmentStatus } from "./ProjectEnvironmentStatus";
import { ProjectRunStats } from "./ProjectRunStats";
import { isProjectInDeployPhase } from "./project-environment-status";
import type { ProjectRunSummary } from "./useProjectRunSummary";
import {
  formatDateTime,
  languageColor,
  languageLabel,
  projectDisplayName,
} from "./project-utils";
import {
  computeProjectProgress,
  projectConfigActionLabel,
  projectPrimaryAction,
  shouldShowEditConfigAction,
  type ProjectProgressStep,
} from "./project-progress";

interface ProjectOverviewCardProps {
  project: ProjectVM;
  heading?: "h2" | "h3";
  detailsHref?: string;
  continueHref?: string;
  deployHref?: string;
  onViewDetails?: () => void;
  onContinueConfig?: () => void;
  onDeploy?: () => void;
  onSubmitReview?: (project: ProjectVM) => void;
  onReview?: (project: ProjectVM) => void;
  onArchive?: (project: ProjectVM) => void;
  deploymentAction?: DeploymentEntryAction;
  runSummary?: ProjectRunSummary;
}

function statusLabelTagClass(label: string) {
  if (label === "审核中") return "project-tag project-tag-blue";
  if (label === "配置中") return "project-tag project-tag-orange";
  if (label === "审核驳回") return "project-tag project-tag-red";
  if (label === "已部署") return "project-tag project-tag-green";
  if (label === "待部署" || label === "待提交审核") return "project-tag project-tag-green";
  if (label === "部署中") return "project-tag project-tag-blue";
  if (label === "部署失败" || label === "已中止") return "project-tag project-tag-red";
  return "project-tag project-tag-gray";
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
  deployHref,
  onViewDetails,
  onContinueConfig,
  onDeploy,
  onSubmitReview,
  onReview,
  onArchive,
  deploymentAction,
  runSummary,
}: ProjectOverviewCardProps) {
  const router = useRouter();
  const { steps, statusLabel, configComplete } = computeProjectProgress(
    project,
    runSummary?.deployment,
  );
  const primaryAction = projectPrimaryAction(project);
  const showEditConfig = shouldShowEditConfigAction(project);
  const configActionLabel = projectConfigActionLabel(project);
  const langColor = languageColor(project.language);
  const initial = languageLabel(project.language).charAt(0).toUpperCase() || "P";
  const displayName = projectDisplayName(project);
  const showTechnicalName = project.name.trim() !== displayName.trim();
  const TitleTag = heading;
  const targetHref = detailsHref ?? `/projects/${project.id}`;
  const cardInteractive = Boolean(detailsHref);
  const inDeployPhase = isProjectInDeployPhase(project);
  const description = project.description?.trim() || "暂无项目描述";

  function handleCardClick(event: React.MouseEvent<HTMLElement>) {
    if (!cardInteractive) return;
    if ((event.target as HTMLElement).closest("a, button, .project-card-more-wrap")) {
      return;
    }
    router.push(targetHref);
  }

  function handleCardKeyDown(event: React.KeyboardEvent<HTMLElement>) {
    if (!cardInteractive) return;
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    router.push(targetHref);
  }

  return (
    <article
      className={`project-overview-card${cardInteractive ? " project-overview-card-interactive" : ""}`}
      style={{ ["--project-accent" as string]: langColor }}
      onClick={handleCardClick}
      onKeyDown={handleCardKeyDown}
      role={cardInteractive ? "link" : undefined}
      tabIndex={cardInteractive ? 0 : undefined}
    >
      <header className="project-overview-header">
        <div className="project-overview-identity">
          <div className="project-lang-icon project-lang-icon-ring" style={{ backgroundColor: langColor }}>
            {initial}
          </div>
          <div className="project-overview-heading">
            <div className="project-overview-title-row">
              <TitleTag>{displayName}</TitleTag>
              <span className={statusLabelTagClass(statusLabel)}>{statusLabel}</span>
            </div>
            {showTechnicalName ? <p className="project-card-subtitle">{project.name}</p> : null}
            <div className="project-overview-meta">
              <span>{languageLabel(project.language)}</span>
              {project.namespace ? (
                <>
                  <span className="project-overview-meta-sep" aria-hidden>
                    ·
                  </span>
                  <span>{project.namespace}</span>
                </>
              ) : null}
            </div>
          </div>
        </div>
        {onArchive ? <MoreMenu onArchive={() => onArchive(project)} /> : null}
      </header>

      <p className={`project-card-desc${description === "暂无项目描述" ? " is-placeholder" : ""}`}>
        {description}
      </p>

      <div className="project-overview-body">
        {inDeployPhase && runSummary ? (
          <ProjectEnvironmentStatus
            items={runSummary.environments.items}
            loading={runSummary.environments.loading}
            error={runSummary.environments.error}
          />
        ) : (
          <ProgressTrack steps={steps} />
        )}

        {runSummary ? (
          <ProjectRunStats deployment={runSummary.deployment} build={runSummary.build} />
        ) : null}
      </div>

      <footer className="project-workbench-footer">
        <span className="text-muted">创建于 {formatDateTime(project.created_at)}</span>
        <div className="project-workbench-actions">
          {onViewDetails ? (
            <button type="button" className="project-action-btn is-view" onClick={onViewDetails}>
              查看详情
            </button>
          ) : (
            <Link href={detailsHref ?? `/projects/${project.id}`} className="project-action-btn is-view">
              查看详情
            </Link>
          )}
          {primaryAction === "pending" ? (
            <button type="button" className="project-action-btn is-submit" disabled>
              审核中
            </button>
          ) : primaryAction === "deploy" ? (
            <>
              {showEditConfig && onContinueConfig ? (
                <button type="button" className="project-action-btn is-config" onClick={onContinueConfig}>
                  修改配置
                </button>
              ) : null}
              {onDeploy ? (
                <button
                  type="button"
                  className="project-action-btn is-deploy"
                  onClick={onDeploy}
                  aria-disabled={deploymentAction?.disabled || undefined}
                  title={deploymentAction?.reason ?? undefined}
                  aria-describedby={deploymentAction?.reason ? `deploy-reason-${project.id}` : undefined}
                >
                  {deploymentAction?.label ?? "发布"}
                </button>
              ) : (
                <Link
                  href={deployHref ?? detailsHref ?? `/projects/${project.id}`}
                  className="project-action-btn is-deploy"
                >
                  发布
                </Link>
              )}
            </>
          ) : primaryAction === "submit" || primaryAction === "review" ? (
            <>
              {showEditConfig && onContinueConfig ? (
                <button type="button" className="project-action-btn is-config" onClick={onContinueConfig}>
                  修改配置
                </button>
              ) : null}
              {primaryAction === "submit" ? (
                <button
                  type="button"
                  className="project-action-btn is-submit"
                  onClick={() => onSubmitReview?.(project)}
                >
                  提交审核
                </button>
              ) : (
                <button
                  type="button"
                  className="project-action-btn is-submit"
                  onClick={() => onReview?.(project)}
                >
                  审核
                </button>
              )}
            </>
          ) : showEditConfig && onContinueConfig ? (
            <button type="button" className="project-action-btn is-config" onClick={onContinueConfig}>
              {configActionLabel}
            </button>
          ) : !configComplete && onContinueConfig ? (
            <button type="button" className="project-action-btn is-config" onClick={onContinueConfig}>
              继续配置
            </button>
          ) : !configComplete ? (
            <Link href={continueHref ?? detailsHref ?? `/projects/${project.id}`} className="project-action-btn is-config">
              继续配置
            </Link>
          ) : null}
        </div>
      </footer>
      {deploymentAction?.reason ? (
        <p id={`deploy-reason-${project.id}`} className="text-sm text-muted" role="status">
          {deploymentAction.reason}
        </p>
      ) : null}
    </article>
  );
}
