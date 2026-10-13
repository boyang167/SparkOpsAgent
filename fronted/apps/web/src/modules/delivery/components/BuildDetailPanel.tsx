"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { BuildVM } from "@sparkops/contracts/delivery";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { BuildLogStream } from "./BuildLogStream";
import { countLogLines } from "./console-log";

interface BuildDetailPanelProps {
  workspaceId: string;
  initialBuild: BuildVM;
}

const STATUS_META: Record<string, { label: string; tone: string }> = {
  success: { label: "成功", tone: "success" },
  failed: { label: "失败", tone: "danger" },
  running: { label: "运行中", tone: "running" },
  aborted: { label: "已中止", tone: "neutral" },
  queued: { label: "排队中", tone: "neutral" },
  pending: { label: "排队中", tone: "neutral" },
};

function formatDateTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { hour12: false });
}

export function formatBuildDuration(startedAt: string, completedAt?: string, now: number = Date.now()) {
  const start = Date.parse(startedAt);
  const end = completedAt ? Date.parse(completedAt) : now;
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return "—";
  const seconds = Math.round((end - start) / 1000);
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分 ${seconds % 60} 秒`;
  return `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分`;
}

export function BuildDetailPanel({ workspaceId, initialBuild }: BuildDetailPanelProps) {
  const router = useRouter();
  const [build, setBuild] = useState(initialBuild);
  const [loading, setLoading] = useState<"abort" | "rollback" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [logText, setLogText] = useState("");
  const [copied, setCopied] = useState(false);

  const isRunning = build.status === "running";
  const canRollback = build.status === "success";
  const status = STATUS_META[build.status] ?? { label: build.status, tone: "neutral" };
  const lineCount = countLogLines(logText);

  const refreshBuild = useCallback(async () => {
    try {
      const next = await delivery.getBuild(build.id, workspaceId);
      setBuild(next);
    } catch {
      // ignore refresh errors
    }
  }, [build.id, workspaceId]);

  async function onAbort() {
    setLoading("abort");
    setError(null);
    try {
      await delivery.abortBuild(build.id, workspaceId);
      await refreshBuild();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "中止构建失败");
    } finally {
      setLoading(null);
    }
  }

  async function onRollback() {
    setLoading("rollback");
    setError(null);
    try {
      const result = await delivery.rollbackBuild(build.id, workspaceId);
      router.push(`/delivery/builds/${result.id}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "回滚构建失败");
    } finally {
      setLoading(null);
    }
  }

  async function onCopy() {
    if (!logText) return;
    try {
      await navigator.clipboard.writeText(logText);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("复制失败，请手动选择日志");
    }
  }

  return (
    <div className="build-detail">
      <section className="build-summary">
        <div className="build-summary-main">
          <span className={`build-status is-${status.tone}`}>
            <span className="build-status-dot" aria-hidden />
            {status.label}
          </span>
          <dl className="build-summary-meta">
            <div>
              <dt>开始</dt>
              <dd>{formatDateTime(build.started_at)}</dd>
            </div>
            <div>
              <dt>{isRunning ? "已运行" : "耗时"}</dt>
              <dd>{formatBuildDuration(build.started_at, build.completed_at)}</dd>
            </div>
            <div>
              <dt>结束</dt>
              <dd>{build.completed_at ? formatDateTime(build.completed_at) : isRunning ? "进行中" : "—"}</dd>
            </div>
            {build.rollback_of_build_id ? (
              <div>
                <dt>回滚自</dt>
                <dd>
                  <Link href={`/delivery/builds/${build.rollback_of_build_id}`} className="build-summary-link">
                    构建 {build.rollback_of_build_id.slice(0, 8)}
                  </Link>
                </dd>
              </div>
            ) : null}
          </dl>
        </div>
        {isRunning || canRollback ? (
          <div className="build-summary-actions">
            {isRunning ? (
              <button className="btn-secondary text-sm" type="button" disabled={loading !== null} onClick={onAbort}>
                {loading === "abort" ? "中止中…" : "中止构建"}
              </button>
            ) : null}
            {canRollback ? (
              <button className="btn-secondary text-sm" type="button" disabled={loading !== null} onClick={onRollback}>
                {loading === "rollback" ? "回滚中…" : "回滚到此版本"}
              </button>
            ) : null}
          </div>
        ) : null}
      </section>

      {build.error ? <InlineAlert tone="error">{build.error}</InlineAlert> : null}
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <section className="build-log">
        <header className="build-log-head">
          <div className="build-log-title-row">
            <h2 className="build-log-title">控制台日志</h2>
            {lineCount > 0 ? <span className="build-log-count">{lineCount} 行</span> : null}
            {isRunning ? (
              <span className="build-log-live">
                <span className="build-log-live-dot" aria-hidden />
                实时输出
              </span>
            ) : null}
          </div>
          <button type="button" className="build-log-tool" disabled={!logText} onClick={() => void onCopy()}>
            {copied ? "已复制" : "复制"}
          </button>
        </header>
        <BuildLogStream
          buildId={build.id}
          initialLog={build.console_log}
          active={isRunning}
          className="build-log-body"
          emptyText={isRunning ? "等待日志输出…" : "暂无日志"}
          onDone={() => void refreshBuild()}
          onTextChange={setLogText}
        />
      </section>
    </div>
  );
}
