"use client";

import { useMemo, useState } from "react";
import type { ProductProjectVM } from "@sparkops/contracts/product";
import { WorkflowListPanel } from "./WorkflowListPanel";

interface ProductWorkflowsPanelProps {
  workspaceId: string;
  productId: string;
  projects: ProductProjectVM[];
  initialWorkflowId?: string | null;
}

export function ProductWorkflowsPanel({
  workspaceId,
  productId,
  projects,
  initialWorkflowId,
}: ProductWorkflowsPanelProps) {
  const [createRequestId, setCreateRequestId] = useState(0);
  const products = useMemo(() => [{ id: productId, name: "当前产品" }], [productId]);
  const projectsByProduct = useMemo(() => ({ [productId]: projects }), [productId, projects]);

  return (
    <div className="wf-product-page">
      <div className="wf-product-head">
        <div className="wf-product-head-copy">
          <span className="wf-product-eyebrow">持续交付</span>
          <h2 className="wf-product-title">工作流</h2>
          <p className="wf-product-desc">浏览阶段路径、成功率与最近执行记录，一键运行或进入可视化编排。</p>
        </div>
        <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateRequestId((n) => n + 1)}>
          新建工作流
        </button>
      </div>
      <section className="wf-hub-card">
        <WorkflowListPanel
          workspaceId={workspaceId}
          productFilter={productId}
          products={products}
          projectsByProduct={projectsByProduct}
          initialWorkflowId={initialWorkflowId}
          createRequestId={createRequestId}
        />
      </section>
    </div>
  );
}
