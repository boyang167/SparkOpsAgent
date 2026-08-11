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

function UsageBar({ used, total, label }: { used: number; total: number; label: string }) {
  const percent = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0;
  return (
    <div>
      <div className="flex justify-between text-xs text-muted mb-1">
        <span>{label}</span>
        <span>
          {used.toFixed(1)} / {total.toFixed(1)} MB ({percent}%)
        </span>
      </div>
      <div className="h-2 rounded-full bg-[var(--surface-muted)] overflow-hidden">
        <div
          className="h-full rounded-full bg-[var(--primary)] transition-all"
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}

function StateCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-5 min-h-[280px]">
      <h3 className="text-sm font-semibold mb-4">{title}</h3>
      {children}
    </div>
  );
}

interface ServerStatusPanelProps {
  workspaceId: string;
}

export function ServerStatusPanel({ workspaceId }: ServerStatusPanelProps) {
  const [status, setStatus] = useState<ServerStatusVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setStatus(await platform.getServerStatus(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

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
        <p className="text-muted text-sm">每 10 秒自动刷新 · 参照 GVA 服务器状态</p>
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          立即刷新
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {status ? (
        <>
          <div className="grid gap-4 lg:grid-cols-2">
            <StateCard title="Runtime">
              <dl className="grid gap-3 text-sm">
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">OS</dt>
                  <dd>{status.go_os ?? "—"}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">Arch</dt>
                  <dd>{status.go_arch ?? "—"}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">Hostname</dt>
                  <dd>{status.hostname}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">CPU 核心</dt>
                  <dd>{status.num_cpu}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">Go Version</dt>
                  <dd className="font-mono text-xs">{status.go_version}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">Goroutines</dt>
                  <dd>{status.goroutines}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-muted">Uptime</dt>
                  <dd>{formatUptime(status.uptime_sec)}</dd>
                </div>
              </dl>
            </StateCard>

            <StateCard title="Ram">
              <div className="stack-md">
                <UsageBar used={status.mem_alloc_mb} total={status.mem_sys_mb} label="堆分配 / 系统内存" />
                <UsageBar used={status.heap_inuse_mb} total={status.mem_sys_mb} label="堆使用 / 系统内存" />
                <dl className="grid gap-2 text-sm pt-2">
                  <div className="flex justify-between gap-4">
                    <dt className="text-muted">堆对象数</dt>
                    <dd>{status.heap_objects}</dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-muted">GC 累计暂停</dt>
                    <dd>{status.gc_pause_ms} ms</dd>
                  </div>
                </dl>
              </div>
            </StateCard>
          </div>
          <p className="text-xs text-muted">
            采集于 {new Date(status.collected_at).toLocaleString()} · 当前为 Go 进程运行时指标，磁盘/CPU 使用率需接入 node exporter
          </p>
        </>
      ) : null}
    </div>
  );
}
