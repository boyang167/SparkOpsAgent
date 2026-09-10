"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import type { ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import {
  formatDateTime,
  languageLabel,
  projectDisplayName,
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
  tone?: "gray" | "orange" | "green" | "blue";
}) {
  return <span className={`project-tag project-tag-${tone}`}>{children}</span>;
}

function reviewStatusLabel(project: ProjectVM) {
  const configComplete = isProjectConfigComplete(project);
  return {
    none: configComplete ? "待提交审核" : "配置中",
    pending: "审核中",
    approved: "审核通过",
    rejected: "审核驳回",
  }[project.review_status ?? "none"];
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
  const statusText = reviewStatusLabel(project);
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

  const headerActions = useMemo(() => {
    if (primaryAction === "deploy") {
      return (
        <>
          <button type="button" className="btn btn-secondary text-sm" disabled title="待开发">
            跨集群发布
          </button>
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
          <div className="project-service-btn-group">
            <button type="button" className="btn btn-secondary text-sm" disabled title="待开发">
              模式
            </button>
            <button
              type="button"
              className="btn btn-secondary text-sm"
              disabled={!canScale}
              title={!canScale ? "请先完成首次发布" : undefined}
              onClick={() => setExpandModalOpen(true)}
            >
              扩容
            </button>
            <button
              type="button"
              className="btn btn-secondary text-sm"
              disabled={!canScale}
              title={!canScale ? "请先完成首次发布" : undefined}
              onClick={() => setScaleModalOpen(true)}
            >
              伸缩
            </button>
          </div>
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
    canScale,
    deployAction,
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
          <div className="project-service-title-row">
            <ProjectIcon language={project.language} />
            <h1>项目名称: {projectDisplayName(project)}</h1>
          </div>

          <div className="project-service-meta-grid">
            <dl>
              <div>
                <dt>创建人</dt>
                <dd>{project.submitted_by || project.reviewed_by || "-"}</dd>
              </div>
              <div>
                <dt>项目中文名</dt>
                <dd>
                  <ProjectDisplayNameField
                    workspaceId={workspaceId}
                    project={project}
                    onSaved={(updated) => onProjectUpdated?.(updated)}
                  />
                </dd>
              </div>
              <div>
                <dt>创建时间</dt>
                <dd>{formatDateTime(project.created_at)}</dd>
              </div>
              <div>
                <dt>项目语言</dt>
                <dd>
                  <MetaTag tone="orange">{languageLabel(project.language)}</MetaTag>
                </dd>
              </div>
            </dl>
            <dl>
              <div>
                <dt>业务空间</dt>
                <dd>
                  <MetaTag tone="orange">{businessSpace}</MetaTag>
                </dd>
              </div>
              <div>
                <dt>项目描述</dt>
                <dd>
                  <ProjectDescriptionField
                    workspaceId={workspaceId}
                    project={project}
                    onSaved={(updated) => onProjectUpdated?.(updated)}
                  />
                </dd>
              </div>
              <div>
                <dt>分布式链路追踪</dt>
                <dd>
                  <span className="text-muted">链路追踪（todo 待开发）</span>
                </dd>
              </div>
              <div>
                <dt>日志平台</dt>
                <dd>
                  <span className="text-muted">日志分析平台（todo 待开发）</span>
                </dd>
              </div>
            </dl>
          </div>
        </div>

        <div className="project-service-header-side">
          <div className="project-service-actions">
            {published ? (
              <button
                type="button"
                className="btn btn-secondary text-sm"
                onClick={() => setTemplateModalOpen(true)}
              >
                模板实例
              </button>
            ) : null}
            {headerActions}
          </div>
          <p className="project-service-status">
            状态: <strong>{statusText}</strong>
          </p>
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
