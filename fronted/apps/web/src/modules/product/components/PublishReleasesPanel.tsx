"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductRequirementVM, ProductVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

const STATUS_LABEL: Record<string, string> = {
  pending: "待审批",
  approved: "已批准",
  rejected: "已驳回",
  running: "发布中",
  completed: "已完成",
};

interface PublishReleasesPanelProps {
  workspaceId: string;
  products: ProductVM[];
}

export function PublishReleasesPanel({ workspaceId, products }: PublishReleasesPanelProps) {
  const [productId, setProductId] = useState(products[0]?.id ?? "");
  const [items, setItems] = useState<ProductRequirementVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectTarget, setRejectTarget] = useState<ProductRequirementVM | null>(null);
  const [rejectReason, setRejectReason] = useState("");

  const load = useCallback(async () => {
    if (!productId) {
      setItems([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listRequirements(workspaceId, productId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function runAction(id: string, fn: () => Promise<unknown>) {
    setActionId(id);
    setError(null);
    try {
      await fn();
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setActionId(null);
    }
  }

  async function onReject(e: React.FormEvent) {
    e.preventDefault();
    if (!rejectTarget) return;
    const target = rejectTarget;
    await runAction(target.id, () =>
      product.rejectRequirement(workspaceId, productId, target.id, rejectReason),
    );
    setRejectOpen(false);
    setRejectTarget(null);
    setRejectReason("");
  }

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
      ) : items.length === 0 ? (
        <div className="empty-state">该产品暂无发布需求</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>项目</th>
                <th>标签</th>
                <th>描述</th>
                <th>状态</th>
                <th>操作人</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id}>
                  <td className="font-medium">{row.project_name ?? "-"}</td>
                  <td>{row.project_tag ?? "-"}</td>
                  <td>{row.description ?? "-"}</td>
                  <td>
                    <span className="badge badge-neutral">{STATUS_LABEL[row.status] ?? row.status}</span>
                  </td>
                  <td>{row.operator ?? "-"}</td>
                  <td>
                    <div className="flex flex-wrap gap-2">
                      {row.status === "pending" ? (
                        <>
                          <button
                            type="button"
                            className="btn-secondary !py-1 !px-2 text-xs"
                            disabled={actionId === row.id}
                            onClick={() =>
                              void runAction(row.id, () =>
                                product.approveRequirement(workspaceId, productId, row.id),
                              )
                            }
                          >
                            批准
                          </button>
                          <button
                            type="button"
                            className="text-xs text-red-600 hover:underline"
                            disabled={actionId === row.id}
                            onClick={() => {
                              setRejectTarget(row);
                              setRejectOpen(true);
                            }}
                          >
                            驳回
                          </button>
                        </>
                      ) : null}
                      {row.status === "approved" ? (
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          disabled={actionId === row.id}
                          onClick={() =>
                            void runAction(row.id, () =>
                              product.releaseRequirement(workspaceId, productId, row.id),
                            )
                          }
                        >
                          发布
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={rejectOpen} onClose={() => setRejectOpen(false)} title="驳回发布需求">
        <form onSubmit={onReject} className="stack-md">
          <textarea
            className="input min-h-24"
            placeholder="驳回原因（可选）"
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
          />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setRejectOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn">确认驳回</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
