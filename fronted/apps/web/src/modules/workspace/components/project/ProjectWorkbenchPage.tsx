"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { AgentVM } from "@sparkops/contracts/agent";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { EditProjectForm, DeleteProjectButton } from "../ProjectSettings";
import { ProjectWorkbenchCard } from "./ProjectWorkbenchCard";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectDetailModal } from "./ProjectDetailModal";

interface ProjectWorkbenchPageProps {
  workspaceId: string;
  project: ProjectVM;
  agents: AgentVM[];
  workflows: WorkflowVM[];
  initialConfigure?: boolean;
}

export function ProjectWorkbenchPage({
  workspaceId,
  project,
  agents,
  workflows,
  initialConfigure = false,
}: ProjectWorkbenchPageProps) {
  const router = useRouter();
  const [configureOpen, setConfigureOpen] = useState(initialConfigure);
  const [detailOpen, setDetailOpen] = useState(false);

  return (
    <div className="stack-lg">
      <Link href="/projects" className="link-back">
        ← 返回项目管理
      </Link>

      <ProjectWorkbenchCard
        project={project}
        onContinueConfig={() => setConfigureOpen(true)}
        onViewDetails={() => setDetailOpen(true)}
      />

      <div className="flex flex-wrap items-center justify-end gap-2">
        <EditProjectForm workspaceId={workspaceId} project={project} />
        <DeleteProjectButton
          workspaceId={workspaceId}
          projectId={project.id}
          projectName={project.name}
        />
      </div>

      <ProjectConfigurePanel
        open={configureOpen}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setConfigureOpen(false)}
        onSaved={() => {
          setConfigureOpen(false);
          router.refresh();
        }}
      />

      <ProjectDetailModal
        open={detailOpen}
        workspaceId={workspaceId}
        project={project}
        agents={agents}
        workflows={workflows}
        onClose={() => setDetailOpen(false)}
      />
    </div>
  );
}
