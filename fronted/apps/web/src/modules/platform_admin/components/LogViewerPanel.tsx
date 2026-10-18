"use client";

import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  AppLogVM,
  ListAppLogsQuery,
  LogFileVM,
  MetricsSummaryVM,
  TraceDetailVM,
} from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MetricCard } from "@/components/ui/MetricCard";

const LOG_LEVELS = ["debug", "info", "warn", "error"] as const;
const LOG_CATEGORIES = [
  { value: "", label: "全部类型" },
  { value: "http", label: "HTTP" },
  { value: "app", label: "应用" },
  { value: "error", label: "错误" },
  { value: "audit", label: "审计" },
  { value: "metric", label: "指标" },
] as const;
const PAGE_SIZES = [20, 50, 100] as const;
type TabKey = "logs" | "files" | "traces" | "metrics";

function levelBadgeClass(level: string) {
  switch (level.toLowerCase()) {
    case "error":
    case "fatal":
    case "panic":
      return "badge badge-warning";
    case "warn":
      return "badge badge-neutral";
    default:
      return "badge badge-neutral";
  }
}

function fmtFields(fields?: Record<string, unknown>) {
  if (!fields || Object.keys(fields).length === 0) return null;
  try {
    return JSON.stringify(fields, null, 2);
  } catch {
    return String(fields);
  }
}

function fmtSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function fmtLogTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function fmtFileLine(line: string) {
  const trimmed = line.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return line;
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return line;
  }
}

interface LogViewerPanelProps {
  workspaceId: string;
  initialTraceId?: string;
}

export function LogViewerPanel({ workspaceId, initialTraceId }: LogViewerPanelProps) {
  const [tab, setTab] = useState<TabKey>(initialTraceId ? "traces" : "logs");
  const [error, setError] = useState<string | null>(null);

  const [logItems, setLogItems] = useState<AppLogVM[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [logLoading, setLogLoading] = useState(true);
  const [expandedLogId, setExpandedLogId] = useState<string | null>(null);
  const [logFilters, setLogFilters] = useState<ListAppLogsQuery>({
    page: 1,
    page_size: PAGE_SIZES[0],
  });
  const [appliedLogFilters, setAppliedLogFilters] = useState(logFilters);

  const [logFiles, setLogFiles] = useState<LogFileVM[]>([]);
  const [logDirectory, setLogDirectory] = useState("");
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileLines, setFileLines] = useState<string[]>([]);
  const [fileOffset, setFileOffset] = useState(0);
  const [fileEof, setFileEof] = useState(true);
  const [fileLoading, setFileLoading] = useState(false);

  const [traceInput, setTraceInput] = useState(initialTraceId ?? "");
  const [traceDetail, setTraceDetail] = useState<TraceDetailVM | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);

  const [metrics, setMetrics] = useState<MetricsSummaryVM | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);

  const logTotalPages = Math.max(
    1,
    Math.ceil(logTotal / (appliedLogFilters.page_size ?? PAGE_SIZES[0])),
  );

  const loadLogs = useCallback(async () => {
    setLogLoading(true);
    setError(null);
    try {
      const data = await platform.listAppLogs(workspaceId, appliedLogFilters);
      setLogItems(data.items);
      setLogTotal(data.total);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载日志失败");
    } finally {
      setLogLoading(false);
    }
  }, [workspaceId, appliedLogFilters]);

  const loadLogFiles = useCallback(async () => {
    setFileLoading(true);
    setError(null);
    try {
      const data = await platform.listLogFiles(workspaceId);
      setLogFiles(data.items);
      setLogDirectory(data.directory);
      if (!selectedFile && data.items.length > 0) {
        setSelectedFile(data.items[0].name);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载日志文件失败");
    } finally {
      setFileLoading(false);
    }
  }, [workspaceId, selectedFile]);

  const loadFileTail = useCallback(
    async (fileName: string, offset: number, append: boolean) => {
      setFileLoading(true);
      setError(null);
      try {
        const data = await platform.tailLogFile(workspaceId, fileName, offset, 200);
        setFileLines((prev) => (append ? [...prev, ...data.lines] : data.lines));
        setFileOffset(data.next_offset);
        setFileEof(data.eof);
      } catch (err) {
        setError(err instanceof BffError ? err.message : "读取日志文件失败");
      } finally {
        setFileLoading(false);
      }
    },
    [workspaceId],
  );

  const loadMetrics = useCallback(async () => {
    setMetricsLoading(true);
    setError(null);
    try {
      const data = await platform.getMetricsSummary(workspaceId);
      setMetrics(data);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载指标失败");
    } finally {
      setMetricsLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    if (tab === "logs") void loadLogs();
  }, [tab, loadLogs]);

  useEffect(() => {
    if (tab === "files") void loadLogFiles();
  }, [tab, loadLogFiles]);

  useEffect(() => {
    if (tab === "files" && selectedFile) {
      setFileLines([]);
      setFileOffset(0);
      void loadFileTail(selectedFile, 0, false);
    }
  }, [tab, selectedFile, loadFileTail]);

  useEffect(() => {
    if (tab === "metrics") void loadMetrics();
  }, [tab, loadMetrics]);

  useEffect(() => {
    if (!initialTraceId) return;
    setTraceLoading(true);
    void platform
      .getTraceDetail(workspaceId, initialTraceId)
      .then(setTraceDetail)
      .catch((err) => {
        setTraceDetail(null);
        setError(err instanceof BffError ? err.message : "查询 Trace 失败");
      })
      .finally(() => setTraceLoading(false));
  }, [workspaceId, initialTraceId]);

  async function onSearchTrace(e: React.FormEvent) {
    e.preventDefault();
    const traceId = traceInput.trim();
    if (!traceId) return;
    setTraceLoading(true);
    setError(null);
    try {
      const data = await platform.getTraceDetail(workspaceId, traceId);
      setTraceDetail(data);
    } catch (err) {
      setTraceDetail(null);
      setError(err instanceof BffError ? err.message : "查询 Trace 失败");
    } finally {
      setTraceLoading(false);
    }
  }

  function openTraceFromLog(traceId: string) {
    setTraceInput(traceId);
    setTab("traces");
    setTraceLoading(true);
    setError(null);
    void platform
      .getTraceDetail(workspaceId, traceId)
      .then(setTraceDetail)
      .catch((err) => {
        setTraceDetail(null);
        setError(err instanceof BffError ? err.message : "查询 Trace 失败");
      })
      .finally(() => setTraceLoading(false));
  }

  const traceTimeline = useMemo(() => {
    if (!traceDetail) return [];
    const events: Array<{ kind: "log" | "operation"; at: string; label: string; detail: string }> =
      [];
    for (const log of traceDetail.logs) {
      events.push({
        kind: "log",
        at: log.created_at,
        label: `[${log.level}] ${log.category}`,
        detail: log.message,
      });
    }
    for (const op of traceDetail.operations) {
      events.push({
        kind: "operation",
        at: op.created_at,
        label: `${op.method} ${op.path}`,
        detail: `status=${op.status} · ${op.latency_ms}ms`,
      });
    }
    return events.sort((a, b) => a.at.localeCompare(b.at));
  }, [traceDetail]);

  const tabs: Array<{ key: TabKey; label: string }> = [
    { key: "logs", label: "结构化日志" },
    { key: "files", label: "本地文件" },
    { key: "traces", label: "Trace 追踪" },
    { key: "metrics", label: "Metrics 指标" },
  ];

  const selectedFileMeta = logFiles.find((file) => file.name === selectedFile);

  return (
    <div className="log-viewer stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div
        className="segmented-control log-viewer-segmented"
        role="tablist"
        aria-label="日志查看方式"
      >
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={tab === item.key}
            className={`segmented-item${tab === item.key ? " segmented-item-active" : ""}`}
            onClick={() => setTab(item.key)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === "logs" ? (
        <section className="log-viewer-section stack-md" aria-label="结构化日志">
          <form
            className="log-viewer-toolbar"
            onSubmit={(e) => {
              e.preventDefault();
              setAppliedLogFilters({ ...logFilters, page: 1 });
            }}
          >
            <label className="log-viewer-field">
              <span className="log-viewer-field-label">级别</span>
              <select
                className="input"
                value={logFilters.level ?? ""}
                onChange={(e) => setLogFilters((f) => ({ ...f, level: e.target.value || undefined }))}
              >
                <option value="">全部</option>
                {LOG_LEVELS.map((level) => (
                  <option key={level} value={level}>
                    {level}
                  </option>
                ))}
              </select>
            </label>
            <label className="log-viewer-field">
              <span className="log-viewer-field-label">类型</span>
              <select
                className="input"
                value={logFilters.category ?? ""}
                onChange={(e) =>
                  setLogFilters((f) => ({ ...f, category: e.target.value || undefined }))
                }
              >
                {LOG_CATEGORIES.map((cat) => (
                  <option key={cat.value || "all"} value={cat.value}>
                    {cat.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="log-viewer-field log-viewer-field--wide">
              <span className="log-viewer-field-label">关键词 / Trace ID</span>
              <input
                className="input"
                placeholder="message、caller 或 trace_id"
                value={logFilters.keyword ?? logFilters.trace_id ?? ""}
                onChange={(e) =>
                  setLogFilters((f) => ({
                    ...f,
                    keyword: e.target.value || undefined,
                    trace_id: undefined,
                  }))
                }
              />
            </label>
            <button type="submit" className="btn btn-primary text-sm">
              查询
            </button>
          </form>

          <div className="table-wrap log-viewer-table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th className="log-viewer-col-time">时间</th>
                  <th>级别</th>
                  <th>类型</th>
                  <th>消息</th>
                  <th className="log-viewer-col-trace">Trace</th>
                </tr>
              </thead>
              <tbody>
                {logLoading ? (
                  <tr>
                    <td colSpan={5}>
                      <div className="empty-state border-0 bg-transparent" role="status">
                        加载日志…
                      </div>
                    </td>
                  </tr>
                ) : logItems.length === 0 ? (
                  <tr>
                    <td colSpan={5}>
                      <div className="empty-state border-0 bg-transparent" role="status">
                        暂无匹配的日志
                      </div>
                    </td>
                  </tr>
                ) : (
                  logItems.map((row) => (
                    <Fragment key={row.id}>
                      <tr
                        className="log-viewer-row"
                        onClick={() => setExpandedLogId((id) => (id === row.id ? null : row.id))}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            setExpandedLogId((id) => (id === row.id ? null : row.id));
                          }
                        }}
                        tabIndex={0}
                      >
                        <td className="log-viewer-col-time whitespace-nowrap text-muted">
                          {fmtLogTime(row.created_at)}
                        </td>
                        <td>
                          <span className={levelBadgeClass(row.level)}>{row.level}</span>
                        </td>
                        <td className="text-muted">{row.category}</td>
                        <td className="log-viewer-message">{row.message}</td>
                        <td className="log-viewer-col-trace">
                          {row.trace_id ? (
                            <button
                              type="button"
                              className="btn-ghost text-xs log-viewer-trace-link"
                              onClick={(e) => {
                                e.stopPropagation();
                                openTraceFromLog(row.trace_id!);
                              }}
                            >
                              {row.trace_id.slice(0, 8)}
                            </button>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                      {expandedLogId === row.id ? (
                        <tr className="log-viewer-detail-row">
                          <td colSpan={5}>
                            <pre className="log-viewer-detail">
                              {row.caller ? `caller: ${row.caller}\n` : ""}
                              {fmtFields(row.fields) ?? "无附加字段"}
                            </pre>
                          </td>
                        </tr>
                      ) : null}
                    </Fragment>
                  ))
                )}
              </tbody>
            </table>
          </div>

          <div className="log-viewer-pagination">
            <span className="text-sm text-muted">共 {logTotal} 条</span>
            <div className="log-viewer-pagination-actions">
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                disabled={(appliedLogFilters.page ?? 1) <= 1}
                onClick={() =>
                  setAppliedLogFilters((f) => ({ ...f, page: Math.max(1, (f.page ?? 1) - 1) }))
                }
              >
                上一页
              </button>
              <span className="text-sm text-muted">
                {appliedLogFilters.page ?? 1} / {logTotalPages}
              </span>
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                disabled={(appliedLogFilters.page ?? 1) >= logTotalPages}
                onClick={() =>
                  setAppliedLogFilters((f) => ({
                    ...f,
                    page: Math.min(logTotalPages, (f.page ?? 1) + 1),
                  }))
                }
              >
                下一页
              </button>
            </div>
          </div>
        </section>
      ) : null}

      {tab === "files" ? (
        <section className="log-viewer-section log-viewer-files" aria-label="本地日志文件">
          <div className="log-viewer-files-layout">
            <aside className="log-viewer-files-sidebar">
              <p className="log-viewer-files-sidebar-label">日志文件</p>
              {logFiles.length === 0 ? (
                <p className="log-viewer-files-empty" role="status">
                  {fileLoading ? "加载中…" : "暂无本地日志文件"}
                </p>
              ) : (
                <ul className="log-viewer-file-list">
                  {logFiles.map((file) => (
                    <li key={file.name}>
                      <button
                        type="button"
                        className={`log-viewer-file-item${selectedFile === file.name ? " log-viewer-file-item--active" : ""}`}
                        onClick={() => setSelectedFile(file.name)}
                      >
                        <span className="log-viewer-file-name">{file.name}</span>
                        <span className="log-viewer-file-size">{fmtSize(file.size)}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </aside>
            <div className="log-viewer-files-main stack-sm">
              <div className="log-viewer-files-head">
                <div className="log-viewer-files-head-copy">
                  <p className="log-viewer-files-title">{selectedFile ?? "选择文件"}</p>
                  <p className="log-viewer-files-meta">
                    {logDirectory ? `目录 · ${logDirectory}` : "目录 · logs"}
                    {selectedFileMeta ? ` · ${fmtSize(selectedFileMeta.size)}` : ""}
                  </p>
                </div>
                <div className="log-viewer-files-actions">
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    disabled={!selectedFile || fileLoading}
                    onClick={() => selectedFile && void loadFileTail(selectedFile, 0, false)}
                  >
                    刷新
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    disabled={!selectedFile || fileLoading || fileEof}
                    onClick={() =>
                      selectedFile && void loadFileTail(selectedFile, fileOffset, true)
                    }
                  >
                    加载更多
                  </button>
                </div>
              </div>
              <pre className="log-viewer-terminal" role="log">
                {fileLines.length === 0
                  ? (fileLoading ? "加载中…" : "选择左侧文件查看内容")
                  : fileLines.map(fmtFileLine).join("\n\n")}
              </pre>
            </div>
          </div>
        </section>
      ) : null}

      {tab === "traces" ? (
        <section className="log-viewer-section stack-md" aria-label="Trace 追踪">
          <form className="log-viewer-trace-form" onSubmit={onSearchTrace}>
            <label className="log-viewer-field log-viewer-field--grow">
              <span className="log-viewer-field-label">Trace ID</span>
              <input
                className="input font-mono"
                value={traceInput}
                onChange={(e) => setTraceInput(e.target.value)}
                placeholder="X-Trace-Id / BFF trace_id"
              />
            </label>
            <button type="submit" className="btn btn-primary text-sm" disabled={traceLoading}>
              {traceLoading ? "查询中…" : "查询"}
            </button>
          </form>

          {traceDetail ? (
            <div className="stack-md">
              <p className="log-viewer-trace-id">
                Trace ID
                <code>{traceDetail.trace_id}</code>
              </p>
              <div className="log-viewer-timeline">
                {traceTimeline.length === 0 ? (
                  <p className="log-viewer-files-empty" role="status">
                    未找到关联事件
                  </p>
                ) : (
                  traceTimeline.map((event, idx) => (
                    <article key={`${event.at}-${idx}`} className="log-viewer-timeline-item">
                      <div className="log-viewer-timeline-meta">
                        <span className="log-viewer-timeline-kind">{event.kind}</span>
                        <time className="log-viewer-timeline-time">{fmtLogTime(event.at)}</time>
                      </div>
                      <p className="log-viewer-timeline-label">{event.label}</p>
                      <p className="log-viewer-timeline-detail">{event.detail}</p>
                    </article>
                  ))
                )}
              </div>
            </div>
          ) : (
            !traceLoading && (
              <p className="log-viewer-files-empty" role="status">
                输入 Trace ID 查看关联日志与操作
              </p>
            )
          )}
        </section>
      ) : null}

      {tab === "metrics" ? (
        <section className="log-viewer-section stack-md" aria-label="Metrics 指标">
          <div className="log-viewer-metrics-actions">
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              disabled={metricsLoading}
              onClick={() => void loadMetrics()}
            >
              刷新
            </button>
            <a
              className="btn btn-ghost btn-sm"
              href="/metrics"
              target="_blank"
              rel="noreferrer"
            >
              打开 Prometheus /metrics
            </a>
          </div>
          {metricsLoading ? (
            <p className="log-viewer-files-empty" role="status">
              加载指标…
            </p>
          ) : metrics ? (
            <>
              <div className="log-viewer-metrics-grid">
                <MetricCard
                  label="HTTP P50 耗时"
                  value={`${metrics.http_duration_p50_ms.toFixed(1)} ms`}
                  tone="sky"
                  icon={
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M12 8v4l2 2" strokeLinecap="round" />
                      <circle cx="12" cy="12" r="9" />
                    </svg>
                  }
                />
                <MetricCard
                  label="请求序列数"
                  value={metrics.http_requests_total.length}
                  tone="violet"
                  icon={
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
                    </svg>
                  }
                />
                <MetricCard
                  label="采集时间"
                  value={fmtLogTime(metrics.collected_at)}
                  tone="emerald"
                  icon={
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <rect x="4" y="5" width="16" height="15" rx="2" />
                      <path d="M8 3v4M16 3v4" strokeLinecap="round" />
                    </svg>
                  }
                />
              </div>
              {metrics.http_requests_total.length > 0 ? (
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>标签</th>
                        <th className="log-viewer-col-count">计数</th>
                      </tr>
                    </thead>
                    <tbody>
                      {metrics.http_requests_total.map((row) => (
                        <tr key={row.label}>
                          <td className="font-mono text-sm">{row.label}</td>
                          <td className="log-viewer-col-count">{row.value}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
            </>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}
