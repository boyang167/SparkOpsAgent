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
  SyncManifestInput,
} from "@sparkops/contracts/delivery";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectReviewModal } from "./ProjectReviewModal";
import { ProjectDeployModal } from "./ProjectDeployModal";
import { ProjectServiceDetailPage } from "./ProjectServiceDetailPage";
import {
  CrossClusterConfirmModal,
  type CrossClusterConfirmMode,
} from "./CrossClusterConfirmModal";
import { projectDisplayName } from "./project-utils";
import { releaseEntryAction } from "./deployment-gate";
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
  const [crossClusterConfirmOpen, setCrossClusterConfirmOpen] = useState(false);
  const [crossClusterConfirmMode, setCrossClusterConfirmMode] =
    useState<CrossClusterConfirmMode>("publish");
  const [crossClusterSyncSubmitting, setCrossClusterSyncSubmitting] = useState(false);
  const [selectedRun, setSelectedRun] = useState<DeploymentRunVM | null>(null);
  const [currentProject, setCurrentProject] = useState(project);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deploymentOptions, setDeploymentOptions] = useState<DeploymentOptionsVM | null>(null);
  const [deploymentPage, setDeploymentPage] = useState<DeploymentRunListVM | null>(null);
  const [deploymentListLoading, setDeploymentListLoading] = useState(true);
  const [deploymentListError, setDeploymentListError] = useState<string | null>(null);
  const [runtimeRefreshKey, setRuntimeRefreshKey] = useState(0);
  const deploymentGenerationRef = useRef(0);
  const deploymentPollGenerationRef = useRef(0);
  const releaseHandledRef = useRef(false);

  const stickyPublished = useStickyPublishedProject(currentProject.id);
  const urlTabParam = searchParams.get("tab");
  const publishedForUrl = resolvePublishedProjectView(deploymentPage, urlTabParam, {
    loading: deploymentListLoading,
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
    setDeploymentListLoading(true);
    setDeploymentListError(null);

    void delivery
      .getDeploymentOptions(currentProject.id, workspaceId)
      .then((value) => {
        if (deploymentGenerationRef.current !== generation) return;
        setDeploymentOptions(value);
      })
      .catch(() => {
        if (deploymentGenerationRef.current !== generation) return;
        setDeploymentOptions(null);
      });

    void delivery
      .listDeployments(currentProject.id, workspaceId)
      .then((page) => {
        if (deploymentGenerationRef.current !== generation) return;
        setDeploymentPage(page);
      })
      .catch((err) => {
        if (deploymentGenerationRef.current !== generation) return;
        setDeploymentListError(
          err instanceof BffError ? err.message : "部署记录加载失败",
        );
      })
      .finally(() => {
        if (deploymentGenerationRef.current === generation) {
          setDeploymentListLoading(false);
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
    () =>
      releaseEntryAction(
        currentProject,
        activeRun,
        deploymentListLoading,
        deploymentListError,
      ),
    [currentProject, activeRun, deploymentListLoading, deploymentListError],
  );

  useEffect(() => {
    if (deploymentListLoading) return;
    setError((current) => (current === "正在检查发布条件" ? null : current));
  }, [deploymentListLoading]);

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

  const crossClusterPlan = useMemo(() => {
    const clusters = deploymentOptions?.clusters ?? [];
    const build = clusters.find((c) => c.is_build_env) ?? clusters[0] ?? null;
    const syncClusterNames = clusters
      .filter((c) => c.id !== build?.id)
      .map((c) => c.cluster_name)
      .filter(Boolean);
    return {
      clusters,
      buildClusterName: build?.cluster_name ?? "",
      syncClusterNames,
    };
  }, [deploymentOptions]);

  const lastSucceededRun = useMemo(
    () =>
      deploymentPage?.items.find(
        (item) => item.status === "succeeded" && Boolean(item.image_tag?.trim()),
      ) ?? null,
    [deploymentPage],
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
      if (crossClusterPlan.clusters.length < 2) {
        setError("跨集群发布需要至少 2 个已授权集群：请先在运行环境中配置构建集群与同步目标");
        return;
      }
      setError(null);
      setCrossClusterConfirmMode("publish");
      setCrossClusterConfirmOpen(true);
      return;
    }
    setError(null);
    setSelectedRun(null);
    setCrossClusterDeploy(false);
    setDeployOpen(true);
  }, [activeRun, crossClusterPlan.clusters.length, deployAction.disabled, deployAction.reason]);

  const openCrossClusterSync = useCallback(() => {
    if (activeRun) {
      setError(null);
      setSelectedRun(activeRun);
      setCrossClusterDeploy(false);
      setDeployOpen(true);
      return;
    }
    if (crossClusterPlan.clusters.length < 2) {
      setError("跨集群同步需要至少 2 个已授权集群：请先在运行环境中配置构建集群与同步目标");
      return;
    }
    if (!lastSucceededRun) {
      setError("跨集群同步需要至少一次成功发布（含镜像标签）");
      return;
    }
    setError(null);
    setCrossClusterConfirmMode("sync");
    setCrossClusterConfirmOpen(true);
  }, [activeRun, crossClusterPlan.clusters.length, lastSucceededRun]);

  const confirmCrossClusterAction = useCallback(
    async (manifests?: SyncManifestInput[]) => {
      if (crossClusterConfirmMode === "publish") {
        setCrossClusterConfirmOpen(false);
        setSelectedRun(null);
        setCrossClusterDeploy(true);
        setDeployOpen(true);
        return;
      }
      if (!manifests || manifests.length === 0) {
        setError("请至少勾选一项同步资源");
        return;
      }
      setCrossClusterSyncSubmitting(true);
      setError(null);
      try {
        const created = await delivery.createDeployment(currentProject.id, workspaceId, {
          sync_only: true,
          git_ref_type: lastSucceededRun?.git_ref_type,
          git_ref_value: lastSucceededRun?.git_ref_value,
          image_tag: lastSucceededRun?.image_tag,
          manifests,
        });
        setCrossClusterConfirmOpen(false);
        setSelectedRun(created);
        handleRunChanged(created);
        setTab("build-logs");
      } catch (err) {
        setError(err instanceof BffError ? err.message : "跨集群同步失败");
      } finally {
        setCrossClusterSyncSubmitting(false);
      }
    },
    [
      crossClusterConfirmMode,
      currentProject.id,
      handleRunChanged,
      lastSucceededRun?.git_ref_type,
      lastSucceededRun?.git_ref_value,
      lastSucceededRun?.image_tag,
      setTab,
      workspaceId,
    ],
  );

  useEffect(() => {
    if (searchParams.get("release") !== "1" || releaseHandledRef.current) return;
    if (deploymentListLoading) return;
    releaseHandledRef.current = true;
    openDeploy();
    const params = new URLSearchParams(searchParams.toString());
    params.delete("release");
    const query = params.toString();
    router.replace(query ? `/projects/${currentProject.id}?${query}` : `/projects/${currentProject.id}`);
  }, [currentProject.id, deploymentListLoading, openDeploy, router, searchParams]);

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
        onCrossClusterSync={openCrossClusterSync}
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
        deploymentLoading={deploymentListLoading}
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
      <CrossClusterConfirmModal
        open={crossClusterConfirmOpen}
        mode={crossClusterConfirmMode}
        workspaceId={workspaceId}
        projectId={currentProject.id}
        buildClusterName={crossClusterPlan.buildClusterName}
        syncClusterNames={crossClusterPlan.syncClusterNames}
        imageTag={lastSucceededRun?.image_tag}
        gitRefLabel={
          lastSucceededRun
            ? `${lastSucceededRun.git_ref_value}（${lastSucceededRun.git_ref_type === "tag" ? "标签" : "分支"}）`
            : undefined
        }
        submitting={crossClusterSyncSubmitting}
        onClose={() => {
          if (crossClusterSyncSubmitting) return;
          setCrossClusterConfirmOpen(false);
        }}
        onConfirm={(manifests) => void confirmCrossClusterAction(manifests)}
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

