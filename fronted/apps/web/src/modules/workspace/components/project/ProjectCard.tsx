"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";

interface ProjectCardProps {
  project: ProjectVM;
  onArchive?: (project: ProjectVM) => void;
  onViewDetails?: (project: ProjectVM) => void;
}

export function ProjectCard({ project, onArchive, onViewDetails }: ProjectCardProps) {
  return (
    <ProjectOverviewCard
      project={project}
      detailsHref={`/projects/${project.id}`}
      continueHref={`/projects/${project.id}?configure=1`}
      onViewDetails={onViewDetails ? () => onViewDetails(project) : undefined}
      onArchive={onArchive}
    />
  );
}
