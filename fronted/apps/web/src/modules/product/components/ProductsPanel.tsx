"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductOverviewVM, ProductVM } from "@sparkops/contracts/product";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { CreateProductDialog } from "./CreateProductDialog";
import { ProductCard } from "./ProductCard";
import {
  formatDateTime,
  labelOf,
  productAccentColor,
  productDisplayName,
  PRODUCT_STATUS_OPTIONS,
  productStatusLabel,
  productStatusTagClass,
} from "./product-utils";

interface ProductsPanelProps {
  workspaceId: string;
  title?: string;
  description?: string;
  linkToDetail?: boolean;
}

type ViewMode = "card" | "table";

const PAGE_SIZES = [6, 9, 12, 24] as const;

function SearchIcon() {
  return (
    <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path
        d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path d="M13.5 13.5 17 17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function ProductsPanel({
  workspaceId,
  title = "产品清单",
  description = "管理工作空间下的产品、环境与发布计划。",
  linkToDetail = false,
}: ProductsPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState<ProductVM[]>([]);
  const [overviews, setOverviews] = useState<Record<string, ProductOverviewVM | null>>({});
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<(typeof PAGE_SIZES)[number]>(9);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listProducts(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载产品失败");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (statusFilter && (item.status ?? "active") !== statusFilter) return false;
      if (!q) return true;
      return [item.name, item.display_name, item.description, item.owner]
        .some((value) => value?.toLowerCase().includes(q));
    });
  }, [items, keyword, statusFilter]);

  const hasActiveFilters = Boolean(keyword.trim() || statusFilter);

  useEffect(() => {
    setPage(1);
  }, [keyword, statusFilter, pageSize, viewMode]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => {
    const current = Math.min(page, totalPages);
    const start = (current - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, page, pageSize, totalPages]);

  const pagedIds = paged.map((item) => item.id).join(",");

  useEffect(() => {
    if (!pagedIds) {
      setOverviews({});
      return;
    }
    let cancelled = false;
    setOverviewLoading(true);
    void Promise.all(
      pagedIds.split(",").map(async (id) => {
        try {
          const overview = await product.getProductOverview(workspaceId, id);
          return [id, overview] as const;
        } catch {
          return [id, null] as const;
        }
      }),
    ).then((entries) => {
      if (cancelled) return;
      setOverviews(Object.fromEntries(entries));
      setOverviewLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [pagedIds, workspaceId]);

  function resetFilters() {
    setKeyword("");
    setStatusFilter("");
  }

  function productHref(productId: string) {
    return `/products/${productId}`;
  }

  async function onCreated(productId: string) {
    setCreateOpen(false);
    await load();
    if (linkToDetail && productId) {
      router.push(`/products/${productId}`);
    }
  }

  return (
    <div className="projects-page">
      <section className="platform-table-card projects-card">
        <PlatformPanelHead
          title={title}
          description={description}
          action={
            <button type="button" className="btn platform-add-btn" onClick={() => setCreateOpen(true)}>
              <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M10 4v12M4 10h12" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
              </svg>
              新建产品
            </button>
          }
        />

        <div className="projects-toolbar">
          <div className="projects-toolbar-filters">
            <label className="projects-search-wrap">
              <SearchIcon />
              <input
                className="projects-search-input"
                placeholder="搜索产品名称、标识或描述"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </label>
            <select
              className="projects-filter-select"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              aria-label="状态筛选"
            >
              <option value="">全部状态</option>
              {PRODUCT_STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            {hasActiveFilters ? (
              <button type="button" className="btn-secondary projects-reset-btn" onClick={resetFilters}>
                重置
              </button>
            ) : null}
          </div>

          <div className="projects-toolbar-actions">
            <span className="projects-count">{filtered.length} 个产品</span>
            <div className="projects-view-toggle" role="group" aria-label="视图切换">
              <button
                type="button"
                className={viewMode === "card" ? "is-active" : ""}
                onClick={() => setViewMode("card")}
              >
                卡片
              </button>
              <button
                type="button"
                className={viewMode === "table" ? "is-active" : ""}
                onClick={() => setViewMode("table")}
              >
                表格
              </button>
            </div>
            <button
              type="button"
              className="btn-secondary projects-refresh-btn"
              onClick={() => void load()}
              disabled={loading}
              aria-label="刷新"
            >
              ↻
            </button>
          </div>
        </div>

        {error ? (
          <div className="projects-alert">
            <InlineAlert tone="error">{error}</InlineAlert>
          </div>
        ) : null}

        <div className="projects-body">
          {loading ? (
            <div className="projects-state">加载中…</div>
          ) : filtered.length === 0 ? (
            <div className="projects-state">
              <p>{hasActiveFilters ? "没有符合筛选条件的产品" : "暂无产品"}</p>
              {!hasActiveFilters ? (
                <button type="button" className="btn platform-add-btn" onClick={() => setCreateOpen(true)}>
                  新建第一个产品
                </button>
              ) : (
                <button type="button" className="btn-secondary" onClick={resetFilters}>
                  清除筛选
                </button>
              )}
            </div>
          ) : (
            <>
              {viewMode === "card" ? (
                <div className="project-card-grid">
                  {paged.map((item) => (
                    <ProductCard
                      key={item.id}
                      product={item}
                      overview={overviews[item.id]}
                      overviewLoading={overviewLoading}
                      detailsHref={productHref(item.id)}
                    />
                  ))}
                </div>
              ) : (
                <div className="project-table-shell">
                  <table className="data-table project-table">
                    <thead>
                      <tr>
                        <th>产品名称</th>
                        <th>标识</th>
                        <th>状态</th>
                        <th>负责人</th>
                        <th>关联项目</th>
                        <th>创建时间</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((item) => {
                        const overview = overviews[item.id];
                        const accent = productAccentColor(item.name);
                        return (
                          <tr key={item.id}>
                            <td>
                              <Link href={productHref(item.id)} className="project-table-name project-table-name-link">
                                <span
                                  className="project-lang-icon is-sm"
                                  style={{ backgroundColor: accent }}
                                >
                                  {productDisplayName(item).charAt(0).toUpperCase() || "P"}
                                </span>
                                <div>
                                  <div className="font-medium">{productDisplayName(item)}</div>
                                  <div className="text-xs text-muted-foreground">{item.description || "暂无描述"}</div>
                                </div>
                              </Link>
                            </td>
                            <td className="font-mono text-xs">{item.name}</td>
                            <td>
                              <span className={productStatusTagClass(item.status)}>
                                {productStatusLabel(item.status)}
                              </span>
                            </td>
                            <td>{item.owner ?? "-"}</td>
                            <td>{overviewLoading ? "—" : overview?.project_count ?? 0}</td>
                            <td>{formatDateTime(item.created_at)}</td>
                            <td>
                              <div className="project-row-actions">
                                <Link href={productHref(item.id)}>进入产品</Link>
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}

              <div className="project-pagination">
                <span className="project-pagination-total">共 {filtered.length} 个产品</span>
                <select
                  className="input project-page-size"
                  value={pageSize}
                  onChange={(e) => setPageSize(Number(e.target.value) as (typeof PAGE_SIZES)[number])}
                >
                  {PAGE_SIZES.map((size) => (
                    <option key={size} value={size}>
                      {size} 条/页
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  className="btn-secondary text-sm"
                  disabled={page <= 1}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  上一页
                </button>
                <span className="project-page-indicator">
                  {Math.min(page, totalPages)} / {totalPages}
                </span>
                <button
                  type="button"
                  className="btn-secondary text-sm"
                  disabled={page >= totalPages}
                  onClick={() => setPage((current) => current + 1)}
                >
                  下一页
                </button>
              </div>
            </>
          )}
        </div>
      </section>

      <CreateProductDialog
        open={createOpen}
        workspaceId={workspaceId}
        onClose={() => setCreateOpen(false)}
        onCreated={(productId) => void onCreated(productId)}
      />
    </div>
  );
}
