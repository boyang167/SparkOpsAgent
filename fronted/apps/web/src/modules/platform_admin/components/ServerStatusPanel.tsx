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
      <div className="monitor-server-usage-label">
        <span>{label}</span>
        <span>
          {used.toFixed(1)} / {total.toFixed(1)} MB ({percent}%)
        </span>
      </div>
      <div className="monitor-server-usage-bar">
        <div className="monitor-server-usage-fill" style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

function StateCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <article className="monitor-server-card">
      <h3 className="monitor-server-card-title">{title}</h3>
      {children}
    </article>
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
      <div className="monitor-server-toolbar">
        <p className="monitor-server-footnote">每 10 秒自动刷新</p>
        <button type="button" className="btn btn-ghost btn-sm" onClick={() => void load()}>
          立即刷新
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {status ? (
        <>
          <div className="monitor-server-grid">
            <StateCard title="Runtime">
              <dl className="monitor-server-dl">
                <div>
                  <dt>OS</dt>
                  <dd>{status.go_os ?? "—"}</dd>
                </div>
                <div>
                  <dt>Arch</dt>
                  <dd>{status.go_arch ?? "—"}</dd>
                </div>
                <div>
                  <dt>Hostname</dt>
                  <dd>{status.hostname}</dd>
                </div>
                <div>
                  <dt>CPU 核心</dt>
                  <dd>{status.num_cpu}</dd>
                </div>
                <div>
                  <dt>Go Version</dt>
                  <dd className="font-mono text-xs">{status.go_version}</dd>
                </div>
                <div>
                  <dt>Goroutines</dt>
                  <dd>{status.goroutines}</dd>
                </div>
                <div>
                  <dt>Uptime</dt>
                  <dd>{formatUptime(status.uptime_sec)}</dd>
                </div>
              </dl>
            </StateCard>

            <StateCard title="Memory">
              <div className="monitor-server-usage">
                <UsageBar used={status.mem_alloc_mb} total={status.mem_sys_mb} label="堆分配 / 系统内存" />
                <UsageBar used={status.heap_inuse_mb} total={status.mem_sys_mb} label="堆使用 / 系统内存" />
                <dl className="monitor-server-dl">
                  <div>
                    <dt>堆对象数</dt>
                    <dd>{status.heap_objects}</dd>
                  </div>
                  <div>
                    <dt>GC 累计暂停</dt>
                    <dd>{status.gc_pause_ms} ms</dd>
                  </div>
                </dl>
              </div>
            </StateCard>
          </div>
          <p className="monitor-server-footnote">
            采集于 {new Date(status.collected_at).toLocaleString()} · 当前为 Go 进程运行时指标，磁盘/CPU 使用率需接入 node exporter
          </p>
        </>
      ) : null}
    </div>
  );
}
