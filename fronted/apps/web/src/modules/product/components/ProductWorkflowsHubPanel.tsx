"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductProjectVM, ProductVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductWorkflowsPanel } from "./ProductWorkflowsPanel";
import { productDisplayName } from "./product-utils";

interface ProductWorkflowsHubPanelProps {
  workspaceId: string;
}

export function ProductWorkflowsHubPanel({ workspaceId }: ProductWorkflowsHubPanelProps) {
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [productId, setProductId] = useState<string | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const loadProducts = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await product.listProducts(workspaceId);
      setProducts(items);
      setProductId((prev) => {
        if (prev && items.some((item) => item.id === prev)) return prev;
        return items[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载产品失败");
      setProducts([]);
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void loadProducts();
  }, [loadProducts]);

  useEffect(() => {
    if (!productId) {
      setProjects([]);
      return;
    }
    let cancelled = false;
    void product
      .listProductProjects(workspaceId, productId)
      .then((items) => {
        if (!cancelled) setProjects(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof BffError ? err.message : "加载产品项目失败");
          setProjects([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, productId]);

  return (
    <div className="stack-lg">
      <PlatformPanelHead
        title="工作流"
        description="按产品查看、创建并执行持续交付工作流。"
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
        <>
          <div className="product-env-tabs" role="tablist" aria-label="选择产品">
            <div className="product-env-tabs-scroll">
              {products.map((item) => {
                const active = item.id === productId;
                return (
                  <button
                    key={item.id}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    className={`product-env-tab${active ? " is-active" : ""}`}
                    onClick={() => setProductId(item.id)}
                  >
                    <span className="product-env-tab-name">{productDisplayName(item)}</span>
                  </button>
                );
              })}
            </div>
          </div>

          {productId ? (
            <ProductWorkflowsPanel workspaceId={workspaceId} productId={productId} projects={projects} />
          ) : null}
        </>
      )}
    </div>
  );
}
