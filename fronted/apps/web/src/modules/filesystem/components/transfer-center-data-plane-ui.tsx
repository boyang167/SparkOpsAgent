import Link from "next/link";
import { createElement, type ReactNode } from "react";
import type { TransferMetricCounts } from "./transfer-center-stats";

function MetricIcon({ tone }: { tone: "sky" | "emerald" | "amber" | "violet" }) {
  const paths: Record<string, string> = {
    sky: "M12 4v16M8 12l4-4 4 4",
    emerald: "M6 12l3 3 9-9",
    amber: "M12 8v5M12 17h.01",
    violet: "M8 8l8 8M16 8l-8 8",
  };
  return createElement(
    "span",
    { className: `filesystem-transfer-metric-icon is-${tone}`, "aria-hidden": true },
    createElement(
      "svg",
      { width: 18, height: 18, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 },
      createElement("path", { d: paths[tone], strokeLinecap: "round", strokeLinejoin: "round" }),
    ),
  );
}

function TransferSparkline() {
  return createElement(
    "svg",
    {
      className: "filesystem-transfer-sparkline",
      viewBox: "0 0 120 40",
      preserveAspectRatio: "none",
      "aria-hidden": true,
    },
    createElement("path", {
      d: "M0 32 L20 28 L35 22 L55 26 L75 14 L95 18 L120 8",
      fill: "none",
      stroke: "var(--primary)",
      strokeWidth: 2,
      strokeLinecap: "round",
      opacity: 0.55,
    }),
    createElement("path", {
      d: "M0 38 L120 38",
      stroke: "var(--border)",
      strokeWidth: 1,
    }),
  );
}

/** 数据面传输统计卡片 */
export function TransferDataPlaneStatsCard({ counts }: { counts: TransferMetricCounts }) {
  const cards = [
    { label: "进行中", value: counts.running, tone: "sky" as const },
    { label: "已完成", value: counts.completed, tone: "emerald" as const },
    { label: "可恢复", value: counts.paused, tone: "amber" as const },
    { label: "失败任务", value: counts.failed, tone: "violet" as const },
  ];
  return createElement(
    "section",
    { className: "filesystem-data-plane-card filesystem-transfer-stats-card", "aria-label": "传输统计" },
    createElement(
      "header",
      { className: "filesystem-transfer-stats-head" },
      createElement(
        "div",
        { className: "min-w-0" },
        createElement("h2", { className: "filesystem-transfer-section-title" }, "传输统计"),
        createElement("p", { className: "filesystem-transfer-section-desc text-muted" }, "实时查看传输任务状态"),
      ),
      createElement(
        "select",
        {
          className: "input input-sm filesystem-transfer-range-select",
          "aria-label": "统计时间范围",
          defaultValue: "7d",
          disabled: true,
        },
        createElement("option", { value: "7d" }, "最近 7 天"),
      ),
    ),
    createElement(
      "div",
      { className: "filesystem-transfer-stats-body" },
      createElement(
        "div",
        { className: "filesystem-transfer-metric-grid" },
        cards.map((card) =>
          createElement(
            "article",
            { key: card.label, className: "filesystem-transfer-metric-tile" },
            createElement(MetricIcon, { tone: card.tone }),
            createElement("p", { className: "filesystem-transfer-metric-value tabular-nums" }, card.value),
            createElement("p", { className: "filesystem-transfer-metric-label text-muted" }, card.label),
          ),
        ),
      ),
      createElement(TransferSparkline),
    ),
  );
}

export function TransferDataPlaneTasksShell({
  totalCount,
  headerActions,
  children,
  footer,
}: {
  totalCount: number;
  headerActions: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return createElement(
    "section",
    { className: "filesystem-data-plane-card filesystem-transfer-tasks-card", "aria-label": "传输任务" },
    createElement(
      "header",
      { className: "filesystem-transfer-tasks-head" },
      createElement(
        "div",
        { className: "min-w-0" },
        createElement("h2", { className: "filesystem-transfer-section-title" }, "传输任务"),
        createElement(
          "p",
          { className: "filesystem-transfer-section-desc text-muted" },
          "管理文件上传、下载任务及传输状态",
        ),
      ),
      createElement("div", { className: "filesystem-transfer-tasks-actions" }, headerActions),
    ),
    createElement("div", { className: "filesystem-transfer-tasks-body" }, children),
    createElement(
      "footer",
      { className: "filesystem-transfer-tasks-foot" },
      createElement("span", { className: "text-muted text-sm" }, `共 ${totalCount} 项`),
      footer ?? null,
    ),
  );
}

export function TransferDataPlaneEmptyState() {
  return createElement(
    "div",
    { className: "filesystem-transfer-empty product-detail-empty" },
    createElement("p", { className: "product-detail-empty-title" }, "暂无传输任务"),
    createElement(
      "p",
      { className: "product-detail-empty-desc" },
      "在文件浏览中上传后，将在此展示进度与历史会话。",
    ),
    createElement(
      "div",
      { className: "product-detail-empty-action" },
      createElement(
        Link,
        { href: "/files/data/workspace", className: "btn btn-secondary btn-sm" },
        "前往文件浏览",
      ),
    ),
  );
}

export function TransferCompactProgress({ label, value }: { label: string; value: number }) {
  const clamped = Math.min(100, Math.max(0, value));
  return createElement(
    "div",
    {
      role: "progressbar",
      "aria-label": label,
      "aria-valuemin": 0,
      "aria-valuemax": 100,
      "aria-valuenow": clamped,
      className: "filesystem-transfer-progress-compact",
    },
    createElement("div", {
      className: `filesystem-transfer-progress-compact-fill${
        clamped >= 100 ? " is-complete" : clamped > 0 ? " is-active" : ""
      }`,
      style: { width: `${clamped}%` },
    }),
  );
}

export function transferTypeBadge(label: string, variant: "upload" | "upload-resume" | "download") {
  const tone =
    variant === "download"
      ? "badge-success"
      : variant === "upload-resume"
        ? "badge-warning"
        : "badge-primary";
  return createElement("span", { className: `badge ${tone} text-xs filesystem-transfer-type-badge` }, label);
}

export const DATA_PLANE_TRANSFER_TABLE_COLUMNS = [
  "任务 ID",
  "文件名",
  "类型",
  "大小",
  "进度",
  "状态",
  "开始时间",
  "操作",
] as const;