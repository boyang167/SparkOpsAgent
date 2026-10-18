"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductVM, WorkspaceReleasePlanVM } from "@sparkops/contracts/product";
import { MetricCard } from "@/components/ui/MetricCard";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ReleasePlanCalendar } from "./ReleasePlanCalendar";
import { ReleasePlanTemplatesPanel } from "./ReleasePlanTemplatesPanel";
import { releasePlanStatusBadgeClass, releasePlanStatusLabel } from "./release-plan-utils";

interface WorkspaceReleasePlansPanelProps {
  workspaceId: string;
  onRequestCreate?: (defaultProductId?: string) => void;
  onPlanCreated?: (productId: string, planId: string) => void;
  onTemplatesLoaded?: (count: number) => void;
  templateCreateOpen?: boolean;
  onTemplateCreateOpenChange?: (open: boolean) => void;
}

function StatIcon({ children }: { children: React.ReactNode }) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
      {children}
    </svg>
  );
}

export function WorkspaceReleasePlansPanel({
  workspaceId,
  onRequestCreate,
  onPlanCreated,
  onTemplatesLoaded,
  templateCreateOpen,
  onTemplateCreateOpenChange,
}: WorkspaceReleasePlansPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState<WorkspaceReleasePlanVM[]>([]);
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [view, setView] = useState<"list" | "calendar">("list");
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [productFilter, setProductFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [plans, productList] = await Promise.all([
        product.listWorkspaceReleasePlans(workspaceId, statusFilter ? { status: statusFilter } : undefined),
        product.listProducts(workspaceId),
      ]);
      setItems(plans);
      setProducts(productList);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载发布计划失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, statusFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const productOptions = useMemo(() => {
    const map = new Map<string, string>();
    for (const p of products) {
      map.set(p.id, p.display_name || p.name);
    }
    for (const item of items) {
      if (!map.has(item.product_id)) {
        map.set(item.product_id, item.product_name ?? item.product_id);
      }
    }
    return [...map.entries()].sort((a, b) => a[1].localeCompare(b[1], "zh-CN"));
  }, [products, items]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((item) => {
      if (productFilter && item.product_id !== productFilter) return false;
      if (!q) return true;
      return (
        item.name.toLowerCase().includes(q) ||
        (item.product_name ?? "").toLowerCase().includes(q) ||
        (item.sprint_name ?? "").toLowerCase().includes(q)
      );
    });
  }, [items, query, productFilter]);

  const stats = useMemo(() => {
    const active = items.filter((i) => ["planning", "wait_for_approval", "executing"].includes(i.status)).length;
    const executing = items.filter((i) => i.status === "executing").length;
    const success = items.filter((i) => i.status === "success").length;
    return { total: items.length, active, executing, success };
  }, [items]);

  function openPlan(item: WorkspaceReleasePlanVM) {
    router.push(`/products/${item.product_id}?tab=releases&planId=${item.id}`);
  }

  function handlePlanCreated(productId: string, planId: string) {
    void load();
    onPlanCreated?.(productId, planId);
  }

  const hasFilters = Boolean(query.trim() || statusFilter || productFilter);

  return (
    <div className="stack-md">
      <div className="release-plans-stats-grid grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="全部计划"
          value={loading ? "—" : stats.total}
          hint="工作区内发布计划总数"
          tone="sky"
          icon={
            <StatIcon>
              <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2" />
              <rect x="9" y="3" width="6" height="4" rx="1" />
            </StatIcon>
          }
        />
        <MetricCard
          label="进行中"
          value={loading ? "—" : stats.active}
          hint="规划 / 审批 / 执行"
          tone="amber"
          icon={
            <StatIcon>
              <circle cx="12" cy="12" r="9" />
              <path d="M12 7v5l3 2" />
            </StatIcon>
          }
        />
        <MetricCard
          label="执行中"
          value={loading ? "—" : stats.executing}
          tone="violet"
          icon={
            <StatIcon>
              <polygon points="5 3 19 12 5 21 5 3" />
            </StatIcon>
          }
        />
        <MetricCard
          label="已完成"
          value={loading ? "—" : stats.success}
          tone="emerald"
          icon={
            <StatIcon>
              <path d="M20 6L9 17l-5-5" />
            </StatIcon>
          }
        />
      </div>

      <section className="card stack-md">
        <div className="section-header">
          <div className="stack-xs">
            <h2 className="section-title">计划列表</h2>
            <ReleasePlanTemplatesPanel
              workspaceId={workspaceId}
              onPlanCreated={handlePlanCreated}
              createOpen={templateCreateOpen}
              onCreateOpenChange={onTemplateCreateOpenChange}
              onTemplatesLoaded={(tpls) => onTemplatesLoaded?.(tpls.length)}
            />
          </div>
          <div className="segmented-control w-[9rem] shrink-0" role="tablist" aria-label="视图切换">
            <button
              type="button"
              role="tab"
              aria-selected={view === "list"}
              className={`segmented-item ${view === "list" ? "segmented-item-active" : ""}`}
              onClick={() => setView("list")}
            >
              列表
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={view === "calendar"}
              className={`segmented-item ${view === "calendar" ? "segmented-item-active" : ""}`}
              onClick={() => setView("calendar")}
            >
              日历
            </button>
          </div>
        </div>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        <div className="release-plans-toolbar">
          <label className="release-plans-toolbar-search">
            <span className="sr-only">搜索</span>
            <input
              className="input"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索计划、产品或迭代…"
            />
          </label>
          <div className="release-plans-toolbar-filters">
            <label className="release-plans-toolbar-filter">
              <span className="sr-only">状态</span>
              <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} aria-label="状态筛选">
                <option value="">全部状态</option>
                <option value="planning">规划中</option>
                <option value="wait_for_approval">待审批</option>
                <option value="executing">执行中</option>
                <option value="success">已完成</option>
                <option value="denied">已驳回</option>
                <option value="cancel">已取消</option>
              </select>
            </label>
            <label className="release-plans-toolbar-filter">
              <span className="sr-only">产品</span>
              <select className="input" value={productFilter} onChange={(e) => setProductFilter(e.target.value)} aria-label="产品筛选">
                <option value="">全部产品</option>
                {productOptions.map(([id, name]) => (
                  <option key={id} value={id}>
                    {name}
                  </option>
                ))}
              </select>
            </label>
            {hasFilters ? (
              <button
                type="button"
                className="btn-ghost text-xs shrink-0"
                onClick={() => {
                  setQuery("");
                  setStatusFilter("");
                  setProductFilter("");
                }}
              >
                清除
              </button>
            ) : null}
          </div>
        </div>

        {view === "calendar" ? (
          loading ? (
            <div className="empty-state" role="status">
              加载日历数据…
            </div>
          ) : (
            <ReleasePlanCalendar items={filtered} onSelect={openPlan} />
          )
        ) : (
          <div className="table-wrap release-plans-table">
            <table className="data-table">
              <thead>
                <tr>
                  <th>计划</th>
                  <th>产品</th>
                  <th>迭代</th>
                  <th>状态</th>
                  <th>发布窗口</th>
                  <th className="release-plans-table-actions">操作</th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <tr>
                    <td colSpan={6}>
                      <div className="empty-state border-0 bg-transparent" role="status">
                        加载发布计划…
                      </div>
                    </td>
                  </tr>
                ) : filtered.length === 0 ? (
                  <tr>
                    <td colSpan={6}>
                      <div className="empty-state border-0 bg-transparent stack-sm items-center">
                        <p>{hasFilters ? "没有符合筛选条件的发布计划" : "还没有发布计划"}</p>
                        {!hasFilters && onRequestCreate ? (
                          <button type="button" className="btn btn-primary text-sm" onClick={() => onRequestCreate(productFilter || undefined)}>
                            新建发布计划
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ) : (
                  filtered.map((item) => (
                    <tr
                      key={item.id}
                      className="cursor-pointer"
                      onClick={() => openPlan(item)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") openPlan(item);
                      }}
                      tabIndex={0}
                    >
                      <td>
                        <span className="font-medium text-foreground">{item.name}</span>
                      </td>
                      <td className="text-muted-foreground">{item.product_name ?? item.product_id.slice(0, 8)}</td>
                      <td className="text-muted-foreground">{item.sprint_name ?? "—"}</td>
                      <td>
                        <span className={`badge ${releasePlanStatusBadgeClass(item.status)}`}>
                          {releasePlanStatusLabel(item.status)}
                        </span>
                      </td>
                      <td className="text-xs text-muted-foreground whitespace-nowrap">
                        {new Date(item.release_window_start).toLocaleDateString()} — {new Date(item.release_window_end).toLocaleDateString()}
                      </td>
                      <td className="release-plans-table-actions">
                        <button
                          type="button"
                          className="btn-ghost text-xs text-primary whitespace-nowrap"
                          onClick={(e) => {
                            e.stopPropagation();
                            openPlan(item);
                          }}
                        >
                          查看
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}

        {!loading && filtered.length > 0 && view === "list" ? (
          <p className="text-xs text-muted-foreground">共 {filtered.length} 条{hasFilters ? "（已筛选）" : ""} · 点击行可打开详情</p>
        ) : null}
      </section>
    </div>
  );
}
