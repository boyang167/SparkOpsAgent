"use client";

import { useState } from "react";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectOverviewCard } from "./ProjectOverviewCard";
import type { EnvironmentClusterRef } from "./project-environment-status";
import { UndeployModal } from "./UndeployModal";
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
  const [undeployClusterId, setUndeployClusterId] = useState<string | null>(null);

  return (
    <>
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
        onUndeploy={(row) => setUndeployClusterId(row.clusterId)}
      />
      <UndeployModal
        open={undeployClusterId !== null}
        workspaceId={workspaceId}
        project={project}
        rows={runSummary.environments.items}
        initialClusterId={undeployClusterId ?? undefined}
        onClose={() => setUndeployClusterId(null)}
        onStarted={() => {
          setUndeployClusterId(null);
          runSummary.reload();
        }}
      />
    </>
  );
}
