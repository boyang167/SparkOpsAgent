"use client";

import { useMemo, useState } from "react";
import type { LinkedWorkItemVM, ReleasePlanVM } from "@sparkops/contracts/product";

interface SprintWorkItemsSummaryProps {
  plan: ReleasePlanVM;
  canEdit: boolean;
  busy?: boolean;
  onGenerateJobs: (workItemIds: string[]) => void | Promise<void>;
}

export function SprintWorkItemsSummary({ plan, canEdit, busy, onGenerateJobs }: SprintWorkItemsSummaryProps) {
  const items = plan.linked_work_items ?? [];
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const jiraSprintName = useMemo(() => {
    const ref = plan.sprint?.external_ref;
    if (!ref) return null;
    if (typeof ref.name === "string") return ref.name;
    if (typeof ref.sprint_name === "string") return ref.sprint_name;
    return null;
  }, [plan.sprint?.external_ref]);

  if (!plan.sprint && items.length === 0) return null;

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="rounded border p-3 stack-sm text-sm">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="font-medium">关联迭代：{plan.sprint?.name ?? plan.sprint_id}</p>
          {jiraSprintName ? <p className="text-xs text-muted-foreground">JIRA 冲刺：{jiraSprintName}</p> : null}
        </div>
        {canEdit ? (
          <button
            type="button"
            className="btn-secondary text-xs"
            disabled={busy || selected.size === 0}
            onClick={() => void onGenerateJobs([...selected])}
          >
            从工作项生成检查项 ({selected.size})
          </button>
        ) : null}
      </div>
      {items.length === 0 ? (
        <p className="text-xs text-muted-foreground">该迭代暂无工作项</p>
      ) : (
        <ul className="stack-xs max-h-48 overflow-auto">
          {items.map((item: LinkedWorkItemVM) => (
            <li key={item.id} className="flex items-start gap-2 border-b py-1">
              {canEdit ? (
                <input type="checkbox" checked={selected.has(item.id)} onChange={() => toggle(item.id)} className="mt-1" />
              ) : null}
              <div className="stack-xs flex-1">
                <span>{item.title}</span>
                <span className="text-xs text-muted-foreground">
                  {item.stage_id} · {item.status}
                </span>
                {item.pm_links?.map((link) => {
                  const label = `${link.provider ?? "pm"}:${link.external_key ?? "—"}`;
                  if (link.url) {
                    return (
                      <a
                        key={`${link.provider}-${link.external_key}`}
                        href={link.url}
                        target="_blank"
                        rel="noreferrer noopener"
                        className="text-xs rounded bg-muted px-1.5 py-0.5 w-fit text-primary hover:underline"
                      >
                        {label}
                      </a>
                    );
                  }
                  return (
                    <span
                      key={`${link.provider}-${link.external_key}`}
                      className="text-xs rounded bg-muted px-1.5 py-0.5 w-fit"
                    >
                      {label}
                    </span>
                  );
                })}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
