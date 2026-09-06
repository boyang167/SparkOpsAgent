"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { product, secret } from "@sparkops/web-sdk";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
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
import { ProjectBuildLogsTab } from "./ProjectBuildLogsTab";
import { ProjectYamlTab } from "./ProjectYamlTab";
import { ProjectJenkinsPreview } from "./ProjectJenkinsPreview";
import { ProjectPublishedOverview } from "./ProjectPublishedOverview";
import { ProjectUnpublishedOverview } from "./ProjectUnpublishedOverview";
import { hasSuccessfulDeployment, isPublishedProjectView } from "./project-deployment-state";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";
import { countGeneratedTemplates } from "./project-template-render";
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

function InfoGrid({
  items,
}: {
  items: Array<{ label: string; value: ReactNode; full?: boolean }>;
}) {
  return (
    <dl className="project-detail-grid">
      {items.map((item) => (
        <div key={item.label} className={`project-detail-cell${item.full ? " is-full" : ""}`}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
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
}: {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  deploymentLoading: boolean;
}) {
  const published = hasSuccessfulDeployment(deploymentPage);

  if (deploymentLoading && !deploymentPage) {
    return <p className="text-muted">概览加载中…</p>;
  }

  if (!published) {
    return <ProjectUnpublishedOverview project={project} workspaceId={workspaceId} />;
  }

  return (
    <ProjectPublishedOverview
      workspaceId={workspaceId}
      project={project}
      deploymentPage={deploymentPage}
    />
  );
}

function ConfigTab({
  workspaceId,
  project,
  secrets,
  templates,
  loading,
  onConfigure,
  canEdit,
}: {
  workspaceId: string;
  project: ProjectVM;
  secrets: SecretVM[];
  templates: TemplateVM[];
  loading: boolean;
  onConfigure: () => void;
  canEdit: boolean;
}) {
  const gitSecret = secrets.find((item) => item.id === project.secret_id);
  const registrySecret = secrets.find((item) => item.id === project.resources?.registry_secret_id);
  const jenkinsSecret = secrets.find((item) => item.id === project.jenkins_secret_id);
  const generatedTemplateCount = countGeneratedTemplates(project, templates);

  return (
    <div className="stack-md">
      {canEdit ? (
        <div className="flex justify-end">
          <button type="button" className="btn btn-secondary text-sm" onClick={onConfigure}>
            {projectConfigActionLabel(project)}
          </button>
        </div>
      ) : null}
      {loading ? <p className="text-muted">配置详情加载中…</p> : null}
      <InfoGrid
        items={[
          { label: "Git 仓库", value: project.git_url || "-", full: true },
          { label: "引用类型", value: project.git_ref_type || "branch" },
          { label: "引用值", value: project.git_ref_value || "-" },
          { label: "Dockerfile", value: project.dockerfile_path || "./Dockerfile" },
          {
            label: "Git 密钥",
            value: gitSecret ? gitSecret.display_name || gitSecret.name : project.secret_id || "未配置",
          },
          {
            label: "镜像仓库密钥",
            value: registrySecret
              ? registrySecret.display_name || registrySecret.name
              : project.resources?.registry_secret_id || "未配置",
          },
          {
            label: "Jenkins 密钥",
            value: jenkinsSecret
              ? jenkinsSecret.display_name || jenkinsSecret.name
              : project.jenkins_secret_id || "未配置",
          },
          { label: "默认集群", value: project.default_cluster_id || "未配置" },
          {
            label: "副本 / CPU / 内存",
            value: `${project.resources?.replicas ?? "-"} · ${project.resources?.cpu_request || "-"} ~ ${project.resources?.cpu_limit || "-"} · ${project.resources?.memory_request || "-"} ~ ${project.resources?.memory_limit || "-"}`,
            full: true,
          },
          {
            label: "生成模板数",
            value: generatedTemplateCount > 0 ? `${generatedTemplateCount} 个` : "暂无",
          },
        ]}
      />
      <section className="stack-sm">
        <h4 className="project-service-section-title">Dockerfile 预览</h4>
        <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
      </section>
    </div>
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
}: ProjectServiceDetailPageProps) {
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [configLoading, setConfigLoading] = useState(false);

  const primaryAction = projectPrimaryAction(project);
  const statusText = reviewStatusLabel(project);
  const businessSpace = project.namespace || workspaceId;
  const published = isPublishedProjectView(deploymentPage, tab);
  const detailTabs = projectDetailTabs(published);

  useEffect(() => {
    if (tab !== "config") return;
    let cancelled = false;
    setConfigLoading(true);
    void Promise.all([
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
      product.listTemplates(workspaceId).catch(() => [] as TemplateVM[]),
    ])
      .then(([secretList, templateList]) => {
        if (cancelled) return;
        setSecrets(secretList);
        setTemplates(templateList);
      })
      .finally(() => {
        if (!cancelled) setConfigLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [tab, workspaceId]);

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
          <button type="button" className="btn btn-secondary text-sm project-service-danger" disabled title="待开发">
            回滚
          </button>
          <div className="project-service-btn-group">
            <button type="button" className="btn btn-secondary text-sm" disabled title="待开发">
              模式
            </button>
            <button type="button" className="btn btn-secondary text-sm" disabled title="待开发">
              扩容
            </button>
            <button type="button" className="btn btn-secondary text-sm" disabled title="待开发">
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
    deployAction,
    onConfigure,
    onRelease,
    onReview,
    onSubmitReview,
    onViewDeployProgress,
    primaryAction,
    project,
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
          <div className="project-service-actions">{headerActions}</div>
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
          {published && tab === "config" ? (
            <ConfigTab
              workspaceId={workspaceId}
              project={project}
              secrets={secrets}
              templates={templates}
              loading={configLoading}
              onConfigure={onConfigure}
              canEdit={shouldShowEditConfigAction(project)}
            />
          ) : null}
          {published && tab === "build-logs" ? (
            <ProjectBuildLogsTab workspaceId={workspaceId} project={project} active={tab === "build-logs"} />
          ) : null}
          {published && tab === "storage" ? <PlaceholderTab title="持久化存储" /> : null}
          {published && tab === "webhooks" ? <PlaceholderTab title="Webhooks" /> : null}
          {published && tab === "monitor" ? <PlaceholderTab title="监控" /> : null}
        </div>
      </div>

      <div className="project-service-footer-links">
        <Link href="/projects" className="btn-ghost text-sm">
          返回项目列表
        </Link>
      </div>
    </div>
  );
}
