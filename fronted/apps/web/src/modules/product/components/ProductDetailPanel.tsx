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
import { ProductVersionsPanel } from "./ProductVersionsPanel";
import { ProductWorkflowsPanel } from "./ProductWorkflowsPanel";
import { ReleasePlansPanel } from "./ReleasePlansPanel";
import { SprintBoardPanel } from "./SprintBoardPanel";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductOverviewTab } from "./ProductOverviewTab";
import { productDetailTabIcons } from "./product-detail-icons";

interface ProductDetailPanelProps {
  workspaceId: string;
  productId: string;
}

type Tab = "overview" | "iterations" | "releases" | "workflows" | "environments" | "projects" | "versions";

const TABS: Array<{ id: Tab; label: string; icon: ReactNode }> = [
  { id: "overview", label: "概览", icon: productDetailTabIcons.overview },
  { id: "iterations", label: "迭代管理", icon: productDetailTabIcons.iterations },
  { id: "releases", label: "发布计划", icon: productDetailTabIcons.releases },
  { id: "workflows", label: "工作流", icon: productDetailTabIcons.workflow },
  { id: "environments", label: "环境", icon: productDetailTabIcons.environment },
  { id: "projects", label: "项目配置", icon: productDetailTabIcons.projects },
  { id: "versions", label: "版本管理", icon: productDetailTabIcons.versions },
];

const TAB_IDS = new Set<Tab>([
  "overview",
  "iterations",
  "releases",
  "workflows",
  "environments",
  "projects",
  "versions",
]);

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
          <ProductOverviewTab
            workspaceId={workspaceId}
            productId={productId}
            overview={overview}
            projects={projects}
            onNavigate={selectTab}
          />
        </div>
      ) : null}

      {!error && tab === "iterations" ? (
        <div className="product-detail-body">
          <SprintBoardPanel workspaceId={workspaceId} productId={productId} />
        </div>
      ) : null}

      {!error && tab === "releases" ? (
        <div className="product-detail-body">
          <ReleasePlansPanel
            workspaceId={workspaceId}
            productId={productId}
            initialPlanId={searchParams.get("planId")}
          />
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

      {!error && tab === "versions" ? (
        <div className="product-detail-body is-versions">
          <ProductVersionsPanel workspaceId={workspaceId} productId={productId} projects={projects} />
        </div>
      ) : null}

    </div>
  );
}
