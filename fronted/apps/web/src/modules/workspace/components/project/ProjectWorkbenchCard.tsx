"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
  onViewDetails: () => void;
}

export function ProjectWorkbenchCard({
  project,
  onContinueConfig,
  onViewDetails,
}: ProjectWorkbenchCardProps) {
  return (
    <ProjectOverviewCard
      project={project}
      heading="h2"
      onViewDetails={onViewDetails}
      onContinueConfig={onContinueConfig}
    />
  );
}
