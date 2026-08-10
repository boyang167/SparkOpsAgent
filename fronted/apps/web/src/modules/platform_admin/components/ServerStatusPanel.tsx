"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ServerStatusVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

function formatUptime(sec: number) {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  return `${h}h ${m}m ${s}s`;
}

function MetricCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4">
      <p className="text-sm text-muted">{label}</p>
      <p className="text-2xl font-semibold mt-1">{value}</p>
      {hint ? <p className="text-xs text-muted mt-1">{hint}</p> : null}
    </div>
  );
}

export function ServerStatusPanel() {
  const [status, setStatus] = useState<ServerStatusVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setStatus(await platform.getServerStatus());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 10_000);
    return () => window.clearInterval(timer);
  }, [load]);

  if (loading && !status) {
    return <div className="empty-state">加载中…</div>;
  }

  return (
    <div className="stack-md">
      <div className="flex justify-between items-center">
        <p className="text-muted text-sm">每 10 秒自动刷新 · 对应旧 PlatformSuite 监控 / GVA 服务器状态</p>
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          立即刷新
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {status ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <MetricCard label="主机名" value={status.hostname} />
            <MetricCard label="CPU 核心" value={String(status.num_cpu)} />
            <MetricCard label="Goroutines" value={String(status.goroutines)} />
            <MetricCard label="运行时长" value={formatUptime(status.uptime_sec)} />
            <MetricCard label="堆内存 (MB)" value={String(status.heap_inuse_mb)} hint={`已分配 ${status.mem_alloc_mb} MB`} />
            <MetricCard label="系统内存 (MB)" value={String(status.mem_sys_mb)} />
            <MetricCard label="堆对象数" value={String(status.heap_objects)} />
            <MetricCard label="GC 累计暂停 (ms)" value={String(status.gc_pause_ms)} />
          </div>
          <p className="text-xs text-muted">
            {status.go_version} · 采集于 {new Date(status.collected_at).toLocaleString()}
          </p>
        </>
      ) : null}
    </div>
  );
}
