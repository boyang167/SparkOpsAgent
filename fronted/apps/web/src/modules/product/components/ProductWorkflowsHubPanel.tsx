"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductProjectVM, ProductVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { WorkflowListPanel } from "./WorkflowListPanel";
import { productDisplayName } from "./product-utils";

interface ProductWorkflowsHubPanelProps {
  workspaceId: string;
  initialProductId?: string | null;
  initialWorkflowId?: string | null;
}

export function ProductWorkflowsHubPanel({
  workspaceId,
  initialProductId,
  initialWorkflowId,
}: ProductWorkflowsHubPanelProps) {
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [productFilter, setProductFilter] = useState<string | null>(initialProductId ?? null);
  const [projectsByProduct, setProjectsByProduct] = useState<Record<string, ProductProjectVM[]>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [createRequestId, setCreateRequestId] = useState(0);

  const loadProducts = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await product.listProducts(workspaceId);
      setProducts(items);
      setProductFilter((prev) => {
        if (initialProductId && items.some((item) => item.id === initialProductId)) return initialProductId;
        return prev;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载产品失败");
      setProducts([]);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, initialProductId]);

  useEffect(() => {
    void loadProducts();
  }, [loadProducts]);

  useEffect(() => {
    if (products.length === 0) return;
    let cancelled = false;
    void (async () => {
      const entries = await Promise.all(
        products.map(async (item) => {
          try {
            const projects = await product.listProductProjects(workspaceId, item.id);
            return [item.id, projects] as const;
          } catch {
            return [item.id, []] as const;
          }
        }),
      );
      if (!cancelled) {
        setProjectsByProduct(Object.fromEntries(entries));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceId, products]);

  const tabProducts = useMemo(
    () => products.map((item) => ({ id: item.id, name: item.name, display_name: item.display_name })),
    [products],
  );

  return (
    <div className="stack-lg wf-hub-page">
      <PlatformPanelHead
        title="工作流"
        description="浏览、执行与管理工作区内的产品工作流。"
        action={
          products.length > 0 ? (
            <button
              type="button"
              className="btn btn-primary text-sm"
              onClick={() => setCreateRequestId((n) => n + 1)}
            >
              新建工作流
            </button>
          ) : null
        }
      />

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state text-sm" role="status">
          加载产品…
        </div>
      ) : products.length === 0 ? (
        <ProductDetailEmptyState
          title="暂无产品"
          description="请先在产品管理中创建产品，再配置工作流。"
          action={
            <Link href="/products" className="btn btn-primary text-sm">
              前往产品管理
            </Link>
          }
        />
      ) : (
        <section className="wf-hub-card">
          <div className="wf-hub-tabs" role="tablist" aria-label="筛选产品">
            <button
              type="button"
              role="tab"
              aria-selected={productFilter === null}
              className={`wf-hub-tab${productFilter === null ? " is-active" : ""}`}
              onClick={() => setProductFilter(null)}
            >
              所有
            </button>
            {products.map((item) => {
              const active = item.id === productFilter;
              return (
                <button
                  key={item.id}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  className={`wf-hub-tab${active ? " is-active" : ""}`}
                  onClick={() => setProductFilter(item.id)}
                >
                  {productDisplayName(item)}
                </button>
              );
            })}
          </div>

          <WorkflowListPanel
            workspaceId={workspaceId}
            productFilter={productFilter}
            products={tabProducts}
            projectsByProduct={projectsByProduct}
            initialWorkflowId={initialWorkflowId}
            createRequestId={createRequestId}
          />
        </section>
      )}
    </div>
  );
}
