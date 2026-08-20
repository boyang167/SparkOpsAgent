"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
  onDeploy: () => void;
  onViewDetails: () => void;
  onSubmitReview: (project: ProjectVM) => void;
  onReview: (project: ProjectVM) => void;
}

export function ProjectWorkbenchCard({
  project,
  onContinueConfig,
  onDeploy,
  onViewDetails,
  onSubmitReview,
  onReview,
}: ProjectWorkbenchCardProps) {
  return (
    <ProjectOverviewCard
      project={project}
      heading="h2"
      onViewDetails={onViewDetails}
      onContinueConfig={onContinueConfig}
      onDeploy={onDeploy}
      onSubmitReview={onSubmitReview}
      onReview={onReview}
    />
  );
}
