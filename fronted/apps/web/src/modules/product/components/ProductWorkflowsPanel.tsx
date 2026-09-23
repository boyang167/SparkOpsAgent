"use client";

import { useMemo } from "react";
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
  const products = useMemo(() => [{ id: productId, name: "当前产品" }], [productId]);
  const projectsByProduct = useMemo(() => ({ [productId]: projects }), [productId, projects]);

  return (
    <div className="wf-product-page">
      <section className="wf-hub-card">
        <WorkflowListPanel
          workspaceId={workspaceId}
          productFilter={productId}
          products={products}
          projectsByProduct={projectsByProduct}
          initialWorkflowId={initialWorkflowId}
          toolbarCreate
        />
      </section>
    </div>
  );
}
