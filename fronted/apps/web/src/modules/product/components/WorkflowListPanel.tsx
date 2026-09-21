"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ProductProjectVM,
  ProductWorkflowRunSummaryVM,
  ProductWorkflowVM,
  WorkspaceProductWorkflowVM,
  WorkflowTemplateVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import {
  computeRunStats,
  formatDurationMs,
  formatLastRun,
  paginationPages,
  readWorkflowFavorites,
  stagePathLabels,
  successRateTone,
  writeWorkflowFavorites,
  type WorkflowRunStats,
} from "./workflow-list-utils";
import { IconPlay, IconSearch, IconSettings, IconSort, IconStar } from "./WorkflowListIcons";
import { WorkflowRowMenu } from "./WorkflowRowMenu";
import { WorkflowListSkeletonRows, WorkflowSparkline, WorkflowStagePath } from "./WorkflowStagePath";

const PAGE_SIZES = [10, 20, 50] as const;

type SortKey = "default" | "name" | "success" | "last_run";

interface WorkflowListPanelProps {
  workspaceId: string;
  productFilter: string | null;
  products: Array<{ id: string; name: string; display_name?: string | null }>;
  projectsByProduct: Record<string, ProductProjectVM[]>;
  initialWorkflowId?: string | null;
  createRequestId?: number;
}

interface EnrichedWorkflow {
  wf: WorkspaceProductWorkflowVM;
  detail: ProductWorkflowVM | null;
  stats: WorkflowRunStats;
  stageLabels: string[];
}

function displayProductName(item: { name: string; display_name?: string | null }) {
  return item.display_name?.trim() || item.name;
}

export function WorkflowListPanel({
  workspaceId,
  productFilter,
  products,
  projectsByProduct,
  initialWorkflowId,
  createRequestId,
}: WorkflowListPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState<WorkspaceProductWorkflowVM[]>([]);
  const [enriched, setEnriched] = useState<Record<string, EnrichedWorkflow>>({});
  const [favorites, setFavorites] = useState<Set<string>>(() => readWorkflowFavorites());
  const [templates, setTemplates] = useState<WorkflowTemplateVM[]>([]);
  const [search, setSearch] = useState("");
  const [favoritesOnly, setFavoritesOnly] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>("default");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<(typeof PAGE_SIZES)[number]>(20);
  const [loading, setLoading] = useState(true);
  const [enriching, setEnriching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [menuOpenId, setMenuOpenId] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [createMode, setCreateMode] = useState<"template" | "blank">("blank");
  const [createProductId, setCreateProductId] = useState("");
  const [createName, setCreateName] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [workflows, tplItems] = await Promise.all([
        product.listWorkspaceWorkflows(workspaceId),
        product.listWorkflowTemplates(workspaceId),
      ]);
      setItems(workflows);
      setTemplates(tplItems);
      if (tplItems[0]) setTemplateId((prev) => prev || tplItems[0].id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作流失败");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setFavorites(readWorkflowFavorites());
  }, []);

  useEffect(() => {
    setPage(1);
  }, [productFilter, search, favoritesOnly, sortKey, pageSize]);

  useEffect(() => {
    if (!menuOpenId) return;
    const onDocClick = () => setMenuOpenId(null);
    document.addEventListener("click", onDocClick);
    return () => document.removeEventListener("click", onDocClick);
  }, [menuOpenId]);

  useEffect(() => {
    if (!createRequestId) return;
    setCreateProductId(productFilter ?? products[0]?.id ?? "");
    setCreateMode("blank");
    setCreateOpen(true);
  }, [createRequestId, productFilter, products]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter((item) => {
      if (productFilter && item.product_id !== productFilter) return false;
      if (favoritesOnly && !favorites.has(item.id)) return false;
      if (!q) return true;
      return (
        item.name.toLowerCase().includes(q) ||
        item.key.toLowerCase().includes(q) ||
        (item.product_name ?? "").toLowerCase().includes(q)
      );
    });
  }, [items, productFilter, search, favoritesOnly, favorites]);

  const sorted = useMemo(() => {
    const list = [...filtered];
    list.sort((a, b) => {
      const af = favorites.has(a.id) ? 1 : 0;
      const bf = favorites.has(b.id) ? 1 : 0;
      if (sortKey === "default" && af !== bf) return bf - af;

      const ea = enriched[a.id];
      const eb = enriched[b.id];

      switch (sortKey) {
        case "name":
          return a.name.localeCompare(b.name, "zh-CN");
        case "success": {
          const ar = ea?.stats.successRate ?? -1;
          const br = eb?.stats.successRate ?? -1;
          return br - ar;
        }
        case "last_run": {
          const at = ea?.stats.lastRunAt ? Date.parse(ea.stats.lastRunAt) : 0;
          const bt = eb?.stats.lastRunAt ? Date.parse(eb.stats.lastRunAt) : 0;
          return bt - at;
        }
        default:
          return a.name.localeCompare(b.name, "zh-CN");
      }
    });
    return list;
  }, [filtered, sortKey, favorites, enriched]);

  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const paged = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return sorted.slice(start, start + pageSize);
  }, [sorted, currentPage, pageSize]);

  // 排序依赖 enriched，不能把 paged 数组本身放进 effect 依赖，否则 setEnriched 会反复触发。
  const pageKey = paged.map((wf) => wf.id).join("|");
  const pagedRef = useRef(paged);
  pagedRef.current = paged;

  useEffect(() => {
    const targets = pagedRef.current;
    if (targets.length === 0) {
      setEnriched((prev) => (Object.keys(prev).length === 0 ? prev : {}));
      setEnriching(false);
      return;
    }
    let cancelled = false;
    setEnriching(true);
    void (async () => {
      const next: Record<string, EnrichedWorkflow> = {};
      await Promise.all(
        targets.map(async (wf) => {
          try {
            const [detail, runs] = await Promise.all([
              product.getProductWorkflow(workspaceId, wf.product_id, wf.id),
              product.listProductWorkflowRuns(workspaceId, wf.product_id, wf.id),
            ]);
            next[wf.id] = {
              wf,
              detail,
              stats: computeRunStats(runs as ProductWorkflowRunSummaryVM[]),
              stageLabels: stagePathLabels(detail.stages),
            };
          } catch {
            next[wf.id] = {
              wf,
              detail: null,
              stats: computeRunStats([]),
              stageLabels: [],
            };
          }
        }),
      );
      if (!cancelled) {
        setEnriched((prev) => ({ ...prev, ...next }));
        setEnriching(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [pageKey, workspaceId]);

  function toggleFavorite(id: string) {
    setFavorites((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      writeWorkflowFavorites(next);
      return next;
    });
  }

  function openWorkflowDetail(wf: WorkspaceProductWorkflowVM, run = false) {
    const suffix = run ? "?run=1" : "";
    router.push(`/products/${wf.product_id}/workflows/${wf.id}${suffix}`);
  }

  useEffect(() => {
    if (!initialWorkflowId || loading || items.length === 0) return;
    const wf = items.find((item) => item.id === initialWorkflowId);
    if (!wf) return;
    router.replace(`/products/${wf.product_id}/workflows/${wf.id}`);
  }, [initialWorkflowId, loading, items, router]);

  async function onCreate() {
    const productId = createProductId || productFilter || products[0]?.id;
    if (!productId) return;
    setCreating(true);
    setError(null);
    try {
      if (createMode === "blank") {
        const created = await product.createBlankProductWorkflow(workspaceId, productId, {
          name: createName.trim() || "未命名工作流",
          activate: false,
        });
        setCreateOpen(false);
        router.push(`/products/${productId}/workflows/${created.id}/edit`);
        return;
      }
      if (!templateId || selectedProjectIds.length === 0) return;
      const created = await product.createProductWorkflowFromTemplate(workspaceId, productId, {
        template_id: templateId,
        name: createName.trim() || undefined,
        project_ids: selectedProjectIds,
      });
      setCreateOpen(false);
      router.push(`/products/${productId}/workflows/${created.id}/edit`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作流失败");
    } finally {
      setCreating(false);
    }
  }

  const createProjects = projectsByProduct[createProductId || productFilter || products[0]?.id || ""] ?? [];
  const pageNumbers = useMemo(() => paginationPages(currentPage, totalPages), [currentPage, totalPages]);

  function openCreateDialog() {
    setCreateProductId(productFilter ?? products[0]?.id ?? "");
    setCreateMode("blank");
    setCreateOpen(true);
  }

  return (
    <>
      <div className="wf-list-shell">
        <div className="wf-list-toolbar">
          <div className="wf-list-toolbar-left">
            <button
              type="button"
              className={`wf-list-icon-btn${favoritesOnly ? " is-active" : ""}`}
              aria-pressed={favoritesOnly}
              title={favoritesOnly ? "显示全部" : "只看收藏"}
              onClick={() => setFavoritesOnly((v) => !v)}
            >
              <IconStar filled={favoritesOnly} />
            </button>
            <label className="wf-list-sort-wrap" title="排序方式">
              <span className="wf-list-sort-icon" aria-hidden="true">
                <IconSort />
              </span>
              <select
                className="input wf-list-sort"
                value={sortKey}
                onChange={(e) => setSortKey(e.target.value as SortKey)}
                aria-label="排序"
              >
                <option value="default">默认排序</option>
                <option value="name">按名称</option>
                <option value="success">按成功率</option>
                <option value="last_run">按最近执行</option>
              </select>
            </label>
          </div>
          <div className="wf-list-toolbar-right">
            <label className="wf-list-search-wrap">
              <span className="wf-list-search-icon" aria-hidden="true">
                <IconSearch />
              </span>
              <input
                className="input wf-list-search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="搜索工作流"
              />
            </label>
          </div>
        </div>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {loading ? (
          <div className="wf-list-panel">
            <WorkflowListSkeletonRows count={6} />
          </div>
        ) : sorted.length === 0 ? (
          <ProductDetailEmptyState
            title="暂无工作流"
            description={productFilter ? "当前产品下还没有工作流，点击新建开始编排。" : "工作区里还没有工作流，先选择产品并创建。"}
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={openCreateDialog}>
                新建工作流
              </button>
            }
          />
        ) : (
          <>
            <div className={`wf-list-panel${enriching ? " is-enriching" : ""}`}>
              <div className="wf-list-columns" aria-hidden="true">
                <span className="wf-list-col-fav" />
                <span className="wf-list-col-name">工作流</span>
                <span className="wf-list-col-metric">平均执行时间</span>
                <span className="wf-list-col-metric">成功率</span>
                <span className="wf-list-col-metric">最近执行</span>
                <span className="wf-list-col-actions">操作</span>
              </div>

              <div className="wf-list-rows">
                {paged.map((wf) => {
                  const row = enriched[wf.id];
                  const loadingRow = !row;
                  const stats = row?.stats;
                  const stageLabels =
                    row?.stageLabels ??
                    (wf.stage_count > 0 ? Array.from({ length: wf.stage_count }, (_, i) => `阶段${i + 1}`) : []);
                  const sparkline = stats?.sparkline ?? Array.from({ length: 20 }, () => "empty" as const);
                  const highlighted = initialWorkflowId === wf.id;
                  const rateTone = successRateTone(stats?.successRate ?? null);

                  return (
                    <article
                      key={wf.id}
                      className={`wf-list-row${highlighted ? " is-highlight" : ""}${wf.status !== "active" ? " is-draft" : ""}${loadingRow ? " is-loading-row" : ""}`}
                    >
                      <div className="wf-list-row-main">
                        <div className="wf-list-row-head">
                          <WorkflowStagePath
                            labels={stageLabels}
                            fallback={wf.stage_count > 0 ? `${wf.stage_count} 个阶段` : "未配置阶段"}
                          />
                          <WorkflowSparkline tones={sparkline} />
                        </div>

                        <div className="wf-list-row-body">
                          <button
                            type="button"
                            className={`wf-list-fav${favorites.has(wf.id) ? " is-on" : ""}`}
                            aria-label={favorites.has(wf.id) ? "取消收藏" : "收藏"}
                            onClick={() => toggleFavorite(wf.id)}
                          >
                            <IconStar filled={favorites.has(wf.id)} />
                          </button>

                          <div className="wf-list-title-wrap">
                            <div className="wf-list-title-row">
                              <Link
                                href={`/products/${wf.product_id}/workflows/${wf.id}`}
                                className="wf-list-title"
                              >
                                {wf.name}
                              </Link>
                              {wf.status !== "active" ? <span className="wf-list-badge">草稿</span> : null}
                            </div>
                            {!productFilter && wf.product_name ? (
                              <span className="wf-list-product">{wf.product_name}</span>
                            ) : null}
                          </div>

                          <div className="wf-list-metric wf-list-col-metric">
                            <span className="wf-list-metric-mobile-label">平均执行时间</span>
                            <span className="wf-list-metric-value">
                              {loadingRow ? "…" : formatDurationMs(stats?.avgDurationMs ?? null)}
                            </span>
                          </div>
                          <div className="wf-list-metric wf-list-col-metric">
                            <span className="wf-list-metric-mobile-label">成功率</span>
                            <span className={`wf-list-metric-value wf-list-rate${rateTone ? ` ${rateTone}` : ""}`}>
                              {loadingRow ? "…" : stats?.successRate != null ? `${stats.successRate.toFixed(2)}%` : "—"}
                            </span>
                          </div>
                          <div className="wf-list-metric wf-list-col-metric">
                            <span className="wf-list-metric-mobile-label">最近执行</span>
                            <span className="wf-list-metric-value">
                              {loadingRow ? "…" : formatLastRun(stats?.lastRunAt ?? null)}
                            </span>
                          </div>

                          <div className="wf-list-actions">
                            <button
                              type="button"
                              className="wf-list-run-btn"
                              disabled={wf.status !== "active"}
                              onClick={() => openWorkflowDetail(wf, true)}
                            >
                              <IconPlay />
                              执行
                            </button>
                            <div className="wf-list-action-icons">
                              <Link
                                href={`/products/${wf.product_id}/workflows/${wf.id}/edit`}
                                className="wf-list-icon-btn"
                                title="编排设置"
                              >
                                <IconSettings />
                              </Link>
                              <WorkflowRowMenu
                                open={menuOpenId === wf.id}
                                detailHref={`/products/${wf.product_id}/workflows/${wf.id}`}
                                editHref={`/products/${wf.product_id}/workflows/${wf.id}/edit`}
                                productHref={`/products/${wf.product_id}`}
                                runDisabled={wf.status !== "active"}
                                onToggle={() => setMenuOpenId((prev) => (prev === wf.id ? null : wf.id))}
                                onClose={() => setMenuOpenId(null)}
                                onRun={() => openWorkflowDetail(wf, true)}
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    </article>
                  );
                })}
              </div>
            </div>

            <div className="wf-list-pagination">
              <span className="wf-list-pagination-total">共 {sorted.length} 条</span>
              <select
                className="input wf-list-page-size"
                value={pageSize}
                onChange={(e) => setPageSize(Number(e.target.value) as (typeof PAGE_SIZES)[number])}
              >
                {PAGE_SIZES.map((size) => (
                  <option key={size} value={size}>
                    {size} 条/页
                  </option>
                ))}
              </select>
              <div className="wf-list-page-nav">
                <button
                  type="button"
                  className="wf-list-page-btn"
                  disabled={currentPage <= 1}
                  aria-label="上一页"
                  onClick={() => setPage((v) => Math.max(1, v - 1))}
                >
                  ‹
                </button>
                {pageNumbers.map((num, index) => {
                  const prev = pageNumbers[index - 1];
                  const gap = prev != null && num - prev > 1;
                  return (
                    <span key={num} className="wf-list-page-slot">
                      {gap ? <span className="wf-list-page-ellipsis">…</span> : null}
                      <button
                        type="button"
                        className={`wf-list-page-btn${num === currentPage ? " is-active" : ""}`}
                        onClick={() => setPage(num)}
                      >
                        {num}
                      </button>
                    </span>
                  );
                })}
                <button
                  type="button"
                  className="wf-list-page-btn"
                  disabled={currentPage >= totalPages}
                  aria-label="下一页"
                  onClick={() => setPage((v) => v + 1)}
                >
                  ›
                </button>
              </div>
            </div>
          </>
        )}
      </div>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title={createMode === "blank" ? "空白创建工作流" : "从模板创建工作流"}
        description="创建后进入可视化编排编辑器。"
        size="lg"
        footer={
          <>
            <button
              type="button"
              className="btn btn-ghost text-sm"
              onClick={() => setCreateMode(createMode === "blank" ? "template" : "blank")}
            >
              {createMode === "blank" ? "改用模板" : "改用空白"}
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={
                creating ||
                !(createProductId || productFilter || products[0]?.id) ||
                (createMode === "template" && (!templateId || selectedProjectIds.length === 0)) ||
                false
              }
              onClick={() => void onCreate()}
            >
              {creating ? "创建中…" : "创建并编辑"}
            </button>
          </>
        }
      >
        <div className="stack-sm text-sm">
          {!productFilter ? (
            <label className="stack-xs">
              <span className="text-muted">所属产品</span>
              <select
                className="input"
                value={createProductId}
                onChange={(e) => {
                  setCreateProductId(e.target.value);
                  setSelectedProjectIds([]);
                }}
              >
                {products.map((item) => (
                  <option key={item.id} value={item.id}>
                    {displayProductName(item)}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          <label className="stack-xs">
            <span className="text-muted">名称</span>
            <input
              className="input"
              value={createName}
              onChange={(e) => setCreateName(e.target.value)}
              placeholder="例如：集成验证"
            />
          </label>
          {createMode === "template" ? (
            <>
              <label className="stack-xs">
                <span className="text-muted">工作流模板</span>
                <select className="input" value={templateId} onChange={(e) => setTemplateId(e.target.value)}>
                  {templates.map((tpl) => (
                    <option key={tpl.id} value={tpl.id}>
                      {tpl.name}
                    </option>
                  ))}
                </select>
              </label>
              <div className="stack-xs">
                <span className="text-muted">关联项目（至少 1 个）</span>
                {createProjects.length === 0 ? (
                  <p className="text-muted">暂无关联项目。</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {createProjects.map((p) => (
                      <label key={p.id} className="inline-flex items-center gap-1">
                        <input
                          type="checkbox"
                          checked={selectedProjectIds.includes(p.id)}
                          onChange={() =>
                            setSelectedProjectIds((prev) =>
                              prev.includes(p.id) ? prev.filter((id) => id !== p.id) : [...prev, p.id],
                            )
                          }
                        />
                        {p.display_name || p.name}
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : null}
        </div>
      </Modal>
    </>
  );
}
