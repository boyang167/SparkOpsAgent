"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import type { MeVM } from "@sparkops/contracts/identity";
import type { OperationRecordVM } from "@sparkops/contracts/platform";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import type { ActiveSprintSummaryVM, WorkspaceReleasePlanVM } from "@sparkops/contracts/product";
import { releasePlanStatusLabel } from "@/modules/product/components/release-plan-utils";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { platform, product, tenant } from "@sparkops/web-sdk";
import {
  formatCompactDateTime,
  languageColor,
  languageLabel,
  projectDisplayName,
} from "@/modules/workspace/components/project/project-utils";

interface WorkplaceDashboardProps {
  session: MeVM;
  workspaces: WorkspaceVM[];
  workspaceId: string;
  workspaceName: string;
  projects: ProjectVM[];
}

interface WorkplaceMetrics {
  products: number | null;
  deploymentTotal: number | null;
  loading: boolean;
}

function workspaceAccent(name: string) {
  const palette = ["#6366f1", "#0ea5e9", "#14b8a6", "#f59e0b", "#ec4899", "#8b5cf6"];
  let hash = 0;
  for (let i = 0; i < name.length; i += 1) hash = (hash + name.charCodeAt(i) * (i + 1)) % palette.length;
  return palette[hash] ?? palette[0];
}

function projectStatusLabel(project: ProjectVM) {
  const review = project.review_status ?? "none";
  if (review === "approved") return "可发布";
  if (review === "pending") return "审核中";
  if (review === "rejected") return "已驳回";
  return "配置中";
}

function projectStatusTone(project: ProjectVM) {
  const review = project.review_status ?? "none";
  if (review === "approved") return "success";
  if (review === "pending") return "running";
  if (review === "rejected") return "failed";
  return "neutral";
}

function operationTitle(record: OperationRecordVM) {
  const path = record.path.replace(/^\/api\/(?:bff\/v1|v1)\//, "").replace(/\/{2,}/g, "/");
  return `${record.method} ${path}`;
}

function operationTone(record: OperationRecordVM) {
  if (record.status >= 500) return "failed";
  if (record.status >= 400) return "running";
  return "success";
}

function PanelCard({
  title,
  action,
  children,
  className = "",
}: {
  title: string;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={`workplace-panel ${className}`.trim()}>
      <header className="workplace-panel-head">
        <h3 className="workplace-panel-title">{title}</h3>
        {action}
      </header>
      {children}
    </section>
  );
}

function EmptyHint({ children }: { children: React.ReactNode }) {
  return <p className="workplace-empty">{children}</p>;
}

export function WorkplaceDashboard({
  session,
  workspaces,
  workspaceId,
  workspaceName,
  projects,
}: WorkplaceDashboardProps) {
  const router = useRouter();
  const [activity, setActivity] = useState<OperationRecordVM[]>([]);
  const [activityLoading, setActivityLoading] = useState(true);
  const [activityError, setActivityError] = useState<string | null>(null);
  const [metrics, setMetrics] = useState<WorkplaceMetrics>({
    products: null,
    deploymentTotal: null,
    loading: true,
  });
  const [switchingWorkspace, setSwitchingWorkspace] = useState<string | null>(null);
  const [activeSprints, setActiveSprints] = useState<ActiveSprintSummaryVM[]>([]);
  const [activeSprintsLoading, setActiveSprintsLoading] = useState(true);
  const [releasePlans, setReleasePlans] = useState<WorkspaceReleasePlanVM[]>([]);
  const [releasePlansLoading, setReleasePlansLoading] = useState(true);

  const displayName = session.display_name?.trim() || session.email.split("@")[0] || "用户";
  const initial = displayName.charAt(0).toUpperCase();

  const activeProjects = useMemo(
    () => projects.filter((project) => (project.review_status ?? "none") === "approved"),
    [projects],
  );

  const quickProjects = useMemo(
    () =>
      [...projects]
        .sort((left, right) => (right.created_at ?? "").localeCompare(left.created_at ?? ""))
        .slice(0, 8),
    [projects],
  );

  useEffect(() => {
    let cancelled = false;
    setActivityLoading(true);
    setActiveSprintsLoading(true);
    setReleasePlansLoading(true);
    setMetrics((prev) => ({ ...prev, loading: true }));

    void Promise.allSettled([
      platform.listOperationRecords(workspaceId, { page: 1, page_size: 12 }),
      platform.getDashboard(workspaceId),
      platform.listDeploymentStats(workspaceId),
      product.listActiveSprints(workspaceId),
      product.listWorkspaceReleasePlans(workspaceId, {
        status: "planning,wait_for_approval,executing",
      }),
    ]).then(([activityResult, dashboardResult, statsResult, sprintsResult, releasePlansResult]) => {
      if (cancelled) return;

      if (activityResult.status === "fulfilled") {
        setActivity(activityResult.value.items);
        setActivityError(null);
      } else {
        setActivity([]);
        setActivityError(
          activityResult.reason instanceof BffError
            ? activityResult.reason.message
            : "动态加载失败",
        );
      }
      setActivityLoading(false);

      const dashboard =
        dashboardResult.status === "fulfilled" ? dashboardResult.value : null;
      const deploymentTotal =
        statsResult.status === "fulfilled"
          ? (statsResult.value ?? []).reduce((sum, item) => sum + (item.count ?? 0), 0)
          : null;

      setMetrics({
        products: dashboard?.products ?? dashboard?.product_count ?? null,
        deploymentTotal,
        loading: false,
      });

      if (sprintsResult.status === "fulfilled") {
        setActiveSprints(sprintsResult.value);
      } else {
        setActiveSprints([]);
      }
      setActiveSprintsLoading(false);

      if (releasePlansResult.status === "fulfilled") {
        setReleasePlans(releasePlansResult.value);
      } else {
        setReleasePlans([]);
      }
      setReleasePlansLoading(false);
    });

    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  async function handleSwitchWorkspace(nextWorkspaceId: string) {
    if (nextWorkspaceId === workspaceId || switchingWorkspace) return;
    setSwitchingWorkspace(nextWorkspaceId);
    try {
      await tenant.selectWorkspace(nextWorkspaceId);
      router.refresh();
    } finally {
      setSwitchingWorkspace(null);
    }
  }

  return (
    <div className="workplace-page">
      <header className="workplace-hero">
        <div className="workplace-hero-main">
          <div className="workplace-avatar" aria-hidden>
            {initial}
          </div>
          <div className="workplace-hero-copy">
            <div className="workplace-hero-title-row">
              <h1 className="workplace-hero-title">{displayName}</h1>
              {session.is_super_admin ? (
                <span className="workplace-hero-badge">超级管理员</span>
              ) : null}
            </div>
            <p className="workplace-hero-meta">
              {session.email}
              <span className="workplace-hero-sep">·</span>
              当前空间 <strong>{workspaceName}</strong>
            </p>
          </div>
        </div>
        <div className="workplace-hero-actions">
          <Link href="/projects" className="workplace-hero-link">
            项目中心
          </Link>
          <Link href="/dashboard?create=1" className="workplace-hero-primary">
            + 创建项目
          </Link>
        </div>
      </header>

      <div className="workplace-layout">
        <div className="workplace-main">
          <PanelCard
            title={`当前 ${workspaceName} 的服务`}
            action={
              <Link href="/projects" className="workplace-panel-link">
                查看全部
              </Link>
            }
          >
            {activeProjects.length === 0 ? (
              <EmptyHint>暂无已审核通过的项目，完成配置并审核后将在此展示。</EmptyHint>
            ) : (
              <ul className="workplace-service-list">
                {activeProjects.slice(0, 6).map((project) => {
                  const tone = projectStatusTone(project);
                  return (
                    <li key={project.id}>
                      <Link href={`/projects/${project.id}`} className="workplace-service-item">
                        <span
                          className="workplace-service-icon"
                          style={{ backgroundColor: languageColor(project.language) }}
                        >
                          {languageLabel(project.language).charAt(0).toUpperCase()}
                        </span>
                        <span className="workplace-service-body">
                          <strong>{projectDisplayName(project)}</strong>
                          <span>{project.namespace || project.name}</span>
                        </span>
                        <span className={`workplace-service-status is-${tone}`}>
                          {projectStatusLabel(project)}
                        </span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
            )}
          </PanelCard>

          <PanelCard title="进行中的迭代">
            {activeSprintsLoading ? (
              <EmptyHint>加载活跃迭代…</EmptyHint>
            ) : activeSprints.length === 0 ? (
              <EmptyHint>当前空间暂无进行中的产品迭代。</EmptyHint>
            ) : (
              <ul className="workplace-service-list">
                {activeSprints.slice(0, 8).map((sprint) => (
                  <li key={sprint.id}>
                    <Link href={`/products/${sprint.product_id}`} className="workplace-service-item">
                      <span className="workplace-service-body">
                        <strong>{sprint.name}</strong>
                        <span>
                          {sprint.product_name ?? "产品"} · 工作项 {sprint.work_item_count ?? 0} / 完成{" "}
                          {sprint.done_work_item_count ?? 0}
                        </span>
                      </span>
                      <span className="workplace-service-status is-running">进行中</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </PanelCard>

          <PanelCard
            title="进行中的发布计划"
            action={
              <Link href="/publish/release-plans" className="workplace-panel-link">
                查看全部
              </Link>
            }
          >
            {releasePlansLoading ? (
              <EmptyHint>加载发布计划…</EmptyHint>
            ) : releasePlans.length === 0 ? (
              <EmptyHint>当前空间暂无进行中的发布计划。</EmptyHint>
            ) : (
              <ul className="workplace-service-list">
                {releasePlans.slice(0, 8).map((plan) => (
                  <li key={plan.id}>
                    <Link
                      href={`/products/${plan.product_id}?tab=releases&planId=${plan.id}`}
                      className="workplace-service-item"
                    >
                      <span className="workplace-service-body">
                        <strong>{plan.name}</strong>
                        <span>
                          {plan.product_name ?? "产品"}
                          {plan.sprint_name ? ` · ${plan.sprint_name}` : ""}
                        </span>
                      </span>
                      <span className="workplace-service-status is-running">
                        {releasePlanStatusLabel(plan.status)}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </PanelCard>

          <PanelCard title="动态">
            {activityLoading ? (
              <EmptyHint>加载最近动态…</EmptyHint>
            ) : activityError ? (
              <EmptyHint>{activityError}</EmptyHint>
            ) : activity.length === 0 ? (
              <EmptyHint>暂无操作记录，平台活动将在这里呈现。</EmptyHint>
            ) : (
              <ul className="workplace-activity-list">
                {activity.map((record) => (
                  <li key={record.id} className="workplace-activity-item">
                    <span className={`workplace-activity-dot is-${operationTone(record)}`} />
                    <div className="workplace-activity-copy">
                      <strong>{operationTitle(record)}</strong>
                      <span>
                        HTTP {record.status}
                        {record.latency_ms ? ` · ${record.latency_ms}ms` : ""}
                      </span>
                    </div>
                    <time className="workplace-activity-time">
                      {formatCompactDateTime(record.created_at)}
                    </time>
                  </li>
                ))}
              </ul>
            )}
          </PanelCard>
        </div>

        <aside className="workplace-aside">
          <PanelCard title="指数">
            <div className="workplace-metrics">
              <div className="workplace-metric">
                <span>项目</span>
                <strong>{projects.length}</strong>
              </div>
              <div className="workplace-metric">
                <span>可发布</span>
                <strong>{activeProjects.length}</strong>
              </div>
              <div className="workplace-metric">
                <span>产品</span>
                <strong>{metrics.loading ? "—" : (metrics.products ?? 0)}</strong>
              </div>
              <div className="workplace-metric">
                <span>部署统计</span>
                <strong>{metrics.loading ? "—" : (metrics.deploymentTotal ?? 0)}</strong>
              </div>
            </div>
          </PanelCard>

          <PanelCard title="业务空间">
            {workspaces.length === 0 ? (
              <EmptyHint>暂无可用 Workspace</EmptyHint>
            ) : (
              <ul className="workplace-space-grid">
                {workspaces.map((workspace) => {
                  const active = workspace.id === workspaceId;
                  const accent = workspaceAccent(workspace.name);
                  return (
                    <li key={workspace.id}>
                      <button
                        type="button"
                        className={`workplace-space-item${active ? " is-active" : ""}`}
                        disabled={Boolean(switchingWorkspace)}
                        onClick={() => void handleSwitchWorkspace(workspace.id)}
                      >
                        <span className="workplace-space-icon" style={{ backgroundColor: accent }}>
                          {workspace.name.charAt(0).toUpperCase()}
                        </span>
                        <span className="workplace-space-name" title={workspace.name}>
                          {workspace.name}
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </PanelCard>

          <PanelCard
            title="快捷项目"
            action={
              <Link href="/projects" className="workplace-panel-link">
                查看详情
              </Link>
            }
          >
            {quickProjects.length === 0 ? (
              <EmptyHint>还没有项目，点击上方创建第一个吧。</EmptyHint>
            ) : (
              <ul className="workplace-quick-list">
                {quickProjects.map((project) => (
                  <li key={project.id}>
                    <Link href={`/projects/${project.id}`} className="workplace-quick-item">
                      <span
                        className="workplace-quick-dot"
                        style={{ backgroundColor: languageColor(project.language) }}
                      />
                      <span>{projectDisplayName(project)}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </PanelCard>
        </aside>
      </div>
    </div>
  );
}
