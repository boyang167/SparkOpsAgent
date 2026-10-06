"use client";

import { useMemo, useState } from "react";
import type { WorkspaceReleasePlanVM } from "@sparkops/contracts/product";
import { releasePlanStatusCalendarClass, releasePlanStatusLabel } from "./release-plan-utils";

interface ReleasePlanCalendarProps {
  items: WorkspaceReleasePlanVM[];
  onSelect: (item: WorkspaceReleasePlanVM) => void;
}

interface WeekSegment {
  plan: WorkspaceReleasePlanVM;
  colStart: number;
  colSpan: number;
  showLabel: boolean;
  lane: number;
}

const WEEKDAY_LABELS = ["日", "一", "二", "三", "四", "五", "六"] as const;
const MS_DAY = 24 * 60 * 60 * 1000;

function dayStamp(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

function buildMonthWeeks(year: number, month: number): Date[][] {
  const weeks: Date[][] = [];
  const cursor = new Date(year, month, 1);
  cursor.setDate(cursor.getDate() - cursor.getDay());

  const last = new Date(year, month + 1, 0);
  last.setDate(last.getDate() + (6 - last.getDay()));

  while (cursor <= last) {
    const week: Date[] = [];
    for (let i = 0; i < 7; i++) {
      week.push(new Date(cursor));
      cursor.setDate(cursor.getDate() + 1);
    }
    weeks.push(week);
  }
  return weeks;
}

function assignLanes(segments: Omit<WeekSegment, "lane">[]): WeekSegment[] {
  const laneEnds: number[] = [];
  const sorted = [...segments].sort((a, b) => a.colStart - b.colStart || b.colSpan - a.colSpan);
  return sorted.map((seg) => {
    let lane = laneEnds.findIndex((end) => end < seg.colStart);
    if (lane === -1) {
      lane = laneEnds.length;
      laneEnds.push(-1);
    }
    laneEnds[lane] = seg.colStart + seg.colSpan - 1;
    return { ...seg, lane };
  });
}

function segmentsForWeek(week: Date[], items: WorkspaceReleasePlanVM[]): WeekSegment[] {
  const weekStart = dayStamp(week[0]);
  const weekEnd = dayStamp(week[6]);
  const raw: Omit<WeekSegment, "lane">[] = [];

  for (const plan of items) {
    const planStart = dayStamp(new Date(plan.release_window_start));
    const planEnd = dayStamp(new Date(plan.release_window_end));
    if (planEnd < weekStart || planStart > weekEnd) continue;

    const segStart = Math.max(planStart, weekStart);
    const segEnd = Math.min(planEnd, weekEnd);
    const colStart = Math.round((segStart - weekStart) / MS_DAY);
    const colSpan = Math.round((segEnd - segStart) / MS_DAY) + 1;

    raw.push({
      plan,
      colStart,
      colSpan,
      showLabel: segStart === planStart,
    });
  }

  return assignLanes(raw);
}

function isSameMonth(a: Date, year: number, month: number) {
  return a.getFullYear() === year && a.getMonth() === month;
}

function isToday(d: Date) {
  const now = new Date();
  return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
}

export function ReleasePlanCalendar({ items, onSelect }: ReleasePlanCalendarProps) {
  const [cursor, setCursor] = useState(() => new Date());
  const year = cursor.getFullYear();
  const month = cursor.getMonth();
  const today = new Date();

  const weeks = useMemo(() => buildMonthWeeks(year, month), [year, month]);

  const monthPlans = useMemo(() => {
    const monthStart = new Date(year, month, 1);
    const monthEnd = new Date(year, month + 1, 0, 23, 59, 59, 999);
    return items.filter((p) => {
      const start = new Date(p.release_window_start);
      const end = new Date(p.release_window_end);
      return start <= monthEnd && end >= monthStart;
    });
  }, [items, year, month]);

  const title = cursor.toLocaleDateString("zh-CN", { year: "numeric", month: "long" });
  const viewingCurrentMonth = today.getFullYear() === year && today.getMonth() === month;

  return (
    <div className="release-plan-calendar">
      <div className="release-plan-cal-toolbar">
        <div className="release-plan-cal-nav">
          <button
            type="button"
            className="btn-secondary text-xs"
            onClick={() => setCursor(new Date(year, month - 1, 1))}
          >
            上月
          </button>
          <button
            type="button"
            className="btn-secondary text-xs"
            onClick={() => setCursor(new Date(year, month + 1, 1))}
          >
            下月
          </button>
          {!viewingCurrentMonth ? (
            <button type="button" className="btn-ghost text-xs" onClick={() => setCursor(new Date())}>
              今天
            </button>
          ) : null}
        </div>
        <h3 className="release-plan-cal-title">{title}</h3>
        <p className="release-plan-cal-subtitle">
          {monthPlans.length > 0 ? `本月 ${monthPlans.length} 个计划` : "本月暂无发布窗口"}
        </p>
      </div>

      {monthPlans.length === 0 ? (
        <div className="release-plan-cal-empty">当前月份没有符合筛选条件的发布计划</div>
      ) : (
        <>
          <div className="release-plan-cal-weekdays" aria-hidden>
            {WEEKDAY_LABELS.map((w) => (
              <div key={w} className="release-plan-cal-weekday">
                {w}
              </div>
            ))}
          </div>

          <div className="release-plan-cal-weeks">
            {weeks.map((week) => {
              const segments = segmentsForWeek(week, monthPlans);
              const laneCount = segments.reduce((max, s) => Math.max(max, s.lane + 1), 0);

              return (
                <div
                  key={week[0].toISOString()}
                  className="release-plan-cal-week"
                  style={{ ["--cal-lanes" as string]: Math.max(laneCount, 1) }}
                >
                  <div className="release-plan-cal-day-grid">
                    {week.map((day) => {
                      const inMonth = isSameMonth(day, year, month);
                      return (
                        <div
                          key={day.toISOString()}
                          className={[
                            "release-plan-cal-day",
                            inMonth ? "" : "release-plan-cal-day-outside",
                            isToday(day) ? "release-plan-cal-day-today" : "",
                          ]
                            .filter(Boolean)
                            .join(" ")}
                        >
                          <span className="release-plan-cal-day-num">{day.getDate()}</span>
                        </div>
                      );
                    })}
                  </div>

                  {segments.length > 0 ? (
                    <div className="release-plan-cal-event-grid" aria-hidden>
                      {segments.map((seg) => (
                        <button
                          key={`${seg.plan.id}-${seg.colStart}-${seg.lane}`}
                          type="button"
                          className={`release-plan-cal-event ${releasePlanStatusCalendarClass(seg.plan.status)}`}
                          style={{
                            gridColumn: `${seg.colStart + 1} / span ${seg.colSpan}`,
                            gridRow: seg.lane + 1,
                          }}
                          title={`${seg.plan.name} · ${seg.plan.product_name ?? ""} · ${releasePlanStatusLabel(seg.plan.status)}`}
                          onClick={() => onSelect(seg.plan)}
                        >
                          {seg.showLabel ? (
                            <>
                              <span className="release-plan-cal-event-name">{seg.plan.name}</span>
                              {seg.colSpan >= 2 ? (
                                <span className="release-plan-cal-event-meta">{seg.plan.product_name ?? ""}</span>
                              ) : null}
                            </>
                          ) : null}
                        </button>
                      ))}
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>

          <div className="release-plan-cal-legend" aria-label="状态图例">
            {(
              [
                ["planning", "规划中"],
                ["wait_for_approval", "待审批"],
                ["executing", "执行中"],
                ["success", "已完成"],
              ] as const
            ).map(([status, label]) => (
              <span key={status} className="release-plan-cal-legend-item">
                <span className={`release-plan-cal-legend-dot ${releasePlanStatusCalendarClass(status)}`} />
                {label}
              </span>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
