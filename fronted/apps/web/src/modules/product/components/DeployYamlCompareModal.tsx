"use client";

import { useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { RunPreviewItemVM } from "@sparkops/contracts/delivery";
import { delivery } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";

interface DeployYamlCompareModalProps {
  open: boolean;
  workspaceId: string;
  projectId: string;
  projectName: string;
  environmentId: string;
  imageTag: string;
  imageRef?: string;
  onClose: () => void;
}

function countYamlDiff(left: string, right: string): { additions: number; deletions: number } {
  const leftLines = left.split("\n");
  const rightLines = right.split("\n");
  const leftSet = new Set(leftLines);
  const rightSet = new Set(rightLines);
  let additions = 0;
  let deletions = 0;
  for (const line of rightLines) {
    if (!leftSet.has(line)) additions += 1;
  }
  for (const line of leftLines) {
    if (!rightSet.has(line)) deletions += 1;
  }
  return { additions, deletions };
}

function filterDiffLines(rendered: string, current: string, onlyDiff: boolean): { rendered: string; current: string } {
  if (!onlyDiff) return { rendered, current };
  const renderedLines = rendered.split("\n");
  const currentLines = current.split("\n");
  const max = Math.max(renderedLines.length, currentLines.length);
  const nextRendered: string[] = [];
  const nextCurrent: string[] = [];
  for (let i = 0; i < max; i += 1) {
    const a = renderedLines[i] ?? "";
    const b = currentLines[i] ?? "";
    if (a === b) continue;
    nextRendered.push(a);
    nextCurrent.push(b);
  }
  return {
    rendered: nextRendered.join("\n") || "（无差异行）",
    current: nextCurrent.join("\n") || "（无差异行）",
  };
}

export function DeployYamlCompareModal({
  open,
  workspaceId,
  projectId,
  projectName,
  environmentId,
  imageTag,
  imageRef,
  onClose,
}: DeployYamlCompareModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [items, setItems] = useState<RunPreviewItemVM[]>([]);
  const [meta, setMeta] = useState<{ clusterName: string; namespace: string } | null>(null);
  const [activeKind, setActiveKind] = useState<string>("Deployment");
  const [singleColumn, setSingleColumn] = useState(false);
  const [onlyDiff, setOnlyDiff] = useState(false);

  useEffect(() => {
    if (!open || !projectId || !environmentId) {
      setItems([]);
      setMeta(null);
      setError(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    void delivery
      .getRunPreview(projectId, workspaceId, { environmentId, imageTag: imageTag || undefined })
      .then((preview) => {
        if (cancelled) return;
        setItems(preview.items);
        setMeta({ clusterName: preview.cluster_name, namespace: preview.namespace });
        setActiveKind(preview.items[0]?.kind ?? "Deployment");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setItems([]);
        setMeta(null);
        setError(err instanceof BffError || err instanceof Error ? err.message : "加载 YAML 预览失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId, projectId, environmentId, imageTag]);

  const activeItem = useMemo(
    () => items.find((item) => item.kind === activeKind) ?? items[0] ?? null,
    [items, activeKind],
  );

  const diffStats = useMemo(() => {
    if (!activeItem) return { additions: 0, deletions: 0 };
    return countYamlDiff(activeItem.current_yaml, activeItem.rendered_yaml);
  }, [activeItem]);

  const panes = useMemo(() => {
    if (!activeItem) return { rendered: "", current: "" };
    return filterDiffLines(activeItem.rendered_yaml, activeItem.current_yaml, onlyDiff);
  }, [activeItem, onlyDiff]);

  async function copyRendered() {
    if (!panes.rendered) return;
    try {
      await navigator.clipboard.writeText(panes.rendered);
    } catch {
      // ignore clipboard failures
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="full"
      priority
      title="服务渲染后的 YAML 结果和当前环境中的服务 YAML 比对"
      description={`${projectName} · ${imageRef || imageTag || "latest"}`}
      footer={
        <div className="wf-yaml-compare-footer">
          <div className="wf-yaml-compare-tools">
            <label className="wf-yaml-compare-toggle">
              <input type="checkbox" checked={singleColumn} onChange={(e) => setSingleColumn(e.target.checked)} />
              单栏对比
            </label>
            <label className="wf-yaml-compare-toggle">
              <input type="checkbox" checked={onlyDiff} onChange={(e) => setOnlyDiff(e.target.checked)} />
              只看差异
            </label>
            <button type="button" className="wf-yaml-compare-copy" onClick={() => void copyRendered()}>
              复制
            </button>
            <span className="wf-yaml-compare-stats">
              +{diffStats.additions} additions · -{diffStats.deletions} deletions
            </span>
          </div>
          <button type="button" className="btn-primary" onClick={onClose}>
            确定
          </button>
        </div>
      }
    >
      <div className="wf-yaml-compare-body">
        {meta ? (
          <div className="wf-yaml-compare-meta">
            <span>集群 {meta.clusterName || "—"}</span>
            <span>命名空间 {meta.namespace || "—"}</span>
          </div>
        ) : null}

        {loading ? <p className="wf-yaml-compare-hint">加载 YAML 预览…</p> : null}
        {error ? <p className="wf-yaml-compare-error">{error}</p> : null}

        {!loading && !error && items.length === 0 ? (
          <p className="wf-yaml-compare-hint">暂无可预览的 Deployment / Service 资源</p>
        ) : null}

        {!loading && !error && items.length > 0 ? (
          <>
            <div className="wf-yaml-compare-tabs">
              {items.map((item) => (
                <button
                  key={item.kind}
                  type="button"
                  className={`wf-yaml-compare-tab${activeKind === item.kind ? " is-active" : ""}`}
                  onClick={() => setActiveKind(item.kind)}
                >
                  {item.kind}
                </button>
              ))}
            </div>

            <div className={`wf-yaml-compare-grid${singleColumn ? " is-single" : ""}`}>
              <section className="wf-yaml-compare-pane">
                <header className="wf-yaml-compare-pane-head">渲染结果</header>
                <textarea
                  className="k8s-yaml-editor wf-yaml-compare-editor"
                  readOnly
                  value={panes.rendered || "（暂无渲染 YAML）"}
                  spellCheck={false}
                />
              </section>
              <section className="wf-yaml-compare-pane">
                <header className="wf-yaml-compare-pane-head">
                  当前环境
                  {activeItem && !activeItem.current_found ? (
                    <span className="wf-yaml-compare-badge">未部署</span>
                  ) : null}
                </header>
                <textarea
                  className="k8s-yaml-editor wf-yaml-compare-editor"
                  readOnly
                  value={panes.current || "（环境中暂无该资源）"}
                  spellCheck={false}
                />
              </section>
            </div>
          </>
        ) : null}
      </div>
    </Modal>
  );
}
