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
    case "debug":
      return "badge badge-neutral";
    default:
      return "badge badge-primary";
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
        detail: `status=${op.status} latency=${op.latency_ms}ms`,
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

  return (
    <div className="stack-lg">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="flex flex-wrap gap-2 border-b border-border pb-2">
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            className={tab === item.key ? "btn btn-primary btn-sm" : "btn btn-ghost btn-sm"}
            onClick={() => setTab(item.key)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === "logs" ? (
        <Fragment>
          <form
            className="grid gap-3 md:grid-cols-5 items-end"
            onSubmit={(e) => {
              e.preventDefault();
              setAppliedLogFilters({ ...logFilters, page: 1 });
            }}
          >
            <label className="stack-xs">
              <span className="text-sm text-muted">级别</span>
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
            <label className="stack-xs">
              <span className="text-sm text-muted">类型</span>
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
            <label className="stack-xs md:col-span-2">
              <span className="text-sm text-muted">关键词 / Trace ID</span>
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
            <button type="submit" className="btn btn-primary">
              查询
            </button>
          </form>

          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>级别</th>
                  <th>类型</th>
                  <th>消息</th>
                  <th>Trace</th>
                </tr>
              </thead>
              <tbody>
                {logLoading ? (
                  <tr>
                    <td colSpan={5} className="text-muted">
                      加载中…
                    </td>
                  </tr>
                ) : logItems.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="text-muted">
                      暂无日志
                    </td>
                  </tr>
                ) : (
                  logItems.map((row) => (
                    <Fragment key={row.id}>
                      <tr
                        className="cursor-pointer hover:bg-muted/30"
                        onClick={() => setExpandedLogId((id) => (id === row.id ? null : row.id))}
                      >
                        <td className="whitespace-nowrap text-sm">{row.created_at}</td>
                        <td>
                          <span className={levelBadgeClass(row.level)}>{row.level}</span>
                        </td>
                        <td>{row.category}</td>
                        <td className="max-w-xl truncate">{row.message}</td>
                        <td>
                          {row.trace_id ? (
                            <button
                              type="button"
                              className="link text-sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                openTraceFromLog(row.trace_id!);
                              }}
                            >
                              {row.trace_id.slice(0, 8)}…
                            </button>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                      {expandedLogId === row.id ? (
                        <tr>
                          <td colSpan={5}>
                            <pre className="text-xs bg-muted/40 p-3 rounded overflow-auto">
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

          <div className="flex items-center justify-between gap-3 flex-wrap">
            <span className="text-sm text-muted">共 {logTotal} 条</span>
            <div className="flex items-center gap-2">
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
              <span className="text-sm">
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
        </Fragment>
      ) : null}

      {tab === "files" ? (
        <div className="grid gap-4 lg:grid-cols-[240px_1fr]">
          <div className="stack-sm">
            <p className="text-sm text-muted">目录：{logDirectory || "logs"}</p>
            <div className="stack-xs">
              {logFiles.length === 0 ? (
                <p className="text-sm text-muted">暂无本地日志文件</p>
              ) : (
                logFiles.map((file) => (
                  <button
                    key={file.name}
                    type="button"
                    className={
                      selectedFile === file.name
                        ? "btn btn-primary btn-sm justify-start"
                        : "btn btn-ghost btn-sm justify-start"
                    }
                    onClick={() => setSelectedFile(file.name)}
                  >
                    <span className="truncate">{file.name}</span>
                    <span className="text-xs opacity-70 ml-auto">{fmtSize(file.size)}</span>
                  </button>
                ))
              )}
            </div>
          </div>
          <div className="stack-sm">
            <div className="flex items-center gap-2">
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
            <pre className="text-xs bg-[#0f172a] text-[#e2e8f0] p-4 rounded min-h-[420px] max-h-[620px] overflow-auto font-mono">
              {fileLines.length === 0 ? (fileLoading ? "加载中…" : "选择文件查看内容") : fileLines.join("\n")}
            </pre>
          </div>
        </div>
      ) : null}

      {tab === "traces" ? (
        <div className="stack-md">
          <form className="flex flex-wrap gap-2 items-end" onSubmit={onSearchTrace}>
            <label className="stack-xs flex-1 min-w-[240px]">
              <span className="text-sm text-muted">Trace ID</span>
              <input
                className="input"
                value={traceInput}
                onChange={(e) => setTraceInput(e.target.value)}
                placeholder="X-Trace-Id / BFF trace_id"
              />
            </label>
            <button type="submit" className="btn btn-primary" disabled={traceLoading}>
              {traceLoading ? "查询中…" : "查询"}
            </button>
          </form>

          {traceDetail ? (
            <div className="stack-md">
              <p className="text-sm">
                Trace ID：<code>{traceDetail.trace_id}</code>
              </p>
              <div className="stack-sm">
                {traceTimeline.length === 0 ? (
                  <p className="text-muted text-sm">未找到关联事件</p>
                ) : (
                  traceTimeline.map((event, idx) => (
                    <div key={`${event.at}-${idx}`} className="border border-border rounded p-3">
                      <div className="flex flex-wrap gap-2 items-center text-sm">
                        <span className="badge badge-neutral">{event.kind}</span>
                        <span className="text-muted">{event.at}</span>
                        <span className="font-medium">{event.label}</span>
                      </div>
                      <p className="text-sm mt-1">{event.detail}</p>
                    </div>
                  ))
                )}
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      {tab === "metrics" ? (
        <div className="stack-md">
          <div className="flex items-center gap-2">
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
            <p className="text-muted text-sm">加载中…</p>
          ) : metrics ? (
            <div className="grid gap-4 md:grid-cols-3">
              <div className="card p-4">
                <p className="text-sm text-muted">HTTP 平均耗时 (ms)</p>
                <p className="text-2xl font-semibold mt-1">
                  {metrics.http_duration_p50_ms.toFixed(1)}
                </p>
              </div>
              <div className="card p-4">
                <p className="text-sm text-muted">请求序列数</p>
                <p className="text-2xl font-semibold mt-1">{metrics.http_requests_total.length}</p>
              </div>
              <div className="card p-4">
                <p className="text-sm text-muted">采集时间</p>
                <p className="text-sm font-medium mt-1">{metrics.collected_at}</p>
              </div>
            </div>
          ) : null}
          {metrics && metrics.http_requests_total.length > 0 ? (
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>标签</th>
                    <th>计数</th>
                  </tr>
                </thead>
                <tbody>
                  {metrics.http_requests_total.map((row) => (
                    <tr key={row.label}>
                      <td>{row.label}</td>
                      <td>{row.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
