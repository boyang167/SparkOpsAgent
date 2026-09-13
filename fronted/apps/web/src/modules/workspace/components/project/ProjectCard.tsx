"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";
import { useProjectRunSummary } from "./useProjectRunSummary";

interface ProjectCardProps {
  workspaceId: string;
  project: ProjectVM;
  onArchive?: (project: ProjectVM) => void;
  onViewDetails?: (project: ProjectVM) => void;
  onContinueConfig?: (project: ProjectVM) => void;
  onSubmitReview?: (project: ProjectVM) => void;
  onReview?: (project: ProjectVM) => void;
  onDeploy?: (project: ProjectVM) => void;
}

export function ProjectCard({
  workspaceId,
  project,
  onArchive,
  onViewDetails,
  onContinueConfig,
  onSubmitReview,
  onReview,
  onDeploy,
}: ProjectCardProps) {
  const runSummary = useProjectRunSummary(workspaceId, project.id);

  return (
    <ProjectOverviewCard
      project={project}
      runSummary={runSummary}
      detailsHref={`/projects/${project.id}`}
      deployHref={`/projects/${project.id}?release=1`}
      onViewDetails={onViewDetails ? () => onViewDetails(project) : undefined}
      onContinueConfig={onContinueConfig ? () => onContinueConfig(project) : undefined}
      onSubmitReview={onSubmitReview}
      onReview={onReview}
      onArchive={onArchive}
      onDeploy={onDeploy ? () => onDeploy(project) : undefined}
    />
  );
}
