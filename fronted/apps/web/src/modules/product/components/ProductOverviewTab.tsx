"use client";

import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";
import { product } from "@sparkops/web-sdk";
import {
  environmentTypeLabel,
  type EnvironmentVM,
  type ProductOverviewVM,
  type ProductProjectVM,
  type ProductVM,
  type ReleasePlanVM,
  type SprintSummaryVM,
} from "@sparkops/contracts/product";
import { formatDateTime, productStatusLabel } from "./product-utils";
import { releasePlanStatusLabel } from "./release-plan-utils";

type OverviewTarget = "iterations" | "releases" | "projects" | "environments";

interface ProductOverviewTabProps {
  workspaceId: string;
  productId: string;
  overview: ProductOverviewVM;
  projects: ProductProjectVM[];
  onNavigate: (tab: OverviewTarget) => void;
}

interface OverviewExtras {
  loaded: boolean;
  info: ProductVM | null;
  environments: EnvironmentVM[] | null;
  plans: ReleasePlanVM[] | null;
  /** undefined = request failed, null = no active sprint */
  sprint: SprintSummaryVM | null | undefined;
}

const RECENT_PLAN_LIMIT = 3;
const LIST_LIMIT = 6;

function settled<T>(result: PromiseSettledResult<T>): T | null {
  return result.status === "fulfilled" ? result.value : null;
}

function formatDate(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleDateString("zh-CN", { month: "numeric", day: "numeric" });
}

function formatWindow(start?: string, end?: string) {
  const from = formatDate(start);
  const to = formatDate(end);
  if (from && to) return `${from} – ${to}`;
  return from || to || "";
}

function planTone(status: string) {
  if (status === "success") return "success";
  if (status === "executing") return "running";
  if (status === "wait_for_approval") return "warning";
  if (status === "denied" || status === "cancel" || status.endsWith("_failed")) return "danger";
  return "neutral";
}

function Section({
  title,
  count,
  onMore,
  children,
}: {
  title: string;
  count?: number;
  onMore: () => void;
  children: ReactNode;
}) {
  return (
    <section className="pov-section">
      <header className="pov-section-head">
        <h3 className="pov-section-title">
          {title}
          {count !== undefined ? <span className="pov-section-count">{count}</span> : null}
        </h3>
        <button type="button" className="pov-section-more" onClick={onMore}>
          查看全部
        </button>
      </header>
      {children}
    </section>
  );
}

function Empty({ text }: { text: string }) {
  return <p className="pov-empty">{text}</p>;
}

export function ProductOverviewTab({ workspaceId, productId, overview, projects, onNavigate }: ProductOverviewTabProps) {
  const [extras, setExtras] = useState<OverviewExtras>({
    loaded: false,
    info: null,
    environments: null,
    plans: null,
    sprint: null,
  });

  useEffect(() => {
    let cancelled = false;
    void Promise.allSettled([
      product.getProduct(workspaceId, productId),
      product.listEnvironments(workspaceId, productId),
      product.listReleasePlans(workspaceId, productId),
      product.listSprints(workspaceId, productId, "active"),
    ]).then(([info, environments, plans, sprints]) => {
      if (cancelled) return;
      const sprintItems = settled(sprints);
      setExtras({
        loaded: true,
        info: settled(info),
        environments: settled(environments),
        plans: settled(plans),
        sprint: sprintItems ? (sprintItems[0] ?? null) : undefined,
      });
    });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, productId]);

  const { loaded, info, environments, plans, sprint } = extras;
  const pendingText = loaded ? "加载失败" : "加载中…";
  const title = overview.display_name?.trim() || overview.name;
  const description = overview.description?.trim();
  const showDescription = Boolean(description) && description !== overview.name;
  const inactive = (overview.status ?? "active").toLowerCase() === "inactive";

  const recentPlans = (plans ?? [])
    .slice()
    .sort((a, b) => (b.created_at ?? "").localeCompare(a.created_at ?? ""))
    .slice(0, RECENT_PLAN_LIMIT);

  const sprintTotal = sprint?.work_item_count ?? 0;
  const sprintDone = sprint?.done_work_item_count ?? 0;
  const sprintPercent = sprintTotal > 0 ? Math.round((sprintDone / sprintTotal) * 100) : 0;

  const facts: Array<{ label: string; value: string; muted?: boolean }> = [
    { label: "当前版本", value: info?.version?.trim() || "—", muted: !info?.version?.trim() },
    { label: "下一版本", value: info?.next_version?.trim() || "—", muted: !info?.next_version?.trim() },
    { label: "项目", value: String(overview.project_count) },
    { label: "环境", value: String(overview.environment_count ?? environments?.length ?? 0) },
  ];

  return (
    <div className="pov">
      <header className="pov-hero">
        <div className="pov-hero-title-row">
          <h2 className="pov-hero-title">{title}</h2>
          <span className={`pov-status${inactive ? " is-inactive" : ""}`}>{productStatusLabel(overview.status)}</span>
        </div>
        <p className="pov-hero-meta">
          {title !== overview.name ? <span>{overview.name}</span> : null}
          <span>{info?.owner?.trim() || "未指定负责人"}</span>
          {info?.created_at ? <span>创建于 {formatDateTime(info.created_at)}</span> : null}
        </p>
        {showDescription ? <p className="pov-hero-desc">{description}</p> : null}
        <dl className="pov-facts">
          {facts.map((fact) => (
            <div key={fact.label} className="pov-fact">
              <dt>{fact.label}</dt>
              <dd className={fact.muted ? "is-muted" : undefined}>{fact.value}</dd>
            </div>
          ))}
        </dl>
      </header>

      <div className="pov-grid">
        <Section title="进行中的迭代" onMore={() => onNavigate("iterations")}>
          {!loaded || sprint === undefined ? (
            <Empty text={pendingText} />
          ) : sprint ? (
            <button type="button" className="pov-sprint" onClick={() => onNavigate("iterations")}>
              <span className="pov-sprint-head">
                <strong>{sprint.name}</strong>
                <span className="pov-sprint-window">{formatWindow(sprint.planned_start, sprint.planned_end)}</span>
              </span>
              {sprint.goal?.trim() ? <span className="pov-sprint-goal">{sprint.goal}</span> : null}
              <span className="pov-progress" aria-label={`完成 ${sprintPercent}%`}>
                <span className="pov-progress-bar" style={{ width: `${sprintPercent}%` }} />
              </span>
              <span className="pov-sprint-foot">
                <span>
                  已完成 {sprintDone} / {sprintTotal}
                </span>
                <span>{sprintPercent}%</span>
              </span>
            </button>
          ) : (
            <Empty text="暂无进行中的迭代" />
          )}
        </Section>

        <Section title="最近发布计划" count={plans?.length} onMore={() => onNavigate("releases")}>
          {plans === null ? (
            <Empty text={pendingText} />
          ) : recentPlans.length === 0 ? (
            <Empty text="还没有发布计划" />
          ) : (
            <ul className="pov-list">
              {recentPlans.map((plan) => (
                <li key={plan.id}>
                  <button type="button" className="pov-row" onClick={() => onNavigate("releases")}>
                    <span className={`pov-dot is-${planTone(plan.status)}`} aria-hidden />
                    <span className="pov-row-main">{plan.name}</span>
                    <span className="pov-row-meta">{releasePlanStatusLabel(plan.status)}</span>
                    <span className="pov-row-meta">{formatWindow(plan.release_window_start, plan.release_window_end)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Section>

        <Section title="关联项目" count={projects.length} onMore={() => onNavigate("projects")}>
          {projects.length === 0 ? (
            <Empty text="还没有关联项目" />
          ) : (
            <ul className="pov-list">
              {projects.slice(0, LIST_LIMIT).map((item) => {
                const name = item.display_name?.trim() || item.name;
                return (
                  <li key={item.id}>
                    <Link href={`/projects/${item.id}`} className="pov-row">
                      <span className="pov-row-main">{name}</span>
                      {name !== item.name ? <span className="pov-row-meta is-mono">{item.name}</span> : null}
                    </Link>
                  </li>
                );
              })}
            </ul>
          )}
        </Section>

        <Section title="环境" count={environments?.length} onMore={() => onNavigate("environments")}>
          {environments === null ? (
            <Empty text={pendingText} />
          ) : environments.length === 0 ? (
            <Empty text="还没有环境" />
          ) : (
            <ul className="pov-list">
              {environments.slice(0, LIST_LIMIT).map((env) => (
                <li key={env.id}>
                  <button type="button" className="pov-row" onClick={() => onNavigate("environments")}>
                    <span className="pov-row-main">{env.name}</span>
                    <span className="pov-row-meta">{environmentTypeLabel(env.env_type)}</span>
                    <span className="pov-row-meta is-mono">{env.namespace}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Section>
      </div>
    </div>
  );
}
