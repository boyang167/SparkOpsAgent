"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import type { ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import {
  formatDateTime,
  languageLabel,
} from "./project-utils";
import {
  isProjectConfigComplete,
  projectConfigActionLabel,
  projectPrimaryAction,
  shouldShowEditConfigAction,
} from "./project-progress";
import type { DeploymentEntryAction } from "./deployment-gate";
import { ProjectDeploymentConsolePanel } from "./ProjectDeploymentConsolePanel";
import { ProjectConfigMapTab } from "./ProjectConfigMapTab";
import { ProjectStorageTab } from "./ProjectStorageTab";
import { ProjectWebhookTab } from "./ProjectWebhookTab";
import { ProjectYamlTab } from "./ProjectYamlTab";
import { ProjectJenkinsPreview } from "./ProjectJenkinsPreview";
import { ProjectPublishedOverview } from "./ProjectPublishedOverview";
import { ProjectUnpublishedOverview } from "./ProjectUnpublishedOverview";
import { ProjectTemplateArtifactsModal } from "./ProjectTemplateArtifactsModal";
import { RuntimeExpandModal } from "@/modules/runtime/components/RuntimeExpandModal";
import { RuntimeRollbackModal } from "@/modules/runtime/components/RuntimeRollbackModal";
import { RuntimeScaleModal } from "@/modules/runtime/components/RuntimeConfigModals";
import {
  canRollbackDeployment,
  hasSuccessfulDeployment,
  isPublishedProjectView,
  markStickyPublishedProject,
  resolvePublishedProjectView,
  useStickyPublishedProject,
} from "./project-deployment-state";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";
import { ProjectDisplayNameField, ProjectDescriptionField } from "./ProjectDisplayNameEditModal";
import {
  projectDetailTabs,
  type ProjectDetailTab,
} from "./project-detail-tabs";

interface ProjectServiceDetailPageProps {
  workspaceId: string;
  project: ProjectVM;
  tab: ProjectDetailTab;
  onTabChange: (tab: ProjectDetailTab) => void;
  deployAction: DeploymentEntryAction;
  onRelease: () => void;
  onCrossClusterRelease?: () => void;
  onCrossClusterSync?: () => void;
  onViewDeployProgress: () => void;
  onConfigure: () => void;
  onSubmitReview: () => void;
  onReview: () => void;
  onProjectUpdated?: (project: ProjectVM) => void;
  deploymentPage?: DeploymentRunListVM | null;
  deploymentLoading?: boolean;
  runtimeRefreshKey?: number;
  onRunChanged?: (run: DeploymentRunVM) => void;
}

function ProjectIcon({ language }: { language?: string }) {
  return (
    <span className="project-service-icon" aria-hidden>
      {languageLabel(language).charAt(0).toUpperCase()}
    </span>
  );
}

function MetaTag({
  children,
  tone = "gray",
}: {
  children: ReactNode;
  tone?: "gray" | "orange" | "green" | "blue" | "red";
}) {
  return <span className={`project-tag project-tag-${tone}`}>{children}</span>;
}

function reviewStatusMeta(project: ProjectVM): {
  label: string;
  tone: "gray" | "orange" | "green" | "blue" | "red";
} {
  const configComplete = isProjectConfigComplete(project);
  switch (project.review_status) {
    case "pending":
      return { label: "审核中", tone: "orange" };
    case "approved":
      return { label: "审核通过", tone: "green" };
    case "rejected":
      return { label: "审核驳回", tone: "red" };
    default:
      return {
        label: configComplete ? "待提交审核" : "配置中",
        tone: configComplete ? "blue" : "gray",
      };
  }
}

function CopyableId({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <button
      type="button"
      className="project-service-id-copy"
      title={copied ? "已复制" : "复制项目标识"}
      onClick={() => {
        void navigator.clipboard.writeText(value).then(() => {
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1500);
        });
      }}
    >
      <code>{value}</code>
      <span>{copied ? "已复制" : "复制"}</span>
    </button>
  );
}

function MoreActionsMenu({
  items,
}: {
  items: Array<{
    key: string;
    label: string;
    disabled?: boolean;
    title?: string;
    onClick: () => void;
  }>;
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onPointerDown(event: PointerEvent) {
      if (!wrapRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  if (!items.length) return null;

  return (
    <div className="project-service-more-wrap" ref={wrapRef}>
      <button
        type="button"
        className="btn btn-secondary text-sm"
        aria-expanded={open}
        aria-haspopup="menu"
        onClick={() => setOpen((value) => !value)}
      >
        更多
      </button>
      {open ? (
        <div className="project-service-more-menu" role="menu">
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              title={item.title}
              onClick={() => {
                setOpen(false);
                item.onClick();
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function PlaceholderTab({ title }: { title: string }) {
  return (
    <div className="project-service-placeholder">
      <p>{title}功能待开发</p>
    </div>
  );
}

function OverviewTab({
  workspaceId,
  project,
  deploymentPage,
  deploymentLoading,
  publishedView,
  runtimeRefreshKey,
}: {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  deploymentLoading: boolean;
  publishedView: boolean;
  runtimeRefreshKey: number;
}) {
  const published = hasSuccessfulDeployment(deploymentPage);

  if (deploymentLoading && !deploymentPage) {
    if (publishedView) {
      return (
        <ProjectPublishedOverview
          workspaceId={workspaceId}
          project={project}
          deploymentPage={null}
          refreshKey={runtimeRefreshKey}
        />
      );
    }
    return (
      <p className="text-muted project-overview-loading">概览加载中…</p>
    );
  }

  if (!published && !publishedView) {
    return (
      <ProjectUnpublishedOverview
        project={project}
        workspaceId={workspaceId}
        deploymentPage={deploymentPage}
      />
    );
  }

  return (
    <ProjectPublishedOverview
      workspaceId={workspaceId}
      project={project}
      deploymentPage={deploymentPage}
      refreshKey={runtimeRefreshKey}
    />
  );
}

export function ProjectServiceDetailPage({
  workspaceId,
  project,
  tab,
  onTabChange,
  deployAction,
  onRelease,
  onCrossClusterRelease,
  onCrossClusterSync,
  onViewDeployProgress,
  onConfigure,
  onSubmitReview,
  onReview,
  onProjectUpdated,
  deploymentPage = null,
  deploymentLoading = false,
  runtimeRefreshKey: externalRuntimeRefreshKey = 0,
  onRunChanged,
}: ProjectServiceDetailPageProps) {
  const [templateModalOpen, setTemplateModalOpen] = useState(false);
  const [expandModalOpen, setExpandModalOpen] = useState(false);
  const [scaleModalOpen, setScaleModalOpen] = useState(false);
  const [rollbackModalOpen, setRollbackModalOpen] = useState(false);
  const [runtimeOverview, setRuntimeOverview] = useState<ProjectRuntimeOverviewVM | null>(null);
  const [localRuntimeRefreshKey, setLocalRuntimeRefreshKey] = useState(0);
  const stickyPublished = useStickyPublishedProject(project.id);
  const runtimeRefreshKey = externalRuntimeRefreshKey + localRuntimeRefreshKey;
  const primaryAction = projectPrimaryAction(project);
  const statusMeta = reviewStatusMeta(project);
  const businessSpace = project.namespace || workspaceId;
  const published = resolvePublishedProjectView(deploymentPage, tab, {
    loading: deploymentLoading,
    stickyPublished,
  });
  const detailTabs = projectDetailTabs(published);

  useEffect(() => {
    if (!isPublishedProjectView(deploymentPage, tab)) return;
    markStickyPublishedProject(project.id);
  }, [deploymentPage, project.id, tab]);
  const canScale = hasSuccessfulDeployment(deploymentPage);
  const hasActiveRun = deployAction.label === "查看部署进度";
  const rollbackGate = canRollbackDeployment(deploymentPage, hasActiveRun);
  const currentReplicas =
    runtimeOverview?.deployment?.replicas ?? project.resources?.replicas ?? 1;

  const loadRuntimeOverview = useCallback(async () => {
    if (!canScale) {
      setRuntimeOverview(null);
      return;
    }
    try {
      const data = await cluster.getProjectRuntimeOverview(project.id, workspaceId);
      setRuntimeOverview(data);
    } catch {
      setRuntimeOverview(null);
    }
  }, [canScale, project.id, workspaceId]);

  useEffect(() => {
    void loadRuntimeOverview();
  }, [loadRuntimeOverview, runtimeRefreshKey]);

  const handleRuntimeScaled = useCallback(() => {
    setLocalRuntimeRefreshKey((value) => value + 1);
    void loadRuntimeOverview();
  }, [loadRuntimeOverview]);

  const moreMenuItems = useMemo(() => {
    if (primaryAction !== "deploy") return [];
    const items: Array<{
      key: string;
      label: string;
      disabled?: boolean;
      title?: string;
      onClick: () => void;
    }> = [
      {
        key: "cross-cluster",
        label: "跨集群发布",
        disabled: deployAction.disabled && deployAction.label === "发布",
        title: deployAction.reason ?? undefined,
        onClick: () => onCrossClusterRelease?.(),
      },
      {
        key: "cross-cluster-sync",
        label: "跨集群同步",
        disabled: !canScale,
        title: !canScale ? "请先完成首次发布" : undefined,
        onClick: () => onCrossClusterSync?.(),
      },
      {
        key: "expand",
        label: "扩容",
        disabled: !canScale,
        title: !canScale ? "请先完成首次发布" : undefined,
        onClick: () => setExpandModalOpen(true),
      },
      {
        key: "scale",
        label: "伸缩",
        disabled: !canScale,
        title: !canScale ? "请先完成首次发布" : undefined,
        onClick: () => setScaleModalOpen(true),
      },
    ];
    if (published) {
      items.push({
        key: "template",
        label: "模板实例",
        onClick: () => setTemplateModalOpen(true),
      });
    }
    return items;
  }, [
    canScale,
    deployAction.disabled,
    deployAction.label,
    deployAction.reason,
    onCrossClusterRelease,
    onCrossClusterSync,
    primaryAction,
    published,
  ]);

  const headerActions = useMemo(() => {
    if (primaryAction === "deploy") {
      return (
        <>
          <button
            type="button"
            className="btn project-btn-primary text-sm"
            disabled={deployAction.disabled && deployAction.label === "发布"}
            title={deployAction.reason ?? undefined}
            onClick={() => {
              if (deployAction.label === "查看部署进度") {
                onViewDeployProgress();
                return;
              }
              onRelease();
            }}
          >
            {deployAction.label === "查看部署进度" ? "查看部署进度" : "发布"}
          </button>
          <button
            type="button"
            className="btn btn-secondary text-sm project-service-danger"
            disabled={!rollbackGate.allowed}
            title={rollbackGate.reason ?? undefined}
            onClick={() => setRollbackModalOpen(true)}
          >
            回滚
          </button>
          <MoreActionsMenu items={moreMenuItems} />
        </>
      );
    }

    return (
      <>
        {shouldShowEditConfigAction(project) ? (
          <button type="button" className="btn btn-secondary text-sm" onClick={onConfigure}>
            {projectConfigActionLabel(project)}
          </button>
        ) : primaryAction === "configure" ? (
          <button type="button" className="btn project-btn-primary text-sm" onClick={onConfigure}>
            继续配置
          </button>
        ) : null}
        {primaryAction === "submit" ? (
          <button type="button" className="btn project-btn-primary text-sm" onClick={onSubmitReview}>
            提交审核
          </button>
        ) : null}
        {primaryAction === "review" ? (
          <button type="button" className="btn project-btn-primary text-sm" onClick={onReview}>
            审核
          </button>
        ) : null}
        {primaryAction === "pending" ? (
          <button type="button" className="btn btn-secondary text-sm" disabled>
            审核中
          </button>
        ) : null}
      </>
    );
  }, [
    deployAction,
    moreMenuItems,
    onConfigure,
    onRelease,
    onReview,
    onSubmitReview,
    onViewDeployProgress,
    primaryAction,
    project,
    rollbackGate.allowed,
    rollbackGate.reason,
  ]);

  return (
    <div className="project-service-detail">
      <Breadcrumb items={[{ label: "首页", href: "/projects" }, { label: "服务详情" }]} />

      <header className="project-service-header">
        <div className="project-service-header-main">
          <div className="project-service-identity">
            <ProjectIcon language={project.language} />
            <div className="project-service-identity-body">
              <div className="project-service-title-row">
                <h1>
                  <ProjectDisplayNameField
                    workspaceId={workspaceId}
                    project={project}
                    onSaved={(updated) => onProjectUpdated?.(updated)}
                  />
                </h1>
                <MetaTag tone="orange">{languageLabel(project.language)}</MetaTag>
                <MetaTag tone="blue">{businessSpace}</MetaTag>
                <span className={`project-service-status-badge is-${statusMeta.tone}`}>
                  {statusMeta.label}
                </span>
              </div>
              <div className="project-service-meta-line">
                <span>
                  创建人 <strong>{project.submitted_by || project.reviewed_by || "-"}</strong>
                </span>
                <span>
                  创建时间 <strong>{formatDateTime(project.created_at)}</strong>
                </span>
                <span className="project-service-meta-id">
                  项目 ID <CopyableId value={project.name || project.id} />
                </span>
              </div>
              <div className="project-service-description">
                <ProjectDescriptionField
                  workspaceId={workspaceId}
                  project={project}
                  onSaved={(updated) => onProjectUpdated?.(updated)}
                />
              </div>
            </div>
          </div>
        </div>

        <div className="project-service-header-side">
          <div className="project-service-actions">{headerActions}</div>
        </div>
      </header>

      <div className="template-tabs project-service-tabs">
        <div className="template-tabs-nav project-service-tab-nav" role="tablist" aria-label="项目详情">
          {detailTabs.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={`project-tab-${item.id}`}
              aria-selected={tab === item.id}
              aria-controls={`project-tabpanel-${item.id}`}
              className={tab === item.id ? "is-active" : ""}
              onClick={() => onTabChange(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="template-tabs-body project-service-tab-body">
          {tab === "overview" ? (
            <div role="tabpanel" id="project-tabpanel-overview" aria-labelledby="project-tab-overview">
              <OverviewTab
                workspaceId={workspaceId}
                project={project}
                deploymentPage={deploymentPage}
                deploymentLoading={deploymentLoading}
                publishedView={published}
                runtimeRefreshKey={runtimeRefreshKey}
              />
            </div>
          ) : null}
          {!published && tab === "yaml" ? (
            <div role="tabpanel" id="project-tabpanel-yaml" aria-labelledby="project-tab-yaml">
              <ProjectYamlTab workspaceId={workspaceId} project={project} active={tab === "yaml"} />
            </div>
          ) : null}
          {!published && tab === "jenkins" ? (
            <div role="tabpanel" id="project-tabpanel-jenkins" aria-labelledby="project-tab-jenkins">
              <ProjectJenkinsPreview workspaceId={workspaceId} project={project} active={tab === "jenkins"} />
            </div>
          ) : null}
          {!published && tab === "dockerfile" ? (
            <div role="tabpanel" id="project-tabpanel-dockerfile" aria-labelledby="project-tab-dockerfile">
              <section className="stack-sm">
                <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
              </section>
            </div>
          ) : null}
          {tab === "build-logs" ? (
            <ProjectDeploymentConsolePanel
              workspaceId={workspaceId}
              project={project}
              deploymentPage={deploymentPage}
              active={tab === "build-logs"}
              onRunChanged={onRunChanged}
            />
          ) : null}
          {tab === "configmap" ? (
            <ProjectConfigMapTab
              workspaceId={workspaceId}
              project={project}
              active={tab === "configmap"}
            />
          ) : null}
          {tab === "storage" ? (
            <ProjectStorageTab workspaceId={workspaceId} project={project} active={tab === "storage"} />
          ) : null}
          {tab === "webhooks" ? (
            <ProjectWebhookTab workspaceId={workspaceId} project={project} active={tab === "webhooks"} />
          ) : null}
          {tab === "monitor" ? <PlaceholderTab title="监控" /> : null}
        </div>
      </div>

      <ProjectTemplateArtifactsModal
        open={templateModalOpen}
        onClose={() => setTemplateModalOpen(false)}
        workspaceId={workspaceId}
        project={project}
      />
      <RuntimeExpandModal
        open={expandModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        deployment={runtimeOverview?.deployment}
        fallback={{
          cpu_request: project.resources?.cpu_request,
          cpu_limit: project.resources?.cpu_limit,
          memory_request: project.resources?.memory_request,
          memory_limit: project.resources?.memory_limit,
        }}
        onClose={() => setExpandModalOpen(false)}
        onSaved={handleRuntimeScaled}
      />
      <RuntimeScaleModal
        open={scaleModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        currentReplicas={currentReplicas}
        onClose={() => setScaleModalOpen(false)}
        onSaved={handleRuntimeScaled}
      />
      <RuntimeRollbackModal
        open={rollbackModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        deploymentPage={deploymentPage}
        deployment={runtimeOverview?.deployment}
        onClose={() => setRollbackModalOpen(false)}
        onRollbackStarted={(run) => {
          onRunChanged?.(run);
          onViewDeployProgress();
        }}
      />
    </div>
  );
}
