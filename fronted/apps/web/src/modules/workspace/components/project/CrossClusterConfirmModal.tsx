"use client";

import { useEffect, useMemo, useState } from "react";
import { parse as parseYaml, stringify as stringifyYaml } from "yaml";
import { BffError } from "@sparkops/contracts";
import type { SyncManifestInput, SyncPreviewItemVM } from "@sparkops/contracts/delivery";
import { delivery } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";

export type CrossClusterConfirmMode = "publish" | "sync";

interface CrossClusterConfirmModalProps {
  open: boolean;
  mode: CrossClusterConfirmMode;
  workspaceId: string;
  projectId: string;
  buildClusterName: string;
  syncClusterNames: string[];
  imageTag?: string;
  gitRefLabel?: string;
  submitting?: boolean;
  onClose: () => void;
  onConfirm: (manifests?: SyncManifestInput[]) => void;
}

type DraftItem = SyncPreviewItemVM & {
  selected: boolean;
  draftYaml: string;
  error?: string;
};

const KIND_ORDER = ["Deployment", "Service", "Secret", "ConfigMap", "Ingress"] as const;

const KIND_LABEL: Record<string, string> = {
  Deployment: "Deployment",
  Service: "Service",
  Secret: "Secret",
  ConfigMap: "ConfigMap",
  Ingress: "Ingress",
};

export function CrossClusterConfirmModal({
  open,
  mode,
  workspaceId,
  projectId,
  buildClusterName,
  syncClusterNames,
  imageTag,
  gitRefLabel,
  submitting = false,
  onClose,
  onConfirm,
}: CrossClusterConfirmModalProps) {
  const isSync = mode === "sync";
  const title = isSync ? "跨集群同步" : "跨集群发布";
  const confirmLabel = isSync ? "开始同步" : "继续发布";

  const [loadingPreview, setLoadingPreview] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [items, setItems] = useState<DraftItem[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);

  useEffect(() => {
    if (!open || !isSync) {
      setItems([]);
      setActiveId(null);
      setPreviewError(null);
      setLoadingPreview(false);
      return;
    }
    let cancelled = false;
    setLoadingPreview(true);
    setPreviewError(null);
    void delivery
      .getSyncPreview(projectId, workspaceId, imageTag)
      .then((preview) => {
        if (cancelled) return;
        const next = preview.items.map((item) => ({
          ...item,
          selected: item.default_selected,
          draftYaml: item.yaml ?? "",
          error: undefined,
        }));
        setItems(next);
        setActiveId(next.find((item) => item.selected)?.id ?? next[0]?.id ?? null);
      })
      .catch((err) => {
        if (cancelled) return;
        setItems([]);
        setActiveId(null);
        setPreviewError(err instanceof BffError ? err.message : "加载同步资源预览失败");
      })
      .finally(() => {
        if (!cancelled) setLoadingPreview(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, isSync, projectId, workspaceId, imageTag]);

  const activeItem = useMemo(
    () => items.find((item) => item.id === activeId) ?? null,
    [activeId, items],
  );

  const grouped = useMemo(() => {
    const map = new Map<string, DraftItem[]>();
    for (const kind of KIND_ORDER) map.set(kind, []);
    for (const item of items) {
      const list = map.get(item.kind) ?? [];
      list.push(item);
      map.set(item.kind, list);
    }
    return KIND_ORDER.map((kind) => ({ kind, items: map.get(kind) ?? [] })).filter(
      (group) => group.items.length > 0,
    );
  }, [items]);

  const selectedCount = items.filter((item) => item.selected).length;
  const hasYamlErrors = items.some((item) => item.selected && item.error);
  const canSubmit =
    !submitting &&
    syncClusterNames.length > 0 &&
    (!isSync || (!loadingPreview && !previewError && selectedCount > 0 && !hasYamlErrors));

  function updateItem(id: string, patch: Partial<DraftItem>) {
    setItems((prev) => prev.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  }

  function validateYaml(item: DraftItem, raw: string): string | undefined {
    if (item.managed) return undefined;
    try {
      const doc = parseYaml(raw) as Record<string, unknown>;
      if (!doc || typeof doc !== "object") return "YAML 无效";
      if (String(doc.kind ?? "") !== item.kind) return `kind 必须保持为 ${item.kind}`;
      const metadata = (doc.metadata ?? {}) as Record<string, unknown>;
      if (String(metadata.name ?? "") !== item.name) return `name 必须保持为 ${item.name}`;
      const ns = metadata.namespace;
      if (ns != null && String(ns).trim() === "") {
        return "namespace 不能为空字符串";
      }
      return undefined;
    } catch (err) {
      return err instanceof Error ? err.message : "YAML 解析失败";
    }
  }

  function handleYamlChange(value: string) {
    if (!activeItem || activeItem.managed) return;
    updateItem(activeItem.id, {
      draftYaml: value,
      error: validateYaml(activeItem, value),
    });
  }

  function toggleItemSelected(id: string, selected: boolean) {
    updateItem(id, { selected });
    setActiveId(id);
  }

  function handleConfirm() {
    if (!canSubmit) return;
    if (!isSync) {
      onConfirm();
      return;
    }
    const manifests: SyncManifestInput[] = [];
    for (const item of items) {
      if (!item.selected) continue;
      if (item.managed) {
        manifests.push({ kind: item.kind, name: item.name, managed_pull_secret: true });
        continue;
      }
      const error = validateYaml(item, item.draftYaml);
      if (error) {
        updateItem(item.id, { error });
        setActiveId(item.id);
        return;
      }
      manifests.push({
        kind: item.kind,
        name: item.name,
        yaml: normalizeYaml(item.draftYaml),
      });
    }
    if (manifests.length === 0) return;
    onConfirm(manifests);
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size={isSync ? "xl" : "md"}
      panelClassName={isSync ? "cross-cluster-sync-modal" : undefined}
      description={
        isSync
          ? "勾选要同步的资源并按需编辑 YAML；改动仅作用于本次同步到目标集群。"
          : "将在构建集群构建并部署，随后把同一镜像同步到其余授权集群。"
      }
      footer={
        <div className="cross-cluster-sync-footer">
          {isSync && !loadingPreview && !previewError ? (
            <span className="cross-cluster-sync-footer-meta">
              已选 <strong>{selectedCount}</strong> 项
              {hasYamlErrors ? (
                <span className="cross-cluster-sync-footer-warn"> · 存在 YAML 校验错误</span>
              ) : null}
            </span>
          ) : (
            <span />
          )}
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={onClose} disabled={submitting}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              onClick={handleConfirm}
              disabled={!canSubmit}
            >
              {submitting ? "处理中…" : confirmLabel}
            </button>
          </div>
        </div>
      }
    >
      <div className="cross-cluster-sync-body">
        <section className="cross-cluster-sync-summary" aria-label="同步概览">
          <div className="cross-cluster-sync-summary-card">
            <span className="cross-cluster-sync-summary-label">构建集群</span>
            <strong className="cross-cluster-sync-summary-value">{buildClusterName || "-"}</strong>
          </div>
          <div className="cross-cluster-sync-summary-card">
            <span className="cross-cluster-sync-summary-label">同步目标</span>
            <div className="cross-cluster-sync-targets">
              {syncClusterNames.length > 0 ? (
                syncClusterNames.map((name) => (
                  <span key={name} className="cross-cluster-sync-target-chip">
                    {name}
                  </span>
                ))
              ) : (
                <span className="cross-cluster-sync-summary-empty">暂无可用目标</span>
              )}
            </div>
          </div>
          {isSync ? (
            <>
              <div className="cross-cluster-sync-summary-card">
                <span className="cross-cluster-sync-summary-label">Git 引用</span>
                <strong className="cross-cluster-sync-summary-value">{gitRefLabel || "-"}</strong>
              </div>
              <div className="cross-cluster-sync-summary-card">
                <span className="cross-cluster-sync-summary-label">镜像标签</span>
                <strong className="cross-cluster-sync-summary-value mono">{imageTag || "-"}</strong>
              </div>
            </>
          ) : null}
        </section>

        {isSync ? (
          <section className="cross-cluster-sync-workbench" aria-label="资源清单">
            {loadingPreview ? <SyncPreviewSkeleton /> : null}
            {previewError ? (
              <div className="cross-cluster-sync-alert is-error" role="alert">
                <strong>无法加载预览</strong>
                <p>{previewError}</p>
              </div>
            ) : null}
            {!loadingPreview && !previewError ? (
              items.length === 0 ? (
                <div className="cross-cluster-sync-empty">
                  <p>未发现可同步资源</p>
                  <span>请确认构建集群中已有对应工作负载，或检查集群连接。</span>
                </div>
              ) : (
                <div className="cross-cluster-sync-grid">
                  <aside className="cross-cluster-sync-picker">
                    <header className="cross-cluster-sync-pane-head">
                      <h3>同步资源</h3>
                      <span>{items.length} 项</span>
                    </header>
                    <div className="cross-cluster-sync-picker-scroll">
                      {grouped.map((group) => (
                        <div key={group.kind} className="cross-cluster-sync-kind-group">
                          <div className="cross-cluster-sync-kind-head">
                            <span className={`cross-cluster-sync-kind-badge is-${group.kind.toLowerCase()}`}>
                              {KIND_LABEL[group.kind] ?? group.kind}
                            </span>
                            <span className="cross-cluster-sync-kind-count">{group.items.length}</span>
                          </div>
                          <ul className="cross-cluster-sync-resource-list">
                            {group.items.map((item) => (
                              <li key={item.id}>
                                <div
                                  className={[
                                    "cross-cluster-sync-resource",
                                    activeId === item.id ? "is-active" : "",
                                    item.selected ? "is-checked" : "",
                                    item.error ? "is-invalid" : "",
                                  ]
                                    .filter(Boolean)
                                    .join(" ")}
                                >
                                  <label className="cross-cluster-sync-checkbox" title="参与本次同步">
                                    <input
                                      type="checkbox"
                                      checked={item.selected}
                                      onChange={(event) => {
                                        event.stopPropagation();
                                        toggleItemSelected(item.id, event.target.checked);
                                      }}
                                    />
                                    <span className="cross-cluster-sync-checkbox-ui" aria-hidden />
                                  </label>
                                  <button
                                    type="button"
                                    className="cross-cluster-sync-resource-main"
                                    onClick={() => setActiveId(item.id)}
                                  >
                                    <span className="cross-cluster-sync-resource-name" title={item.name}>
                                      {item.name}
                                    </span>
                                    <span className="cross-cluster-sync-resource-desc">
                                      {item.managed
                                        ? "Harbor 托管拉取 Secret"
                                        : item.related_reason || item.kind}
                                    </span>
                                  </button>
                                  {item.managed ? (
                                    <span className="cross-cluster-sync-managed-tag">托管</span>
                                  ) : null}
                                  {item.error ? (
                                    <span className="cross-cluster-sync-error-dot" title={item.error} />
                                  ) : null}
                                </div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      ))}
                    </div>
                  </aside>

                  <div className="cross-cluster-sync-editor-pane">
                    <header className="cross-cluster-sync-pane-head">
                      <h3>YAML 预览</h3>
                      {activeItem ? (
                        <span className="cross-cluster-sync-editor-path">
                          {activeItem.kind}/{activeItem.name}
                        </span>
                      ) : null}
                    </header>
                    {activeItem ? (
                      <>
                        {activeItem.managed ? (
                          <div className="cross-cluster-sync-managed-callout">
                            <strong>镜像拉取 Secret（托管）</strong>
                            <p>
                              {activeItem.note ||
                                "目标集群将使用 Harbor 凭据自动创建，不会复制构建集群中的 dockerconfigjson 明文。"}
                            </p>
                          </div>
                        ) : (
                          <>
                            <div className="cross-cluster-sync-editor-toolbar">
                              <span className={`cross-cluster-sync-kind-badge is-${activeItem.kind.toLowerCase()}`}>
                                {activeItem.kind}
                              </span>
                              {!activeItem.selected ? (
                                <span className="cross-cluster-sync-editor-hint">未勾选，不会参与同步</span>
                              ) : (
                                <span className="cross-cluster-sync-editor-hint">
                                  仅可编辑 spec / data；不可修改 kind、name、namespace
                                </span>
                              )}
                            </div>
                            <div className="cross-cluster-sync-code-wrap">
                              <textarea
                                className="cross-cluster-sync-code"
                                value={activeItem.draftYaml}
                                onChange={(event) => handleYamlChange(event.target.value)}
                                spellCheck={false}
                                aria-label={`${activeItem.kind} ${activeItem.name} YAML`}
                              />
                            </div>
                            {activeItem.error ? (
                              <p className="cross-cluster-sync-inline-error" role="alert">
                                {activeItem.error}
                              </p>
                            ) : null}
                          </>
                        )}
                      </>
                    ) : (
                      <div className="cross-cluster-sync-editor-placeholder">
                        <p>从左侧选择资源</p>
                        <span>勾选后将同步到目标集群；点击名称可预览并编辑 YAML。</span>
                      </div>
                    )}
                  </div>
                </div>
              )
            ) : null}
          </section>
        ) : null}
      </div>
    </Modal>
  );
}

function SyncPreviewSkeleton() {
  return (
    <div className="cross-cluster-sync-skeleton" aria-hidden>
      <div className="cross-cluster-sync-grid">
        <div className="cross-cluster-sync-picker">
          <div className="template-skeleton-line short" />
          <div className="template-skeleton-line" />
          <div className="template-skeleton-line" />
          <div className="template-skeleton-line short" />
        </div>
        <div className="cross-cluster-sync-editor-pane">
          <div className="template-skeleton-line short" />
          <div className="template-skeleton-line" />
          <div className="template-skeleton-line" />
        </div>
      </div>
    </div>
  );
}

function normalizeYaml(raw: string): string {
  try {
    return stringifyYaml(parseYaml(raw));
  } catch {
    return raw;
  }
}
