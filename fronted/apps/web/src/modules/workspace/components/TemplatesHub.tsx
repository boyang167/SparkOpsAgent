"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { WorkspaceSprintProcessPanel } from "@/modules/product/components/WorkspaceSprintProcessPanel";
import { TemplatePanel } from "./TemplatePanel";

type TemplatesTab = "project" | "sprint-process";

interface TemplatesHubProps {
  workspaceId: string;
}

function parseTab(raw: string | null): TemplatesTab {
  return raw === "sprint-process" ? "sprint-process" : "project";
}

export function TemplatesHub({ workspaceId }: TemplatesHubProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [tab, setTab] = useState<TemplatesTab>(() => parseTab(searchParams.get("tab")));

  useEffect(() => {
    setTab(parseTab(searchParams.get("tab")));
  }, [searchParams]);

  function selectTab(next: TemplatesTab) {
    setTab(next);
    const params = new URLSearchParams(searchParams.toString());
    if (next === "project") {
      params.delete("tab");
    } else {
      params.set("tab", next);
    }
    const qs = params.toString();
    router.replace(qs ? `/templates?${qs}` : "/templates");
  }

  return (
    <div className="templates-hub-panel card">
      <div className="templates-hub-tabs product-detail-tabs" role="tablist" aria-label="模板类型">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "project"}
          className={`product-detail-tab${tab === "project" ? " is-active" : ""}`}
          onClick={() => selectTab("project")}
        >
          项目模板
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "sprint-process"}
          className={`product-detail-tab${tab === "sprint-process" ? " is-active" : ""}`}
          onClick={() => selectTab("sprint-process")}
        >
          迭代流程模板
        </button>
      </div>

      {tab === "project" ? (
        <div className="templates-hub-content">
          <TemplatePanel workspaceId={workspaceId} />
        </div>
      ) : null}
      {tab === "sprint-process" ? <WorkspaceSprintProcessPanel workspaceId={workspaceId} /> : null}
    </div>
  );
}
