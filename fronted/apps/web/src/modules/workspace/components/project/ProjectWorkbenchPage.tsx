"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { workspace } from "@sparkops/web-sdk";
import type { AgentVM } from "@sparkops/contracts/agent";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { EditProjectForm, DeleteProjectButton } from "../ProjectSettings";
import { ProjectWorkbenchCard } from "./ProjectWorkbenchCard";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectDetailModal } from "./ProjectDetailModal";
import { ProjectReviewModal } from "./ProjectReviewModal";
import { projectDisplayName } from "./project-utils";

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
  const [currentProject, setCurrentProject] = useState(project);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setCurrentProject(project);
  }, [project]);

  async function submitReview() {
    if (!window.confirm(`确定提交项目「${projectDisplayName(currentProject)}」审核？提交后配置将暂时锁定。`)) {
      return;
    }
    setError(null);
    try {
      setCurrentProject(await workspace.submitProjectReview(workspaceId, currentProject.id));
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "提交审核失败");
    }
  }

  async function reviewProject(input: Parameters<typeof workspace.reviewProject>[2]) {
    setReviewLoading(true);
    setError(null);
    try {
      setCurrentProject(await workspace.reviewProject(workspaceId, currentProject.id, input));
      setReviewOpen(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "项目审核失败");
    } finally {
      setReviewLoading(false);
    }
  }

  return (
    <div className="stack-lg">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <ProjectWorkbenchCard
        project={currentProject}
        onContinueConfig={() => setConfigureOpen(true)}
        onDeploy={() => setDetailOpen(true)}
        onViewDetails={() => setDetailOpen(true)}
        onSubmitReview={() => void submitReview()}
        onReview={() => setReviewOpen(true)}
      />

      <div className="flex flex-wrap items-center justify-end gap-2">
        <EditProjectForm workspaceId={workspaceId} project={currentProject} />
        <DeleteProjectButton
          workspaceId={workspaceId}
          projectId={currentProject.id}
          projectName={currentProject.name}
        />
      </div>

      <ProjectConfigurePanel
        open={configureOpen}
        workspaceId={workspaceId}
        project={currentProject}
        onClose={() => setConfigureOpen(false)}
        onSaved={() => {
          setConfigureOpen(false);
          router.refresh();
        }}
      />

      <ProjectDetailModal
        open={detailOpen}
        workspaceId={workspaceId}
        project={currentProject}
        agents={agents}
        workflows={workflows}
        onClose={() => setDetailOpen(false)}
      />
      <ProjectReviewModal
        open={reviewOpen}
        workspaceId={workspaceId}
        project={currentProject}
        loading={reviewLoading}
        onClose={() => {
          if (!reviewLoading) setReviewOpen(false);
        }}
        onSubmit={(input) => void reviewProject(input)}
      />
    </div>
  );
}
