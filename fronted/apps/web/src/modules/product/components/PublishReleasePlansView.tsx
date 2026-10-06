"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReleasePlanCreateModal } from "./ReleasePlanCreateModal";
import { WorkspaceReleasePlansPanel } from "./WorkspaceReleasePlansPanel";

interface PublishReleasePlansViewProps {
  workspaceId: string;
}

export function PublishReleasePlansView({ workspaceId }: PublishReleasePlansViewProps) {
  const router = useRouter();
  const [createOpen, setCreateOpen] = useState(false);
  const [templateCreateOpen, setTemplateCreateOpen] = useState(false);
  const [templateCount, setTemplateCount] = useState(0);
  const [defaultProductId, setDefaultProductId] = useState<string | undefined>();

  function openCreate(productId?: string) {
    setDefaultProductId(productId);
    setCreateOpen(true);
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
      <ReleasePlanCreateModal
        open={createOpen}
        workspaceId={workspaceId}
        defaultProductId={defaultProductId}
        onClose={() => setCreateOpen(false)}
        onCreated={onPlanCreated}
      />
    </div>
  );
}
