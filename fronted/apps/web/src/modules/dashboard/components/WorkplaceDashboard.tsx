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
import { isWorkspaceReadOnly } from "@/modules/tenant/lib/org-display";
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

function operationLabel(record: OperationRecordVM) {
  const path = record.path.replace(/^\/api\/(?:bff\/v1|v1)\//, "");
  const verb =
    record.method === "POST"
      ? "创建"
      : record.method === "PUT" || record.method === "PATCH"
        ? "更新"
        : record.method === "DELETE"
          ? "删除"
          : record.method;
  if (path.includes("release-plans")) return `${verb}发布计划`;
  if (path.includes("sprints")) return `${verb}迭代`;
  if (path.includes("products")) return `${verb}产品`;
  if (path.includes("projects")) return `${verb}项目`;
  if (path.includes("members")) return `${verb}成员`;
  return `${verb}记录`;
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

function isWriteOperation(record: OperationRecordVM) {
  return !["GET", "HEAD", "OPTIONS"].includes(record.method);
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
  workspaceId,
  workspaces,
  projects,
}: WorkplaceDashboardProps) {
  const router = useRouter();
  const [activity, setActivity] = useState<OperationRecordVM[]>([]);
  const [activityError, setActivityError] = useState<string | null>(null);
  const [activeSprints, setActiveSprints] = useState<ActiveSprintSummaryVM[]>([]);
  const [activeSprintsLoading, setActiveSprintsLoading] = useState(true);
  const [releasePlans, setReleasePlans] = useState<WorkspaceReleasePlanVM[]>([]);
  const [releasePlansLoading, setReleasePlansLoading] = useState(true);
  const [switchingWorkspace, setSwitchingWorkspace] = useState<string | null>(null);

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
    setActiveSprintsLoading(true);
    setReleasePlansLoading(true);

    void Promise.allSettled([
      platform.listOperationRecords(workspaceId, { page: 1, page_size: 12 }),
      product.listActiveSprints(workspaceId),
      product.listWorkspaceReleasePlans(workspaceId, {
        status: "planning,wait_for_approval,executing",
      }),
    ]).then(([activityResult, sprintsResult, releasePlansResult]) => {
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
    const target = workspaces.find((workspace) => workspace.id === nextWorkspaceId);
    if (!target || isWorkspaceReadOnly(target)) return;
    setSwitchingWorkspace(nextWorkspaceId);
    try {
      await tenant.selectWorkspace(nextWorkspaceId);
      router.refresh();
    } finally {
      setSwitchingWorkspace(null);
    }
  }

  const workLoading = activeSprintsLoading || releasePlansLoading;
  const hasWork = activeProjects.length > 0 || activeSprints.length > 0 || releasePlans.length > 0;
  const changes = activity.filter(isWriteOperation);
  const displayName = session.display_name.trim() || session.email.split("@")[0] || "用户";

  return (
    <div className="workplace-page">
      <header className="workplace-welcome">
        <div>
          <h1 className="workplace-welcome-title">你好，{displayName}</h1>
          <p className="workplace-welcome-meta">
            {workLoading ? "正在汇总当前工作" : hasWork ? "这里是当前进行中的工作。" : "从一个项目开始构建你的工作空间。"}
          </p>
        </div>
        <Link href="/dashboard?create=1" className="workplace-hero-primary">
          创建项目
        </Link>
      </header>

      <div className="workplace-layout">
        <div className="workplace-main">
          <section className="workplace-overview-panel">
            <header className="workplace-panel-head">
              <div>
                <h2 className="workplace-panel-title">工作概览</h2>
                <p className="workplace-panel-description">当前空间的交付进展</p>
              </div>
              <Link href="/projects" className="workplace-panel-link">
                项目中心
              </Link>
            </header>
            <dl className="workplace-keyline">
              <div>
                <dt>项目</dt>
                <dd>{projects.length}</dd>
              </div>
              <div>
                <dt>可发布</dt>
                <dd>{activeProjects.length}</dd>
              </div>
              <div>
                <dt>进行中的迭代</dt>
                <dd>{workLoading ? "—" : activeSprints.length}</dd>
              </div>
              <div>
                <dt>发布计划</dt>
                <dd>{workLoading ? "—" : releasePlans.length}</dd>
              </div>
            </dl>
            {!workLoading && !hasWork ? (
              <p className="workplace-overview-hint">暂无进行中的工作，可以从创建项目开始。</p>
            ) : null}
          </section>

          {hasWork ? (
            <>
              {activeProjects.length > 0 ? (
                <PanelCard
                  title="可发布项目"
                  action={
                    <Link href="/projects" className="workplace-panel-link">
                      查看全部
                    </Link>
                  }
                >
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
                </PanelCard>
              ) : null}

              {activeSprints.length > 0 ? (
                <PanelCard title="进行中的迭代">
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
                </PanelCard>
              ) : null}

              {releasePlans.length > 0 ? (
                <PanelCard
                  title="进行中的发布计划"
                  action={
                    <Link href="/publish/release-plans" className="workplace-panel-link">
                      查看全部
                    </Link>
                  }
                >
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
                </PanelCard>
              ) : null}
            </>
          ) : null}

          {changes.length > 0 ? (
            <PanelCard
              title="最近变更"
              action={
                <Link href="/admin/operation-logs" className="workplace-panel-link">
                  操作日志
                </Link>
              }
            >
              <ul className="workplace-activity-list">
                {changes.map((record) => (
                  <li key={record.id} className="workplace-activity-item">
                    <span className={`workplace-activity-dot is-${operationTone(record)}`} />
                    <div className="workplace-activity-copy">
                      <strong>{operationLabel(record)}</strong>
                      <span>{record.status >= 400 ? "未完成" : "已完成"}</span>
                    </div>
                    <time className="workplace-activity-time">
                      {formatCompactDateTime(record.created_at)}
                    </time>
                  </li>
                ))}
              </ul>
            </PanelCard>
          ) : activityError ? (
            <EmptyHint>{activityError}</EmptyHint>
          ) : null}
        </div>

        <aside className="workplace-aside">
          <section className="workplace-context-panel">
            <div className="workplace-context-section">
              <header className="workplace-context-head">
                <h2>业务空间</h2>
                <span>{workspaces.length} 个</span>
              </header>
              {workspaces.length === 0 ? (
                <EmptyHint>暂无可用工作区</EmptyHint>
              ) : (
                <ul className="workplace-space-grid">
                  {workspaces.map((workspace) => {
                    const active = workspace.id === workspaceId;
                    return (
                      <li key={workspace.id}>
                        <button
                          type="button"
                          className={`workplace-space-item${active ? " is-active" : ""}`}
                          disabled={
                            Boolean(switchingWorkspace) ||
                            (!active && isWorkspaceReadOnly(workspace))
                          }
                          onClick={() => void handleSwitchWorkspace(workspace.id)}
                        >
                          <span className="workplace-space-icon">
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
            </div>

            <div className="workplace-context-section">
              <header className="workplace-context-head">
                <h2>快捷项目</h2>
                <Link href="/projects" className="workplace-panel-link">
                  查看全部
                </Link>
              </header>
              {quickProjects.length === 0 ? (
                <p className="workplace-context-empty">创建项目后，可从这里快速进入。</p>
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
            </div>
          </section>
        </aside>
      </div>
    </div>
  );
}
