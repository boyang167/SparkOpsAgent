"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery, workspace } from "@sparkops/web-sdk";
import type {
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
} from "@sparkops/contracts/delivery";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectReviewModal } from "./ProjectReviewModal";
import { ProjectDeployModal } from "./ProjectDeployModal";
import { ProjectServiceDetailPage } from "./ProjectServiceDetailPage";
import { projectDisplayName } from "./project-utils";
import { deploymentEntryAction } from "./deployment-gate";
import {
  createSerialPoller,
  shouldContinueDeploymentPoll,
} from "./deployment-poll";
import { resolvePublishedProjectView, useStickyPublishedProject } from "./project-deployment-state";
import { parseProjectDetailTab, type ProjectDetailTab } from "./project-detail-tabs";

const TERMINAL_DEPLOYMENT_STATUSES = new Set(["succeeded", "failed", "aborted"]);

interface ProjectWorkbenchPageProps {
  workspaceId: string;
  project: ProjectVM;
  initialConfigure?: boolean;
}

export function ProjectWorkbenchPage({
  workspaceId,
  project,
  initialConfigure = false,
}: ProjectWorkbenchPageProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [configureOpen, setConfigureOpen] = useState(initialConfigure);
  const [deployOpen, setDeployOpen] = useState(false);
  const [crossClusterDeploy, setCrossClusterDeploy] = useState(false);
  const [selectedRun, setSelectedRun] = useState<DeploymentRunVM | null>(null);
  const [currentProject, setCurrentProject] = useState(project);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deploymentOptions, setDeploymentOptions] = useState<DeploymentOptionsVM | null>(null);
  const [deploymentPage, setDeploymentPage] = useState<DeploymentRunListVM | null>(null);
  const [deploymentLoading, setDeploymentLoading] = useState(true);
  const [deploymentOptionsError, setDeploymentOptionsError] = useState<string | null>(null);
  const [deploymentListError, setDeploymentListError] = useState<string | null>(null);
  const [runtimeRefreshKey, setRuntimeRefreshKey] = useState(0);
  const deploymentGenerationRef = useRef(0);
  const deploymentPollGenerationRef = useRef(0);
  const releaseHandledRef = useRef(false);

  const stickyPublished = useStickyPublishedProject(currentProject.id);
  const urlTabParam = searchParams.get("tab");
  const publishedForUrl = resolvePublishedProjectView(deploymentPage, urlTabParam, {
    loading: deploymentLoading,
    stickyPublished,
  });
  const urlTab = parseProjectDetailTab(urlTabParam, publishedForUrl);
  const [activeTab, setActiveTab] = useState<ProjectDetailTab>(urlTab);

  useEffect(() => {
    setActiveTab(urlTab);
  }, [urlTab]);

  useEffect(() => {
    setCurrentProject(project);
  }, [project]);

  useEffect(() => {
    const generation = ++deploymentGenerationRef.current;
    setDeploymentLoading(true);
    setDeploymentOptionsError(null);
    setDeploymentListError(null);
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

  const refreshDeploymentList = useCallback(async () => {
    try {
      const page = await delivery.listDeployments(currentProject.id, workspaceId);
      setDeploymentPage(page);
    } catch (err) {
      setDeploymentListError(
        err instanceof BffError ? err.message : "部署记录加载失败",
      );
    }
  }, [currentProject.id, workspaceId]);

  const handleRunChanged = useCallback(
    (run: DeploymentRunVM) => {
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
      if (TERMINAL_DEPLOYMENT_STATUSES.has(run.status)) {
        void refreshDeploymentList();
        if (run.status === "succeeded") {
          setRuntimeRefreshKey((value) => value + 1);
        }
      }
    },
    [refreshDeploymentList],
  );

  useEffect(() => {
    if (!activeRun) return;
    const generation = ++deploymentPollGenerationRef.current;
    const poller = createSerialPoller({
      refresh: () => delivery.getDeployment(activeRun.id, workspaceId),
      onValue: handleRunChanged,
      onError: () => {
        /* 轮询失败时静默，下一次继续 */
      },
      shouldContinue: (next) =>
        shouldContinueDeploymentPoll(next.status, generation, deploymentPollGenerationRef.current),
      schedule: (callback, delayMs) => window.setTimeout(callback, delayMs),
      cancel: (timer) => window.clearTimeout(timer),
      delayMs: 2000,
    });
    poller.start();
    return () => {
      if (deploymentPollGenerationRef.current === generation) {
        deploymentPollGenerationRef.current += 1;
      }
      poller.stop();
    };
  }, [activeRun?.id, workspaceId, handleRunChanged]);

  const setTab = useCallback(
    (next: ProjectDetailTab) => {
      setActiveTab(next);
      const params = new URLSearchParams(searchParams.toString());
      if (next === "overview") {
        params.delete("tab");
      } else {
        params.set("tab", next);
      }
      params.delete("release");
      const query = params.toString();
      router.replace(query ? `/projects/${currentProject.id}?${query}` : `/projects/${currentProject.id}`);
    },
    [currentProject.id, router, searchParams],
  );

  const openDeploy = useCallback((crossCluster = false) => {
    if (activeRun) {
      setError(null);
      setSelectedRun(activeRun);
      setCrossClusterDeploy(false);
      setDeployOpen(true);
      return;
    }
    if (deployAction.disabled) {
      setError(deployAction.reason ?? "当前不可发布，请先完成部署前置条件");
      return;
    }
    if (crossCluster) {
      const clusters = deploymentOptions?.clusters ?? [];
      if (clusters.length < 2) {
        setError("跨集群发布需要至少 2 个已授权集群：请先在运行环境中配置构建集群与同步目标");
        return;
      }
      const build = clusters.find((c) => c.is_build_env) ?? clusters[0];
      const syncNames = clusters.filter((c) => c.id !== build?.id).map((c) => c.cluster_name);
      if (
        !window.confirm(
          `将在构建集群「${build?.cluster_name ?? "-"}」构建并部署，随后同步到：${syncNames.join("、")}。\n确定开始跨集群发布？`,
        )
      ) {
        return;
      }
    }
    setError(null);
    setSelectedRun(null);
    setCrossClusterDeploy(crossCluster);
    setDeployOpen(true);
  }, [activeRun, deployAction.disabled, deployAction.reason, deploymentOptions]);

  useEffect(() => {
    if (searchParams.get("release") !== "1" || releaseHandledRef.current) return;
    releaseHandledRef.current = true;
    openDeploy();
    const params = new URLSearchParams(searchParams.toString());
    params.delete("release");
    const query = params.toString();
    router.replace(query ? `/projects/${currentProject.id}?${query}` : `/projects/${currentProject.id}`);
  }, [currentProject.id, openDeploy, router, searchParams]);

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

      <ProjectServiceDetailPage
        workspaceId={workspaceId}
        project={currentProject}
        tab={activeTab}
        onTabChange={setTab}
        deployAction={deployAction}
        onRelease={() => openDeploy(false)}
        onCrossClusterRelease={() => openDeploy(true)}
        onViewDeployProgress={() => {
          setError(null);
          setSelectedRun(activeRun);
          setDeployOpen(true);
        }}
        onConfigure={() => setConfigureOpen(true)}
        onSubmitReview={() => void submitReview()}
        onReview={() => setReviewOpen(true)}
        onProjectUpdated={(updated) => setCurrentProject(updated)}
        deploymentPage={deploymentPage}
        deploymentLoading={deploymentLoading}
        runtimeRefreshKey={runtimeRefreshKey}
        onRunChanged={handleRunChanged}
      />

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

      <ProjectDeployModal
        open={deployOpen}
        workspaceId={workspaceId}
        project={currentProject}
        initialRun={selectedRun ?? activeRun}
        deploymentPage={deploymentPage}
        crossCluster={crossClusterDeploy}
        onRunChanged={handleRunChanged}
        onDeployStarted={() => {
          setDeployOpen(false);
          setCrossClusterDeploy(false);
          setSelectedRun(null);
          setTab("build-logs");
        }}
        onClose={() => {
          setDeployOpen(false);
          setCrossClusterDeploy(false);
          setSelectedRun(null);
        }}
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

