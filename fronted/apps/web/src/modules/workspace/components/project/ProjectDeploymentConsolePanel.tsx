"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { BuildVM, DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { BuildLogStream } from "@/modules/delivery/components/BuildLogStream";
import { formatDateTime } from "./project-utils";
import {
  createSerialPoller,
  shouldContinueDeploymentPoll,
} from "./deployment-poll";
import {
  deploymentRunStatusLabel,
  deploymentRunTitle,
  deploymentStatusTone,
  isActiveDeploymentRun,
} from "./deployment-console-utils";

interface ProjectDeploymentConsolePanelProps {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  active?: boolean;
  selectedRunId?: string | null;
  onSelectedRunIdChange?: (runId: string | null) => void;
  onRunChanged?: (run: DeploymentRunVM) => void;
  headerExtra?: React.ReactNode;
}

function StatusIcon({ tone }: { tone: string }) {
  const symbol =
    tone === "success"
      ? "✓"
      : tone === "failed"
        ? "✕"
        : tone === "aborted"
          ? "!"
          : tone === "running"
            ? "…"
            : "?";
  return <span className={`project-build-status-icon is-${tone}`}>{symbol}</span>;
}

export function ProjectDeploymentConsolePanel({
  workspaceId,
  project,
  deploymentPage,
  active = true,
  selectedRunId,
  onSelectedRunIdChange,
  onRunChanged,
  headerExtra,
}: ProjectDeploymentConsolePanelProps) {
  const runs = deploymentPage?.items ?? [];
  const [internalSelectedId, setInternalSelectedId] = useState<string | null>(null);
  const [run, setRun] = useState<DeploymentRunVM | null>(null);
  const [builds, setBuilds] = useState<BuildVM[]>([]);
  const [streamOutput, setStreamOutput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [aborting, setAborting] = useState(false);
  const pollGenerationRef = useRef(0);

  const selectedId = selectedRunId ?? internalSelectedId;
  const setSelectedId = onSelectedRunIdChange ?? setInternalSelectedId;

  useEffect(() => {
    if (!active) return;
    if (selectedRunId !== undefined) return;
    setInternalSelectedId((current) => {
      if (current && runs.some((item) => item.id === current)) return current;
      const activeRun = runs.find((item) => isActiveDeploymentRun(item));
      return activeRun?.id ?? runs[0]?.id ?? null;
    });
  }, [active, runs, selectedRunId]);

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    void delivery
      .listBuilds(project.id, workspaceId)
      .then((items) => {
        if (!cancelled) setBuilds(items);
      })
      .catch(() => {
        if (!cancelled) setBuilds([]);
      });
    return () => {
      cancelled = true;
    };
  }, [active, project.id, workspaceId, runs.length]);

  useEffect(() => {
    if (!selectedId) {
      setRun(null);
      setStreamOutput("");
      return;
    }
    const fromPage = runs.find((item) => item.id === selectedId) ?? null;
    setRun(fromPage);
    setStreamOutput("");
    let cancelled = false;
    void delivery
      .getDeployment(selectedId, workspaceId)
      .then((next) => {
        if (!cancelled) setRun(next);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof BffError ? err.message : "加载部署详情失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedId, workspaceId, runs]);

  useEffect(() => {
    if (!active || !run || !isActiveDeploymentRun(run)) return;
    const generation = ++pollGenerationRef.current;
    let stopStream: (() => void) | null = null;
    const poller = createSerialPoller({
      refresh: () => delivery.getDeployment(run.id, workspaceId),
      onValue: (next) => {
        setRun(next);
        onRunChanged?.(next);
        if (!shouldContinueDeploymentPoll(next.status, generation, pollGenerationRef.current)) {
          stopStream?.();
          stopStream = null;
        }
      },
      onError: (err) => {
        setError(err instanceof BffError ? err.message : "刷新部署状态失败");
      },
      shouldContinue: (next) =>
        shouldContinueDeploymentPoll(next.status, generation, pollGenerationRef.current),
      schedule: (callback, delayMs) => window.setTimeout(callback, delayMs),
      cancel: (timer) => window.clearTimeout(timer),
      delayMs: 2000,
    });
    stopStream = delivery.streamDeployment(run.id, {
      onDelta: (delta) => {
        if (pollGenerationRef.current !== generation) return;
        setStreamOutput((value) => value + delta);
      },
      onDone: () => {
        stopStream?.();
        stopStream = null;
      },
      onError: () => {
        stopStream?.();
        stopStream = null;
      },
    });
    poller.start();
    return () => {
      if (pollGenerationRef.current === generation) pollGenerationRef.current += 1;
      poller.stop();
      stopStream?.();
    };
  }, [active, run?.id, run?.status, workspaceId, onRunChanged]);

  const selectedBuild = useMemo(() => {
    if (!run?.build_id) return null;
    return builds.find((item) => item.id === run.build_id) ?? null;
  }, [builds, run?.build_id]);

  async function handleAbort() {
    if (!run || !isActiveDeploymentRun(run)) return;
    if (!window.confirm("确定要终止本次部署？")) return;
    setAborting(true);
    setError(null);
    try {
      const aborted = await delivery.abortDeployment(run.id, workspaceId);
      setRun(aborted);
      onRunChanged?.(aborted);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "终止部署失败");
    } finally {
      setAborting(false);
    }
  }

  const showBuildStream = Boolean(run?.build_id);
  const running = run ? isActiveDeploymentRun(run) : false;

  return (
    <div className="project-build-logs-layout">
      <aside className="project-build-logs-history">
        <div className="project-build-logs-history-header">Build history</div>
        {runs.length === 0 ? (
          <p className="project-build-logs-empty">暂无部署记录</p>
        ) : (
          <ul className="project-build-logs-list">
            {runs.map((item) => {
              const selected = item.id === selectedId;
              return (
                <li key={item.id}>
                  <button
                    type="button"
                    className={`project-build-logs-item${selected ? " is-selected" : ""}`}
                    onClick={() => setSelectedId(item.id)}
                  >
                    <StatusIcon tone={deploymentStatusTone(item.status)} />
                    <span className="project-build-logs-item-copy">
                      <strong>{deploymentRunTitle(item)}</strong>
                      <span>{formatDateTime(item.created_at)}</span>
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
          <h4>项目【{project.name}】控制台输出</h4>
          <div className="flex items-center gap-2 flex-wrap justify-end">
            {headerExtra}
            {running ? (
              <button
                type="button"
                className="btn btn-secondary text-sm project-build-logs-abort"
                disabled={aborting}
                onClick={() => void handleAbort()}
              >
                {aborting ? "终止中…" : "终止部署"}
              </button>
            ) : null}
          </div>
        </div>

        {error ? <p className="text-sm text-[var(--danger)]">{error}</p> : null}

        {run ? (
          <>
            <div className="project-build-logs-meta">
              <span
                className={`project-tag project-tag-${run.status === "succeeded" ? "green" : run.status === "failed" ? "orange" : "blue"}`}
              >
                {deploymentRunStatusLabel(run.status)}
                {run.current_stage ? ` · ${run.current_stage}` : ""}
              </span>
              {run.error_message ? (
                <span className="text-xs text-[var(--danger)]">{run.error_message}</span>
              ) : null}
            </div>
            {showBuildStream && run.build_id ? (
              <BuildLogStream
                buildId={run.build_id}
                initialLog={selectedBuild?.console_log}
                active={run.status === "building"}
                terminal
                onDone={() => {
                  void delivery.getDeployment(run.id, workspaceId).then((next) => {
                    setRun(next);
                    onRunChanged?.(next);
                  });
                }}
              />
            ) : (
              <pre className="project-build-logs-terminal">
                {streamOutput || "等待日志…"}
              </pre>
            )}
          </>
        ) : (
          <pre className="project-build-logs-placeholder">请选择左侧部署记录，查看控制台输出</pre>
        )}
      </section>
    </div>
  );
}
