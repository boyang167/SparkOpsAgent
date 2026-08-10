"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductPlanVM, ProductVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface PublishPlansPanelProps {
  workspaceId: string;
  products: ProductVM[];
}

export function PublishPlansPanel({ workspaceId, products }: PublishPlansPanelProps) {
  const [productId, setProductId] = useState(products[0]?.id ?? "");
  const [plans, setPlans] = useState<ProductPlanVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!productId) {
      setPlans([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setPlans(await product.listProductPlans(workspaceId, productId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="stack-md">
      <select className="input max-w-sm" value={productId} onChange={(e) => setProductId(e.target.value)}>
        {products.length === 0 ? <option value="">无产品</option> : null}
        {products.map((p) => (
          <option key={p.id} value={p.id}>{p.name}</option>
        ))}
      </select>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : plans.length === 0 ? (
        <div className="empty-state">该产品暂无迭代计划</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>计划名称</th>
                <th>版本</th>
                <th>描述</th>
              </tr>
            </thead>
            <tbody>
              {plans.map((plan) => (
                <tr key={plan.id}>
                  <td>{plan.name}</td>
                  <td>{plan.version ?? "-"}</td>
                  <td>{plan.description ?? "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
