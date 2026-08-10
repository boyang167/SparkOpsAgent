"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface ProductsPanelProps {
  workspaceId: string;
}

export function ProductsPanel({ workspaceId }: ProductsPanelProps) {
  const [items, setItems] = useState<ProductVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listProducts(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {items.length === 0 ? (
        <div className="empty-state">暂无产品，可通过 API 创建</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>描述</th>
                <th>状态</th>
                <th>负责人</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id}>
                  <td className="font-medium">{row.name}</td>
                  <td>{row.description ?? "-"}</td>
                  <td>{row.status ?? "-"}</td>
                  <td>{row.owner ?? "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
