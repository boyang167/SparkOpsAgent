"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";
import type { EnvironmentClusterRef } from "./project-environment-status";
import { useProjectRunSummary } from "./useProjectRunSummary";

interface ProjectCardProps {
  workspaceId: string;
  project: ProjectVM;
  workspaceClusters?: EnvironmentClusterRef[] | null;
  workspaceClustersLoading?: boolean;
  workspaceClustersError?: string | null;
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
  workspaceClusters,
  workspaceClustersLoading,
  workspaceClustersError,
  onArchive,
  onViewDetails,
  onContinueConfig,
  onSubmitReview,
  onReview,
  onDeploy,
}: ProjectCardProps) {
  const runSummary = useProjectRunSummary(workspaceId, project.id, {
    workspaceClusters,
    workspaceClustersLoading,
    workspaceClustersError,
  });

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
