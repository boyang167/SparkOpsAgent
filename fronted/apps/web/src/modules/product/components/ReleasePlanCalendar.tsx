"use client";

import { useMemo, useState } from "react";
import type { WorkspaceReleasePlanVM } from "@sparkops/contracts/product";
import { releasePlanStatusLabel } from "./release-plan-utils";

interface ReleasePlanCalendarProps {
  items: WorkspaceReleasePlanVM[];
  onSelect: (item: WorkspaceReleasePlanVM) => void;
}

function monthKey(d: Date) {
  return `${d.getFullYear()}-${d.getMonth()}`;
}

function daysInMonth(year: number, month: number) {
  return new Date(year, month + 1, 0).getDate();
}

export function ReleasePlanCalendar({ items, onSelect }: ReleasePlanCalendarProps) {
  const [cursor, setCursor] = useState(() => new Date());

  const grid = useMemo(() => {
    const year = cursor.getFullYear();
    const month = cursor.getMonth();
    const firstDow = new Date(year, month, 1).getDay();
    const total = daysInMonth(year, month);
    const cells: Array<{ day: number | null; plans: WorkspaceReleasePlanVM[] }> = [];
    for (let i = 0; i < firstDow; i++) cells.push({ day: null, plans: [] });
    for (let day = 1; day <= total; day++) {
      const dayStart = new Date(year, month, day, 0, 0, 0, 0);
      const dayEnd = new Date(year, month, day, 23, 59, 59, 999);
      const plans = items.filter((p) => {
        const start = new Date(p.release_window_start);
        const end = new Date(p.release_window_end);
        return start <= dayEnd && end >= dayStart;
      });
      cells.push({ day, plans });
    }
    return cells;
  }, [cursor, items]);

  const title = cursor.toLocaleDateString(undefined, { year: "numeric", month: "long" });

  return (
    <div className="stack-sm">
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          className="btn-secondary text-xs"
          onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() - 1, 1))}
        >
          上月
        </button>
        <span className="text-sm font-medium">{title}</span>
        <button
          type="button"
          className="btn-secondary text-xs"
          onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1))}
        >
          下月
        </button>
      </div>
      <div className="grid grid-cols-7 gap-1 text-xs">
        {["日", "一", "二", "三", "四", "五", "六"].map((w) => (
          <div key={w} className="text-center text-muted-foreground py-1">
            {w}
          </div>
        ))}
        {grid.map((cell, i) => (
          <div key={`${monthKey(cursor)}-${i}`} className="min-h-[72px] border rounded p-1 stack-xs bg-card">
            {cell.day ? <span className="text-muted-foreground">{cell.day}</span> : null}
            {cell.plans.slice(0, 3).map((plan) => (
              <button
                key={plan.id}
                type="button"
                className="text-left truncate rounded bg-primary/10 text-primary px-1 py-0.5 hover:bg-primary/20"
                title={`${plan.name} (${releasePlanStatusLabel(plan.status)})`}
                onClick={() => onSelect(plan)}
              >
                {plan.name}
              </button>
            ))}
            {cell.plans.length > 3 ? <span className="text-muted-foreground">+{cell.plans.length - 3}</span> : null}
          </div>
        ))}
      </div>
    </div>
  );
}
