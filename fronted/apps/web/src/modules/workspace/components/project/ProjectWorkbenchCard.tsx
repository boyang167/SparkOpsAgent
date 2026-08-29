"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { DeploymentEntryAction } from "./deployment-gate";
import { ProjectOverviewCard } from "./ProjectOverviewCard";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
  onDeploy: () => void;
  onViewDetails: () => void;
  onSubmitReview: (project: ProjectVM) => void;
  onReview: (project: ProjectVM) => void;
  deploymentAction: DeploymentEntryAction;
  deploymentSummary: {
    total: number | null;
    latest: DeploymentRunVM | null;
    loading: boolean;
    error: string | null;
  };
}

export function ProjectWorkbenchCard({
  project,
  onContinueConfig,
  onDeploy,
  onViewDetails,
  onSubmitReview,
  onReview,
  deploymentAction,
  deploymentSummary,
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
      deploymentAction={deploymentAction}
      deploymentSummary={deploymentSummary}
    />
  );
}
