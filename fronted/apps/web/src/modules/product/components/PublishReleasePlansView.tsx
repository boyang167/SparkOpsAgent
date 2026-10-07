"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { WorkspaceReleasePlansPanel } from "./WorkspaceReleasePlansPanel";

interface PublishReleasePlansViewProps {
  workspaceId: string;
}

function buildCreateHref(productId?: string) {
  const params = new URLSearchParams();
  if (productId) params.set("productId", productId);
  const query = params.toString();
  return query ? `/publish/release-plans/new?${query}` : "/publish/release-plans/new";
}

export function PublishReleasePlansView({ workspaceId }: PublishReleasePlansViewProps) {
  const router = useRouter();
  const [templateCreateOpen, setTemplateCreateOpen] = useState(false);
  const [templateCount, setTemplateCount] = useState(0);

  function openCreate(productId?: string) {
    router.push(buildCreateHref(productId));
  }

  function onPlanCreated(productId: string, planId: string) {
    router.push(`/products/${productId}?tab=releases&planId=${planId}`);
  }

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Publish"
        title="发布计划"
        description="跨产品统筹发布窗口、审批与执行进度"
        actions={
          <>
            <button
              type="button"
              className="btn btn-secondary text-sm"
              disabled={templateCount === 0}
              title={templateCount === 0 ? "请先在计划详情中存为模板" : undefined}
              onClick={() => setTemplateCreateOpen(true)}
            >
              从模板创建
            </button>
            <button type="button" className="btn btn-primary text-sm" onClick={() => openCreate()}>
              新建发布计划
            </button>
          </>
        }
      />
      <WorkspaceReleasePlansPanel
        workspaceId={workspaceId}
        onRequestCreate={openCreate}
        onPlanCreated={onPlanCreated}
        onTemplatesLoaded={setTemplateCount}
        templateCreateOpen={templateCreateOpen}
        onTemplateCreateOpenChange={setTemplateCreateOpen}
      />
    </div>
  );
}
