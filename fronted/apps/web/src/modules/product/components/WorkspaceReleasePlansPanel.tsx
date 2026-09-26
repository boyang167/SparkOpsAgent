"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkspaceReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ReleasePlanCalendar } from "./ReleasePlanCalendar";
import { ReleasePlanTemplatesPanel } from "./ReleasePlanTemplatesPanel";
import { releasePlanStatusLabel } from "./release-plan-utils";

interface WorkspaceReleasePlansPanelProps {
  workspaceId: string;
}

export function WorkspaceReleasePlansPanel({ workspaceId }: WorkspaceReleasePlansPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState<WorkspaceReleasePlanVM[]>([]);
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
      setItems(await product.listWorkspaceReleasePlans(workspaceId, statusFilter ? { status: statusFilter } : undefined));
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
    for (const item of items) {
      map.set(item.product_id, item.product_name ?? item.product_id);
    }
    return [...map.entries()];
  }, [items]);

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

  function openPlan(item: WorkspaceReleasePlanVM) {
    router.push(`/products/${item.product_id}?tab=releases&planId=${item.id}`);
  }

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <ReleasePlanTemplatesPanel
        workspaceId={workspaceId}
        onPlanCreated={(productId, planId) => router.push(`/products/${productId}?tab=releases&planId=${planId}`)}
      />

      <div className="flex flex-wrap gap-2 items-end">
        <label className="stack-xs text-sm">
          搜索
          <input className="input" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="计划 / 产品 / 迭代" />
        </label>
        <label className="stack-xs text-sm">
          状态
          <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">全部</option>
            <option value="planning">规划中</option>
            <option value="wait_for_approval">待审批</option>
            <option value="executing">执行中</option>
            <option value="success">已完成</option>
          </select>
        </label>
        <label className="stack-xs text-sm">
          产品
          <select className="input" value={productFilter} onChange={(e) => setProductFilter(e.target.value)}>
            <option value="">全部</option>
            {productOptions.map(([id, name]) => (
              <option key={id} value={id}>
                {name}
              </option>
            ))}
          </select>
        </label>
        <div className="flex gap-1 ml-auto">
          <button type="button" className={`btn-secondary text-xs ${view === "list" ? "border-primary" : ""}`} onClick={() => setView("list")}>
            列表
          </button>
          <button type="button" className={`btn-secondary text-xs ${view === "calendar" ? "border-primary" : ""}`} onClick={() => setView("calendar")}>
            日历
          </button>
        </div>
      </div>

      {view === "calendar" ? (
        <ReleasePlanCalendar items={filtered} onSelect={openPlan} />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="py-2 pr-3">计划</th>
                <th className="py-2 pr-3">产品</th>
                <th className="py-2 pr-3">迭代</th>
                <th className="py-2 pr-3">状态</th>
                <th className="py-2 pr-3">发布窗口</th>
                <th className="py-2">操作</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-muted-foreground">
                    暂无发布计划
                  </td>
                </tr>
              ) : (
                filtered.map((item) => (
                  <tr key={item.id} className="border-b">
                    <td className="py-2 pr-3 font-medium">{item.name}</td>
                    <td className="py-2 pr-3">{item.product_name ?? item.product_id.slice(0, 8)}</td>
                    <td className="py-2 pr-3">{item.sprint_name ?? "—"}</td>
                    <td className="py-2 pr-3">{releasePlanStatusLabel(item.status)}</td>
                    <td className="py-2 pr-3 text-xs text-muted-foreground">
                      {new Date(item.release_window_start).toLocaleDateString()} — {new Date(item.release_window_end).toLocaleDateString()}
                    </td>
                    <td className="py-2">
                      <Link href={`/products/${item.product_id}?tab=releases&planId=${item.id}`} className="text-primary text-xs hover:underline">
                        查看
                      </Link>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
