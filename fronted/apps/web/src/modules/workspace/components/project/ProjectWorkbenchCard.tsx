"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
}

export function ProjectWorkbenchCard({ project, onContinueConfig }: ProjectWorkbenchCardProps) {
  return (
    <ProjectOverviewCard
      project={project}
      heading="h2"
      detailsHref="#project-relations"
      onContinueConfig={onContinueConfig}
    />
  );
}
