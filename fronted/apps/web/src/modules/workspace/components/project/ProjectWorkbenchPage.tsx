"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery, workspace } from "@sparkops/web-sdk";
import type {
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
} from "@sparkops/contracts/delivery";
import type { AgentVM } from "@sparkops/contracts/agent";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { EditProjectForm, DeleteProjectButton } from "../ProjectSettings";
import { ProjectWorkbenchCard } from "./ProjectWorkbenchCard";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectDetailModal } from "./ProjectDetailModal";
import { ProjectReviewModal } from "./ProjectReviewModal";
import { ProjectDeployModal } from "./ProjectDeployModal";
import { projectDisplayName } from "./project-utils";
import { deploymentEntryAction } from "./deployment-gate";

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
  const [deployOpen, setDeployOpen] = useState(false);
  const [currentProject, setCurrentProject] = useState(project);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deploymentOptions, setDeploymentOptions] = useState<DeploymentOptionsVM | null>(null);
  const [deploymentPage, setDeploymentPage] = useState<DeploymentRunListVM | null>(null);
  const [deploymentLoading, setDeploymentLoading] = useState(true);
  const [deploymentOptionsError, setDeploymentOptionsError] = useState<string | null>(null);
  const [deploymentListError, setDeploymentListError] = useState<string | null>(null);
  const deploymentGenerationRef = useRef(0);

  useEffect(() => {
    setCurrentProject(project);
  }, [project]);

  useEffect(() => {
    const generation = ++deploymentGenerationRef.current;
    setDeploymentLoading(true);
    setDeploymentOptionsError(null);
    setDeploymentListError(null);
    setDeploymentOptions(null);
    setDeploymentPage(null);
    void Promise.allSettled([
      delivery.getDeploymentOptions(currentProject.id, workspaceId),
      delivery.listDeployments(currentProject.id, workspaceId),
    ])
      .then(([optionsResult, pageResult]) => {
        if (deploymentGenerationRef.current !== generation) return;
        if (optionsResult.status === "fulfilled") {
          setDeploymentOptions(optionsResult.value);
        } else {
          setDeploymentOptionsError(
            optionsResult.reason instanceof BffError
              ? optionsResult.reason.message
              : "部署条件加载失败",
          );
        }
        if (pageResult.status === "fulfilled") {
          setDeploymentPage(pageResult.value);
        } else {
          setDeploymentListError(
            pageResult.reason instanceof BffError
              ? pageResult.reason.message
              : "部署记录加载失败",
          );
        }
      })
      .finally(() => {
        if (deploymentGenerationRef.current === generation) {
          setDeploymentLoading(false);
        }
      });
    return () => {
      if (deploymentGenerationRef.current === generation) {
        deploymentGenerationRef.current += 1;
      }
    };
  }, [currentProject.id, currentProject.review_status, workspaceId]);

  const activeRun = useMemo(
    () =>
      deploymentPage?.items.find((item) =>
        ["queued", "preparing", "building", "deploying"].includes(item.status),
      ) ?? null,
    [deploymentPage],
  );
  const deployAction = useMemo(
    () => {
      const preflightError = [deploymentOptionsError, deploymentListError]
        .filter(Boolean)
        .join("；");
      return deploymentEntryAction(
        currentProject,
        deploymentOptions,
        activeRun,
        deploymentLoading,
        preflightError || null,
      );
    },
    [
      currentProject,
      deploymentOptions,
      activeRun,
      deploymentLoading,
      deploymentOptionsError,
      deploymentListError,
    ],
  );
  const handleRunChanged = useCallback((run: DeploymentRunVM) => {
    setDeploymentPage((current) => {
      if (!current) {
        return { items: [run], total: 1, page: 1, page_size: 20 };
      }
      const exists = current.items.some((item) => item.id === run.id);
      return {
        ...current,
        total: exists ? current.total : current.total + 1,
        items: exists
          ? current.items.map((item) => (item.id === run.id ? run : item))
          : [run, ...current.items].slice(0, current.page_size),
      };
    });
  }, []);

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
        onDeploy={() => {
          if (deployAction.disabled) {
            setError(deployAction.reason ?? "当前不可部署，请先补齐部署条件");
            return;
          }
          setError(null);
          setDeployOpen(true);
        }}
        onViewDetails={() => setDetailOpen(true)}
        onSubmitReview={() => void submitReview()}
        onReview={() => setReviewOpen(true)}
        deploymentAction={deployAction}
        deploymentSummary={{
          total: deploymentPage?.total ?? null,
          latest: deploymentPage?.items[0] ?? null,
          loading: deploymentLoading,
          error: deploymentListError,
        }}
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
        onProjectUpdated={(updated) => setCurrentProject(updated)}
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
      <ProjectDeployModal
        open={deployOpen}
        workspaceId={workspaceId}
        project={currentProject}
        initialRun={activeRun}
        onRunChanged={handleRunChanged}
        onClose={() => setDeployOpen(false)}
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
