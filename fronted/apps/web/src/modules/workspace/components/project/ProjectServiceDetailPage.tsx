"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { product, secret } from "@sparkops/web-sdk";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import {
  formatDateTime,
  languageLabel,
  projectDisplayName,
} from "./project-utils";
import {
  computeProjectProgress,
  isProjectConfigComplete,
  projectConfigActionLabel,
  projectPrimaryAction,
  shouldShowEditConfigAction,
  type ProjectProgressStep,
} from "./project-progress";
import type { DeploymentEntryAction } from "./deployment-gate";
import { deploymentSummaryState } from "./deployment-summary";
import { ProjectGeneratedTemplates } from "./ProjectGeneratedTemplates";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";
import { countGeneratedTemplates } from "./project-template-render";
import { ProjectDisplayNameField, ProjectDescriptionField } from "./ProjectDisplayNameEditModal";
import {
  PROJECT_DETAIL_TABS,
  type ProjectDetailTab,
} from "./project-detail-tabs";

interface ProjectServiceDetailPageProps {
  workspaceId: string;
  project: ProjectVM;
  tab: ProjectDetailTab;
  onTabChange: (tab: ProjectDetailTab) => void;
  deploymentPage: DeploymentRunListVM | null;
  deploymentLoading: boolean;
  deploymentListError: string | null;
  deployAction: DeploymentEntryAction;
  onRelease: () => void;
  onViewDeployProgress: () => void;
  onConfigure: () => void;
  onSubmitReview: () => void;
  onReview: () => void;
  onViewRun?: (run: DeploymentRunVM) => void;
  onProjectUpdated?: (project: ProjectVM) => void;
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

function ProgressTrack({ steps }: { steps: ProjectProgressStep[] }) {
  return (
    <section className="project-workbench-progress">
      <div className="project-workbench-progress-title">项目进度</div>
      <ol className="project-workbench-steps">
        {steps.map((step, index) => (
          <li key={step.key} className={`project-workbench-step is-${step.status}`}>
            <div className="project-workbench-step-node">
              <span className="project-workbench-step-circle">
                {step.status === "done" ? "✓" : index + 1}
              </span>
            </div>
            <div className="project-workbench-step-copy">
              <strong>{step.title}</strong>
              <span>{step.subtitle}</span>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

function deploymentStatusLabel(status: DeploymentRunVM["status"]) {
  const map: Record<DeploymentRunVM["status"], string> = {
    queued: "排队中",
    preparing: "准备中",
    building: "构建中",
    deploying: "部署中",
    succeeded: "成功",
    failed: "失败",
    aborted: "已中止",
  };
  return map[status] ?? status;
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
  deploymentListError,
}: {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  deploymentLoading: boolean;
  deploymentListError: string | null;
}) {
  const progress = computeProjectProgress(project);
  const deploymentSummary = deploymentSummaryState(
    deploymentPage,
    deploymentLoading,
    deploymentListError,
  );

  return (
    <div className="stack-md">
      <ProgressTrack steps={progress.steps} />
      <div className="project-workbench-stats">
        <span>
          部署次数 <strong>{deploymentSummary.count}</strong>
        </span>
        <span>
          最后部署{" "}
          <strong>
            {deploymentSummary.latest
              ? `${deploymentStatusLabel(deploymentSummary.latest.status)} · ${formatDateTime(deploymentSummary.latest.created_at)}`
              : deploymentSummary.unavailable
                ? "不可用"
                : "暂无"}
          </strong>
        </span>
        <span>
          配置进度{" "}
          <strong>
            {progress.steps.filter((step) => step.status === "done").length} / 4
          </strong>
        </span>
      </div>
      <section className="stack-sm">
        <h4 className="project-service-section-title">生成的模板</h4>
        <ProjectGeneratedTemplates workspaceId={workspaceId} project={project} />
      </section>
    </div>
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

function BuildLogsTab({
  deploymentPage,
  deploymentLoading,
  deploymentListError,
  onViewRun,
}: {
  deploymentPage: DeploymentRunListVM | null;
  deploymentLoading: boolean;
  deploymentListError: string | null;
  onViewRun?: (run: DeploymentRunVM) => void;
}) {
  if (deploymentLoading) {
    return <p className="text-muted">Build 日志加载中…</p>;
  }
  if (deploymentListError) {
    return <p className="text-muted">{deploymentListError}</p>;
  }
  if (!deploymentPage?.items.length) {
    return <p className="text-muted">暂无构建 / 部署记录</p>;
  }

  return (
    <div className="project-build-log-table-wrap">
      <table className="data-table project-build-log-table">
        <thead>
          <tr>
            <th>记录 ID</th>
            <th>镜像标签</th>
            <th>Git 引用</th>
            <th>集群</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {deploymentPage.items.map((run) => (
            <tr key={run.id}>
              <td className="font-mono text-xs">{run.id.slice(0, 8)}</td>
              <td>{run.image_tag || "-"}</td>
              <td>
                {run.git_ref_type}/{run.git_ref_value}
              </td>
              <td className="font-mono text-xs">{run.cluster_id.slice(0, 8)}</td>
              <td>
                <span className="project-tag project-tag-blue">{deploymentStatusLabel(run.status)}</span>
              </td>
              <td>{formatDateTime(run.created_at)}</td>
              <td>
                {onViewRun ? (
                  <button type="button" className="btn-ghost text-sm" onClick={() => onViewRun(run)}>
                    查看日志
                  </button>
                ) : (
                  "-"
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function ProjectServiceDetailPage({
  workspaceId,
  project,
  tab,
  onTabChange,
  deploymentPage,
  deploymentLoading,
  deploymentListError,
  deployAction,
  onRelease,
  onViewDeployProgress,
  onConfigure,
  onSubmitReview,
  onReview,
  onViewRun,
  onProjectUpdated,
}: ProjectServiceDetailPageProps) {
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [configLoading, setConfigLoading] = useState(false);

  const primaryAction = projectPrimaryAction(project);
  const statusText = reviewStatusLabel(project);
  const businessSpace = project.namespace || workspaceId;

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
        <div className="template-tabs-nav">
          {PROJECT_DETAIL_TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              className={tab === item.id ? "is-active" : ""}
              onClick={() => onTabChange(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="template-tabs-body">
          {tab === "overview" ? (
            <OverviewTab
              workspaceId={workspaceId}
              project={project}
              deploymentPage={deploymentPage}
              deploymentLoading={deploymentLoading}
              deploymentListError={deploymentListError}
            />
          ) : null}
          {tab === "config" ? (
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
          {tab === "build-logs" ? (
            <BuildLogsTab
              deploymentPage={deploymentPage}
              deploymentLoading={deploymentLoading}
              deploymentListError={deploymentListError}
              onViewRun={onViewRun}
            />
          ) : null}
          {tab === "storage" ? <PlaceholderTab title="持久化存储" /> : null}
          {tab === "webhooks" ? <PlaceholderTab title="Webhooks" /> : null}
          {tab === "monitor" ? <PlaceholderTab title="监控" /> : null}
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
