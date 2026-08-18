"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { AgentVM } from "@sparkops/contracts/agent";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { AgentList } from "@/modules/agent/components/AgentList";
import { CreateAgentForm } from "@/modules/agent/components/CreateAgentForm";
import { CreateWorkflowForm, WorkflowSettings } from "@/modules/workflow/components/WorkflowPanel";
import { WorkflowList } from "@/modules/workflow/components/WorkflowList";
import { ProjectMembersPanel } from "../ProjectMembersPanel";
import { EditProjectForm, DeleteProjectButton } from "../ProjectSettings";
import { ProjectWorkbenchCard } from "./ProjectWorkbenchCard";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";

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

  return (
    <div className="stack-lg">
      <Link href="/projects" className="link-back">
        ← 返回项目管理
      </Link>

      <ProjectWorkbenchCard
        project={project}
        onContinueConfig={() => setConfigureOpen(true)}
      />

      <div className="flex flex-wrap items-center justify-end gap-2">
        <EditProjectForm workspaceId={workspaceId} project={project} />
        <DeleteProjectButton
          workspaceId={workspaceId}
          projectId={project.id}
          projectName={project.name}
        />
      </div>

      <div id="project-relations" className="stack-lg">
        <ProjectMembersPanel workspaceId={workspaceId} projectId={project.id} />

        <section className="section">
          <div className="section-header">
            <div>
              <h2 className="section-title">Agents</h2>
              <p className="text-muted mt-1">创建并发布 Agent，进入对话调试</p>
            </div>
            <CreateAgentForm workspaceId={workspaceId} projectId={project.id} />
          </div>
          <AgentList agents={agents} projectId={project.id} />
        </section>

        <section className="section">
          <div className="section-header">
            <div>
              <h2 className="section-title">Workflows</h2>
              <p className="text-muted mt-1">编排项目内的自动化流程</p>
            </div>
            <CreateWorkflowForm workspaceId={workspaceId} projectId={project.id} />
          </div>
          <WorkflowList workflows={workflows} projectId={project.id} />
          <WorkflowSettings workspaceId={workspaceId} workflows={workflows} />
        </section>
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
    </div>
  );
}
