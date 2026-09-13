"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { DeploymentEntryAction } from "./deployment-gate";
import { ProjectOverviewCard } from "./ProjectOverviewCard";
import type { ProjectRunSummary } from "./useProjectRunSummary";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
  onDeploy: () => void;
  onViewDetails: () => void;
  onSubmitReview: (project: ProjectVM) => void;
  onReview: (project: ProjectVM) => void;
  deploymentAction: DeploymentEntryAction;
  runSummary: ProjectRunSummary;
}

export function ProjectWorkbenchCard({
  project,
  onContinueConfig,
  onDeploy,
  onViewDetails,
  onSubmitReview,
  onReview,
  deploymentAction,
  runSummary,
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
      runSummary={runSummary}
    />
  );
}
