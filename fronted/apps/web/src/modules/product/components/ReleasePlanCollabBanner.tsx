"use client";

import type { ReleasePlanCollabEditor } from "@sparkops/contracts/product";
import { MemberNameLabel } from "./MemberNameLabel";

const SECTION_LABELS: Record<string, string> = {
  plan: "基本信息",
  jobs: "发布项列表",
};

function sectionLabel(section: string): string {
  if (SECTION_LABELS[section]) return SECTION_LABELS[section];
  if (section.startsWith("job:")) return "发布项";
  return section || "编辑中";
}

interface ReleasePlanCollabBannerProps {
  workspaceId: string;
  editors: ReleasePlanCollabEditor[];
}

export function ReleasePlanCollabBanner({ workspaceId, editors }: ReleasePlanCollabBannerProps) {
  if (editors.length === 0) return null;

  return (
    <div className="release-plan-collab-banner" role="status" aria-live="polite">
      <span className="release-plan-collab-banner-dot" aria-hidden />
      <span className="text-sm">
        {editors.map((editor, index) => (
          <span key={`${editor.member_id}-${editor.section}-${index}`}>
            {index > 0 ? "、" : ""}
            <MemberNameLabel workspaceId={workspaceId} memberId={editor.member_id} fallback="成员" />
            正在编辑{sectionLabel(editor.section)}
          </span>
        ))}
      </span>
    </div>
  );
}
