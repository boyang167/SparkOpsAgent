"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { BuildVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { BuildLogStream } from "@/modules/delivery/components/BuildLogStream";
import { formatDateTime } from "./project-utils";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface ProjectBuildLogsTabProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

function buildStatusLabel(status: string) {
  const map: Record<string, string> = {
    running: "构建中",
    success: "成功",
    failed: "失败",
    aborted: "已中止",
  };
  return map[status] ?? status;
}

function BuildStatusIcon({ status }: { status: string }) {
  const tone =
    status === "success"
      ? "success"
      : status === "failed"
        ? "failed"
        : status === "aborted"
          ? "aborted"
          : status === "running"
            ? "running"
            : "neutral";
  const symbol =
    status === "success"
      ? "✓"
      : status === "failed"
        ? "✕"
        : status === "aborted"
          ? "!"
          : status === "running"
            ? "…"
            : "?";
  return <span className={`project-build-status-icon is-${tone}`}>{symbol}</span>;
}

export function ProjectBuildLogsTab({ workspaceId, project, active }: ProjectBuildLogsTabProps) {
  const { confirm, ConfirmDialog } = useConfirmDialog();
  const [builds, setBuilds] = useState<BuildVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedBuild, setSelectedBuild] = useState<BuildVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [aborting, setAborting] = useState(false);

  const loadBuilds = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await delivery.listBuilds(project.id, workspaceId);
      setBuilds(items);
      setSelectedId((current) => {
        if (current && items.some((item) => item.id === current)) return current;
        return items[0]?.id ?? null;
      });
    } catch (err) {
      setBuilds([]);
      setSelectedId(null);
      setError(err instanceof BffError ? err.message : "加载 Build 历史失败");
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void loadBuilds();
  }, [active, loadBuilds]);

  useEffect(() => {
    if (!selectedId) {
      setSelectedBuild(null);
      return;
    }
    let cancelled = false;
    void delivery
      .getBuild(selectedId, workspaceId)
      .then((build) => {
        if (!cancelled) setSelectedBuild(build);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof BffError ? err.message : "加载 Build 详情失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedId, workspaceId]);

  useEffect(() => {
    if (!active || !selectedBuild || selectedBuild.status !== "running") return;
    const timer = window.setInterval(() => {
      void delivery.getBuild(selectedBuild.id, workspaceId).then(setSelectedBuild);
      void loadBuilds();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [active, selectedBuild, workspaceId, loadBuilds]);

  async function handleAbort() {
    if (!selectedBuild || selectedBuild.status !== "running") return;
    const ok = await confirm({ title: "确定要终止本次 Build？", danger: true });
    if (!ok) return;
    setAborting(true);
    setError(null);
    try {
      await delivery.abortBuild(selectedBuild.id, workspaceId);
      const next = await delivery.getBuild(selectedBuild.id, workspaceId);
      setSelectedBuild(next);
      await loadBuilds();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "终止 Build 失败");
    } finally {
      setAborting(false);
    }
  }

  const isRunning = selectedBuild?.status === "running";

  return (
    <><div className="project-build-logs-layout">
      <aside className="project-build-logs-history">
        <div className="project-build-logs-history-header">Build history</div>
        {loading && builds.length === 0 ? (
          <p className="project-build-logs-empty">Build 历史加载中…</p>
        ) : null}
        {!loading && builds.length === 0 ? (
          <p className="project-build-logs-empty">暂无 build 数据</p>
        ) : (
          <ul className="project-build-logs-list">
            {builds.map((build) => {
              const selected = build.id === selectedId;
              const title =
                build.build_number > 0
                  ? `#${build.build_number} · ${build.branch}`
                  : build.branch || build.id.slice(0, 8);
              return (
                <li key={build.id}>
                  <button
                    type="button"
                    className={`project-build-logs-item${selected ? " is-selected" : ""}`}
                    onClick={() => setSelectedId(build.id)}
                  >
                    <BuildStatusIcon status={build.status} />
                    <span className="project-build-logs-item-copy">
                      <strong>{title}</strong>
                      <span>{formatDateTime(build.started_at)}</span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </aside>

      <section className="project-build-logs-console stack-sm">
        <div className="project-build-logs-console-header">
          <h4>项目 【{project.name}】 控制台输出</h4>
          {isRunning ? (
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted animate-pulse">实时流式输出</span>
              <button
                type="button"
                className="btn btn-secondary text-sm project-build-logs-abort"
                disabled={aborting}
                onClick={() => void handleAbort()}
              >
                {aborting ? "终止中…" : "终止 Build"}
              </button>
            </div>
          ) : null}
        </div>

        {error ? <p className="text-sm text-[var(--danger)]">{error}</p> : null}

        {selectedBuild ? (
          <>
            <div className="project-build-logs-meta">
              <span className={`project-tag project-tag-${selectedBuild.status === "success" ? "green" : selectedBuild.status === "failed" ? "orange" : "blue"}`}>
                {buildStatusLabel(selectedBuild.status)}
              </span>
              {selectedBuild.jenkins_job ? (
                <span className="text-xs text-muted">{selectedBuild.jenkins_job}</span>
              ) : null}
              {selectedBuild.error ? (
                <span className="text-xs text-[var(--danger)]">{selectedBuild.error}</span>
              ) : null}
            </div>
            <BuildLogStream
              buildId={selectedBuild.id}
              initialLog={selectedBuild.console_log}
              active={isRunning}
              onDone={() => {
                void delivery.getBuild(selectedBuild.id, workspaceId).then(setSelectedBuild);
                void loadBuilds();
              }}
            />
          </>
        ) : (
          <pre className="project-build-logs-placeholder">请选择左侧版本，查看 Build 控制台输出</pre>
        )}
      </section>
    </div><ConfirmDialog /></>
  );
}
