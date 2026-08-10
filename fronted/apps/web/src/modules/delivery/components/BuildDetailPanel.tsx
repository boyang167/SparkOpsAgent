"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { BuildVM } from "@sparkops/contracts/delivery";
import { BuildLogStream } from "./BuildLogStream";

interface BuildDetailPanelProps {
  workspaceId: string;
  initialBuild: BuildVM;
}

export function BuildDetailPanel({ workspaceId, initialBuild }: BuildDetailPanelProps) {
  const router = useRouter();
  const [build, setBuild] = useState(initialBuild);
  const [loading, setLoading] = useState<"abort" | "rollback" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const isRunning = build.status === "running";
  const canAbort = isRunning;
  const canRollback = build.status === "success";

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

  return (
    <>
      <PageHeaderActions
        status={build.status}
        canAbort={canAbort}
        canRollback={canRollback}
        loading={loading}
        onAbort={onAbort}
        onRollback={onRollback}
      />

      <section className="card stack-md">
        <dl className="grid gap-3 sm:grid-cols-2 text-sm">
          <div>
            <dt className="text-muted">开始时间</dt>
            <dd>{new Date(build.started_at).toLocaleString("zh-CN")}</dd>
          </div>
          <div>
            <dt className="text-muted">结束时间</dt>
            <dd>
              {build.completed_at
                ? new Date(build.completed_at).toLocaleString("zh-CN")
                : isRunning
                  ? "进行中…"
                  : "—"}
            </dd>
          </div>
          {build.rollback_of_build_id ? (
            <div className="sm:col-span-2">
              <dt className="text-muted">回滚自</dt>
              <dd>
                <Link
                  href={`/delivery/builds/${build.rollback_of_build_id}`}
                  className="link text-sm"
                >
                  构建 {build.rollback_of_build_id.slice(0, 8)}…
                </Link>
              </dd>
            </div>
          ) : null}
        </dl>
        {build.error ? <p className="error text-sm">{build.error}</p> : null}
        {error ? <p className="error text-sm">{error}</p> : null}
      </section>

      <section className="section stack-sm">
        <div className="flex items-center justify-between gap-2">
          <h2 className="section-title">控制台日志</h2>
          {isRunning ? (
            <span className="text-xs text-muted animate-pulse">实时流式输出</span>
          ) : null}
        </div>
        <BuildLogStream
          buildId={build.id}
          initialLog={build.console_log}
          active={isRunning}
          onDone={() => void refreshBuild()}
        />
      </section>
    </>
  );
}

function PageHeaderActions({
  status,
  canAbort,
  canRollback,
  loading,
  onAbort,
  onRollback,
}: {
  status: string;
  canAbort: boolean;
  canRollback: boolean;
  loading: "abort" | "rollback" | null;
  onAbort: () => void;
  onRollback: () => void;
}) {
  function statusBadgeClass(s: string) {
    if (s === "success") return "badge-success";
    if (s === "failed") return "badge-warning";
    if (s === "aborted") return "badge-neutral";
    return "badge-neutral";
  }

  return (
    <div className="flex flex-wrap items-center gap-2 -mt-2 mb-2">
      <span className={`badge ${statusBadgeClass(status)}`}>{status}</span>
      {canAbort ? (
        <button
          className="btn-secondary text-sm"
          type="button"
          disabled={loading !== null}
          onClick={onAbort}
        >
          {loading === "abort" ? "中止中…" : "中止构建"}
        </button>
      ) : null}
      {canRollback ? (
        <button
          className="btn-secondary text-sm"
          type="button"
          disabled={loading !== null}
          onClick={onRollback}
        >
          {loading === "rollback" ? "回滚中…" : "回滚到此版本"}
        </button>
      ) : null}
    </div>
  );
}
