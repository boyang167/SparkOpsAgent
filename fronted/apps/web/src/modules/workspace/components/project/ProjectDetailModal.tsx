"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { agent, delivery, product, secret, workflow } from "@sparkops/web-sdk";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { AgentVM } from "@sparkops/contracts/agent";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import type { TemplateVM } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import { Modal } from "@/components/ui/Modal";
import { AgentList } from "@/modules/agent/components/AgentList";
import { CreateAgentForm } from "@/modules/agent/components/CreateAgentForm";
import { CreateWorkflowForm, WorkflowSettings } from "@/modules/workflow/components/WorkflowPanel";
import { WorkflowList } from "@/modules/workflow/components/WorkflowList";
import { ProjectMembersPanel } from "../ProjectMembersPanel";
import {
  formatDateTime,
  languageLabel,
  projectDisplayName,
} from "./project-utils";
import { computeProjectProgress, isProjectConfigComplete } from "./project-progress";
import { ProjectGeneratedTemplates } from "./ProjectGeneratedTemplates";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";
import { countGeneratedTemplates } from "./project-template-render";
import { deploymentSummaryState } from "./deployment-summary";

interface ProjectDetailModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM | null;
  onClose: () => void;
  onEditConfig?: () => void;
  agents?: AgentVM[];
  workflows?: WorkflowVM[];
}

type SectionKey =
  | "basic"
  | "git"
  | "dockerfile"
  | "secret"
  | "templates"
  | "stats"
  | "env"
  | "deploys"
  | "members"
  | "agents"
  | "workflows";

function DetailAccordion({
  title,
  icon,
  open,
  onToggle,
  badge,
  children,
}: {
  title: string;
  icon: ReactNode;
  open: boolean;
  onToggle: () => void;
  badge?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className={`project-detail-acc${open ? " is-open" : ""}`}>
      <button type="button" className="project-detail-acc-head" onClick={onToggle}>
        <span className="project-detail-acc-title">
          <span className="project-detail-acc-icon" aria-hidden>
            {icon}
          </span>
          {title}
        </span>
        <span className="project-detail-acc-meta">
          {badge}
          <span className="project-detail-acc-chevron" aria-hidden>
            ›
          </span>
        </span>
      </button>
      {open ? <div className="project-detail-acc-body">{children}</div> : null}
    </section>
  );
}

function InfoGrid({ items }: { items: Array<{ label: string; value: ReactNode; full?: boolean }> }) {
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

function Tag({ children, tone = "gray" }: { children: ReactNode; tone?: "gray" | "orange" | "green" | "blue" }) {
  return <span className={`project-tag project-tag-${tone}`}>{children}</span>;
}

function IconDoc() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
    </svg>
  );
}

function IconGit() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="6" cy="6" r="2" />
      <circle cx="18" cy="6" r="2" />
      <circle cx="12" cy="18" r="2" />
      <path d="M6 8v4a4 4 0 0 0 4 4h0a4 4 0 0 0 4-4V8" />
    </svg>
  );
}

function IconKey() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="8" cy="15" r="4" />
      <path d="M12 15h9M17 15v3M20 15v2" />
    </svg>
  );
}

function IconChart() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 19V5M4 19h16M8 17V10M13 17V7M18 17v-4" />
    </svg>
  );
}

function IconMonitor() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="4" width="18" height="12" rx="2" />
      <path d="M8 20h8M12 16v4" />
    </svg>
  );
}

function IconClock() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="12" r="8" />
      <path d="M12 8v4l2.5 2.5" />
    </svg>
  );
}

function IconUsers() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="3" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a3 3 0 0 1 0 5.74" />
    </svg>
  );
}

function IconBot() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="5" y="8" width="14" height="10" rx="2" />
      <path d="M12 4v4M9 13h.01M15 13h.01M8 18v2M16 18v2" />
    </svg>
  );
}

function IconDocker() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 10h16v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-8z" />
      <path d="M7 10V7h2v3M11 10V6h2v4M15 10V8h2v2" />
    </svg>
  );
}

function IconFlow() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="3" width="6" height="6" rx="1" />
      <rect x="15" y="3" width="6" height="6" rx="1" />
      <rect x="9" y="15" width="6" height="6" rx="1" />
      <path d="M6 9v3a3 3 0 0 0 3 3h6a3 3 0 0 0 3-3V9" />
    </svg>
  );
}

export function ProjectDetailModal({
  open,
  workspaceId,
  project,
  onClose,
  onEditConfig,
  agents: agentsProp,
  workflows: workflowsProp,
}: ProjectDetailModalProps) {
  const [openKeys, setOpenKeys] = useState<SectionKey[]>(["basic"]);
  const [agents, setAgents] = useState<AgentVM[]>(agentsProp ?? []);
  const [workflows, setWorkflows] = useState<WorkflowVM[]>(workflowsProp ?? []);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [deploymentPage, setDeploymentPage] = useState<DeploymentRunListVM | null>(null);
  const [deploymentLoading, setDeploymentLoading] = useState(false);
  const [deploymentError, setDeploymentError] = useState<string | null>(null);
  const [loadingExtra, setLoadingExtra] = useState(false);

  useEffect(() => {
    if (!open || !project) return;
    setOpenKeys(["basic"]);
    setAgents(agentsProp ?? []);
    setWorkflows(workflowsProp ?? []);

    let cancelled = false;
    void (async () => {
      setLoadingExtra(true);
      try {
        const [templateList, secretList, agentList, workflowList] = await Promise.all([
          product.listTemplates(workspaceId).catch(() => [] as TemplateVM[]),
          secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
          agentsProp
            ? Promise.resolve(agentsProp)
            : agent.listAgents(workspaceId, project.id).catch(() => [] as AgentVM[]),
          workflowsProp
            ? Promise.resolve(workflowsProp)
            : workflow.listWorkflows(workspaceId, project.id).catch(() => [] as WorkflowVM[]),
        ]);
        if (cancelled) return;
        setTemplates(templateList);
        setSecrets(secretList);
        setAgents(agentList);
        setWorkflows(workflowList);
      } finally {
        if (!cancelled) setLoadingExtra(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId, agentsProp, workflowsProp]);

  useEffect(() => {
    if (!open || !project) return;
    let cancelled = false;
    setDeploymentPage(null);
    setDeploymentError(null);
    setDeploymentLoading(true);
    void delivery
      .listDeployments(project.id, workspaceId)
      .then((page) => {
        if (!cancelled) setDeploymentPage(page);
      })
      .catch((err) => {
        if (!cancelled) {
          setDeploymentError(err instanceof Error ? err.message : "部署统计不可用");
        }
      })
      .finally(() => {
        if (!cancelled) setDeploymentLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId]);

  const progress = project ? computeProjectProgress(project) : null;
  const configComplete = project ? isProjectConfigComplete(project) : false;

  const generatedTemplateCount = useMemo(
    () => (project ? countGeneratedTemplates(project, templates) : 0),
    [project, templates],
  );

  const gitSecret = secrets.find((s) => s.id === project?.secret_id);
  const registrySecret = secrets.find((s) => s.id === project?.resources?.registry_secret_id);
  const sonarSecret = secrets.find((s) => s.id === project?.resources?.sonarqube_secret_id);
  const jenkinsSecret = secrets.find((s) => s.id === project?.jenkins_secret_id);
  const kubernetesSecret = secrets.find((s) => s.id === project?.kubernetes_secret_id);
  const deploymentSummary = deploymentSummaryState(
    deploymentPage,
    deploymentLoading,
    deploymentError,
  );

  function toggle(key: SectionKey) {
    setOpenKeys((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]));
  }

  function isOpen(key: SectionKey) {
    return openKeys.includes(key);
  }

  if (!project) {
    return (
      <Modal open={open} onClose={onClose} title="项目详情" size="xl" panelClassName="project-detail-modal">
        <p className="text-muted">未选择项目</p>
      </Modal>
    );
  }

  const currentStepLabel = progress?.statusLabel ?? (configComplete ? "待提交审核" : "配置中");
  const reviewStatusLabel = {
    none: configComplete ? "待提交" : "未配置完成",
    pending: "审核中",
    approved: "审核通过",
    rejected: "审核驳回",
  }[project.review_status ?? "none"];

  return (
    <Modal open={open} onClose={onClose} title="项目详情" size="xl" panelClassName="project-detail-modal">
      <div className="project-detail-panel">
        {onEditConfig ? (
          <div className="flex justify-end" style={{ marginBottom: "0.75rem" }}>
            <button type="button" className="project-action-btn is-config" onClick={onEditConfig}>
              修改配置
            </button>
          </div>
        ) : null}
        <DetailAccordion
          title="基本信息"
          icon={<IconDoc />}
          open={isOpen("basic")}
          onToggle={() => toggle("basic")}
        >
          <InfoGrid
            items={[
              { label: "项目名称", value: projectDisplayName(project) },
              { label: "项目标识", value: project.name },
              { label: "命名空间", value: project.namespace || "-" },
              {
                label: "项目状态",
                value: <Tag>{project.status === "active" ? "草稿" : project.status}</Tag>,
              },
              { label: "可见性", value: <Tag>组内</Tag> },
              {
                label: "当前步骤",
                value: <Tag tone="orange">{currentStepLabel}</Tag>,
              },
              {
                label: "开发语言",
                value: <Tag tone="orange">{languageLabel(project.language)}</Tag>,
              },
              { label: "所属产品ID", value: "-" },
              {
                label: "审核状态",
                value: <Tag>{reviewStatusLabel}</Tag>,
              },
              {
                label: "提交时间",
                value: project.submitted_at ? formatDateTime(project.submitted_at) : "-",
              },
              {
                label: "审批时间",
                value: project.reviewed_at ? formatDateTime(project.reviewed_at) : "-",
              },
              {
                label: "审批意见",
                value: project.review_comment || "-",
                full: true,
              },
              { label: "发布状态", value: <Tag>未发布</Tag> },
              {
                label: "项目描述",
                value: project.description || "暂无描述",
                full: true,
              },
              { label: "所有者ID", value: "-" },
              {
                label: "创建时间",
                value: formatDateTime(project.created_at)?.slice(0, 10) || "-",
              },
              {
                label: "更新时间",
                value: formatDateTime(project.created_at)?.slice(0, 10) || "-",
              },
            ]}
          />
        </DetailAccordion>

        <DetailAccordion
          title="Git配置"
          icon={<IconGit />}
          open={isOpen("git")}
          onToggle={() => toggle("git")}
        >
          <InfoGrid
            items={[
              { label: "Git 仓库地址", value: project.git_url || "-", full: true },
              {
                label: "引用类型",
                value: project.git_ref_type || "branch",
              },
              { label: "引用值", value: project.git_ref_value || "-" },
            ]}
          />
        </DetailAccordion>

        <DetailAccordion
          title="Dockerfile"
          icon={<IconDocker />}
          open={isOpen("dockerfile")}
          onToggle={() => toggle("dockerfile")}
        >
          <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
        </DetailAccordion>

        <DetailAccordion
          title="秘钥配置"
          icon={<IconKey />}
          open={isOpen("secret")}
          onToggle={() => toggle("secret")}
        >
          <InfoGrid
            items={[
              {
                label: "Git 认证密钥",
                value: gitSecret ? gitSecret.display_name || gitSecret.name : project.secret_id || "未配置",
              },
              {
                label: "镜像仓库类型",
                value: project.resources?.registry_type || "Harbor",
              },
              {
                label: "镜像仓库密钥",
                value: registrySecret
                  ? registrySecret.display_name || registrySecret.name
                  : project.resources?.registry_secret_id || "未配置",
              },
              {
                label: "Sonarqube 密钥",
                value: sonarSecret
                  ? sonarSecret.display_name || sonarSecret.name
                  : project.resources?.sonarqube_secret_id || "未配置",
              },
              {
                label: "Jenkins 密钥",
                value: jenkinsSecret
                  ? jenkinsSecret.display_name || jenkinsSecret.name
                  : project.jenkins_secret_id || "未配置",
              },
              {
                label: "Kubernetes 密钥",
                value: kubernetesSecret
                  ? kubernetesSecret.display_name || kubernetesSecret.name
                  : project.kubernetes_secret_id || "未配置",
              },
              {
                label: "默认集群 ID",
                value: project.default_cluster_id || "未配置",
              },
              {
                label: "代码静态扫描",
                value: project.resources?.enable_static_scan ? "已开启" : "关闭",
              },
            ]}
          />
        </DetailAccordion>

        <DetailAccordion
          title="生成的模板"
          icon={<IconDoc />}
          open={isOpen("templates")}
          onToggle={() => toggle("templates")}
          badge={
            generatedTemplateCount > 0 ? (
              <span className="project-detail-badge is-green">{generatedTemplateCount} 个模板</span>
            ) : null
          }
        >
          <ProjectGeneratedTemplates workspaceId={workspaceId} project={project} />
        </DetailAccordion>

        <DetailAccordion
          title="统计信息"
          icon={<IconChart />}
          open={isOpen("stats")}
          onToggle={() => toggle("stats")}
        >
          <InfoGrid
            items={[
              {
                label: "部署次数",
                value: deploymentSummary.count,
              },
              { label: "构建次数", value: "暂无统计" },
              {
                label: "最后部署",
                value: deploymentSummary.latest
                  ? `${deploymentSummary.latest.status} · ${formatDateTime(deploymentSummary.latest.created_at)}`
                  : deploymentLoading
                    ? "加载中…"
                    : deploymentSummary.unavailable
                      ? "不可用"
                      : "暂无",
              },
              {
                label: "配置进度",
                value: progress?.steps.filter((s) => s.status === "done").length
                  ? `${progress.steps.filter((s) => s.status === "done").length} / 4`
                  : "1 / 4",
              },
            ]}
          />
        </DetailAccordion>

        <DetailAccordion
          title="环境信息"
          icon={<IconMonitor />}
          open={isOpen("env")}
          onToggle={() => toggle("env")}
          badge={<span className="project-detail-badge is-red">3</span>}
        >
          <InfoGrid
            items={[
              { label: "命名空间", value: project.namespace || "-" },
              { label: "开发语言", value: languageLabel(project.language) },
              {
                label: "CPU 请求 / 限制",
                value: `${project.resources?.cpu_request || "-"} ~ ${project.resources?.cpu_limit || "-"}`,
              },
              {
                label: "内存请求 / 限制",
                value: `${project.resources?.memory_request || "-"} ~ ${project.resources?.memory_limit || "-"}`,
              },
              { label: "副本数", value: String(project.resources?.replicas ?? 1) },
            ]}
          />
        </DetailAccordion>

        <DetailAccordion
          title="最近部署"
          icon={<IconClock />}
          open={isOpen("deploys")}
          onToggle={() => toggle("deploys")}
          badge={
            deploymentPage && deploymentPage.total > 0 ? (
              <span className="project-detail-badge is-green">{deploymentPage.total}</span>
            ) : null
          }
        >
          {deploymentLoading ? (
            <p className="text-muted">加载中…</p>
          ) : deploymentError ? (
            <p className="text-muted" role="alert">
              部署记录不可用：{deploymentError}
            </p>
          ) : !deploymentPage || deploymentPage.items.length === 0 ? (
            <p className="text-muted">暂无部署记录</p>
          ) : (
            <ul className="stack-sm">
              {deploymentPage.items.map((run) => (
                <li key={run.id} className="flex items-center justify-between gap-2">
                  <span>{run.status}</span>
                  <time dateTime={run.created_at}>{formatDateTime(run.created_at)}</time>
                </li>
              ))}
            </ul>
          )}
        </DetailAccordion>

        <DetailAccordion
          title="项目成员"
          icon={<IconUsers />}
          open={isOpen("members")}
          onToggle={() => toggle("members")}
        >
          <div className="project-detail-embed">
            <ProjectMembersPanel workspaceId={workspaceId} projectId={project.id} />
          </div>
        </DetailAccordion>

        <DetailAccordion
          title="Agents"
          icon={<IconBot />}
          open={isOpen("agents")}
          onToggle={() => toggle("agents")}
          badge={
            agents.length > 0 ? (
              <span className="project-detail-badge is-green">{agents.length}</span>
            ) : null
          }
        >
          <div className="project-detail-embed">
            <div className="project-detail-section-actions">
              <CreateAgentForm workspaceId={workspaceId} projectId={project.id} />
            </div>
            <AgentList agents={agents} projectId={project.id} />
          </div>
        </DetailAccordion>

        <DetailAccordion
          title="Workflows"
          icon={<IconFlow />}
          open={isOpen("workflows")}
          onToggle={() => toggle("workflows")}
          badge={
            workflows.length > 0 ? (
              <span className="project-detail-badge is-green">{workflows.length}</span>
            ) : null
          }
        >
          <div className="project-detail-embed">
            <div className="project-detail-section-actions">
              <CreateWorkflowForm workspaceId={workspaceId} projectId={project.id} />
            </div>
            <WorkflowList workflows={workflows} projectId={project.id} />
            <WorkflowSettings workspaceId={workspaceId} workflows={workflows} />
          </div>
        </DetailAccordion>
      </div>
    </Modal>
  );
}
