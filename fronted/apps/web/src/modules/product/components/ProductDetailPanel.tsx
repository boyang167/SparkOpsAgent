"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductOverviewVM, ProductProjectVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { EnvironmentsPanel } from "./EnvironmentsPanel";
import { ProductMasterDetail } from "./ProductMasterDetail";
import { ProductProjectsPanel } from "./ProductProjectsPanel";
import { ProductWorkflowsPanel } from "./ProductWorkflowsPanel";
import { ReleasePlansPanel } from "./ReleasePlansPanel";
import { SprintBoardPanel } from "./SprintBoardPanel";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { productDetailTabIcons } from "./product-detail-icons";

interface ProductDetailPanelProps {
  workspaceId: string;
  productId: string;
}

type Tab = "overview" | "iterations" | "releases" | "workflows" | "environments" | "projects";

const TABS: Array<{ id: Tab; label: string; icon: ReactNode }> = [
  { id: "overview", label: "概览", icon: productDetailTabIcons.overview },
  { id: "iterations", label: "迭代管理", icon: productDetailTabIcons.iterations },
  { id: "releases", label: "发布计划", icon: productDetailTabIcons.releases },
  { id: "workflows", label: "工作流", icon: productDetailTabIcons.workflow },
  { id: "environments", label: "环境", icon: productDetailTabIcons.environment },
  { id: "projects", label: "项目配置", icon: productDetailTabIcons.projects },
];

const TAB_IDS = new Set<Tab>(["overview", "iterations", "releases", "workflows", "environments", "projects"]);

function parseTab(value: string | null): Tab {
  if (value === "settings" || value === "process") return "iterations";
  if (value && TAB_IDS.has(value as Tab)) {
    return value as Tab;
  }
  return "overview";
}

export function ProductDetailPanel({ workspaceId, productId }: ProductDetailPanelProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [tab, setTab] = useState<Tab>(() => parseTab(searchParams.get("tab")));
  const [overview, setOverview] = useState<ProductOverviewVM | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [overviewData, projectItems] = await Promise.all([
        product.getProductOverview(workspaceId, productId),
        product.listProductProjects(workspaceId, productId),
      ]);
      setOverview(overviewData);
      setProjects(projectItems);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setTab(parseTab(searchParams.get("tab")));
  }, [searchParams]);

  const title = overview?.display_name?.trim() || overview?.name || "产品";

  function selectTab(next: Tab) {
    setTab(next);
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", next);
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }

  if (loading) {
    return (
      <div className="product-detail-shell">
        <div className="product-detail-topbar">
          <div className="product-detail-breadcrumb">
            <span className="product-detail-breadcrumb-current">加载中…</span>
          </div>
        </div>
        <div className="product-detail-body is-overview">
          <div className="empty-state" role="status">
            加载产品详情…
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="product-detail-shell">
      <div className="product-detail-topbar">
        <nav className="product-detail-breadcrumb" aria-label="面包屑">
          <Link href="/products">产品</Link>
          <span className="product-detail-breadcrumb-sep">/</span>
          <span className="product-detail-breadcrumb-current">{title}</span>
        </nav>
      </div>

      <div className="product-detail-tabs" role="tablist" aria-label="产品详情">
        {TABS.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            className={`product-detail-tab${tab === item.id ? " is-active" : ""}`}
            onClick={() => selectTab(item.id)}
          >
            {item.icon}
            {item.label}
          </button>
        ))}
      </div>

      {error ? (
        <div className="product-detail-body is-overview">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      {!error && tab === "overview" && overview ? (
        <div className="product-detail-body is-overview">
          <div className="product-detail-overview-grid">
            <div className="product-detail-stat">
              <div className="product-detail-stat-label">关联项目</div>
              <div className="product-detail-stat-value">{overview.project_count}</div>
            </div>
            <div className="product-detail-stat">
              <div className="product-detail-stat-label">环境</div>
              <div className="product-detail-stat-value">{overview.environment_count ?? 0}</div>
            </div>
          </div>

          <dl className="product-detail-meta">
            <div>
              <dt>产品标识</dt>
              <dd>{overview.name}</dd>
            </div>
            <div>
              <dt>显示名称</dt>
              <dd>{overview.display_name || "-"}</dd>
            </div>
            <div className="sm:col-span-2">
              <dt>描述</dt>
              <dd>{overview.description || "-"}</dd>
            </div>
          </dl>

          {(overview.environment_count ?? 0) > 0 ? (
            <p className="text-sm text-muted-foreground mt-4">
              在「环境」Tab 中查看各环境下 Project 的部署版本矩阵。
            </p>
          ) : null}
        </div>
      ) : null}

      {!error && tab === "iterations" ? (
        <div className="product-detail-body">
          <SprintBoardPanel workspaceId={workspaceId} productId={productId} />
        </div>
      ) : null}

      {!error && tab === "releases" ? (
        <div className="product-detail-body">
          <ReleasePlansPanel workspaceId={workspaceId} productId={productId} />
        </div>
      ) : null}

      {!error && tab === "workflows" ? (
        <div className="product-detail-body is-workflows">
          <ProductWorkflowsPanel workspaceId={workspaceId} productId={productId} projects={projects} />
        </div>
      ) : null}

      {!error && tab === "environments" ? (
        <div className="product-detail-body">
          <EnvironmentsPanel workspaceId={workspaceId} productId={productId} />
        </div>
      ) : null}

      {!error && tab === "projects" ? (
        <div className="product-detail-body">
          <ProductProjectsPanel workspaceId={workspaceId} productId={productId} onChanged={() => void load()} />
        </div>
      ) : null}

    </div>
  );
}
