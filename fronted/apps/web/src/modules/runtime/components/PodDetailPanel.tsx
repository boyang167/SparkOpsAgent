"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PodVM } from "@sparkops/contracts/cluster";
import { Modal } from "@/components/ui/Modal";
import { ProjectPodTerminal } from "./ProjectPodTerminal";

export type PodDetailTab = "overview" | "logs" | "console";

interface PodDetailPanelProps {
  workspaceId: string;
  projectId: string;
  pod: PodVM;
  initialTab?: PodDetailTab;
  onClose: () => void;
}

const DETAIL_TABS: { id: PodDetailTab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "logs", label: "日志" },
  { id: "console", label: "控制台" },
];

function formatDateTime(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN");
}

function downloadText(content: string, filename: string) {
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

function ToolbarIconButton({
  label,
  title,
  active,
  onClick,
  children,
}: {
  label: string;
  title: string;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={active ? "pod-detail-toolbar-btn is-active" : "pod-detail-toolbar-btn"}
      aria-label={label}
      title={title}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function PodOverviewTab({ pod }: { pod: PodVM }) {
  return (
    <dl className="pod-detail-overview-grid">
      <div className="pod-detail-overview-item">
        <dt>名称</dt>
        <dd>{pod.name}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>命名空间</dt>
        <dd>{pod.namespace}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>状态</dt>
        <dd>{pod.phase}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>节点</dt>
        <dd>{pod.node || "-"}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>容器</dt>
        <dd>{pod.container || "-"}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>镜像</dt>
        <dd className="pod-detail-mono">{pod.image || "-"}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>重启次数</dt>
        <dd>{pod.restarts}</dd>
      </div>
      <div className="pod-detail-overview-item">
        <dt>启动时间</dt>
        <dd>{formatDateTime(pod.started_at)}</dd>
      </div>
      {pod.message ? (
        <div className="pod-detail-overview-item pod-detail-overview-item-wide">
          <dt>状态信息</dt>
          <dd className="pod-detail-error-text">{pod.message}</dd>
        </div>
      ) : null}
    </dl>
  );
}

function PodLogsTab({
  workspaceId,
  projectId,
  pod,
  active,
}: {
  workspaceId: string;
  projectId: string;
  pod: PodVM;
  active: boolean;
}) {
  const [logs, setLogs] = useState("");
  const [loading, setLoading] = useState(true);
  const [paused, setPaused] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const viewportRef = useRef<HTMLPreElement>(null);
  const stickToBottomRef = useRef(true);

  const loadLogs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await cluster.getPodLogs(projectId, pod.id, workspaceId);
      setLogs(data.logs);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载日志失败");
    } finally {
      setLoading(false);
    }
  }, [projectId, pod.id, workspaceId]);

  useEffect(() => {
    if (!active || paused) return;
    void loadLogs();
  }, [active, paused, loadLogs]);

  useEffect(() => {
    if (!active || paused) return;
    const timer = window.setInterval(() => {
      void loadLogs();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [active, paused, loadLogs]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !stickToBottomRef.current) return;
    viewport.scrollTop = viewport.scrollHeight;
  }, [logs]);

  function handleScroll() {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const distance = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    stickToBottomRef.current = distance < 48;
  }

  return (
    <div className="pod-detail-stream-panel">
      <div className="pod-detail-toolbar">
        <div className="pod-detail-toolbar-filters">
          <label className="pod-detail-toolbar-field">
            <span>容器</span>
            <select value={pod.container || pod.name} disabled aria-label="容器">
              <option value={pod.container || pod.name}>{pod.container || pod.name}</option>
            </select>
          </label>
          <label className="pod-detail-toolbar-field">
            <span>位于</span>
            <select value={pod.name} disabled aria-label="Pod">
              <option value={pod.name}>{pod.name}</option>
            </select>
          </label>
        </div>
        <div className="pod-detail-toolbar-actions">
          <ToolbarIconButton
            label={paused ? "继续刷新" : "暂停刷新"}
            title={paused ? "继续刷新" : "暂停刷新"}
            active={paused}
            onClick={() => setPaused((value) => !value)}
          >
            {paused ? (
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden>
                <path d="M8 5v14l11-7z" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden>
                <path d="M6 5h4v14H6V5zm8 0h4v14h-4V5z" />
              </svg>
            )}
          </ToolbarIconButton>
          <ToolbarIconButton
            label="清空"
            title="清空当前显示"
            onClick={() => {
              setLogs("");
              setError(null);
            }}
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </ToolbarIconButton>
          <ToolbarIconButton
            label="下载"
            title="下载日志"
            onClick={() => downloadText(logs || "", `${pod.name}.log`)}
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <path d="M12 3v12m0 0 4-4m-4 4-4-4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </ToolbarIconButton>
        </div>
      </div>

      {error ? <p className="pod-detail-inline-error">{error}</p> : null}

      <pre
        ref={viewportRef}
        className="pod-detail-viewport"
        onScroll={handleScroll}
      >
        {loading && !logs ? "加载中…" : logs || "暂无日志"}
      </pre>
    </div>
  );
}

function PodConsoleTab({
  workspaceId,
  projectId,
  pod,
  active,
}: {
  workspaceId: string;
  projectId: string;
  pod: PodVM;
  active: boolean;
}) {
  const container = pod.container || pod.name;

  return (
    <div className="pod-detail-stream-panel">
      <div className="pod-detail-toolbar">
        <div className="pod-detail-toolbar-filters">
          <label className="pod-detail-toolbar-field">
            <span>容器</span>
            <select value={container} disabled aria-label="容器">
              <option value={container}>{container}</option>
            </select>
          </label>
          <label className="pod-detail-toolbar-field">
            <span>位于</span>
            <select value={pod.name} disabled aria-label="Pod">
              <option value={pod.name}>{pod.name}</option>
            </select>
          </label>
        </div>
      </div>

      <ProjectPodTerminal
        workspaceId={workspaceId}
        projectId={projectId}
        podId={pod.id}
        container={container}
        active={active}
      />
    </div>
  );
}

export function PodDetailPanel({
  workspaceId,
  projectId,
  pod,
  initialTab = "overview",
  onClose,
}: PodDetailPanelProps) {
  const [tab, setTab] = useState<PodDetailTab>(initialTab);
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    setTab(initialTab);
  }, [pod.id, initialTab]);

  return (
    <Modal
      open
      onClose={onClose}
      title={`Pods详情 - ${pod.name}`}
      size="xl"
      panelClassName={
        fullscreen ? "pod-detail-modal pod-detail-modal-fullscreen" : "pod-detail-modal"
      }
      headerExtra={
        tab !== "overview" ? (
          <button
            type="button"
            className="modal-header-action"
            aria-label={fullscreen ? "退出全屏" : "全屏"}
            title={fullscreen ? "退出全屏" : "全屏"}
            onClick={() => setFullscreen((value) => !value)}
          >
            <span aria-hidden>{fullscreen ? "⊙" : "⛶"}</span>
          </button>
        ) : undefined
      }
    >
      <div className="pod-detail-root">
        <button type="button" className="pod-detail-back" onClick={onClose}>
          ← 返回服务详情
        </button>

        <div className="pod-detail-tabs" role="tablist" aria-label="Pod 详情">
          {DETAIL_TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={tab === item.id}
              className={tab === item.id ? "pod-detail-tab is-active" : "pod-detail-tab"}
              onClick={() => setTab(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>

        <div className="pod-detail-body">
          {tab === "overview" ? <PodOverviewTab pod={pod} /> : null}
          {tab === "logs" ? (
            <PodLogsTab
              workspaceId={workspaceId}
              projectId={projectId}
              pod={pod}
              active={tab === "logs"}
            />
          ) : null}
          {tab === "console" ? (
            <PodConsoleTab
              workspaceId={workspaceId}
              projectId={projectId}
              pod={pod}
              active={tab === "console"}
            />
          ) : null}
        </div>
      </div>
    </Modal>
  );
}
